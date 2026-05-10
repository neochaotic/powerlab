import type { Page } from '@playwright/test';

// Shared backend mocks — extends the smoke baseline so per-area tests
// don't fight the global empty-success mock. Each mock is opt-in;
// pass only what your spec needs.
//
// Pattern: register specific routes BEFORE the catch-all in
// installBaselineMocks. Per-test specifics override here.

/**
 * Mock the auth + system-info baseline so the layout's auth gate
 * resolves to "logged-in user, system initialized" and the route
 * being tested is reached without a real backend.
 *
 * `initialized: false` triggers the SetupWizard instead — pass
 * `{initialized: false}` to verify the first-run flow.
 */
export async function installBaselineMocks(
	page: Page,
	opts: {
		initialized?: boolean;
		username?: string;
	} = {}
) {
	const { initialized = true, username = 'admin' } = opts;

	await page.route('**/v1/users/status', (route) =>
		route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify({
				success: 200,
				data: { initialized, key: '' }
			})
		})
	);

	await page.route('**/v1/users/info', (route) =>
		route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify({
				success: 200,
				data: { id: 1, username, avatar: '' }
			})
		})
	);

	// versionHandshake.check() reads {version} directly (no Result
	// wrapper). Without this mock the catch-all returns the wrapped
	// shape, version becomes undefined, and the layout shows the
	// non-dismissible "UI cached, please reload" banner that
	// intercepts pointer events and breaks click-driven tests.
	// "dev" tells the handshake to treat this as a dev build and
	// not warn (matches the runtime POWERLAB_VERSION sentinel).
	await page.route('**/v1/powerlab/version', (route) =>
		route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify({ version: 'dev' })
		})
	);

	// Pretend the user is authenticated so the layout doesn't redirect to
	// LoginScreen. The auth store reads `powerlab_token` + `powerlab_user`
	// via localStorage before issuing /v1/users/info — set them here so
	// the gate passes. Keys MUST match auth.svelte.ts; using 'token'
	// silently fails (CI caught this on 2026-05-10).
	const userPayload = JSON.stringify({ id: 1, username });
	await page.addInitScript((u) => {
		localStorage.setItem('powerlab_token', 'test-token');
		localStorage.setItem('powerlab_user', u);
	}, userPayload);

	// Catch-all: any other /v1/* call returns success with null data so
	// the page render path doesn't 500-out on missing endpoints. Specific
	// routes added before this one win.
	await page.route('**/v1/**', (route) => {
		const url = route.request().url();
		if (url.includes('/users/status') || url.includes('/users/info')) return;
		void route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify({ success: 200, data: null })
		});
	});

	// Same catch-all for v2 endpoints.
	await page.route('**/v2/**', (route) =>
		route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify({ success: 200, data: null })
		})
	);
}
