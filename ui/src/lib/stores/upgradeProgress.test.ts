import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';

(globalThis as unknown as { __APP_VERSION__: string }).__APP_VERSION__ = '0.6.6';

const { upgradeProgress, UPGRADE_POLL_INTERVAL_MS, UPGRADE_TIMEOUT_MS } = await import(
	'./upgradeProgress.svelte'
);

const realFetch = global.fetch;

beforeEach(() => {
	vi.useFakeTimers();
	upgradeProgress.reset();
});

afterEach(() => {
	vi.useRealTimers();
	global.fetch = realFetch;
});

function mockFetchSequence(responses: Array<{ ok: boolean; status: number; body?: unknown; throws?: Error }>) {
	let i = 0;
	global.fetch = vi.fn().mockImplementation(() => {
		const r = responses[Math.min(i, responses.length - 1)];
		i++;
		if (r.throws) return Promise.reject(r.throws);
		return Promise.resolve({
			ok: r.ok,
			status: r.status,
			json: () => Promise.resolve(r.body ?? {})
		});
	}) as unknown as typeof fetch;
}

describe('upgradeProgress', () => {
	it('starts in idle state', () => {
		expect(upgradeProgress.state).toBe('idle');
		expect(upgradeProgress.targetVersion).toBe(null);
	});

	it('start() transitions idle → starting and stores target version', async () => {
		mockFetchSequence([{ ok: true, status: 202, body: {} }]);
		const p = upgradeProgress.start('0.6.7');
		expect(upgradeProgress.state).toBe('starting');
		expect(upgradeProgress.targetVersion).toBe('0.6.7');
		await p;
	});

	it('on POST install 202 → transitions starting → restarting and begins polling', async () => {
		mockFetchSequence([
			{ ok: true, status: 202, body: {} }, // install accepted
			{ ok: false, status: 502 } // first poll, gateway restarting
		]);
		await upgradeProgress.start('0.6.7');
		expect(upgradeProgress.state).toBe('restarting');

		await vi.advanceTimersByTimeAsync(UPGRADE_POLL_INTERVAL_MS + 10);
		// Still restarting — 502 is expected during the window.
		expect(upgradeProgress.state).toBe('restarting');
	});

	it('on POST install non-202 → transitions to error with message', async () => {
		mockFetchSequence([{ ok: false, status: 400, body: { message: 'host is not eligible' } }]);
		await upgradeProgress.start('0.6.7');
		expect(upgradeProgress.state).toBe('error');
		expect(upgradeProgress.error).toContain('400');
	});

	it('when version poll returns target version → transitions restarting → success', async () => {
		mockFetchSequence([
			{ ok: true, status: 202, body: {} },
			{ ok: false, status: 502 }, // first poll - down
			{ ok: true, status: 200, body: { version: '0.6.7' } } // second poll - up with new version
		]);
		await upgradeProgress.start('0.6.7');
		expect(upgradeProgress.state).toBe('restarting');
		await vi.advanceTimersByTimeAsync(UPGRADE_POLL_INTERVAL_MS + 10);
		await vi.advanceTimersByTimeAsync(UPGRADE_POLL_INTERVAL_MS + 10);
		expect(upgradeProgress.state).toBe('success');
	});

	it('poll returning old version stays in restarting (services not yet swapped)', async () => {
		mockFetchSequence([
			{ ok: true, status: 202, body: {} },
			{ ok: true, status: 200, body: { version: '0.6.6' } } // still old
		]);
		await upgradeProgress.start('0.6.7');
		await vi.advanceTimersByTimeAsync(UPGRADE_POLL_INTERVAL_MS + 10);
		expect(upgradeProgress.state).toBe('restarting');
	});

	it('poll network error is suppressed (treated like 502)', async () => {
		mockFetchSequence([
			{ ok: true, status: 202, body: {} },
			{ ok: false, status: 0, throws: new Error('Network refused') }
		]);
		await upgradeProgress.start('0.6.7');
		await vi.advanceTimersByTimeAsync(UPGRADE_POLL_INTERVAL_MS + 10);
		// Still restarting — net errors are expected during the window.
		expect(upgradeProgress.state).toBe('restarting');
	});

	it('after UPGRADE_TIMEOUT_MS in restarting with no success → state = error', async () => {
		mockFetchSequence([
			{ ok: true, status: 202, body: {} },
			{ ok: false, status: 502 }
		]);
		await upgradeProgress.start('0.6.7');
		await vi.advanceTimersByTimeAsync(UPGRADE_TIMEOUT_MS + 100);
		expect(upgradeProgress.state).toBe('error');
		expect(upgradeProgress.error).toMatch(/timeout/i);
	});

	it('reset() returns state to idle and clears target/error', () => {
		upgradeProgress.state = 'error';
		upgradeProgress.targetVersion = '0.6.7';
		upgradeProgress.error = 'something';
		upgradeProgress.reset();
		expect(upgradeProgress.state).toBe('idle');
		expect(upgradeProgress.targetVersion).toBe(null);
		expect(upgradeProgress.error).toBe(null);
	});

	it('isOverlayActive is true for starting/restarting/success/error, false for idle', () => {
		upgradeProgress.state = 'idle';
		expect(upgradeProgress.isOverlayActive).toBe(false);

		upgradeProgress.state = 'starting';
		expect(upgradeProgress.isOverlayActive).toBe(true);

		upgradeProgress.state = 'restarting';
		expect(upgradeProgress.isOverlayActive).toBe(true);

		upgradeProgress.state = 'success';
		expect(upgradeProgress.isOverlayActive).toBe(true);

		upgradeProgress.state = 'error';
		expect(upgradeProgress.isOverlayActive).toBe(true);
	});
});
