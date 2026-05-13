import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';

(globalThis as unknown as { __APP_VERSION__: string }).__APP_VERSION__ = '0.5.13-test';

const localStorageStore = new Map<string, string>();
(globalThis as unknown as { localStorage: Storage }).localStorage = {
	getItem: (k: string) => localStorageStore.get(k) ?? null,
	setItem: (k: string, v: string) => {
		localStorageStore.set(k, v);
	},
	removeItem: (k: string) => {
		localStorageStore.delete(k);
	},
	clear: () => localStorageStore.clear(),
	key: (i: number) => Array.from(localStorageStore.keys())[i] ?? null,
	get length() {
		return localStorageStore.size;
	}
} satisfies Storage;

const { versionHandshake, RELOAD_ATTEMPTS_KEY } = await import('./versionHandshake.svelte');

const realFetch = global.fetch;

beforeEach(() => {
	versionHandshake.uiVersion = '0.5.13-test';
	versionHandshake.backendVersion = null;
	versionHandshake.mismatch = false;
	versionHandshake.dismissed = false;
	versionHandshake.checking = false;
	versionHandshake.error = null;
	localStorage.removeItem(RELOAD_ATTEMPTS_KEY);
});

afterEach(() => {
	global.fetch = realFetch;
});

function mockFetch(body: unknown, ok = true, status = 200) {
	global.fetch = vi.fn().mockResolvedValue({
		ok,
		status,
		json: () => Promise.resolve(body)
	}) as unknown as typeof fetch;
}

describe('Version handshake', () => {
	it('flips mismatch=true when backend reports a different version', async () => {
		versionHandshake.uiVersion = '0.5.13';
		mockFetch({ version: '0.6.0' });
		await versionHandshake.check();
		expect(versionHandshake.backendVersion).toBe('0.6.0');
		expect(versionHandshake.mismatch).toBe(true);
		expect(versionHandshake.checking).toBe(false);
	});

	it('keeps mismatch=false when versions match', async () => {
		versionHandshake.uiVersion = '0.5.13';
		mockFetch({ version: '0.5.13' });
		await versionHandshake.check();
		expect(versionHandshake.mismatch).toBe(false);
	});

	it('treats backend "dev" as never-mismatch', async () => {
		versionHandshake.uiVersion = '0.5.13';
		mockFetch({ version: 'dev' });
		await versionHandshake.check();
		expect(versionHandshake.mismatch).toBe(false);
		expect(versionHandshake.backendVersion).toBe('dev');
	});

	it('treats empty backend version as never-mismatch', async () => {
		versionHandshake.uiVersion = '0.5.13';
		mockFetch({ version: '' });
		await versionHandshake.check();
		expect(versionHandshake.mismatch).toBe(false);
	});

	it('captures HTTP error message and does NOT claim mismatch', async () => {
		mockFetch({}, false, 500);
		await versionHandshake.check();
		expect(versionHandshake.mismatch).toBe(false);
		expect(versionHandshake.error).toContain('HTTP 500');
	});

	it('captures network error message and does NOT claim mismatch', async () => {
		global.fetch = vi.fn().mockRejectedValue(new Error('offline')) as unknown as typeof fetch;
		await versionHandshake.check();
		expect(versionHandshake.error).toBe('offline');
		expect(versionHandshake.mismatch).toBe(false);
	});

	it('checking flag flips true→false across the call', async () => {
		mockFetch({ version: 'dev' });
		const p = versionHandshake.check();
		expect(versionHandshake.checking).toBe(true);
		await p;
		expect(versionHandshake.checking).toBe(false);
	});

	it('dismiss() flips dismissed=true so the banner can hide', () => {
		versionHandshake.mismatch = true;
		versionHandshake.dismissed = false;
		versionHandshake.dismiss();
		expect(versionHandshake.dismissed).toBe(true);
	});

	it('reloadAttempts reads zero before any forceReload call', () => {
		expect(versionHandshake.reloadAttempts).toBe(0);
	});

	it('forceReload() increments localStorage counter and triggers cache-busted nav', () => {
		const replaceMock = vi.fn();
		Object.defineProperty(window, 'location', {
			value: { pathname: '/apps', search: '', replace: replaceMock },
			writable: true
		});

		versionHandshake.forceReload();

		expect(localStorage.getItem(RELOAD_ATTEMPTS_KEY)).toBe('1');
		expect(replaceMock).toHaveBeenCalledTimes(1);
		const navUrl = replaceMock.mock.calls[0][0] as string;
		expect(navUrl).toMatch(/^\/apps\?powerlab_v=\d+$/);
	});

	it('forceReload() preserves existing query params alongside cache-buster', () => {
		const replaceMock = vi.fn();
		Object.defineProperty(window, 'location', {
			value: { pathname: '/apps', search: '?install=blinko', replace: replaceMock },
			writable: true
		});

		versionHandshake.forceReload();

		const navUrl = replaceMock.mock.calls[0][0] as string;
		expect(navUrl).toContain('install=blinko');
		expect(navUrl).toMatch(/powerlab_v=\d+/);
	});

	it('after 2 reload attempts, persistentFailure is true', () => {
		const replaceMock = vi.fn();
		Object.defineProperty(window, 'location', {
			value: { pathname: '/', search: '', replace: replaceMock },
			writable: true
		});

		versionHandshake.forceReload();
		expect(versionHandshake.persistentFailure).toBe(false);
		versionHandshake.forceReload();
		expect(versionHandshake.persistentFailure).toBe(true);
	});

	it('successful check (versions match) resets reloadAttempts counter', async () => {
		localStorage.setItem(RELOAD_ATTEMPTS_KEY, '3');
		versionHandshake.uiVersion = '0.5.13';
		mockFetch({ version: '0.5.13' });
		await versionHandshake.check();
		expect(localStorage.getItem(RELOAD_ATTEMPTS_KEY)).toBeNull();
		expect(versionHandshake.reloadAttempts).toBe(0);
	});
});
