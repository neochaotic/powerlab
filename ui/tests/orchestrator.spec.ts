import { expect, test } from '@playwright/test';
import { installBaselineMocks } from './helpers/mock-backend';

// /apps/new (YAML-first compose orchestrator) smoke.
//
// Custom-app page is YAML-first — no visual form. User writes
// docker-compose YAML directly; a derived read-only preview panel
// shows summary info. Tests below assert page shell + edit-mode
// routing decision (PUT vs POST). YAML-shape correctness lives in
// the unit suite (YAMLPreview.test.ts).

test.describe('/apps/new orchestrator', () => {
	test.beforeEach(async ({ page }) => {
		await installBaselineMocks(page);
	});

	test('renders the YAML editor + read-only preview', async ({ page }) => {
		await page.goto('/apps/new');

		await expect(page).toHaveTitle(/PowerLab/i);

		// YAML editor textarea + preview panel both render.
		await expect(page.getByTestId('yaml-editor')).toBeVisible();
		await expect(page.getByTestId('yaml-preview')).toBeVisible();
	});

	// Regression for #65 — re-deploying an existing Custom App via
	// the install (POST) endpoint flagged the app's own running ports
	// as conflicts because POST has no skip-self logic. Edit-mode
	// (URL has ?id=X) routes to applyComposeAppSettings (PUT), which
	// DOES skip-self. The routing decision is in handleDeploy().
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

	// Deploy button is disabled when the YAML's resolved project
	// name fails validation (empty / invalid chars). This replaces
	// the form-input validation specs that lived under #48 — the
	// validation itself is unchanged, only the source is now the
	// YAML parse instead of an input element.
	test('Deploy button disabled while YAML has no valid project name', async ({ page }) => {
		await page.goto('/apps/new');
		const editor = page.getByTestId('yaml-editor');
		const deploy = page.getByTestId('deploy-button');

		// Replace the seeded YAML with one that has no top-level name
		// AND no services key — both name sources empty.
		await editor.fill('# empty compose');
		await expect(deploy).toBeDisabled();

		// Invalid chars (uppercase, space) in service key.
		await editor.fill('services:\n  "My App":\n    image: nginx\n');
		await expect(deploy).toBeDisabled();

		// Valid kebab-case service key — button enables.
		await editor.fill('services:\n  my-app:\n    image: nginx\n');
		await expect(deploy).toBeEnabled();
	});
});
