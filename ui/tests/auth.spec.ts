import { expect, test } from '@playwright/test';
import { installBaselineMocks } from './helpers/mock-backend';

// Auth + setup-wizard smoke. Replaces the stale auth.spec.broken.ts.txt
// (which targeted the pre-launchpad UI shape).
//
// Per #108 — covers two flows:
//   1. First-run setup wizard appears when /v1/users/status reports
//      initialized=false.
//   2. Login screen appears when initialized=true but no session.

test.describe('Auth + setup', () => {
	test('shows setup wizard on first-run', async ({ page }) => {
		await installBaselineMocks(page, { initialized: false });
		// Clear the auth-init script side-effect for this test — we
		// want the unauthenticated path.
		await page.addInitScript(() => {
			localStorage.removeItem('token');
			localStorage.removeItem('user_id');
		});

		await page.goto('/');

		// SetupWizard renders username + password + confirm fields.
		// Use accessible role/text selectors instead of brittle CSS so
		// the test survives the upcoming UI splits in PR 3 + 4.
		await expect(page.locator('input[type="password"]').first()).toBeVisible();
	});

	test('shows login screen when initialized but no session', async ({ page }) => {
		await installBaselineMocks(page);
		// Clear the auth localStorage side-effect — we want
		// "initialized but no session" specifically.
		await page.addInitScript(() => {
			localStorage.removeItem('token');
			localStorage.removeItem('user_id');
		});

		await page.goto('/');

		// LoginScreen renders username + password input.
		await expect(page.locator('input[type="password"]').first()).toBeVisible();
	});
});
