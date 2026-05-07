<script lang="ts">
	import { fly } from 'svelte/transition';
	import { ShieldOff, X } from 'lucide-svelte';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';

	// Soft, dismissible HTTP-status pill. Shows in the top-right
	// corner only when the connection is unencrypted. Designed to
	// inform without interrupting — users who deliberately stay on
	// HTTP (homelab, dev, behind a reverse proxy) can dismiss and
	// the pill stays gone for the session.
	//
	// Click the pill body → /settings#security walkthrough.
	// Click the × → dismiss for this session (sessionStorage).

	const DISMISS_KEY = 'powerlab_http_banner_dismissed';

	let isVisible = $state(false);

	onMount(() => {
		if (window.location.protocol !== 'http:') return;
		if (sessionStorage.getItem(DISMISS_KEY) === '1') return;
		isVisible = true;
	});

	async function goToSecuritySettings() {
		await goto('/settings#security');
	}

	function dismiss(e: MouseEvent) {
		e.stopPropagation();
		sessionStorage.setItem(DISMISS_KEY, '1');
		isVisible = false;
	}
</script>

{#if isVisible}
	<div
		class="fixed top-3 right-3 z-[200] flex items-center gap-1 rounded-full border border-amber-500/20 bg-amber-500/[0.08] pl-3 pr-1 py-1 text-[11px] font-medium text-amber-300 backdrop-blur-md transition-colors hover:border-amber-500/40 hover:bg-amber-500/[0.12]"
		transition:fly={{ y: -8, duration: 250 }}
	>
		<button
			type="button"
			class="group flex items-center gap-2"
			onclick={goToSecuritySettings}
			title="Connection is unencrypted — click to enable HTTPS"
		>
			<ShieldOff class="h-3.5 w-3.5" />
			<span>HTTP</span>
			<span class="hidden text-zinc-400 group-hover:inline">· Enable HTTPS</span>
		</button>
		<button
			type="button"
			class="ml-1 inline-flex h-5 w-5 items-center justify-center rounded-full text-zinc-400 hover:bg-amber-500/20 hover:text-amber-200"
			onclick={dismiss}
			title="Dismiss for this session"
			aria-label="Dismiss"
		>
			<X class="h-3 w-3" />
		</button>
	</div>
{/if}
