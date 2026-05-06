#!/usr/bin/env node
/**
 * Captures a 1280×640 PNG suitable for the GitHub social preview / Open Graph
 * image. Uses the same auth strategy as capture-screenshots.mjs (PWLAB_JWT).
 *
 * Renders the launchpad — it is the densest "what is this product" view.
 *
 * Output: docs/img/social-preview.png
 */

import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const projectRoot = path.resolve(__dirname, '..');

import { createRequire } from 'node:module';
const require = createRequire(import.meta.url);
const { chromium } = require(path.resolve(projectRoot, 'ui', 'node_modules', 'playwright-core'));

const baseURL = process.env.PWLAB_URL || 'http://localhost:5173';
const jwt = process.env.PWLAB_JWT || '';
const out = path.join(projectRoot, 'docs', 'img', 'social-preview.png');

(async () => {
	const browser = await chromium.launch();
	const ctx = await browser.newContext({
		viewport: { width: 1280, height: 640 },
		deviceScaleFactor: 2, // retina
	});
	const page = await ctx.newPage();

	if (jwt) {
		await ctx.addInitScript((token) => {
			try { window.localStorage.setItem('powerlab_token', token); } catch {}
		}, jwt);
	}

	await page.goto(baseURL + '/', { waitUntil: 'networkidle' });
	await page.waitForTimeout(1000);
	await page.screenshot({ path: out, fullPage: false });
	await browser.close();
	console.log('✓ wrote', out);
})().catch((e) => { console.error(e); process.exit(1); });
