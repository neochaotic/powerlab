// See https://svelte.dev/docs/kit/types#app.d.ts
// for information about these interfaces
declare global {
	namespace App {
		// interface Error {}
		// interface Locals {}
		// interface PageData {}
		// interface PageState {}
		// interface Platform {}
	}

	// Build-time-injected version string from ui/package.json.
	// Defined in vite.config.ts via Vite's `define` option so it lands
	// in the bundle as a literal — no runtime fetch.
	const __APP_VERSION__: string;
}

export {};
