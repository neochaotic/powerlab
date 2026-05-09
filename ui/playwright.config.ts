import { defineConfig, devices } from '@playwright/test';

// PowerLab E2E baseline — see issue #86.
//
// The dev server boots automatically via `webServer` so contributors
// can run `npm run test:e2e` without spinning anything up first.
// CI does the same on Ubuntu. We use Chromium-only for the baseline;
// Firefox / WebKit can be added when there's a concrete need.
//
// Tests are expected to mock API responses at the route level
// (see tests/auth.spec.ts for the pattern) so the baseline is
// independent of backend services running.
export default defineConfig({
	testDir: './tests',
	fullyParallel: true,
	// Disallow .only / .skip-only in shared CI runs.
	forbidOnly: !!process.env.CI,
	// Retry once on CI (flake mitigation), zero retries locally.
	retries: process.env.CI ? 1 : 0,
	// Single worker on CI to keep memory pressure low and logs readable.
	workers: process.env.CI ? 1 : undefined,
	reporter: process.env.CI ? [['github'], ['list']] : 'list',

	use: {
		baseURL: 'http://localhost:5173',
		trace: 'on-first-retry',
		screenshot: 'only-on-failure'
	},

	projects: [
		{
			name: 'chromium',
			use: { ...devices['Desktop Chrome'] }
		}
	],

	webServer: {
		command: 'npm run dev -- --port 5173',
		url: 'http://localhost:5173',
		// Re-use a running dev server when available locally; CI always
		// spins fresh.
		reuseExistingServer: !process.env.CI,
		timeout: 120_000
	}
});
