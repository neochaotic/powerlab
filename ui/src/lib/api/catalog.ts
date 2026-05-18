/**
 * Catalog sources API client (ADR-0039).
 *
 * PowerLab ships one default curated catalog (`community-catalog/`)
 * and lets operators register ADDITIONAL custom sources at their own
 * risk via the admin escape hatch. The custom sources have no
 * PowerLab audit — they exist for advanced operators who want to add
 * their own catalogs (forks, mirrors, private app sets).
 *
 * Maps to the existing app-management v2 endpoints:
 *
 *   GET    /v2/app_management/appstore         → list catalog sources
 *   POST   /v2/app_management/appstore?url=…   → register a new source (async)
 *   DELETE /v2/app_management/appstore/{id}    → unregister an operator-added source
 *
 * The default PowerLab-curated catalog is always at id=0 and is not
 * removable from the UI — guard this on the client side.
 */

import { api } from './client';

export interface AppStoreSource {
	/** Stable index in the configured list. id=0 is the PowerLab default. */
	id: number;
	/**
	 * Original URL or filesystem path operator registered. For the
	 * PowerLab default this is the local catalog dir.
	 */
	url: string;
	/**
	 * Resolved on-disk root where the catalog content lives — used
	 * for display only.
	 */
	store_root: string;
}

interface Envelope<T> {
	data: T;
	message?: string;
}

/**
 * Fetch the registered catalog sources. The first entry (id=0) is
 * always the PowerLab-curated default; subsequent entries are
 * operator-added.
 */
export async function listCatalogSources(): Promise<AppStoreSource[]> {
	const res = await api.get<Envelope<AppStoreSource[]>>('/v2/app_management/appstore');
	return res.data ?? [];
}

/**
 * Register a new catalog source. Backend resolves the URL (git
 * clone for github URLs, copy for local paths, etc.) async; the
 * UI should poll listCatalogSources after this returns to see
 * the new entry surface.
 *
 * PowerLab does NOT audit the contents of operator-registered
 * catalogs. The caller MUST have surfaced an explicit warning + got
 * operator confirmation before calling this.
 */
export async function addCatalogSource(url: string): Promise<void> {
	await api.post(`/v2/app_management/appstore?url=${encodeURIComponent(url)}`);
}

/**
 * Remove an operator-added catalog source by id. The PowerLab
 * default (id=0) cannot be removed — UI must guard this.
 */
export async function removeCatalogSource(id: number): Promise<void> {
	await api.delete(`/v2/app_management/appstore/${id}`);
}
