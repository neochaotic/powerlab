/**
 * Upgrade-in-progress store — drives the full-screen overlay shown
 * while the in-UI updater is running install.sh on the host.
 *
 * The gateway is shut down + restarted as part of the upgrade
 * (~60s window). During that window the user's open browser tab
 * would otherwise see 502/503 errors from background API calls
 * (system status, disk, utilization). This store turns that into a
 * single intentional "PowerLab atualizando…" overlay, polls
 * /v1/powerlab/version until it sees the target version, and then
 * auto-reloads.
 *
 * State machine:
 *   idle ─ start() ──▶ starting ─ POST 202 ──▶ restarting ─ poll match ──▶ success ─ 2s ──▶ reload
 *                          │                       │
 *                          └─ POST !202 ──▶ error  └─ timeout >5min ──▶ error
 *
 * During `restarting`, all transient errors (5xx, network refused)
 * are suppressed — they are *expected* while gateway is down.
 *
 * Tested in upgradeProgress.test.ts (10 cases).
 */

import { api } from '../api/client';

export const UPGRADE_POLL_INTERVAL_MS = 3000;
export const UPGRADE_TIMEOUT_MS = 5 * 60 * 1000; // 5 min
export const UPGRADE_SUCCESS_RELOAD_DELAY_MS = 2000;

export type UpgradeState = 'idle' | 'starting' | 'restarting' | 'success' | 'error';

class UpgradeProgress {
	state: UpgradeState = $state('idle');
	targetVersion: string | null = $state(null);
	error: string | null = $state(null);

	private pollTimer: ReturnType<typeof setInterval> | null = null;
	private timeoutTimer: ReturnType<typeof setTimeout> | null = null;
	private reloadTimer: ReturnType<typeof setTimeout> | null = null;

	get isOverlayActive(): boolean {
		return this.state !== 'idle';
	}

	reset(): void {
		this.state = 'idle';
		this.targetVersion = null;
		this.error = null;
		this.clearTimers();
	}

	async start(targetVersion: string): Promise<void> {
		this.reset();
		this.state = 'starting';
		this.targetVersion = targetVersion;

		// Route POST through the shared api client so the JWT Authorization
		// header is attached automatically. The raw fetch this replaces
		// 401'd against the gateway for every authenticated user and was
		// the reason the in-UI upgrade button didn't work in v0.6.9 and
		// earlier. Regression test in upgradeProgress.test.ts.
		try {
			await api.post<unknown>('/v1/powerlab-update/install');
			this.state = 'restarting';
			this.beginPolling();
		} catch (e) {
			this.state = 'error';
			const err = e as { status?: number; message?: string };
			if (typeof err?.status === 'number') {
				this.error = `Upgrade refused (HTTP ${err.status}): ${err.message ?? 'no message'}`;
			} else {
				this.error = err?.message ?? String(e);
			}
		}
	}

	private beginPolling(): void {
		this.clearTimers();
		this.pollTimer = setInterval(() => {
			this.poll().catch(() => {
				// All polling errors handled inside poll().
			});
		}, UPGRADE_POLL_INTERVAL_MS);
		this.timeoutTimer = setTimeout(() => {
			if (this.state === 'restarting') {
				this.state = 'error';
				this.error = 'Upgrade timeout — services did not come back within 5 minutes. The host may need manual intervention.';
				this.clearTimers();
			}
		}, UPGRADE_TIMEOUT_MS);
	}

	private async poll(): Promise<void> {
		if (this.state !== 'restarting' || !this.targetVersion) return;
		try {
			const res = await fetch('/v1/powerlab/version');
			if (!res.ok) {
				// Expected during the window. Stay in restarting.
				return;
			}
			const data = (await res.json()) as { version: string };
			if (data.version === this.targetVersion) {
				this.state = 'success';
				this.clearTimers();
				this.reloadTimer = setTimeout(() => {
					if (typeof window !== 'undefined') window.location.reload();
				}, UPGRADE_SUCCESS_RELOAD_DELAY_MS);
			}
			// else: services still on old version, keep polling.
		} catch {
			// Network error == expected during restart. Keep polling.
		}
	}

	private clearTimers(): void {
		if (this.pollTimer) {
			clearInterval(this.pollTimer);
			this.pollTimer = null;
		}
		if (this.timeoutTimer) {
			clearTimeout(this.timeoutTimer);
			this.timeoutTimer = null;
		}
		if (this.reloadTimer) {
			clearTimeout(this.reloadTimer);
			this.reloadTimer = null;
		}
	}
}

export const upgradeProgress = new UpgradeProgress();
