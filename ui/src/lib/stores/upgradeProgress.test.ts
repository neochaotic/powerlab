import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';

(globalThis as unknown as { __APP_VERSION__: string }).__APP_VERSION__ = '0.6.6';

const { upgradeProgress, UPGRADE_POLL_INTERVAL_MS, UPGRADE_TIMEOUT_MS } = await import(
	'./upgradeProgress.svelte'
);
const { setAuthToken } = await import('../api/client');

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
		const text = r.body === undefined ? '' : JSON.stringify(r.body);
		return Promise.resolve({
			ok: r.ok,
			status: r.status,
			statusText: r.ok ? 'OK' : 'Error',
			headers: new Headers({ 'content-type': 'application/json' }),
			text: () => Promise.resolve(text),
			json: () => Promise.resolve(r.body ?? {})
		});
	}) as unknown as typeof fetch;
}

describe('upgradeProgress', () => {
	// Regression lock for the v0.6.9 "Upgrade refused (HTTP 401)" bug.
	// The store used to call `fetch('/v1/powerlab-update/install')` directly,
	// bypassing the API client and never attaching the Authorization header
	// that the gateway's auth middleware demands. Users couldn't upgrade
	// from the UI — every click 401'd. Locked here so any future raw-fetch
	// regression trips a red.
	it('start() sends Authorization header on the POST install call (#bug-401)', async () => {
		setAuthToken('test-jwt-token');
		const fetchSpy = vi.fn().mockImplementation(() =>
			Promise.resolve({
				ok: true,
				status: 202,
				text: () => Promise.resolve(''),
				json: () => Promise.resolve({}),
				headers: new Headers({ 'content-type': 'application/json' })
			})
		);
		global.fetch = fetchSpy as unknown as typeof fetch;

		await upgradeProgress.start('0.6.10');

		expect(fetchSpy).toHaveBeenCalled();
		const [, init] = fetchSpy.mock.calls[0];
		const headers = init?.headers as Record<string, string> | Headers | undefined;
		const auth =
			headers instanceof Headers
				? headers.get('Authorization')
				: (headers ?? {})['Authorization'];
		expect(auth, 'POST /v1/powerlab-update/install must carry the JWT or the gateway 401s').toBe(
			'test-jwt-token'
		);
		setAuthToken(null);
	});

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
