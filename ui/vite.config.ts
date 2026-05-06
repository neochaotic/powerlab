import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

// `__APP_VERSION__` resolution order, highest priority first:
//   1. POWERLAB_VERSION env var — set by scripts/package-linux.sh and CI
//      from the git tag, so the JS bundle ALWAYS matches the released
//      version. Without this, a tag-time release would have to bump
//      ui/package.json by hand and we'd have v0.2.5 tarballs shipping
//      v0.2.0 in their bundle (which is exactly what happened in the
//      first attempt at v0.2.5).
//   2. ui/package.json `version` — fallback for `npm run build` during
//      day-to-day dev, when no tag is in flight.
//   3. literal "dev" — last resort.
const pkg = JSON.parse(
	readFileSync(fileURLToPath(new URL('./package.json', import.meta.url)), 'utf8')
);
const APP_VERSION: string = process.env.POWERLAB_VERSION || pkg.version || 'dev';

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
