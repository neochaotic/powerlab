import { expect, test } from '@playwright/test';
import { installBaselineMocks } from './helpers/mock-backend';

// /files page smoke — verifies the file browser renders even when the
// /v1/file/dirpath endpoint returns an empty payload (catch-all mock).
//
// Per #108 — "list + folder navigation" coverage. The bug-#2 editor
// regression test (TextEditor save) lives in
// ui/src/lib/components/files/TextEditor.test.ts and is enforced
// separately by vitest; this E2E pass is the page-level smoke.

test.describe('/files page', () => {
	test.beforeEach(async ({ page }) => {
		await installBaselineMocks(page);
		// /v1/file/dirpath is the directory-listing endpoint. The
		// catch-all returns {success: 200, data: null}; the page needs
		// at least an empty array to render the empty-state without
		// throwing.
		await page.route('**/v1/file/dirpath**', (route) =>
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({
					success: 200,
					data: { content: [], total: 0, index: 1, size: 100000 }
				})
			})
		);
		await page.route('**/v1/file/get_default_path', (route) =>
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({ success: 200, data: '/' })
			})
		);
	});

	test('renders file browser shell', async ({ page }) => {
		await page.goto('/files');

		// Title contains PowerLab branding.
		await expect(page).toHaveTitle(/PowerLab/i);

		// AppHeader renders the page title bar — check it's present
		// rather than asserting on its exact i18n text.
		const header = page.locator('header').first();
		await expect(header).toBeVisible();
	});
});
