import type { Page, APIRequestContext } from '@playwright/test';
import { test as base } from '@playwright/test';

/**
 * Real-backend smoke harness.
 *
 * Mock-driven specs (page.route()) prove the UI renders given a
 * fixed API. They do NOT prove the API actually exists or returns
 * what the UI expects. Production bug class: a core endpoint moves
 * or breaks, the UI silently shows empty / wrong state, CI is green
 * because the mock still serves the old shape.
 *
 * This module wires specs against a real PowerLab backend via the
 * POWERLAB_E2E_BASE env var. Spec names ending in `.smoke.spec.ts`
 * are opt-in via `--grep @smoke` so the default mock-driven suite
 * stays unchanged.
 *
 * Pattern:
 *   import { expect } from '@playwright/test';
 *   import { realBackendTest as test, REAL_BACKEND_BASE, loginIntoPage } from './helpers/real-backend';
 *
 *   test('AuditPane renders @smoke', async ({ page }) => {
 *     await loginIntoPage(page);
 *     await page.goto(`${REAL_BACKEND_BASE}/settings`);
 *     // ...
 *   });
 *
 * Env vars:
 *   POWERLAB_E2E_BASE     — gateway URL (e.g. http://192.168.18.86:8765)
 *                           Whole suite skips when unset.
 *   POWERLAB_E2E_USER     — login username (default: admin)
 *   POWERLAB_E2E_PASSWORD — login password (required)
 */

export const REAL_BACKEND_BASE = process.env.POWERLAB_E2E_BASE ?? '';

/**
 * `realBackendTest` is a `test` clone that automatically skips
 * every test inside it when POWERLAB_E2E_BASE is unset. Use it
 * instead of Playwright's default `test` in smoke specs so the
 * normal mock-driven suite doesn't try to hit a non-existent
 * backend.
 *
 * The skip happens in `beforeEach` because Playwright fixtures
 * are resolved per-test, not per-suite.
 */
export const realBackendTest = base.extend<object>({});
realBackendTest.beforeEach(({}, testInfo) => {
	if (!REAL_BACKEND_BASE) {
		testInfo.skip(true, 'POWERLAB_E2E_BASE not set — real-backend smoke specs require a running gateway URL');
	}
});

/**
 * Backwards-compatible alias for files that want an explicit
 * "skipIfNoBackend()" call rather than relying on the test fixture.
 * No-op now that realBackendTest handles the skip per-test.
 */
export function skipIfNoBackend(_testRef: typeof base) {
	// All skipping is per-test via the fixture above; this exists
	// so existing imports don't break.
}

// Cache the access token across tests in a single run — login is
// rate-limited at the backend (HTTP 429 after a few attempts), so a
// suite that does 5+ tests would fail without this.
let cachedToken: string | null = null;
let cachedAt = 0;

/**
 * Login against the real backend's `/v1/users/login`. Returns the
 * access_token. Throws on any failure (smoke specs hard-fail on
 * bad creds — they don't silently skip).
 *
 * Token is cached for 5 minutes across the test run to avoid the
 * backend's login rate limit (HTTP 429).
 */
export async function loginAndGetToken(
	request: APIRequestContext,
	creds: { username?: string; password?: string } = {}
): Promise<string> {
	if (cachedToken && Date.now() - cachedAt < 5 * 60 * 1000) {
		return cachedToken;
	}
	const user = creds.username ?? process.env.POWERLAB_E2E_USER ?? 'admin';
	const pass = creds.password ?? process.env.POWERLAB_E2E_PASSWORD;
	if (!pass) {
		throw new Error('POWERLAB_E2E_PASSWORD env var is required for real-backend login');
	}
	const r = await request.post(`${REAL_BACKEND_BASE}/v1/users/login`, {
		data: { username: user, password: pass },
		headers: { 'Content-Type': 'application/json' }
	});
	if (!r.ok()) {
		throw new Error(`login failed: HTTP ${r.status()} ${await r.text()}`);
	}
	const body = await r.json();
	const tok: string | undefined = body?.data?.token?.access_token;
	if (!tok) {
		throw new Error(`login response missing data.token.access_token: ${JSON.stringify(body)}`);
	}
	cachedToken = tok;
	cachedAt = Date.now();
	return tok;
}

/**
 * Convenience: login + set Authorization header on the page's
 * extra HTTP headers. Returns the token so the spec can also do
 * raw page.request calls (use full URLs with REAL_BACKEND_BASE).
 */
export async function loginIntoPage(
	page: Page,
	creds?: { username?: string; password?: string }
): Promise<string> {
	const tok = await loginAndGetToken(page.request, creds);
	await page.setExtraHTTPHeaders({ Authorization: `Bearer ${tok}` });
	return tok;
}
