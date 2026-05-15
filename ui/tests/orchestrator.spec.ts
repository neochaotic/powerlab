import { expect, test } from '@playwright/test';
import { installBaselineMocks } from './helpers/mock-backend';

// /apps/new compose orchestrator smoke (one-way restored form).
//
// Custom-app page renders the form + YAML editor side-by-side
// (Split mode default). The form is a READ-ONLY view derived from
// the YAML via $derived(viewFromYaml(yaml)); every edit emits a new
// YAML via onChange. No bidirectional binding, no ComposeModel.
// YAML-shape correctness (long-form preservation, etc.) is locked
// in compose-mutate.test.ts.

test.describe('/apps/new orchestrator', () => {
	test.beforeEach(async ({ page }) => {
		await installBaselineMocks(page);
	});

	test('renders the YAML editor + form panel', async ({ page }) => {
		await page.goto('/apps/new');

		await expect(page.getByTestId('yaml-editor')).toBeVisible();
		// Form panel: the service-name input is the most stable anchor.
		await expect(page.locator('#service-name')).toBeVisible();
	});

	// Regression for #65 — re-deploying an existing Custom App via
	// the install (POST) endpoint flagged the app's own running ports
	// as conflicts because POST has no skip-self logic. Edit-mode
	// (URL has ?id=X) routes to applyComposeAppSettings (PUT), which
	// DOES skip-self.
	test('edit-mode hits PUT /v2/app_management/compose/{id}, not POST (#65)', async ({ page }) => {
		await page.route('**/v2/app_management/compose/my-nginx', async (route) => {
			const req = route.request();
			if (req.method() === 'GET') {
				return route.fulfill({
					status: 200,
					contentType: 'application/yaml',
					body: 'name: my-nginx\nservices:\n  web:\n    image: nginx:latest\n    ports:\n      - "8080:80"\n'
				});
			}
			if (req.method() === 'PUT') {
				return route.fulfill({
					status: 200,
					contentType: 'application/json',
					body: JSON.stringify({ message: 'ok' })
				});
			}
			return route.continue();
		});

		let postHits = 0;
		await page.route('**/v2/app_management/compose', (route) => {
			if (route.request().method() === 'POST') postHits++;
			return route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({ message: 'ok' })
			});
		});

		let putHit = false;
		page.on('request', (req) => {
			if (req.method() === 'PUT' && req.url().includes('/v2/app_management/compose/my-nginx')) {
				putHit = true;
			}
		});

		await page.goto('/apps/new?id=my-nginx');
		await page.addStyleTag({ content: '.fixed.top-3.right-3 { display: none !important; }' });

		const deployBtn = page.getByTestId('deploy-button');
		await expect(deployBtn).toBeEnabled({ timeout: 5000 });
		await deployBtn.click();

		await expect.poll(() => putHit, { timeout: 5000 }).toBe(true);
		expect(postHits).toBe(0);
	});

	// Deploy button is disabled when the YAML's resolved project name
	// fails validation. Form input #service-name binds to the project
	// (services map's first key); we exercise via the form.
	test('Deploy button disabled while service name fails validation', async ({ page }) => {
		await page.goto('/apps/new');
		const nameInput = page.locator('#service-name');
		const deploy = page.getByTestId('deploy-button');

		// Clear → empty validation error.
		await nameInput.fill('');
		await expect(deploy).toBeDisabled();

		// Uppercase + space → invalid_chars.
		await nameInput.fill('My App');
		await expect(deploy).toBeDisabled();

		// Valid kebab-case → enabled.
		await nameInput.fill('my-app');
		await expect(deploy).toBeEnabled();
	});
});
