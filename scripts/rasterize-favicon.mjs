#!/usr/bin/env node
/**
 * Rasterizes ui/static/favicon.svg into the PNG sizes the PWA needs.
 * Output: ui/static/icon-{192,512}.png and a 32×32 favicon.png.
 *
 * Run after editing favicon.svg to keep the rasters in sync.
 */
import path from 'node:path';
import fs from 'node:fs';
import { fileURLToPath } from 'node:url';
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const projectRoot = path.resolve(__dirname, '..');

import { createRequire } from 'node:module';
const require = createRequire(import.meta.url);
const { chromium } = require(path.resolve(projectRoot, 'ui', 'node_modules', 'playwright-core'));

const svgPath = path.join(projectRoot, 'ui', 'static', 'favicon.svg');
const svg = fs.readFileSync(svgPath, 'utf8');
const out = path.join(projectRoot, 'ui', 'static');

const sizes = [
	{ size: 32,  name: 'favicon.png' },
	{ size: 180, name: 'apple-touch-icon.png' },
	{ size: 192, name: 'icon-192.png' },
	{ size: 512, name: 'icon-512.png' },
];

(async () => {
	const browser = await chromium.launch();
	for (const { size, name } of sizes) {
		const ctx = await browser.newContext({
			viewport: { width: size, height: size },
			deviceScaleFactor: 1,
		});
		const page = await ctx.newPage();
		// Inline the SVG and stretch it to the viewport. Page background
		// is transparent so the squircle keeps its real shape.
		const html = `<!doctype html><html><head><style>
			html, body { margin: 0; padding: 0; background: transparent; }
			svg { width: ${size}px; height: ${size}px; display: block; }
		</style></head><body>${svg}</body></html>`;
		await page.setContent(html);
		await page.waitForTimeout(50);
		await page.screenshot({
			path: path.join(out, name),
			omitBackground: true,
			clip: { x: 0, y: 0, width: size, height: size },
		});
		await ctx.close();
		console.log(`✓ ${name} (${size}×${size})`);
	}
	await browser.close();
})().catch((e) => { console.error(e); process.exit(1); });
