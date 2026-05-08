/**
 * Trust-dance helpers extracted for testability.
 *
 * The full trust dance (testHttpsConnection in settings/+page.svelte)
 * mixes fetch calls, window.location mutation, and toast side effects
 * into one big async function — hard to unit test. The pieces below
 * are pure and have a single responsibility each, so the test suite
 * can lock down their semantics without spinning up the whole page.
 */

export type RedirectIntent =
	| { kind: 'redirect'; targetUrl: string }
	| { kind: 'already-secure'; currentUrl: string };

/**
 * Decides what should happen after the guards pass: a real redirect
 * to the secure URL, or a no-op because the user is already there.
 *
 * Why this exists: the user-visible "Verify" button in dev runs
 * from http://localhost:5173 → redirect to https://localhost:8443
 * (different URL, navigation visible). In production the user is
 * often already at https://m900:8443/settings → redirect target is
 * identical → window.location.href = same URL is a silent no-op,
 * which the user perceived as "I clicked Verify and nothing
 * happened." We surface this branch explicitly so the UI can show
 * a distinct toast ("Already on secure URL — trust verified.")
 * instead of misleading "Redirecting…" + actual no-op.
 *
 * Always sets HTTPS protocol and the configured HTTPS port.
 * Preserves path, hash, and query so deep links survive.
 */
export function computeRedirectIntent(
	currentUrl: string,
	httpsPort: string
): RedirectIntent {
	const secureUrl = new URL(currentUrl);
	secureUrl.protocol = 'https:';
	secureUrl.port = httpsPort;
	const targetUrl = secureUrl.toString();

	if (targetUrl === currentUrl) {
		return { kind: 'already-secure', currentUrl };
	}
	return { kind: 'redirect', targetUrl };
}
