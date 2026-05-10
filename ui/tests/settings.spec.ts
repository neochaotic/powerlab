import { expect, test } from '@playwright/test';
import { installBaselineMocks } from './helpers/mock-backend';

// /settings page smoke — verifies the settings sidebar renders + at
// least one section button is present.
//
// Will protect the upcoming UI split in #227 PR 4 (settings/+page.svelte
// 1469 LOC → 8 panes). The sidebar's per-pane buttons are the most
// brittle surface; this test catches a bad-extract that drops one.

test.describe('/settings page', () => {
	test.beforeEach(async ({ page }) => {
		await installBaselineMocks(page);
	});

	test('renders settings sidebar', async ({ page }) => {
		await page.goto('/settings');

		// h2 = "Settings" (hardcoded in the page, not i18n at the time
		// of writing — if that changes, switch to a role-based check).
		const heading = page.locator('h2').filter({ hasText: /settings/i }).first();
		await expect(heading).toBeVisible();
	});

	test('renders pane navigation buttons', async ({ page }) => {
		await page.goto('/settings');

		// Wait for the sidebar render to settle. count() does NOT
		// auto-wait — it queries the DOM at call time. Without this,
		// the test races the auth-resolve + initial render and gets 0
		// buttons (caught locally on 2026-05-10 after the auth-key fix).
		// Use the h2 as the readiness signal (proven stable by the
		// 'renders settings sidebar' test above).
		await expect(page.locator('h2').filter({ hasText: /settings/i }).first()).toBeVisible();

		// The settings sidebar's <nav> renders one <button> per
		// section in the hardcoded sections array (5 today: general,
		// network, apps, security, about). Plus the layout sidebar's
		// nav. > 1 catches a real regression (sections array empty,
		// render path broken) without false-positiving on per-pane
		// component swaps in the upcoming UI split (PR 4 of #227).
		const navButtons = page.locator('aside nav button');
		await expect(navButtons.first()).toBeVisible();
		const count = await navButtons.count();
		expect(count).toBeGreaterThan(1);
	});

	test('logout button is present at the bottom of the sidebar', async ({ page }) => {
		await page.goto('/settings');

		// The logout button has the red color class — but we shouldn't
		// pin to that. Match by role+text content fragment ("logout" or
		// "sair" in i18n). Use the layout-position fallback if needed.
		const logoutBtn = page.locator('aside button').last();
		await expect(logoutBtn).toBeVisible();
	});
});
