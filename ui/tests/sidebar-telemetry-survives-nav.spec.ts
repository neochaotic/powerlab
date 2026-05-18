import { test, expect, Page } from '@playwright/test';
import { installBaselineMocks } from './helpers/mock-backend';

// Regression test for #453.
//
// Before the refcount fix in system.svelte.ts, navigating from a route
// that called `store.startPolling/stopPolling` to a route that did NOT
// call `startPolling` (notably `/settings` and `/files`) silently killed
// the sidebar's telemetry interval. Locking the contract via Playwright
// is a defence-in-depth layer over the vitest unit refcount tests — it
// catches future regressions like "someone added a new route that calls
// stopPolling on the singleton store again."

const SIDEBAR_CPU = '[data-testid="sidebar-cpu-widget"]';

async function readCpuPercent(page: Page): Promise<number | null> {
	const handle = await page.locator(SIDEBAR_CPU).first();
	const attr = await handle.getAttribute('data-cpu-percent');
	return attr === null ? null : parseFloat(attr);
}

async function waitForTelemetryChange(page: Page, baseline: number, maxMs: number): Promise<boolean> {
	const deadline = Date.now() + maxMs;
	while (Date.now() < deadline) {
		const current = await readCpuPercent(page);
		// CPU readings should change tick-to-tick — even at idle the EMA-smoothed
		// percent moves in the 3rd or 4th decimal. If three poll cycles pass with
		// EXACTLY the same value, the interval is dead.
		if (current !== null && current !== baseline) return true;
		await page.waitForTimeout(500);
	}
	return false;
}

test.describe('Sidebar telemetry survives route navigation (#453)', () => {
	test.beforeEach(async ({ page }) => {
		await installBaselineMocks(page);
		await page.route('**/v1/sys/utilization', (route) => {
			// Each call returns a slightly different CPU percent so we can
			// detect interval ticks via attribute changes. Counter is
			// per-test isolated via the route closure.
			counter += 1;
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({
					data: {
						cpu: {
							percent: 30 + (counter % 50),
							num: 4,
							temperature: 50,
							model: 'test-cpu'
						},
						mem: { total: 1000, used: 500, free: 500, usedPercent: 50 },
						net: []
					}
				})
			});
		});
		await page.route('**/v1/sys/disk*', (route) =>
			route.fulfill({ status: 200, contentType: 'application/json', body: '{"data":[]}' })
		);
		await page.route('**/v1/disks*', (route) =>
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: '{"data":{"disks":[]}}'
			})
		);
		counter = 0;
	});

	test('CPU keeps updating after navigating to /settings', async ({ page }) => {
		await page.goto('/');
		await expect(page.locator(SIDEBAR_CPU)).toBeVisible({ timeout: 10_000 });
		const baseline = await readCpuPercent(page);
		expect(baseline).not.toBeNull();

		await page.goto('/settings');
		await expect(page.locator(SIDEBAR_CPU)).toBeVisible({ timeout: 5_000 });

		// After the launchpad's stopPolling on destroy + settings page
		// not calling startPolling, the OLD code path would freeze
		// telemetry here. Refcount fix keeps the sidebar's consumer alive.
		const changed = await waitForTelemetryChange(page, baseline as number, 5_000);
		expect(changed, 'CPU value did NOT change within 5s on /settings — refcount regression').toBe(true);
	});

	test('CPU keeps updating after navigating to /files', async ({ page }) => {
		await page.goto('/');
		await expect(page.locator(SIDEBAR_CPU)).toBeVisible({ timeout: 10_000 });
		const baseline = await readCpuPercent(page);

		await page.goto('/files');
		await expect(page.locator(SIDEBAR_CPU)).toBeVisible({ timeout: 5_000 });

		const changed = await waitForTelemetryChange(page, baseline as number, 5_000);
		expect(changed, 'CPU value did NOT change within 5s on /files — refcount regression').toBe(true);
	});
});

let counter = 0;
