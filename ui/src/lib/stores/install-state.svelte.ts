/**
 * Cross-page install state — bridges the install flow in the store
 * (/apps page modal) with the launchpad's tile rendering (/ page).
 *
 * Sprint 13.2.2 use case: when the user closes the install modal
 * without canceling, the install keeps running and the launchpad
 * shows a "ghost tile" at the position the app will occupy when
 * installed, with the install progress rendered as an overlay on
 * the icon (iOS-style "icon is loading" pattern).
 *
 * This store owns ONLY the per-app install progress state. It does
 * NOT replace the installedApps catalog (`apps.svelte.ts`) — once
 * an install completes, the app moves out of this store and into
 * the regular installed list via `fetchInstalledApps()`.
 *
 * Design choice — separate store rather than extending apps.svelte
 * with install state: keeps the install-progress signal addressable
 * by id without forcing every consumer of the apps store to handle
 * a new shape. Launchpad reads both stores; store-level concerns
 * stay separated.
 */

import type { ComposeAppStoreInfo } from '$lib/api/apps';

export type InstallPhase =
	| 'installing' // HTTP POST in flight
	| 'starting' //   POST returned, SSE log stream open
	| 'success'
	| 'error'
	| 'timeout';

export interface InstallProgress {
	step: number;
	total: number;
	label?: string;
}

export interface InstallStateEntry {
	id: string;
	storeInfo: ComposeAppStoreInfo;
	phase: InstallPhase;
	currentPhase: InstallProgress | null;
	progress: number; // 0..1
	logs: string;
	error?: string;
	startedAt: number; // epoch ms — used for "still installing after 2min" UI hints
}

let installing = $state<Record<string, InstallStateEntry>>({});

/**
 * Begin tracking an install. Caller passes the app's store_info so
 * the launchpad can render the icon + title without re-fetching.
 */
function start(id: string, storeInfo: ComposeAppStoreInfo): void {
	installing[id] = {
		id,
		storeInfo,
		phase: 'installing',
		currentPhase: null,
		progress: 0,
		logs: '',
		startedAt: Date.now(),
	};
}

/**
 * Update a tracked install. Merges the patch into the existing
 * entry — fields not present in `patch` are preserved.
 *
 * Returns false if the install isn't tracked (caller should not
 * race against a finished install).
 */
function update(id: string, patch: Partial<Omit<InstallStateEntry, 'id' | 'storeInfo' | 'startedAt'>>): boolean {
	const cur = installing[id];
	if (!cur) return false;
	installing[id] = { ...cur, ...patch };
	return true;
}

/**
 * Stop tracking. Caller invokes this after success / final error /
 * user-cancel. The launchpad's regular installed-apps fetch picks
 * up the real entry afterwards (for success) — there's a brief
 * blink as the ghost tile is replaced by the real tile, which is
 * acceptable for the visual flow.
 */
function finish(id: string): void {
	delete installing[id];
}

/**
 * Read-only access for components that want to react to changes.
 */
function get(id: string): InstallStateEntry | null {
	return installing[id] ?? null;
}

export function useInstallState() {
	return {
		get all() {
			return Object.values(installing);
		},
		get(id: string) {
			return get(id);
		},
		start,
		update,
		finish,
	};
}
