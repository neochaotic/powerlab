/**
 * Resolve the **source** of a catalog app — which upstream store it
 * came from. Used by the AppCard's discrete source badge per Phase
 * 5 of #307 (ADR-0024).
 *
 * Resolution precedence (first match wins):
 *
 *   1. Explicit `app.store_info.source.catalog` from the backend
 *      (only populated by the Umbrel sync pipeline today; future
 *      catalog sources may populate it too).
 *   2. Heuristic match on the icon URL host/path:
 *        getumbrel.github.io/umbrel-apps-gallery → "umbrel"
 *        IceWhaleTech/CasaOS-AppStore           → "casaos"
 *        bigbeartechworld/big-bear              → "big-bear"
 *   3. Fallback: "store" (a generic, non-vendor label so the
 *      metadata row still has SOMETHING — discreet, no claim).
 *
 * Apple-style: never lies. Returns the best-known label without
 * a colored badge / pill / brand chip. The component renders it
 * as muted text in the metadata row, not as a brand badge.
 */

export type AppSource = 'umbrel' | 'casaos' | 'big-bear' | 'store';

interface StoreInfoLike {
	icon?: string;
	source?: { catalog?: string };
}

/**
 * detectAppSource returns the source label for an app's store_info.
 * Pure function — no DOM, no side effects. The test surface is
 * the explicit-field path + the icon-URL heuristic per row.
 */
export function detectAppSource(storeInfo: StoreInfoLike | undefined | null): AppSource {
	if (!storeInfo) return 'store';

	// 1. Explicit backend field (Umbrel-synced apps only, today)
	const explicit = storeInfo.source?.catalog;
	if (explicit) {
		// Normalise: backend writes `umbrel-apps`; UI shows `umbrel`.
		if (explicit.startsWith('umbrel')) return 'umbrel';
		if (explicit.startsWith('casaos')) return 'casaos';
		if (explicit.startsWith('big-bear')) return 'big-bear';
		// Unknown explicit value — surface as generic store so we don't
		// silently render an unrecognized brand name.
		return 'store';
	}

	// 2. Heuristic on icon URL
	const icon = storeInfo.icon ?? '';
	if (icon.includes('getumbrel.github.io')) return 'umbrel';
	if (icon.includes('IceWhaleTech/CasaOS-AppStore')) return 'casaos';
	if (icon.includes('bigbeartechworld') || icon.includes('big-bear-casaos')) return 'big-bear';

	// 3. Fallback
	return 'store';
}

/**
 * appSourceLabel maps the canonical source token to the short
 * label shown in the metadata row.
 */
export function appSourceLabel(source: AppSource): string {
	switch (source) {
		case 'umbrel':
			return 'umbrel';
		case 'casaos':
			return 'casaos';
		case 'big-bear':
			return 'big-bear';
		case 'store':
			return 'store';
	}
}

/**
 * appSourceUpstreamURL returns a click-through URL for the source,
 * if available. The explicit `source.upstream_repo` from the
 * backend wins; otherwise we fall back to a well-known catalog
 * URL based on the heuristic source.
 */
interface StoreInfoWithUpstream extends StoreInfoLike {
	source?: { catalog?: string; upstream_repo?: string };
}

export function appSourceUpstreamURL(storeInfo: StoreInfoWithUpstream | undefined | null): string | null {
	if (!storeInfo) return null;
	const explicit = storeInfo.source?.upstream_repo;
	if (explicit) return explicit;

	const source = detectAppSource(storeInfo);
	switch (source) {
		case 'umbrel':
			return 'https://github.com/getumbrel/umbrel-apps';
		case 'casaos':
			return 'https://github.com/IceWhaleTech/CasaOS-AppStore';
		case 'big-bear':
			return 'https://github.com/bigbeartechworld/big-bear-casaos';
		case 'store':
			return null;
	}
}
