import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [tailwindcss(), sveltekit()],
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
