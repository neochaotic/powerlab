/**
 * Updater failure-UX state machine.
 *
 * Pinned contract (see `updater-failure-ux.test.ts`):
 *
 *   - Single failure → `transientFailure: true`, no banner shown.
 *   - 3+ failures AND last success older than 6h → `persistentFailure`.
 *     Banner shown (amber, not red).
 *   - Any success → counter resets, last-success timestamp updates.
 *   - `lastCheckedHumanRelative` is the friendly "Xm ago" string for
 *     the subtle status line on the AboutPane; null when we never
 *     succeeded this session.
 *
 * The state is process-local (module-scoped). Refreshes between
 * server-side renders reset it — acceptable because the polling
 * cadence (1h) is much wider than typical SSR cycles.
 */

const PERSISTENT_FAILURES_THRESHOLD = 3;
const STALE_SUCCESS_MS = 6 * 60 * 60 * 1000; // 6 hours

interface UpdaterFailureState {
	consecutiveFailures: number;
	lastSuccessTs: number | null;
	transientFailure: boolean;
	persistentFailure: boolean;
	lastCheckedHumanRelative: string | null;
}

let state = {
	consecutiveFailures: 0,
	lastSuccessTs: null as number | null
};

export function resetUpdaterFailureState(): void {
	state = { consecutiveFailures: 0, lastSuccessTs: null };
}

export function recordCheckSuccess(): void {
	state = {
		consecutiveFailures: 0,
		lastSuccessTs: Date.now()
	};
}

export function recordCheckFailure(_reason?: string): void {
	state = {
		consecutiveFailures: state.consecutiveFailures + 1,
		lastSuccessTs: state.lastSuccessTs
	};
}

export function getUpdaterFailureState(): UpdaterFailureState {
	const persistent = isPersistent();
	const transient = state.consecutiveFailures > 0 && !persistent;
	return {
		consecutiveFailures: state.consecutiveFailures,
		lastSuccessTs: state.lastSuccessTs,
		transientFailure: transient,
		persistentFailure: persistent,
		lastCheckedHumanRelative: formatRelative(state.lastSuccessTs)
	};
}

function isPersistent(): boolean {
	if (state.consecutiveFailures < PERSISTENT_FAILURES_THRESHOLD) return false;
	if (state.lastSuccessTs === null) return true; // never succeeded this session
	const age = Date.now() - state.lastSuccessTs;
	return age >= STALE_SUCCESS_MS;
}

/**
 * Turns whatever `checkForUpdate()` rejected with into a human reason
 * that names the HTTP status. The api client rejects with an ApiError
 * (`{ status, message }`): status 0 means the host was unreachable
 * (network/CORS/DNS), 401 means the session token is stale, any other
 * non-2xx is a gateway-side error. A 401 and a timeout are different
 * problems the user fixes differently — so we never collapse them into
 * a single opaque "check failed".
 */
export function describeCheckFailure(e: unknown): string {
	const status =
		typeof e === 'object' && e !== null && 'status' in e
			? (e as { status: unknown }).status
			: undefined;

	if (status === 0) {
		return 'the host is unreachable — check your network connection';
	}
	if (status === 401) {
		return 'HTTP 401 — your session expired; sign in again and retry';
	}
	if (typeof status === 'number' && status > 0) {
		return `HTTP ${status} — the gateway could not complete the check`;
	}

	const msg =
		typeof e === 'object' && e !== null && 'message' in e
			? String((e as { message: unknown }).message)
			: e instanceof Error
				? e.message
				: '';
	return msg || 'the update check failed for an unknown reason';
}

function formatRelative(ts: number | null): string | null {
	if (ts === null) return null;
	const ageMs = Date.now() - ts;
	if (ageMs < 60_000) return 'just now';
	const minutes = Math.floor(ageMs / 60_000);
	if (minutes < 60) return `${minutes}m ago`;
	const hours = Math.floor(minutes / 60);
	if (hours < 24) return `${hours}h ago`;
	const days = Math.floor(hours / 24);
	return `${days}d ago`;
}
