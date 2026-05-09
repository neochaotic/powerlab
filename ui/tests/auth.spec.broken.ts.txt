import { test, expect } from '@playwright/test';

test.describe('Auth & Setup Flow', () => {
	test('should show Setup Wizard if system is not initialized', async ({ page }) => {
		// Mock initialization status to false
		await page.route('**/v1/users/status', async (route) => {
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({
					success: 200,
					data: { initialized: false, key: 'test_setup_key' }
				})
			});
		});

		await page.goto('/');

		// Should see the Welcome screen
		await expect(page.locator('h1')).toContainText('Bem-vindo ao PowerLab');

		// Click "Começar Instalação"
		await page.click('button:has-text("Começar Instalação")');

		// Fill registration form
		await page.fill('input#password', 'admin_pass_123');
		await page.fill('input#confirm', 'admin_pass_123');

		// Mock the registration and subsequent login
		await page.route('**/v1/users/register', async (route) => {
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({ success: 200, message: 'ok' })
			});
		});

		await page.route('**/v1/users/login', async (route) => {
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({
					success: 200,
					data: {
						token: { access_token: 'fake_jwt_token' },
						user: { username: 'admin', id: 1 }
					}
				})
			});
		});

		// Submit setup
		await page.click('button:has-text("Concluir Setup")');

		// Should be redirected/authenticated and see the Sidebar
		await expect(page.locator('aside')).toBeVisible();
		await expect(page.locator('span:has-text("PowerLab OS")')).toBeVisible();
	});

	test('should show Login screen if system is initialized but not authenticated', async ({ page }) => {
		// Mock initialization status to true
		await page.route('**/v1/users/status', async (route) => {
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({
					success: 200,
					data: { initialized: true }
				})
			});
		});

		await page.goto('/');

		// Should see the Login screen (Sarah Jenkins placeholder was updated to System Administrator)
		await expect(page.locator('h2')).toContainText('System Administrator');
		await expect(page.locator('input[type="password"]')).toBeVisible();
	});
});
