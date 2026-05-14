import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { api, onAuthError, setAuthToken } from './client';

// Regression lock for the v0.6.11 UX bug a user hit live: a stale
// JWT in localStorage produced an opaque "Could not reach the
// release manifest: Unauthorized" toast for the updater check, with
// no hint that the user just needed to re-login. The 401 was real
// (token expired) but the UI hid the cause behind the calling
// store's catch-all error path.
//
// Fix contract: the api client emits a single centralized signal
// on 401 via onAuthError. The auth store's handler logs the user
// out + surfaces a clear toast. Every store that catches an api
// error now benefits without per-call special-casing.

describe('api client 401 handling', () => {
	let originalFetch: typeof global.fetch;
	beforeEach(() => {
		originalFetch = global.fetch;
		setAuthToken('expired-jwt');
	});
	afterEach(() => {
		global.fetch = originalFetch;
		setAuthToken(null);
	});

	function mock401() {
		global.fetch = vi.fn().mockResolvedValue({
			ok: false,
			status: 401,
			statusText: 'Unauthorized',
			headers: new Headers({ 'content-type': 'application/json' }),
			text: () => Promise.resolve(JSON.stringify({ message: 'invalid or expired jwt' }))
		}) as unknown as typeof fetch;
	}

	it('fires onAuthError handler when response is 401', async () => {
		mock401();
		const handler = vi.fn();
		const unsubscribe = onAuthError(handler);

		try {
			await api.get('/v1/protected');
		} catch {
			// expected: the request still throws ApiError; the handler
			// fires in parallel with the throw, not in place of it.
		}

		expect(handler).toHaveBeenCalledTimes(1);
		const [info] = handler.mock.calls[0];
		expect(info.status).toBe(401);
		expect(info.url).toContain('/v1/protected');
		unsubscribe();
	});

	it('does NOT fire onAuthError handler on non-401 errors', async () => {
		global.fetch = vi.fn().mockResolvedValue({
			ok: false,
			status: 500,
			statusText: 'Internal',
			headers: new Headers({ 'content-type': 'application/json' }),
			text: () => Promise.resolve(JSON.stringify({ message: 'boom' }))
		}) as unknown as typeof fetch;

		const handler = vi.fn();
		const unsubscribe = onAuthError(handler);
		try {
			await api.get('/v1/protected');
		} catch {
			// expected
		}
		expect(handler).not.toHaveBeenCalled();
		unsubscribe();
	});

	it('does NOT fire onAuthError on successful 200 response', async () => {
		global.fetch = vi.fn().mockResolvedValue({
			ok: true,
			status: 200,
			statusText: 'OK',
			headers: new Headers({ 'content-type': 'application/json' }),
			text: () => Promise.resolve('{}')
		}) as unknown as typeof fetch;

		const handler = vi.fn();
		const unsubscribe = onAuthError(handler);
		await api.get('/v1/protected');
		expect(handler).not.toHaveBeenCalled();
		unsubscribe();
	});

	it('unsubscribe() prevents further callbacks', async () => {
		mock401();
		const handler = vi.fn();
		const unsubscribe = onAuthError(handler);
		unsubscribe();

		try {
			await api.get('/v1/protected');
		} catch {
			// expected
		}
		expect(handler).not.toHaveBeenCalled();
	});

	it('multiple subscribers are all invoked on 401', async () => {
		mock401();
		const h1 = vi.fn();
		const h2 = vi.fn();
		const u1 = onAuthError(h1);
		const u2 = onAuthError(h2);

		try {
			await api.get('/v1/protected');
		} catch {
			// expected
		}
		expect(h1).toHaveBeenCalledTimes(1);
		expect(h2).toHaveBeenCalledTimes(1);
		u1();
		u2();
	});
});
