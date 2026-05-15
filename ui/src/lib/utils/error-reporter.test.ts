import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
	captureFrontendError,
	createErrorReporter,
	isNoiseError,
	resetReporterStateForTest
} from './error-reporter';

// The reporter is the SvelteKit-side half of the UI JS error capture
// pipeline (backend half is /v1/audit/frontend-error). Three concerns:
//
//   - Noise filter: ignore the well-known browser noise (ResizeObserver
//     loop, chrome-extension:// stacks, opaque "Script error.") so the
//     audit log doesn't drown in irrelevant events.
//   - Dedupe: identical errors fired in rapid succession (a render
//     loop emitting the same line) collapse to ONE POST per window
//     so a runaway component doesn't DoS the gateway.
//   - Transport: use the api client (per the raw-fetch bug class
//     memory), never raw fetch().

describe('isNoiseError', () => {
	it('filters ResizeObserver loop messages', () => {
		expect(isNoiseError({ message: 'ResizeObserver loop limit exceeded' })).toBe(true);
		expect(
			isNoiseError({
				message: 'ResizeObserver loop completed with undelivered notifications.'
			})
		).toBe(true);
	});

	it('filters extension-origin stacks', () => {
		expect(
			isNoiseError({
				message: 'something',
				stack: 'at chrome-extension://abc/content.js:1:1'
			})
		).toBe(true);
		expect(
			isNoiseError({
				message: 'something',
				stack: 'at moz-extension://xyz/main.js:1:1'
			})
		).toBe(true);
	});

	it('filters opaque cross-origin "Script error."', () => {
		expect(isNoiseError({ message: 'Script error.' })).toBe(true);
	});

	it('accepts real errors', () => {
		expect(
			isNoiseError({
				message: "TypeError: Cannot read properties of undefined (reading 'foo')",
				stack: 'at /apps/+page.svelte:42:7'
			})
		).toBe(false);
	});
});

describe('createErrorReporter', () => {
	let posted: Array<Record<string, unknown>>;
	let postFn: (body: unknown) => Promise<unknown>;

	beforeEach(() => {
		resetReporterStateForTest();
		posted = [];
		postFn = vi.fn(async (body: unknown) => {
			posted.push(body as Record<string, unknown>);
			return { ok: true };
		});
		vi.useFakeTimers();
	});

	afterEach(() => {
		vi.useRealTimers();
	});

	it('posts a real error with message + stack + url', async () => {
		const reporter = createErrorReporter(postFn);
		await reporter.report({
			message: "TypeError: x is undefined",
			stack: 'at /apps/+page.svelte:42',
			url: '/apps'
		});
		expect(posted).toHaveLength(1);
		expect(posted[0]).toMatchObject({
			message: "TypeError: x is undefined",
			stack: 'at /apps/+page.svelte:42',
			url: '/apps'
		});
	});

	it('skips noise (does not post)', async () => {
		const reporter = createErrorReporter(postFn);
		await reporter.report({ message: 'ResizeObserver loop limit exceeded' });
		await reporter.report({ message: 'Script error.' });
		expect(posted).toHaveLength(0);
	});

	it('dedupes identical errors fired in rapid succession', async () => {
		const reporter = createErrorReporter(postFn);
		const err = {
			message: 'TypeError: same',
			stack: 'at /apps/+page.svelte:42'
		};
		await reporter.report(err);
		await reporter.report(err);
		await reporter.report(err);
		expect(posted).toHaveLength(1);
	});

	it('re-allows the same error after the dedupe window expires', async () => {
		const reporter = createErrorReporter(postFn);
		const err = { message: 'TypeError: same' };
		await reporter.report(err);
		expect(posted).toHaveLength(1);

		// Advance past the 5-minute dedupe window.
		vi.advanceTimersByTime(5 * 60 * 1000 + 1);

		await reporter.report(err);
		expect(posted).toHaveLength(2);
	});

	it('caps total posts within a window to prevent DoS', async () => {
		const reporter = createErrorReporter(postFn);
		// 12 distinct errors — cap is 10 per window.
		for (let i = 0; i < 12; i++) {
			await reporter.report({ message: `unique error ${i}` });
		}
		expect(posted.length).toBeLessThanOrEqual(10);
	});

	it('does not throw if the post fails (audit must not break UI)', async () => {
		const failing: (body: unknown) => Promise<unknown> = vi.fn(async () => {
			throw new Error('network down');
		});
		const reporter = createErrorReporter(failing);
		await expect(reporter.report({ message: 'boom' })).resolves.not.toThrow();
	});
});

describe('captureFrontendError integration with window', () => {
	beforeEach(() => {
		resetReporterStateForTest();
	});

	it('handles ErrorEvent shape (window.onerror)', async () => {
		const posted: unknown[] = [];
		const reporter = createErrorReporter(async (body) => {
			posted.push(body);
			return { ok: true };
		});
		await captureFrontendError(
			{
				message: "TypeError: bla",
				error: new Error("TypeError: bla"),
				filename: '/apps/+page.svelte',
				lineno: 42,
				colno: 7
			} as ErrorEvent,
			reporter
		);
		expect(posted).toHaveLength(1);
		expect((posted[0] as { message: string }).message).toContain('TypeError');
	});

	it('handles PromiseRejectionEvent shape (unhandledrejection)', async () => {
		const posted: unknown[] = [];
		const reporter = createErrorReporter(async (body) => {
			posted.push(body);
			return { ok: true };
		});
		await captureFrontendError(
			{
				type: 'unhandledrejection',
				reason: new Error("rejected")
			} as PromiseRejectionEvent,
			reporter
		);
		expect(posted).toHaveLength(1);
		expect((posted[0] as { message: string }).message).toContain('rejected');
	});
});
