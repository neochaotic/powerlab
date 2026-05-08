/**
 * Regression: the "Verify" button silently no-ops when the user is
 * already on the secure URL.
 *
 * Bug report: user installed CA in production, clicked Verify, "nada
 * acontece" — nothing happened. In dev (http://localhost:5173) the
 * button works because the redirect targets a different URL. In
 * production the user often opens https://m900:8443/settings#security
 * directly, so the redirect target equals the current URL → assigning
 * window.location.href = same URL is a no-op → no visible feedback.
 *
 * computeRedirectIntent must distinguish these two cases so the UI
 * shows a distinct "already-secure" toast instead of the misleading
 * "redirecting…" message that never produces a redirect.
 */

import { describe, it, expect } from 'vitest';
import { computeRedirectIntent } from './trust-dance';

describe('computeRedirectIntent', () => {
	it('returns redirect when on plain HTTP — dev workflow (Vite at :5173)', () => {
		const result = computeRedirectIntent('http://localhost:5173/settings#security', '8443');
		expect(result).toEqual({
			kind: 'redirect',
			targetUrl: 'https://localhost:8443/settings#security'
		});
	});

	it('returns redirect when on production HTTP at non-default port', () => {
		const result = computeRedirectIntent('http://m900:8765/settings#security', '8443');
		expect(result).toEqual({
			kind: 'redirect',
			targetUrl: 'https://m900:8443/settings#security'
		});
	});

	it('returns already-secure when current URL is the redirect target — production no-op case', () => {
		const url = 'https://m900:8443/settings#security';
		const result = computeRedirectIntent(url, '8443');
		expect(result).toEqual({
			kind: 'already-secure',
			currentUrl: url
		});
	});

	it('returns already-secure for IP-based access on the secure port', () => {
		const url = 'https://192.168.18.86:8443/apps';
		expect(computeRedirectIntent(url, '8443')).toEqual({
			kind: 'already-secure',
			currentUrl: url
		});
	});

	it('returns redirect when on HTTPS but different (non-secure) port', () => {
		// Edge case: an admin proxy sets up HTTPS on a non-standard
		// port. We still want to push the user to the gateway's
		// canonical HTTPS port (8443) because that's where the cert
		// is bound.
		const result = computeRedirectIntent('https://m900:9443/settings', '8443');
		expect(result).toEqual({
			kind: 'redirect',
			targetUrl: 'https://m900:8443/settings'
		});
	});

	it('preserves the URL path, hash, and query string when redirecting', () => {
		const result = computeRedirectIntent(
			'http://m900:8765/files?path=/etc&filter=*.conf#editor',
			'8443'
		);
		expect(result).toEqual({
			kind: 'redirect',
			targetUrl: 'https://m900:8443/files?path=/etc&filter=*.conf#editor'
		});
	});
});
