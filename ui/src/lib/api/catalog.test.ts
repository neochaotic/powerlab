import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
	listCatalogSources,
	addCatalogSource,
	removeCatalogSource,
	type AppStoreSource
} from './catalog';

// Contract tests for the catalog sources API client (ADR-0039).
// Endpoint shapes match backend/app-management/route/v2/appstore.go.

describe('catalog API client', () => {
	let originalFetch: typeof global.fetch;

	beforeEach(() => {
		originalFetch = global.fetch;
	});
	afterEach(() => {
		global.fetch = originalFetch;
	});

	function mockResponse(data: unknown, status = 200) {
		global.fetch = vi.fn().mockResolvedValue({
			ok: status >= 200 && status < 300,
			status,
			statusText: status === 200 ? 'OK' : String(status),
			headers: new Headers({ 'content-type': 'application/json' }),
			text: () => Promise.resolve(JSON.stringify({ data }))
		}) as unknown as typeof fetch;
	}

	it('listCatalogSources GETs /v2/app_management/appstore', async () => {
		mockResponse([]);
		await listCatalogSources();
		const url = (global.fetch as ReturnType<typeof vi.fn>).mock.calls[0][0] as string;
		expect(url).toBe('/v2/app_management/appstore');
	});

	it('listCatalogSources unwraps data envelope into AppStoreSource[]', async () => {
		const sources: AppStoreSource[] = [
			{ id: 0, url: '/var/lib/powerlab/community-catalog', store_root: '/var/lib/powerlab/community-catalog' },
			{ id: 1, url: 'https://github.com/operator/custom-catalog', store_root: '/var/lib/powerlab/appstore/github.com/<hash>' }
		];
		mockResponse(sources);
		const out = await listCatalogSources();
		expect(out).toEqual(sources);
	});

	it('listCatalogSources returns [] when backend sends data:null', async () => {
		mockResponse(null);
		const out = await listCatalogSources();
		expect(out).toEqual([]);
	});

	it('addCatalogSource POSTs /v2/app_management/appstore?url=…', async () => {
		mockResponse({ id: 1 });
		await addCatalogSource('https://github.com/operator/custom-catalog');
		const [url, opts] = (global.fetch as ReturnType<typeof vi.fn>).mock.calls[0];
		expect(url).toBe('/v2/app_management/appstore?url=https%3A%2F%2Fgithub.com%2Foperator%2Fcustom-catalog');
		expect((opts as RequestInit).method).toBe('POST');
	});

	it('addCatalogSource URL-encodes special characters in URL param', async () => {
		// Defensive: a URL with `?` `&` `=` in it must not collapse the
		// outer ?url= query encoding.
		mockResponse({ id: 1 });
		await addCatalogSource('https://example.com/path?branch=main&token=abc');
		const url = (global.fetch as ReturnType<typeof vi.fn>).mock.calls[0][0] as string;
		expect(url).toContain('%3F'); // ? encoded
		expect(url).toContain('%26'); // & encoded
		expect(url).toContain('%3D'); // = encoded
	});

	it('removeCatalogSource DELETEs /v2/app_management/appstore/{id}', async () => {
		mockResponse({ removed: true });
		await removeCatalogSource(3);
		const [url, opts] = (global.fetch as ReturnType<typeof vi.fn>).mock.calls[0];
		expect(url).toBe('/v2/app_management/appstore/3');
		expect((opts as RequestInit).method).toBe('DELETE');
	});

	it('removeCatalogSource accepts numeric id without coercion issues', async () => {
		// id is `number`, but URL building uses string interpolation —
		// confirm "5" goes through as expected, not "5.0" or something.
		mockResponse({ removed: true });
		await removeCatalogSource(5);
		const url = (global.fetch as ReturnType<typeof vi.fn>).mock.calls[0][0] as string;
		expect(url).toBe('/v2/app_management/appstore/5');
	});
});
