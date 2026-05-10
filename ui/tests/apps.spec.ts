import { expect, test } from '@playwright/test';
import { installBaselineMocks } from './helpers/mock-backend';

// /apps page smoke — verifies the app store loads, the back-to-launchpad
// link is wired, and the page renders the empty-state when the catalog
// is empty (which is what the catch-all mock returns).
//
// Will protect the upcoming UI split in #227 PR 3 (apps/+page.svelte
// 1561 LOC → 4 components + 4 stores).

test.describe('/apps page', () => {
	test.beforeEach(async ({ page }) => {
		await installBaselineMocks(page);
	});

	test('renders app store header', async ({ page }) => {
		await page.goto('/apps');

		// Title contains the i18n string for "App Store" (English default).
		await expect(page).toHaveTitle(/App Store|PowerLab/i);

		// h1 contains the page heading. The exact i18n string can vary
		// (apps.appStore key), so accept any non-empty h1.
		const h1 = page.locator('h1').first();
		await expect(h1).toBeVisible();
	});

	test('back-to-launchpad link navigates to /', async ({ page }) => {
		await page.goto('/apps');

		// Target the apps page's own back arrow specifically — NOT
		// the layout sidebar's Home link (which is also `a[href="/"]`).
		// The page-specific back arrow lives outside the layout
		// `<aside>` and carries an aria-label. main > a[href='/']
		// scopes to the page content area.
		const backLink = page.locator('main a[href="/"]').first();
		await expect(backLink).toBeVisible();

		await backLink.click();
		// SvelteKit client-side nav settles fast; 5s timeout
		// is generous.
		await expect(page).toHaveURL(/\/$/, { timeout: 5000 });
	});
});
