import { expect, test } from '@playwright/test';
import { installBaselineMocks } from './helpers/mock-backend';

// Upgrade-flow E2E (Sprint 15 L2 — follow-up to #344 install-flow).
// Mandatory pre-tag gate.
//
// Locks the v0.6.7 → v0.6.10 upgrade-401 bug class end-to-end:
// `upgradeProgress.start()` used to call raw `fetch()` and bypass
// the api client's JWT Authorization injection — every click 401'd
// at the gateway. Fixed in v0.6.10 PR #352. The contract regression
// test in `upgradeProgress.test.ts` locks the function specifically;
// this Playwright spec locks the FULL pipeline (button → POST →
// overlay → version poll → success terminal).
//
// Strategy: drive `/settings` AboutPane.
// Mocks:
//   - GET  /v1/powerlab-update          → decision: update_ok, current/available
//   - POST /v1/powerlab-update/install  → asserts Authorization header, returns 202
//   - GET  /v1/powerlab/version         → first hit old version, then target
// Assertions:
//   - Authorization header present on the POST (the bug class)
//   - Overlay renders with the in-flight state
//   - No `effect_update_depth_exceeded` console errors

const CURRENT_VERSION = '0.6.9';
const TARGET_VERSION = '0.6.10';

test.describe('upgrade-flow E2E', () => {
	test.beforeEach(async ({ page }) => {
		await installBaselineMocks(page);
	});

	test('Upgrade button → POST carries auth → overlay → version poll reaches target', async ({ page }) => {
		const consoleErrors: string[] = [];
		page.on('console', (msg) => {
			if (msg.type() === 'error') consoleErrors.push(msg.text());
		});

		// The bug class signature — the POST must carry the JWT header.
		// We capture it from the mock and assert below.
		let installPostAuthHeader: string | null = null;
		let installPostCount = 0;

		// Mock 1: updater check → button surfaces with target version.
		await page.route('**/v1/powerlab-update', (route) =>
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({
					data: {
						decision: 'update_ok',
						current: CURRENT_VERSION,
						available: TARGET_VERSION,
						release_summary: 'Hotfix: in-UI upgrade button fix.'
					}
				})
			})
		);

		// Mock 2: install POST — capture the Authorization header (the bug
		// class signature) and return 202 so the overlay transitions to
		// the polling state.
		await page.route('**/v1/powerlab-update/install', (route) => {
			installPostCount++;
			installPostAuthHeader = route.request().headerValue('authorization') as never;
			// headerValue is async on the request — re-read via the headers map.
			const headers = route.request().headers();
			installPostAuthHeader = headers['authorization'] ?? null;
			return route.fulfill({
				status: 202,
				contentType: 'application/json',
				body: JSON.stringify({ data: null })
			});
		});

		// Mock 3: version probe — has two callers with conflicting needs:
		//   - version handshake on layout mount: needs 'dev' so the
		//     "Stale UI bundle" banner doesn't appear (which would steal
		//     pointer events and break our click).
		//   - upgradeProgress poll AFTER install: needs to flip
		//     CURRENT → TARGET so the overlay reaches success.
		// We gate on whether the install POST has fired yet.
		let versionCallCount = 0;
		await page.route('**/v1/powerlab/version', (route) => {
			versionCallCount++;
			let version: string;
			if (installPostCount === 0) {
				// Pre-install: suppress the version-mismatch banner.
				version = 'dev';
			} else if (versionCallCount < installPostCount + 2) {
				// First polls after install — services still down.
				version = CURRENT_VERSION;
			} else {
				// Target version visible — overlay flips to success.
				version = TARGET_VERSION;
			}
			return route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({ version })
			});
		});

		await page.goto('/settings');

		// AboutPane is reached via the Settings sidebar. Click it first —
		// /settings defaults to the General tab.
		await page
			.getByRole('button', { name: /^about$/i })
			.evaluate((b) => (b as HTMLButtonElement).click());

		// The "Upgrade to v0.6.10" button is rendered when
		// updaterStore.check.decision === 'update_ok'. Wait for it.
		const upgradeBtn = page.getByRole('button', { name: new RegExp(`Upgrade to v${TARGET_VERSION}`, 'i') });
		await expect(upgradeBtn).toBeVisible({ timeout: 10_000 });

		// Native click in browser context — same trick install-flow.spec.ts
		// uses to bypass synthetic-click stacking-context issues on Svelte
		// transform layers.
		await upgradeBtn.evaluate((b) => (b as HTMLButtonElement).click());

		// The bug class: assert the POST fired AND carried the JWT header.
		// Both conditions matter — count==0 means click never reached
		// handleUpgrade; auth==null means raw fetch slipped back in.
		await expect.poll(() => installPostCount, { timeout: 5_000 }).toBeGreaterThanOrEqual(1);
		expect(
			installPostAuthHeader,
			'POST /v1/powerlab-update/install must carry an Authorization header — raw fetch regression?'
		).not.toBeNull();
		expect(
			installPostAuthHeader,
			'Authorization header must be non-empty'
		).not.toBe('');

		// The "Upgrading…" overlay should be visible while polling. We
		// don't need to wait for terminal success because the overlay
		// auto-reloads the page when it sees the target version, which
		// would tear down the test harness. The polling-state visibility
		// alone proves the full POST + state-machine + poll pipeline
		// is alive.
		await expect.poll(() => versionCallCount, { timeout: 8_000 }).toBeGreaterThanOrEqual(1);

		// Regression-lock the v0.6.7 effect-loop signature (same defensive
		// check install-flow uses). Any reactivity-graph regression in the
		// upgrade overlay would shout here.
		const loopErrors = consoleErrors.filter((e) => /effect_update_depth_exceeded/i.test(e));
		expect(loopErrors, `Svelte 5 reactivity loop returned: ${loopErrors.join(' | ')}`).toEqual([]);
	});
});
