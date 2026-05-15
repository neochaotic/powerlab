import { expect, request as pwRequest } from '@playwright/test';
import { realBackendTest as test, REAL_BACKEND_BASE, loginAndGetToken } from './helpers/real-backend';

// One login per file run — the user-service rate-limits login
// aggressively (HTTP 429 after a few attempts). Cache the token at
// suite level and inject as Authorization header in each test.
let sharedToken = '';
test.beforeAll(async () => {
	if (!REAL_BACKEND_BASE) return;
	const ctx = await pwRequest.newContext();
	sharedToken = await loginAndGetToken(ctx);
	await ctx.dispose();
});

// Real-backend smoke for the AuditPane. Set:
//   POWERLAB_E2E_BASE=http://<host>:8765 \
//   POWERLAB_E2E_USER=<user> \
//   POWERLAB_E2E_PASSWORD=<pass> \
//   npx playwright test audit-pane.smoke.spec.ts
//
// Without those env vars the suite skips. Sister spec audit-pane.spec.ts
// (mock-driven) keeps proving the UI renders given a fixed shape — this
// one proves the API actually exists and returns what the UI expects.
//
// Catches the v0.6.12 bug class: a core endpoint moves from
// /v1/audit/recent to a different port and the AuditPane silently
// shows empty. Mock-driven suite was green for that bug.

test('AuditPane renders against real backend @smoke', async ({ page }) => {
	await page.setExtraHTTPHeaders({ Authorization: `Bearer ${sharedToken}` });
	await page.goto(`${REAL_BACKEND_BASE}/settings`);

	// Sidebar entry exists (Svelte renders it as <button> with
	// label "Audit" — but tolerate a different element wrapper).
	const auditLink = page.locator(':is(button,a):has-text("Audit")').first();
	await expect(auditLink).toBeVisible({ timeout: 10_000 });
	await auditLink.click();

	// Stats card or the empty state shows up — both are valid
	// (a fresh box has no rows yet but stats endpoint still returns).
	await expect(
		page.locator('[data-testid="audit-stats"], :has-text("no audit records yet")').first()
	).toBeVisible({ timeout: 5000 });

	// No console errors during render.
	const consoleErrors: string[] = [];
	page.on('console', (m) => {
		if (m.type() === 'error') consoleErrors.push(m.text());
	});
	await page.waitForTimeout(500);
	expect(consoleErrors, `console errors: ${consoleErrors.join(' | ')}`).toEqual([]);
});

test('audit endpoints return JSON not HTML @smoke', async ({ page }) => {
	// Bug class lock — the original symptom was that
	// /v1/audit/recent fell through to index.html (catch-all). The
	// content-type assertion catches that immediately.
	for (const path of ['/v1/audit/recent?limit=1', '/v1/audit/stats']) {
		const res = await page.request.get(`${REAL_BACKEND_BASE}${path}`, {
			headers: { Authorization: `Bearer ${sharedToken}` }
		});
		expect(res.status(), `${path} status`).toBe(200);
		const ct = res.headers()['content-type'] ?? '';
		expect(ct, `${path} returned Content-Type ${ct} — should be JSON, not HTML fallback`).toMatch(
			/^application\/json/
		);
	}
});
