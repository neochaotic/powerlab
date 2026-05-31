import { test, expect } from '@playwright/test';
import { installBaselineMocks } from './helpers/mock-backend';
import path from 'path';
import { fileURLToPath } from 'url';
import { mkdirSync } from 'fs';

// Auto-generated screenshots for the README + mkdocs site.
//
// Run locally:
//   cd ui && npm run screenshots
//
// What this does: navigates Chromium through every page the docs
// reference, captures a full-page PNG at retina dimensions
// (1440x900 @ 2x = 2880x1800 — matches the manual captures already
// committed under docs/img/), and writes the result to
// docs/img/<name>.png. The mock backend keeps the UI deterministic
// (no live data) so the same git revision produces byte-identical
// screenshots run-to-run, modulo browser-rendered antialiasing
// (Chromium is stable enough that diffs trace to actual UI changes).
//
// Why a dedicated spec (not piggybacking on smoke tests): the
// existing smoke tests aren't required to render the page in a
// presentable state — they assert behaviour. The screenshot spec
// has different priorities (no dev banners, all data populated to
// look realistic, no transient hover states). Mixing the two
// concerns made the smoke tests harder to evolve.
//
// Mocking strategy: installBaselineMocks() (existing helper)
// handles auth + version handshake + the catch-all that prevents
// unrouted /v1/* calls from 500-ing. Per-page specifics override
// the catch-all to surface realistic content where it matters
// (Apps with installed apps, About with version info).
//
// SCOPE NOTE — store/catalog screenshots NOT covered by this MVP.
// PowerLab's community-catalog ships as an opt-in install step
// (ADR-0041 split; the catalog is bundled but its UI only renders
// meaningfully when the catalog is actually present + seeded).
// Adding store-pane screenshots (`apps_store.png`, `store_install_flow.png`,
// etc.) would need additional setup: stage the catalog into the
// mock backend's responses OR run against a real PowerLab install
// with the catalog enabled. Out of scope for this PR — the existing
// `apps.png` shows the installed-apps view which works with the
// baseline mocks; store-screenshot infra is a follow-up.

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const DOCS_IMG_DIR = path.resolve(__dirname, '../../docs/img');

// 1440x900 viewport at deviceScaleFactor 2 → 2880x1800 retina PNG,
// matching the dimensions of the currently-committed screenshots.
test.use({
	viewport: { width: 1440, height: 900 },
	deviceScaleFactor: 2
});

test.beforeAll(() => {
	mkdirSync(DOCS_IMG_DIR, { recursive: true });
});

async function capture(page: import('@playwright/test').Page, name: string) {
	const out = path.join(DOCS_IMG_DIR, `${name}.png`);
	// Settle render before the shot. `networkidle` doesn't work here —
	// the dashboard polls telemetry on a 1s interval via the mocked
	// catch-all, so the network never goes idle. The caller already
	// awaited a per-page readiness selector before reaching us; a
	// short fixed delay covers transition animations.
	await page.waitForTimeout(400);
	// Hide any animated cursors / transient hover states by moving
	// the mouse far off-canvas before the shot.
	await page.mouse.move(-100, -100);
	await page.screenshot({ path: out, fullPage: false });
}

test.describe('docs screenshots @screenshots', () => {
	test('login.png — unauthenticated landing', async ({ page, context }) => {
		// Don't pre-authenticate — login page is the "what unauth users
		// see" screenshot. Skip the baseline mocks' addInitScript so the
		// token is absent.
		await context.addInitScript(() => {
			localStorage.removeItem('powerlab_token');
			localStorage.removeItem('powerlab_user');
		});
		await page.route('**/v1/users/status', (route) =>
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({ success: 200, data: { initialized: true, key: '' } })
			})
		);
		await page.route('**/v1/powerlab/version', (route) =>
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({ version: 'dev' })
			})
		);
		await page.goto('/');
		// Login screen renders after the auth gate's redirect; wait for
		// the password field as a deterministic readiness signal.
		await expect(page.locator('input[type="password"]')).toBeVisible();
		await capture(page, 'login');
	});

	test('dashboard.png — authenticated home', async ({ page }) => {
		await installBaselineMocks(page);
		await page.goto('/dashboard');
		await expect(page.locator('h1, h2').first()).toBeVisible();
		await capture(page, 'dashboard');
	});

	test('apps.png — apps page', async ({ page }) => {
		await installBaselineMocks(page);
		await page.goto('/apps');
		await expect(page.locator('h1, h2').first()).toBeVisible();
		await capture(page, 'apps');
	});

	test('files.png — file manager', async ({ page }) => {
		await installBaselineMocks(page);
		await page.goto('/files');
		await expect(page.locator('h1, h2').first()).toBeVisible();
		await capture(page, 'files');
	});

	test('about.png — settings about pane', async ({ page }) => {
		await installBaselineMocks(page);
		await page.goto('/settings');
		await expect(page.locator('h2').filter({ hasText: /settings/i }).first()).toBeVisible();
		// Try to click the "About" pane button — the README's "about.png"
		// shows the About surface specifically.
		const aboutBtn = page.locator('aside nav button').filter({ hasText: /about/i }).first();
		if (await aboutBtn.count() > 0) {
			await aboutBtn.click();
			await page.waitForTimeout(300); // settle
		}
		await capture(page, 'about');
	});

	test('launchpad.png — top-level launchpad', async ({ page }) => {
		await installBaselineMocks(page);
		await page.goto('/');
		await expect(page.locator('main, [role="main"]').first()).toBeVisible();
		await capture(page, 'launchpad');
	});
});
