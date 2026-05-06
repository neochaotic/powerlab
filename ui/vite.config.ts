import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

// Read the version from package.json once at config-eval time so the
// LoginScreen footer (and anywhere else that wants it) can show a
// build-stamped version without anyone having to manually edit a
// hardcoded string before each release.
const pkg = JSON.parse(
	readFileSync(fileURLToPath(new URL('./package.json', import.meta.url)), 'utf8')
);
const APP_VERSION: string = pkg.version || 'dev';

export default defineConfig({
	plugins: [tailwindcss(), sveltekit()],
	define: {
		// `__APP_VERSION__` is replaced at build time. Reference it from
		// any .svelte / .ts file as a literal — Vite handles the
		// substitution. Wrapped in JSON.stringify so the value lands
		// as a quoted string in the emitted JS rather than an identifier.
		__APP_VERSION__: JSON.stringify(APP_VERSION)
	},
	server: {
		allowedHosts: ['powerlab.test'],
		proxy: {
			// Proxy CasaOS API routes to the Go backend during development
			'/v1': {
				target: 'http://127.0.0.1:80',
				changeOrigin: true,
				ws: true
			},
			'/v2': {
				target: 'http://127.0.0.1:80',
				changeOrigin: true
			}
		}
	}
});
