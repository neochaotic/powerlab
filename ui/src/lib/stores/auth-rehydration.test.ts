/**
 * Regression: JWT must be rehydrated SYNCHRONOUSLY at module init.
 *
 * Bug shipped on 2026-05-07: after a page refresh, the launchpad showed
 * "No apps installed yet" even when the user had apps. Root cause was
 * a race between +layout.svelte's async auth.checkSession() (which
 * called setAuthToken) and +page.svelte's fetchInstalledApps() — the
 * apps fetch fired without an Authorization header, the gateway
 * responded 401, and the store's `installedApps` stayed `{}`.
 *
 * This test asserts that simply *importing* auth.svelte (which is
 * what every component does) populates the http client's authToken
 * from localStorage, without waiting for any async hydration.
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';

describe('auth.svelte module init — synchronous JWT rehydration', () => {
	beforeEach(() => {
		vi.resetModules();
		const mockStorage: Record<string, string> = {};
		global.localStorage = {
			getItem: vi.fn((key: string) => mockStorage[key] ?? null),
			setItem: vi.fn((key: string, value: string) => { mockStorage[key] = value; }),
			removeItem: vi.fn((key: string) => { delete mockStorage[key]; }),
			clear: vi.fn(() => { for (const k in mockStorage) delete mockStorage[k]; }),
			length: 0,
			key: () => null
		} as unknown as Storage;
	});

	it('seeds client.authToken from localStorage the moment the store is imported', async () => {
		localStorage.setItem('powerlab_token', 'rehydrated_jwt');

		// Import in order: client first (clean state), then auth (which
		// runs its module-init side effect on import). If the side
		// effect is missing, getAuthToken() returns null and the test
		// fails — exactly what would have caught the original bug.
		const { getAuthToken } = await import('$lib/api/client');
		await import('./auth.svelte');

		expect(getAuthToken()).toBe('rehydrated_jwt');
	});

	it('leaves authToken null when localStorage has no token', async () => {
		const { getAuthToken } = await import('$lib/api/client');
		await import('./auth.svelte');

		expect(getAuthToken()).toBeNull();
	});
});
