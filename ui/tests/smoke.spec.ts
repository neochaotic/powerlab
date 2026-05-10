import { expect, test } from '@playwright/test';
import { installBaselineMocks } from './helpers/mock-backend';

// PowerLab E2E baseline — issue #86.
//
// The launchpad smoke. Per-area smoke tests live in:
//   apps.spec.ts, settings.spec.ts, files.spec.ts,
//   orchestrator.spec.ts, auth.spec.ts
//
// Sprint 7 #108 expanded this from a single home-page-renders
// assertion to per-area coverage. The shared mock baseline lives
// in helpers/mock-backend.ts.

test.describe('PowerLab E2E baseline', () => {
	test.beforeEach(async ({ page }) => {
		await installBaselineMocks(page);
	});

	test('launchpad renders', async ({ page }) => {
		await page.goto('/');
		// The launchpad has an <html> element. If we get to the DOM
		// at all, the dev server + SvelteKit rendered something.
		await expect(page.locator('html')).toBeVisible();
		await expect(page).toHaveTitle(/PowerLab/i);
	});
});
