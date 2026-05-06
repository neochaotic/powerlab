/**
 * Updater store — polls the gateway for available PowerLab updates and
 * exposes the result to UI components (Settings → About card today,
 * sidebar pill in a follow-up).
 *
 * Polling cadence is intentionally low: once on mount, then once per
 * hour. The host hits the GitHub Releases API on every call, and we
 * do not want to hammer the rate limit while a user is just navigating
 * around the panel. The user can force a re-check via the "Check
 * again" button on the Settings card.
 */

import { checkForUpdate, preflightUpdate, installUpdate } from '$lib/api/updater';
import type { CheckResult, PreflightResult } from '$lib/api/updater';

const POLL_INTERVAL_MS = 60 * 60 * 1000; // 1 h

class UpdaterStore {
	check: CheckResult | null = $state(null);
	loading: boolean = $state(false);
	error: string | null = $state(null);

	preflight: PreflightResult | null = $state(null);
	preflightLoading: boolean = $state(false);

	installing: boolean = $state(false);
	installError: string | null = $state(null);

	private pollTimer: ReturnType<typeof setInterval> | null = null;

	async refresh(): Promise<void> {
		this.loading = true;
		this.error = null;
		try {
			this.check = await checkForUpdate();
		} catch (e) {
			// Non-fatal — the user might be offline or behind a captive
			// portal. Surface as an error string but don't toast,
			// because the updater check is implicit.
			this.error = (e as Error).message;
		} finally {
			this.loading = false;
		}
	}

	async runPreflight(): Promise<void> {
		this.preflightLoading = true;
		try {
			this.preflight = await preflightUpdate();
		} catch (e) {
			this.preflight = null;
			this.error = (e as Error).message;
		} finally {
			this.preflightLoading = false;
		}
	}

	/**
	 * Triggers the install. The backend returns 501 in v0.2 (Phase 4
	 * of #21 not done). Surface that as a friendly message instead of
	 * a crash. When Phase 4 lands, this becomes a full "stop services
	 * → swap binaries → health-check → reload" flow with progress.
	 */
	async install(): Promise<void> {
		this.installing = true;
		this.installError = null;
		try {
			await installUpdate();
		} catch (e) {
			this.installError = (e as Error).message;
		} finally {
			this.installing = false;
		}
	}

	startPolling(): void {
		if (this.pollTimer !== null) return;
		// First poll immediately, then on the hourly cadence.
		this.refresh();
		this.pollTimer = setInterval(() => this.refresh(), POLL_INTERVAL_MS);
	}

	stopPolling(): void {
		if (this.pollTimer !== null) {
			clearInterval(this.pollTimer);
			this.pollTimer = null;
		}
	}
}

export const updaterStore = new UpdaterStore();
