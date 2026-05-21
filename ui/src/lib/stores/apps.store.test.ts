/**
 * Tests for apps.svelte.ts store behaviour.
 *
 * Critical regression: the backend returns store_app_id only as the dict KEY,
 * never in the value body. If fetchAppStore() doesn't inject it, every catalog
 * item has store_app_id=undefined → the {#each} key collapses 350 items into
 * one → the store tab renders nothing.
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { useAppStore } from './apps.svelte';

// Build a fake fetch that returns a realistic /v2/app_management/apps payload.
// The backend puts store_app_id as the dict KEY only — not in the value body.
function mockStoreResponse(apps: Record<string, object>) {
	return vi.fn().mockResolvedValue({
		ok: true,
		status: 200,
		statusText: 'OK',
		headers: new Headers({ 'content-type': 'application/json' }),
		text: () =>
			Promise.resolve(
				JSON.stringify({
					message: '',
					data: {
						installed: [],
						list: apps
					}
				})
			)
	});
}

describe('useAppStore — catalog loading', () => {
	beforeEach(() => {
		vi.restoreAllMocks();
	});

	it('injects store_app_id from dict key so catalog items are uniquely identifiable', async () => {
		vi.stubGlobal(
			'fetch',
			mockStoreResponse({
				nginx: {
					title: { en_us: 'Nginx' },
					author: 'PowerLab Team',
					icon: 'https://example.com/nginx.png',
					tagline: { en_us: 'Web server' }
				},
				pihole: {
					title: { en_us: 'Pi-hole' },
					author: 'PowerLab Team',
					icon: 'https://example.com/pihole.png',
					tagline: { en_us: 'Ad blocker' }
				}
			})
		);

		const store = useAppStore();
		await store.fetchAppStore();

		const catalog = store.appStoreCatalog;

		// Must have both apps
		expect(catalog).toHaveLength(2);

		// store_app_id must be injected from the dict key
		const ids = catalog.map((a) => a.store_app_id).sort();
		expect(ids).toEqual(['nginx', 'pihole']);
	});

	it('all catalog items have distinct store_app_id (no undefined keys)', async () => {
		const appEntries: Record<string, object> = {};
		// Simulate a realistic 5-app response without store_app_id in body
		['alpha', 'beta', 'gamma', 'delta', 'epsilon'].forEach((name) => {
			appEntries[name] = {
				title: { en_us: name },
				author: 'CasaOS Team',
				icon: '',
				tagline: { en_us: name }
			};
		});

		vi.stubGlobal('fetch', mockStoreResponse(appEntries));

		const store = useAppStore();
		await store.fetchAppStore();

		const catalog = store.appStoreCatalog;
		expect(catalog).toHaveLength(5);

		// No undefined keys — every item must have store_app_id set
		const undefinedCount = catalog.filter((a) => a.store_app_id === undefined).length;
		expect(undefinedCount).toBe(0);

		// All ids must be unique
		const ids = catalog.map((a) => a.store_app_id);
		const unique = new Set(ids);
		expect(unique.size).toBe(5);
	});

	it('empty list response results in empty catalog, not an error', async () => {
		vi.stubGlobal('fetch', mockStoreResponse({}));

		const store = useAppStore();
		await store.fetchAppStore();

		expect(store.appStoreCatalog).toHaveLength(0);
		expect(store.error).toBeNull();
	});

	it('isPowerLabApp returns true when store_app_id is in the catalog', async () => {
		vi.stubGlobal(
			'fetch',
			mockStoreResponse({
				nginx: { title: { en_us: 'Nginx' }, author: 'PowerLab Team', icon: '', tagline: {} }
			})
		);

		const store = useAppStore();
		await store.fetchAppStore();

		const installedApp = {
			store_info: {
				store_app_id: 'nginx',
				title: { en_us: 'Nginx' },
				icon: '',
				tagline: {},
				description: {},
				image: {},
				author: 'PowerLab Team',
				developer: '',
				category: '',
				thumbnail: ''
			},
			compose: null,
			status: 'running'
		};

		expect(store.isPowerLabApp(installedApp)).toBe(true);
	});

	it('isPowerLabApp returns false for apps not in the catalog (Custom Apps)', async () => {
		vi.stubGlobal('fetch', mockStoreResponse({}));

		const store = useAppStore();
		await store.fetchAppStore();

		const customApp = {
			store_info: {
				store_app_id: 'my-custom-app',
				title: { en_us: 'My Custom App' },
				icon: '',
				tagline: {},
				description: {},
				image: {},
				author: '',
				developer: '',
				category: '',
				thumbnail: ''
			},
			compose: null,
			status: 'running'
		};

		expect(store.isPowerLabApp(customApp)).toBe(false);
	});

	it('isPowerLabApp uses baked provenance, NOT catalog membership (catalog disabled → still a store app)', async () => {
		// Opt-in / catalog disabled: the browse list comes back empty.
		vi.stubGlobal('fetch', mockStoreResponse({}));
		const store = useAppStore();
		await store.fetchAppStore();
		expect(store.appStoreCatalog).toHaveLength(0);

		// An installed store app carries provenance in its own compose
		// (source.catalog + author). It must stay a PowerLab app even with
		// nothing in the live catalog — this is the regression where
		// disabling the catalog flipped every installed app to Custom.
		const installedStoreApp = {
			store_info: {
				store_app_id: 'enclosed',
				title: { en_us: 'Enclosed' },
				icon: '',
				tagline: {},
				description: {},
				image: {},
				author: 'Corentin Thomasset',
				developer: 'Corentin Thomasset',
				category: '',
				thumbnail: '',
				source: { catalog: 'umbrel-apps' }
			},
			compose: null,
			status: 'running'
		};

		expect(store.isPowerLabApp(installedStoreApp)).toBe(true);
	});
});
