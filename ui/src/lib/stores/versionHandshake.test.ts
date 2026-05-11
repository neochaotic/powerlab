import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';

(globalThis as unknown as { __APP_VERSION__: string }).__APP_VERSION__ = '0.5.13-test';

const { versionHandshake } = await import('./versionHandshake.svelte');

const realFetch = global.fetch;

beforeEach(() => {
	versionHandshake.uiVersion = '0.5.13-test';
	versionHandshake.backendVersion = null;
	versionHandshake.mismatch = false;
	versionHandshake.checking = false;
	versionHandshake.error = null;
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
});
