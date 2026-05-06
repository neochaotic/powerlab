/**
 * In-UI updater API client.
 *
 * Backed by `backend/core/route/v1/powerlab_update.go`. The contract is
 * the manifest format documented in `docs/UPDATE_MANIFEST.md`.
 *
 * Three endpoints:
 *
 *   GET  /v1/powerlab-update           — what does the host think it
 *                                          should do? (CheckResult)
 *   GET  /v1/powerlab-update/preflight — run the manifest's pre-install
 *                                          checks against this host.
 *   POST /v1/powerlab-update/install   — kick off the install (Phase 4
 *                                          — currently returns 501).
 *
 * Decisions returned by `check`:
 *
 *   up_to_date     — host already runs the manifest's version
 *   update_ok      — newer version available, host can upgrade now
 *   too_old        — host is older than min_upgrade_from; needs to
 *                    upgrade to an intermediate first
 *   skipped        — manifest has skip_release: true (the maintainer
 *                    pulled this release after publishing)
 *   no_arch        — the manifest does not ship a tarball for the
 *                    host's architecture
 */

import { api } from './client';

export type UpdateDecision =
	| 'up_to_date'
	| 'update_ok'
	| 'too_old'
	| 'skipped'
	| 'no_arch';

export interface TarballEntry {
	url: string;
	sha256: string;
	size_bytes: number;
}

export interface BreakingChange {
	kind: string;
	description: string;
	manual_action?: string | null;
}

export interface Manifest {
	version: string;
	released_at: string;
	min_upgrade_from: string;
	skip_release: boolean;
	summary: string;
	changelog_url: string;
	tarball: Record<string, TarballEntry>;
	breaking_changes: BreakingChange[];
	pre_install_checks: Array<Record<string, unknown>>;
	db_migrations: Array<Record<string, unknown>>;
}

export interface CheckResult {
	current: string;
	available?: string;
	decision: UpdateDecision;
	release_summary?: string;
	changelog_url?: string;
	manifest?: Manifest;
}

export interface PreflightCheck {
	kind: string;
	status: 'pass' | 'warn' | 'fail';
	message: string;
}

export interface PreflightResult {
	decision: UpdateDecision;
	checks: PreflightCheck[];
}

type Envelope<T> = { success: number; message: string; data?: T };

/**
 * Returns what the host believes its update state to be. Safe to poll
 * — the gateway caches the manifest fetch internally for the GitHub
 * rate-limit window.
 */
export async function checkForUpdate(): Promise<CheckResult> {
	const res = await api.get<Envelope<CheckResult>>('/v1/powerlab-update');
	return res.data!;
}

/**
 * Re-fetches the manifest and runs each pre-install check on the
 * host. Use right before showing the "Upgrade" button so the user
 * sees the current state, not a cached copy from a minute ago.
 */
export async function preflightUpdate(): Promise<PreflightResult> {
	const res = await api.get<Envelope<PreflightResult>>(
		'/v1/powerlab-update/preflight'
	);
	return res.data!;
}

/**
 * Kicks off the install. The backend returns 202 Accepted as soon as
 * install.sh is spawned in --upgrade mode. The actual snapshot + swap
 * + health-check + rollback runs asynchronously inside install.sh;
 * poll `getUpgradeStatus()` to learn when it finishes.
 */
export async function installUpdate(): Promise<void> {
	await api.post<Envelope<void>>('/v1/powerlab-update/install', {});
}

/**
 * The result of the most recent upgrade attempt on this host.
 * `null` when no upgrade has been attempted (fresh install).
 *
 * `result` is `"success"` when install.sh finished and the gateway
 * responded to the health-check, or `"rolled_back"` when the
 * health-check failed and install.sh restored the previous version
 * from the snapshot.
 */
export interface LastUpgrade {
	from: string;
	to: string;
	result: 'success' | 'rolled_back';
	succeeded_at?: string;
	failed_at?: string;
	snapshot_path?: string;
	diagnostic?: string;
}

export async function getUpgradeStatus(): Promise<LastUpgrade | null> {
	const res = await api.get<Envelope<LastUpgrade | null>>(
		'/v1/powerlab-update/status'
	);
	return res.data ?? null;
}
