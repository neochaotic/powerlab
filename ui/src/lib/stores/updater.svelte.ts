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

import {
	checkForUpdate,
	preflightUpdate,
	installUpdate,
	getUpgradeStatus
} from '$lib/api/updater';
import type { CheckResult, PreflightResult, LastUpgrade } from '$lib/api/updater';
import { toast } from '$lib/stores/toast.svelte';
import {
	recordCheckSuccess,
	recordCheckFailure,
	getUpdaterFailureState,
	describeCheckFailure
} from './updater-failure-state';

const POLL_INTERVAL_MS = 60 * 60 * 1000; // 1 h

class UpdaterStore {
	check: CheckResult | null = $state(null);
	loading: boolean = $state(false);
	error: string | null = $state(null);

	/**
	 * True when the most recent check was triggered by the user (the
	 * "Check now" button) AND it failed. A user who explicitly asked
	 * for a check must always see the outcome — including the reason —
	 * whereas a single failed *background* poll stays quiet (the
	 * failure-state machine only escalates a streak). Without this the
	 * manual failure fell through to the silent transient branch and
	 * read as "no update available".
	 */
	lastCheckFailedManually: boolean = $state(false);

	preflight: PreflightResult | null = $state(null);
	preflightLoading: boolean = $state(false);

	installing: boolean = $state(false);
	installError: string | null = $state(null);

	/**
	 * The result of the previous upgrade attempt on this host. Read
	 * once on store mount, then again after every install kicked off
	 * from this UI (polled while `installing === true`).
	 */
	lastUpgrade: LastUpgrade | null = $state(null);

	private pollTimer: ReturnType<typeof setInterval> | null = null;
	private statusPollTimer: ReturnType<typeof setInterval> | null = null;

	async refresh(userInitiated = false): Promise<void> {
		this.loading = true;
		this.error = null;
		try {
			this.check = await checkForUpdate();
			recordCheckSuccess();
			this.lastCheckFailedManually = false;
		} catch (e) {
			// Non-fatal — the user might be offline or behind a captive
			// portal. The failure-state machine decides whether to
			// surface a banner for *background* polls (3+ consecutive AND
			// no recent success); transient ones stay silent. A
			// user-initiated check is different: they asked, so always
			// surface the reason (with the HTTP status).
			const reason = describeCheckFailure(e);
			recordCheckFailure(reason);
			this.error = reason;
			this.lastCheckFailedManually = userInitiated;
		} finally {
			this.loading = false;
		}
	}

	/**
	 * Failure UX flags (transient vs persistent + "last checked Xm ago").
	 * Reactive via $derived in the consumer when it polls `refresh()`.
	 */
	get failureState() {
		return getUpdaterFailureState();
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
	/**
	 * Kicks off the install. The backend returns 202 immediately and
	 * install.sh runs in the background. We poll the status endpoint
	 * every 2 s while the upgrade is in flight; when install.sh
	 * writes /var/lib/powerlab/last-upgrade.json with a result we
	 * stop polling and update lastUpgrade so the UI can render the
	 * outcome.
	 *
	 * The whole upgrade window can include `core` itself being
	 * restarted — at which point the gateway is briefly unreachable
	 * and our poll fetches will fail. That's fine: we keep retrying
	 * until either we get a fresh status object or the user reloads.
	 */
	async install(): Promise<void> {
		this.installing = true;
		this.installError = null;
		try {
			await installUpdate();
		} catch (e) {
			this.installError = (e as Error).message;
			this.installing = false;
			return;
		}
		// Snapshot the current lastUpgrade so we can detect the moment
		// install.sh writes a new one (different succeeded_at /
		// failed_at). We compare on the timestamp because it changes
		// on every run.
		const previousMarker = this.lastUpgrade
			? this.lastUpgrade.succeeded_at || this.lastUpgrade.failed_at || ''
			: '';
		this.startStatusPolling(previousMarker);
	}

	async refreshStatus(): Promise<void> {
		try {
			this.lastUpgrade = await getUpgradeStatus();
		} catch {
			// Status fetch can fail mid-restart (gateway swapping
			// listener). Swallow and let the next poll retry.
		}
	}

	private startStatusPolling(previousMarker: string): void {
		if (this.statusPollTimer !== null) clearInterval(this.statusPollTimer);
		this.statusPollTimer = setInterval(async () => {
			await this.refreshStatus();
			const marker = this.lastUpgrade
				? this.lastUpgrade.succeeded_at || this.lastUpgrade.failed_at || ''
				: '';
			if (marker && marker !== previousMarker) {
				// install.sh wrote a fresh status — upgrade has
				// finished, one way or the other. Stop polling and
				// let the UI render the outcome.
				if (this.statusPollTimer !== null) {
					clearInterval(this.statusPollTimer);
					this.statusPollTimer = null;
				}
				this.installing = false;
				// Re-check so the version banner updates if the
				// upgrade succeeded.
				this.refresh();

				// v0.5.9 fix: surface a visible success/failure toast
				// AND auto-reload after a short delay so the user
				// doesn't have to refresh manually + sees the new UI
				// version. Pre-v0.5.9 the upgrade silently completed
				// and the user was left staring at "Upgrading…" until
				// they refreshed by hand.
				const succeeded = !!this.lastUpgrade?.succeeded_at;
				if (succeeded) {
					toast.success(
						'PowerLab updated successfully — reloading…',
						3000
					);
					// Window check guards against SSR / test
					// environments where window is undefined.
					if (typeof window !== 'undefined') {
						setTimeout(() => window.location.reload(), 2500);
					}
				} else {
					const failMsg =
						this.lastUpgrade?.diagnostic ||
						'Upgrade failed — see Settings → System for details.';
					// Long-lived (8s) so the user can read it; no
					// auto-reload, the previous version is still
					// running and reloading would be confusing.
					toast.error(failMsg, 8000);
				}
			}
		}, 2000);
	}

	startPolling(): void {
		if (this.pollTimer !== null) return;
		// First poll immediately, then on the hourly cadence.
		this.refresh();
		this.refreshStatus(); // also load the last-upgrade banner
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
