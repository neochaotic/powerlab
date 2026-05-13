/**
 * Version handshake — protect against stale-UI-vs-fresh-backend drift.
 *
 * On app boot, fetch /v1/powerlab/version and compare to the version
 * compiled into THIS bundle (__APP_VERSION__, set by Vite from
 * POWERLAB_VERSION env var or ui/package.json). On mismatch, set
 * `mismatch = true` so the root layout can render a banner.
 *
 * Banner UX (post-v0.6.6 user feedback "burrenr horrivel deveria dar
 * para fechar"):
 *  - Dismissible via `dismiss()`.
 *  - Force-reload via `forceReload()` adds a `?powerlab_v=<ts>`
 *    query so cache-aggressive proxies and the browser itself
 *    re-fetch the JS bundle. Counter persisted in localStorage so a
 *    second reload (after still-mismatch) can show a louder
 *    "deploy is broken — run install.sh again" message instead of
 *    re-spinning the same wheel.
 *  - When `persistentFailure` flips true (after ≥2 force-reload
 *    attempts), the banner stops claiming "just reload" and tells
 *    the user the bundle on the server is wrong.
 *
 * Why this exists: the v0.2.4→v0.2.5 release shipped a backend that
 * accepted new JSON keys for the editor (`file_path`/`file_content`)
 * but a tarball-cached UI bundle still sent the old keys
 * (`path`/`content`). Save silently failed. The user thought
 * everything was broken but really only the UI was stale. With this
 * handshake plus Cache-Control: no-cache on index.html, that drift
 * is impossible — either the UI matches, or the user is told to
 * reload.
 *
 * Skips on dev backends ("dev"/empty version) — those constantly
 * differ from any concrete UI version and the warning is just noise.
 */

export const RELOAD_ATTEMPTS_KEY = 'powerlab.versionHandshake.reloadAttempts';
const PERSISTENT_FAILURE_THRESHOLD = 2;

class VersionHandshake {
	uiVersion: string = $state(__APP_VERSION__);
	backendVersion: string | null = $state(null);
	mismatch: boolean = $state(false);
	dismissed: boolean = $state(false);
	checking: boolean = $state(false);
	error: string | null = $state(null);

	get reloadAttempts(): number {
		const raw = typeof localStorage !== 'undefined' ? localStorage.getItem(RELOAD_ATTEMPTS_KEY) : null;
		const n = raw === null ? 0 : Number(raw);
		return Number.isFinite(n) && n >= 0 ? n : 0;
	}

	get persistentFailure(): boolean {
		return this.reloadAttempts >= PERSISTENT_FAILURE_THRESHOLD;
	}

	async check(): Promise<void> {
		this.checking = true;
		this.error = null;
		try {
			const res = await fetch('/v1/powerlab/version');
			if (!res.ok) {
				throw new Error(`HTTP ${res.status}`);
			}
			const data = (await res.json()) as { version: string };
			this.backendVersion = data.version;
			if (data.version === 'dev' || data.version === '') {
				this.mismatch = false;
			} else {
				this.mismatch = data.version !== this.uiVersion;
			}
			if (!this.mismatch && typeof localStorage !== 'undefined') {
				// Versions match — the previous reload(s) succeeded, clear
				// the counter so a future drift starts a fresh attempt
				// budget rather than instantly hitting "persistentFailure".
				localStorage.removeItem(RELOAD_ATTEMPTS_KEY);
			}
		} catch (e) {
			this.error = (e as Error).message;
			this.mismatch = false;
		} finally {
			this.checking = false;
		}
	}

	dismiss(): void {
		this.dismissed = true;
	}

	forceReload(): void {
		if (typeof localStorage !== 'undefined') {
			localStorage.setItem(RELOAD_ATTEMPTS_KEY, String(this.reloadAttempts + 1));
		}
		if (typeof window === 'undefined') return;
		const path = window.location.pathname;
		const existing = window.location.search;
		// Strip a previous powerlab_v= if present so the URL doesn't grow
		// across attempts. Anything else (?install=blinko etc.) is kept.
		const cleaned = existing
			.replace(/[?&]powerlab_v=\d+/g, '')
			.replace(/^&/, '?');
		const sep = cleaned === '' ? '?' : '&';
		const buster = `powerlab_v=${Date.now()}`;
		const next = `${path}${cleaned}${sep}${buster}`;
		window.location.replace(next);
	}
}

export const versionHandshake = new VersionHandshake();
