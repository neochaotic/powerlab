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
		setupFiles: ['./src/lib/__mocks__/dom-polyfills.ts']
	}
});
