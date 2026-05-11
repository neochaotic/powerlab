import { defineConfig } from 'vitest/config';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import { resolve } from 'path';

export default defineConfig({
	plugins: [svelte()],
	resolve: {
		conditions: ['browser'],
		alias: {
			$lib: resolve('./src/lib'),
			'$app/stores': resolve('./src/lib/__mocks__/app-stores.ts'),
			'$app/navigation': resolve('./src/lib/__mocks__/app-navigation.ts'),
			'$app/environment': resolve('./src/lib/__mocks__/app-environment.ts')
		}
	},
	test: {
		include: ['src/**/*.{test,spec}.{js,ts}'],
		environment: 'jsdom',
		globals: true,
		setupFiles: ['./src/lib/__mocks__/dom-polyfills.ts'],
		coverage: {
			// Per the Sprint 8 v0.6 audit: v0.6 readiness needs a
			// measured frontend coverage baseline, not a guess. v8
			// provider is fast and accurate enough; text + html +
			// json-summary so CI can diff and developers can drill
			// into uncovered lines locally.
			provider: 'v8',
			reporter: ['text', 'html', 'json-summary'],
			include: ['src/**/*.{ts,svelte}'],
			exclude: [
				'src/**/*.{test,spec}.{js,ts}',
				'src/**/__mocks__/**',
				'src/lib/i18n/locales/**',
				// SvelteKit-generated runtime — not our code.
				'.svelte-kit/**'
			],
			// No threshold gates on Sprint 9. The first run
			// establishes the baseline; gates land once we know
			// what passing-now looks like (post-Sprint-9 decision).
			thresholds: undefined
		}
	}
});
