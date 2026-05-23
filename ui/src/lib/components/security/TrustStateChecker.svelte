<script lang="ts">
	/**
	 * Detects CA-mismatch states early — before the user hits a
	 * cert-error wall — and surfaces a banner offering to re-run the
	 * trust dance.
	 *
	 * Trigger conditions:
	 *
	 *   1. We have a previously-trusted CA fingerprint stored in
	 *      localStorage (`powerlab_trusted_ca_fp`).
	 *   2. The server's current CA fingerprint differs from the
	 *      stored one (CA was regenerated or rotated since the
	 *      user's last successful trust dance).
	 *
	 * In that case the user already trusted SOMETHING, and the
	 * server has moved on. Without this banner the next HTTPS visit
	 * lands on Chrome's red wall and looks like the panel is
	 * broken. With it, the user gets a clear path back into the
	 * walkthrough.
	 *
	 * The component is silent on the happy paths:
	 *   - First-ever visit (no stored fingerprint) — nothing to warn
	 *     about until a fingerprint has been pinned.
	 *   - Match (server CA == stored fingerprint) — nothing to do.
	 *   - Server cannot return a fingerprint (CA not generated yet,
	 *     network error) — fail open, don't pester the user.
	 */
	import { onMount } from 'svelte';
	import { fly } from 'svelte/transition';
	import { ShieldAlert, X } from 'lucide-svelte';
	import { goto } from '$app/navigation';

	const STORAGE_KEY_FP = 'powerlab_trusted_ca_fp';
	const DISMISS_KEY = 'powerlab_ca_mismatch_dismissed';

	let mismatch = $state(false);
	let serverFingerprint = $state<string | null>(null);

	onMount(async () => {
		const storedFP = readStorage(STORAGE_KEY_FP);
		if (!storedFP) return; // user has never completed the dance; not our case
		if (readStorage(DISMISS_KEY) === '1') return;

		try {
			const r = await fetch('/v1/sys/trust-state', {
				signal: AbortSignal.timeout(3000)
			});
			if (!r.ok) return;
			const j = (await r.json()) as { ca_fingerprint?: string };
			serverFingerprint = j.ca_fingerprint || null;
			if (!serverFingerprint) return;
			if (serverFingerprint !== storedFP) {
				mismatch = true;
			}
		} catch {
			// Network blip / 5xx — silent.
		}
	});

	function goToWalkthrough() {
		void goto('/settings#security');
	}

	function dismiss(e: MouseEvent) {
		e.stopPropagation();
		writeStorage(DISMISS_KEY, '1');
		mismatch = false;
	}

	function readStorage(key: string): string | null {
		try {
			return window.localStorage.getItem(key);
		} catch {
			return null;
		}
	}

	function writeStorage(key: string, value: string): void {
		try {
			window.localStorage.setItem(key, value);
		} catch {
			/* sessionStorage may be unavailable; surface is degrade-friendly */
		}
	}
</script>

{#if mismatch}
	<div
		class="fixed top-3 left-1/2 z-[200] flex -translate-x-1/2 items-center gap-2 rounded-full border border-amber-500/30 bg-amber-500/[0.10] pl-3 pr-1 py-1.5 text-[12px] font-medium text-amber-200 backdrop-blur-md shadow-lg"
		transition:fly={{ y: -8, duration: 250 }}
	>
		<button
			type="button"
			class="group flex items-center gap-2"
			onclick={goToWalkthrough}
			title="The CA on this server changed — re-install the new one"
		>
			<ShieldAlert class="h-3.5 w-3.5" />
			<span>Trust changed — re-install CA</span>
		</button>
		<button
			type="button"
			class="ml-1 inline-flex h-5 w-5 items-center justify-center rounded-full text-amber-400/70 hover:bg-amber-500/20 hover:text-amber-100"
			onclick={dismiss}
			aria-label="Dismiss"
			title="Dismiss for this browser (you can re-install later via Settings → Security)"
		>
			<X class="h-3 w-3" />
		</button>
	</div>
{/if}
