/**
 * Docker & App Orchestration store using Svelte 5 runes.
 *
 * Manages: installed apps, app store catalog, loading states.
 * All data comes from the API — zero local business logic.
 */

import {
	getInstalledApps,
	getAppStoreList,
	setComposeAppStatus,
	uninstallComposeApp,
	updateComposeApp,
	getComposeAppDiskUsage,
	type ComposeAppWithStoreInfo,
	type ComposeAppStoreInfo
} from '$lib/api/apps';

let installedApps = $state<Record<string, ComposeAppWithStoreInfo>>({});
let appStoreCatalog = $state<Record<string, ComposeAppStoreInfo>>({});
// Start true so the first render shows a skeleton instead of "No apps installed"
// while the initial fetch is in flight. Switches to false after first response.
let loading = $state(true);
// True once the FIRST successful catalog fetch has populated appStoreCatalog.
// Consumers (e.g. Launchpad) gate the "Custom" tag on this so PowerLab apps
// don't briefly flash "Custom" while the catalog is still loading.
let catalogLoaded = $state(false);
// True once the first installed-apps fetch has completed (regardless of result).
let installedLoaded = $state(false);
let diskUsages = $state<Record<string, number>>({});
let error = $state<string | null>(null);

// Derived state
const installedList = $derived(Object.entries(installedApps).map(([id, app]) => ({ id, ...app })));
const catalogList = $derived(Object.values(appStoreCatalog));

async function fetchInstalledApps() {
	loading = true;
	error = null;
	try {
		const res = await getInstalledApps();
		if (res.data) {
			installedApps = res.data;
		}
	} catch (e) {
		error = (e as Error).message ?? 'Failed to load installed apps';
	} finally {
		loading = false;
		installedLoaded = true;
		fetchDiskUsages(); // Fire and forget disk usage fetch
	}
}

async function fetchDiskUsages() {
	for (const app of installedList) {
		try {
			const res = await getComposeAppDiskUsage(app.id);
			if (res.data) {
				diskUsages[app.id] = res.data.bytes;
			}
		} catch (e) {
			console.error(`Failed to fetch disk usage for ${app.id}:`, e);
		}
	}
}

async function fetchAppStore() {
	loading = true;
	error = null;
	try {
		const res = await getAppStoreList();
		if (res.data?.list) {
			// Inject store_app_id from dict key (backend omits it from the value body).
			const entries = Object.entries(res.data.list).map(
				([id, app]) => [id, { ...app, store_app_id: id }] as [string, ComposeAppStoreInfo]
			);

			// Build a set of titles that have a standard (non-prefixed) version.
			// When a "big-bear-*" variant and a standard app share the same display title,
			// the standard version wins and the variant is hidden.
			const standardTitles = new Set<string>();
			for (const [id, app] of entries) {
				if (!id.startsWith('big-bear-')) {
					const t = (app.title?.en_us || id).toLowerCase();
					standardTitles.add(t);
				}
			}

			const deduped = entries.filter(([id, app]) => {
				if (!id.startsWith('big-bear-')) return true;
				const t = ((app as ComposeAppStoreInfo).title?.en_us || id).toLowerCase();
				return !standardTitles.has(t);
			});

			appStoreCatalog = Object.fromEntries(deduped);
		}
	} catch (e) {
		error = (e as Error).message ?? 'Failed to load app store';
	} finally {
		loading = false;
		catalogLoaded = true;
	}
}

async function toggleAppStatus(id: string, currentStatus: string) {
	const newStatus = currentStatus === 'running' ? 'stop' : 'start';
	try {
		// Optimistic update
		if (installedApps[id]) {
			installedApps[id].status = newStatus === 'start' ? 'running' : 'exited';
		}
		await setComposeAppStatus(id, newStatus);
	} catch (e) {
		// Revert on error
		await fetchInstalledApps();
		throw e;
	}
}

async function uninstallApp(id: string) {
	try {
		await uninstallComposeApp(id);
		const next = { ...installedApps };
		delete next[id];
		installedApps = next;
	} catch (e) {
		await fetchInstalledApps();
		throw e;
	}
}

/**
 * Returns true when an installed app came from a PowerLab store source.
 *
 * Identity is baked into the app's own compose at install time: a store
 * app carries provenance (`x-powerlab.source.catalog` and/or `author`)
 * in its extension block. We classify from THAT — never from whether the
 * catalog is currently loaded. The opt-in gate empties the browse list
 * when the operator disables the catalog, but already-installed apps must
 * keep their identity (otherwise every installed app would flip to
 * "Custom" the moment the catalog is turned off). Catalog membership
 * stays as a fallback for apps that predate baked provenance.
 *
 * Custom apps (built from scratch / renamed forks) carry only a
 * store_app_id equal to their own name, with no author or source.
 */
function isPowerLabApp(app: ComposeAppWithStoreInfo): boolean {
	const info = app.store_info;
	const id = info?.store_app_id;
	if (!id) return false;
	if (info?.source?.catalog || info?.author) return true;
	return id in appStoreCatalog;
}

export function useAppStore() {
	return {
		get installedApps() { return installedList; },
		get appStoreCatalog() { return catalogList; },
		get loading() { return loading; },
		get error() { return error; },
		get installedLoaded() { return installedLoaded; },
		get catalogLoaded() { return catalogLoaded; },
		fetchInstalledApps,
		fetchAppStore,
		toggleAppStatus,
		uninstallApp,
		updateApp,
		isPowerLabApp,
		getDiskUsage: (id: string) => diskUsages[id]
	};
}

async function updateApp(id: string) {
	try {
		await updateComposeApp(id);
		await fetchInstalledApps();
	} catch (e) {
		error = (e as Error).message;
		throw e;
	}
}
