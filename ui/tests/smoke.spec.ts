import { expect, test } from '@playwright/test';

// PowerLab E2E baseline — issue #86.
//
// One smoke test per critical route. The goal of this file is NOT to
// exercise feature flows in depth — it's to prove the Playwright +
// dev-server + chromium pipeline runs end-to-end so that future PRs
// can add real feature tests with confidence the infrastructure works.
//
// Real feature coverage is tracked in the follow-up issue
// (rich tests per ui-feature-map.md area).
//
// All tests mock the user-init status endpoint so the page renders
// past the auth gate without a real backend running.

test.describe('PowerLab E2E baseline', () => {
	test.beforeEach(async ({ page }) => {
		// Pretend the system is fully initialized + the user is logged
		// in. Two mocks together cover the two known checks.
		await page.route('**/v1/users/status', (route) =>
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({
					success: 200,
					data: { initialized: true, key: '' }
				})
			})
		);

		// Some routes call /v1/users/info — return a benign user.
		await page.route('**/v1/users/info', (route) =>
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({
					success: 200,
					data: { id: 1, username: 'admin', avatar: '' }
				})
			})
		);

		// Treat any other backend call as a successful empty payload.
		await page.route('**/v1/**', (route) => {
			if (route.request().url().includes('/users/status')) return;
			if (route.request().url().includes('/users/info')) return;
			void route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({ success: 200, data: null })
			});
		});
	});

	test('home page renders', async ({ page }) => {
		await page.goto('/');
		// The launchpad has an <html> element. If we get to the DOM
		// at all, the dev server + SvelteKit rendered something. The
		// stronger assertions live in the follow-up tests.
		await expect(page.locator('html')).toBeVisible();
		// Title should mention PowerLab.
		await expect(page).toHaveTitle(/PowerLab/i);
	});
});
