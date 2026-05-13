/**
 * buildAppURL returns the URL the launchpad / store opens when the
 * user clicks an installed app tile. Pure function — no DOM, no
 * side effects — so the various edge cases below can be locked by
 * unit tests.
 *
 * Inputs:
 *   - hostname: typically `window.location.hostname` (the user's box)
 *   - storeInfo: the app's metadata (scheme, port_map, index)
 *
 * The non-obvious cases this handles:
 *
 *   1. `scheme: 'https'` (CasaOS-curated apps for `crafty`,
 *      `mineos-node`, `netbird`, `obsidian`, `openclaw`,
 *      `unifi-network-application`). The launchpad was hard-coding
 *      `http://` so these silently failed: the container actually
 *      serves HTTPS, the URL pointed at HTTP, browser got a
 *      blank tab. Respect the scheme.
 *
 *   2. `index: '?token=casaos'` (openclaw and similar CasaOS legacy
 *      apps). CasaOS used a query-token auth proxy; PowerLab
 *      doesn't replicate it. Stripping the `?token=casaos` fragment
 *      lets the app render its own auth (or no-auth) flow without
 *      a phantom query string. Other `index:` values (paths,
 *      anchors) are preserved verbatim.
 *
 *   3. Missing `port_map` → null (caller renders no-op / hides
 *      the open button).
 */

export interface StoreInfoForURL {
	port_map?: string;
	scheme?: string;
	index?: string;
}

export function buildAppURL(hostname: string, storeInfo: StoreInfoForURL | null | undefined): string | null {
	const port = storeInfo?.port_map;
	if (!port) return null;

	const scheme = storeInfo?.scheme === 'https' ? 'https' : 'http';

	let index = storeInfo?.index ?? '';
	// CasaOS legacy auth-proxy query — strip it. PowerLab doesn't
	// have CasaOS's token-aware proxy in front, so passing the token
	// through just gives the underlying app a stray ?token=casaos
	// it doesn't know what to do with.
	if (index === '?token=casaos') {
		index = '';
	}

	return `${scheme}://${hostname}:${port}${index}`;
}
