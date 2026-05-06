#!/usr/bin/env node
/**
 * Captures README screenshots from a running PowerLab dev instance.
 *
 * Pre-requisites:
 *   - backend running (./start.sh)
 *   - frontend dev server running (cd ui && npm run dev)
 *   - PowerLab user account already created (login screen will be skipped
 *     by injecting a JWT into localStorage if PWLAB_JWT env var is set)
 *
 * Output: docs/img/{launchpad,dashboard,files,apps,about}.png
 */

// Resolved via playwright-core from the ui/ workspace where it's installed
// as a transitive dep of @playwright/test.
import { createRequire } from 'node:module';
const require = createRequire(import.meta.url);
const { chromium } = require(path.resolve(
	path.dirname(fileURLToPath(import.meta.url)),
	'..', 'ui', 'node_modules', 'playwright-core'
));
import path from 'node:path';
import { fileURLToPath } from 'node:url';
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const projectRoot = path.resolve(__dirname, '..');
const outDir = path.join(projectRoot, 'docs', 'img');
const baseURL = process.env.PWLAB_URL || 'http://localhost:5173';
const username = process.env.PWLAB_USER || 'neochaotic';
const password = process.env.PWLAB_PASS || '';
const jwt = process.env.PWLAB_JWT || '';

const screens = [
	{ path: '/',          file: 'launchpad.png',  wait: 800 },
	{ path: '/dashboard', file: 'dashboard.png',  wait: 1500 }, // wait for telemetry
	{ path: '/files',     file: 'files.png',      wait: 1200 },
	{ path: '/apps',      file: 'apps.png',       wait: 1000 },
	{ path: '/settings',  file: 'about.png',      wait: 600, click: 'text=About' },
];

(async () => {
	const browser = await chromium.launch();
	const ctx = await browser.newContext({
		viewport: { width: 1440, height: 900 },
		deviceScaleFactor: 2, // retina-quality screenshots
	});
	const page = await ctx.newPage();

	// If a JWT is provided, inject it before navigation so the auth guard
	// in +layout.svelte sees an authenticated session immediately.
	if (jwt) {
		await ctx.addInitScript((token) => {
			try {
				window.localStorage.setItem('powerlab_token', token);
			} catch {}
		}, jwt);
	}

	console.log(`→ ${baseURL}`);
	await page.goto(baseURL, { waitUntil: 'networkidle' });

	// Always capture the login screen first — it's part of the product story.
	{
		const loginVisible = await page
			.locator('input[type="password"]')
			.first()
			.isVisible()
			.catch(() => false);
		if (loginVisible) {
			console.log(`→ /login → login.png`);
			await page.screenshot({ path: path.join(outDir, 'login.png'), fullPage: false });
		}
	}

	// Login if the LoginScreen is showing
	const loginVisible = await page.locator('input[type="password"]').first().isVisible().catch(() => false);
	if (loginVisible) {
		if (!password) {
			console.warn('PWLAB_PASS not set — captured login.png only. Pass PWLAB_PASS env to capture authenticated screens.');
			await browser.close();
			return;
		}
		console.log(`→ logging in as ${username}`);
		await page.locator('input[type="text"], input[type="email"]').first().fill(username);
		await page.locator('input[type="password"]').first().fill(password);
		await page.keyboard.press('Enter');
		await page.waitForLoadState('networkidle');
		await page.waitForTimeout(800);
	}

	for (const s of screens) {
		console.log(`→ ${s.path} → ${s.file}`);
		await page.goto(baseURL + s.path, { waitUntil: 'networkidle' });
		await page.waitForTimeout(s.wait);
		if (s.click) {
			await page.locator(s.click).first().click();
			await page.waitForTimeout(500);
		}
		await page.screenshot({ path: path.join(outDir, s.file), fullPage: false });
	}

	await browser.close();
	console.log(`✓ wrote ${screens.length} screenshots to ${outDir}`);
})().catch((e) => {
	console.error(e);
	process.exit(1);
});
