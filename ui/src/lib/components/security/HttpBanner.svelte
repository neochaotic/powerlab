<script lang="ts">
	import { slide } from 'svelte/transition';
	import { AlertTriangle, ShieldCheck, ChevronRight } from 'lucide-svelte';
	import { onMount } from 'svelte';

	let isVisible = $state(false);

	onMount(() => {
		// Only show if the current connection is unencrypted (HTTP)
		if (window.location.protocol === 'http:') {
			isVisible = true;
		}
	});

	function goToSecuritySettings() {
		// We'll assume the security settings route is /settings/security
		window.location.href = '/settings/security';
	}
</script>

{#if isVisible}
	<div
		class="fixed top-0 left-0 right-0 z-[200] border-b border-amber-500/30 bg-amber-500/90 px-6 py-2.5 text-zinc-950 shadow-lg backdrop-blur-xl"
		transition:slide={{ axis: 'y' }}
	>
		<div class="mx-auto flex max-w-7xl items-center justify-between gap-4">
			<div class="flex items-center gap-3">
				<div class="flex h-8 w-8 items-center justify-center rounded-lg bg-zinc-950/10">
					<AlertTriangle class="h-4.5 w-4.5 text-zinc-950" />
				</div>
				<div class="text-sm font-semibold leading-tight">
					<div>Connection unencrypted</div>
					<div class="text-[11px] font-medium opacity-70">
						Local traffic is visible to your network. Enable secure HTTPS to protect your data.
					</div>
				</div>
			</div>
			
			<button
				class="group flex items-center gap-2 rounded-xl bg-zinc-950 px-4 py-2 text-xs font-bold text-amber-300 hover:bg-zinc-800 transition-all active:scale-95"
				onclick={goToSecuritySettings}
			>
				<ShieldCheck class="h-3.5 w-3.5" />
				Enable Secure Connection
				<ChevronRight class="h-3 w-3 transition-transform group-hover:translate-x-0.5" />
			</button>
		</div>
	</div>
{/if}

<style>
	/* Ensure space for the banner if needed, but since it's fixed, 
	   we might need to push the layout down in the parent. */
</style>
