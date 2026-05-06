/**
 * Version handshake — protect against stale-UI-vs-fresh-backend drift.
 *
 * On app boot, fetch /v1/powerlab/version and compare to the version
 * compiled into THIS bundle (__APP_VERSION__, set by Vite from
 * POWERLAB_VERSION env var or ui/package.json). On mismatch, set
 * `mismatch = true` so the root layout can render a non-dismissible
 * banner instructing the user to reload.
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

class VersionHandshake {
	uiVersion: string = $state(__APP_VERSION__);
	backendVersion: string | null = $state(null);
	mismatch: boolean = $state(false);
	checking: boolean = $state(false);
	error: string | null = $state(null);

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
			// "dev" backend means a developer build with no link-time
			// version stamping. Don't warn — dev iteration would surface
			// the banner constantly.
			if (data.version === 'dev' || data.version === '') {
				this.mismatch = false;
			} else {
				this.mismatch = data.version !== this.uiVersion;
			}
		} catch (e) {
			// Backend unreachable. Don't claim mismatch — could be a
			// transient network blip during the upgrade itself.
			this.error = (e as Error).message;
			this.mismatch = false;
		} finally {
			this.checking = false;
		}
	}
}

export const versionHandshake = new VersionHandshake();
