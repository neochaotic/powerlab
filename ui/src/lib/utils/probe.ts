/**
 * probePortReachable — fast liveness check on an arbitrary URL's
 * `/ping` path. Used by redirect-on-success flows (port change,
 * HTTPS trust dance) to avoid stranding the user on a URL that
 * happens not to answer.
 *
 * Returns true ONLY when the probe target responds with HTTP 2xx
 * within the timeout. Any failure mode — connection refused, TLS
 * error, DNS failure, non-2xx status, timeout — returns false.
 *
 * The caller is responsible for retry / backoff. This function is a
 * single attempt by design so callers can compose retry policies
 * without hidden state.
 */
export async function probePortReachable(
	url: URL,
	options: { timeoutMs?: number; pingPath?: string; fetchImpl?: typeof fetch } = {}
): Promise<boolean> {
	const timeoutMs = options.timeoutMs ?? 2500;
	const pingPath = options.pingPath ?? '/ping';
	const fetchImpl = options.fetchImpl ?? fetch;

	// Build a clean probe URL — origin + ping path only. Stripping
	// search/hash keeps the probe deterministic; otherwise a stale
	// `#security` hash from the calling page leaks into the probe
	// URL and confuses logs / proxy rules.
	const ping = new URL(pingPath, url.origin);

	try {
		const r = await fetchImpl(ping.toString(), {
			method: 'GET',
			mode: 'cors',
			signal: AbortSignal.timeout(timeoutMs)
		});
		return r.ok;
	} catch {
		return false;
	}
}
