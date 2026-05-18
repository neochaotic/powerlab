import { test, expect, type Route } from '@playwright/test';
import { installBaselineMocks } from './helpers/mock-backend';

// Settings → Power pane (#260). Verifies the operator can:
//   1. See the live list of PowerLab systemd units with status badges
//   2. Restart a single service (button → confirm-less, polls list refreshes)
//   3. Open the Reboot host modal, type confirmation, fire
//   4. Open the Shutdown host modal, type confirmation, fire
//   5. NOT trigger reboot/shutdown by clicking the button alone (modal
//      must intercept) — security regression catcher

test.describe('Settings → Power pane', () => {
	test.beforeEach(async ({ page }) => {
		await installBaselineMocks(page);

		// Default services-list mock — 6 PowerLab units, varied states.
		await page.route('**/v1/sys/services', (route: Route) =>
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({
					data: [
						{
							name: 'powerlab-gateway',
							active_state: 'active',
							sub_state: 'running',
							pid: '1001'
						},
						{
							name: 'powerlab-app-management',
							active_state: 'active',
							sub_state: 'running',
							pid: '1002'
						},
						{ name: 'powerlab-core', active_state: 'inactive', sub_state: 'dead' },
						{
							name: 'powerlab-user-service',
							active_state: 'active',
							sub_state: 'running',
							pid: '1004'
						},
						{
							name: 'powerlab-local-storage',
							active_state: 'failed',
							sub_state: 'failed'
						},
						{
							name: 'powerlab-message-bus',
							active_state: 'active',
							sub_state: 'running',
							pid: '1006'
						}
					]
				})
			})
		);
	});

	test('renders all PowerLab services with status badges', async ({ page }) => {
		await page.goto('/settings#power');
		await expect(page.locator('[data-testid="power-pane"]')).toBeVisible();
		await expect(page.locator('[data-testid="service-row-powerlab-gateway"]')).toBeVisible();
		await expect(page.locator('[data-testid="service-row-powerlab-core"]')).toBeVisible();
		await expect(page.locator('[data-testid="service-row-powerlab-local-storage"]')).toBeVisible();

		// Active badge for running services
		await expect(
			page
				.locator('[data-testid="service-row-powerlab-gateway"]')
				.locator('[data-testid="service-state-badge"]')
		).toContainText(/active/i);
		// Failed badge for failed services
		await expect(
			page
				.locator('[data-testid="service-row-powerlab-local-storage"]')
				.locator('[data-testid="service-state-badge"]')
		).toContainText(/failed/i);
	});

	test('restart button posts to /v1/sys/services/{name}/restart', async ({ page }) => {
		let restartCalled = false;
		await page.route('**/v1/sys/services/powerlab-app-management/restart', (route: Route) => {
			restartCalled = true;
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({ data: 'powerlab-app-management' })
			});
		});

		await page.goto('/settings#power');
		await page
			.locator('[data-testid="service-row-powerlab-app-management"]')
			.locator('[data-testid="service-restart-btn"]')
			.click();

		await expect.poll(() => restartCalled, { timeout: 3000 }).toBe(true);
	});

	test('reboot host requires modal confirmation', async ({ page }) => {
		let rebootCalled = false;
		await page.route('**/v1/sys/host/reboot', (route: Route) => {
			rebootCalled = true;
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({ data: 'host reboot initiated' })
			});
		});

		await page.goto('/settings#power');

		// Click reboot button → modal opens
		await page.locator('[data-testid="host-reboot-btn"]').click();
		await expect(page.locator('[data-testid="host-reboot-modal"]')).toBeVisible();

		// Confirm button is disabled until acknowledgement checked
		const confirmBtn = page.locator('[data-testid="host-reboot-confirm"]');
		await expect(confirmBtn).toBeDisabled();

		// Backend not called before confirmation
		await page.waitForTimeout(200);
		expect(rebootCalled).toBe(false);

		// Tick acknowledgement, confirm enabled
		await page.locator('[data-testid="host-reboot-ack"]').check();
		await expect(confirmBtn).toBeEnabled();

		await confirmBtn.click();
		await expect.poll(() => rebootCalled, { timeout: 3000 }).toBe(true);
	});

	test('shutdown host requires modal confirmation', async ({ page }) => {
		let shutdownCalled = false;
		await page.route('**/v1/sys/host/shutdown', (route: Route) => {
			shutdownCalled = true;
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({ data: 'host shutdown initiated' })
			});
		});

		await page.goto('/settings#power');

		await page.locator('[data-testid="host-shutdown-btn"]').click();
		await expect(page.locator('[data-testid="host-shutdown-modal"]')).toBeVisible();

		await page.locator('[data-testid="host-shutdown-ack"]').check();
		await page.locator('[data-testid="host-shutdown-confirm"]').click();
		await expect.poll(() => shutdownCalled, { timeout: 3000 }).toBe(true);
	});
});
