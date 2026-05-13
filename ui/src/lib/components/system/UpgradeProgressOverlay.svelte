<!--
  Full-screen overlay during an in-flight in-UI upgrade. Hides the
  transient 502/503 errors the user would otherwise see while gateway
  is restarting. See upgradeProgress.svelte.ts for the state machine.
-->
<script lang="ts">
	import { fade } from 'svelte/transition';
	import { upgradeProgress } from '$lib/stores/upgradeProgress.svelte';
	import { t } from '$lib/i18n/index.svelte';
	import { RefreshCw, CheckCircle2, AlertTriangle } from 'lucide-svelte';
</script>

{#if upgradeProgress.isOverlayActive}
	<div
		data-testid="upgrade-progress-overlay"
		class="fixed inset-0 z-[300] flex items-center justify-center bg-zinc-950/95 backdrop-blur-xl"
		transition:fade={{ duration: 200 }}
	>
		<div class="mx-6 w-full max-w-md rounded-3xl border border-white/[0.08] bg-zinc-900/90 p-8 shadow-2xl">
			{#if upgradeProgress.state === 'starting' || upgradeProgress.state === 'restarting'}
				<div class="flex flex-col items-center text-center">
					<div class="mb-4 flex h-16 w-16 items-center justify-center rounded-2xl bg-emerald-500/[0.1]">
						<RefreshCw class="h-7 w-7 animate-spin text-emerald-400" strokeWidth={2} />
					</div>
					<h2 class="text-lg font-bold text-white">
						{t('upgrade.inProgress.title')}
					</h2>
					{#if upgradeProgress.targetVersion}
						<p class="mt-1 text-sm text-zinc-400">
							v{upgradeProgress.targetVersion}
						</p>
					{/if}
					<p class="mt-4 text-[13px] leading-relaxed text-zinc-400">
						{t('upgrade.inProgress.help')}
					</p>
					<p class="mt-3 text-[11px] font-medium uppercase tracking-widest text-zinc-600">
						{t(
							upgradeProgress.state === 'starting'
								? 'upgrade.inProgress.phase.starting'
								: 'upgrade.inProgress.phase.restarting'
						)}
					</p>
				</div>
			{:else if upgradeProgress.state === 'success'}
				<div class="flex flex-col items-center text-center">
					<div class="mb-4 flex h-16 w-16 items-center justify-center rounded-2xl bg-emerald-500/[0.15]">
						<CheckCircle2 class="h-7 w-7 text-emerald-400" strokeWidth={2} />
					</div>
					<h2 class="text-lg font-bold text-white">
						{t('upgrade.success.title')}
					</h2>
					{#if upgradeProgress.targetVersion}
						<p class="mt-1 text-sm text-zinc-400">
							v{upgradeProgress.targetVersion}
						</p>
					{/if}
					<p class="mt-4 text-[13px] leading-relaxed text-zinc-400">
						{t('upgrade.success.help')}
					</p>
				</div>
			{:else if upgradeProgress.state === 'error'}
				<div class="flex flex-col items-center text-center">
					<div class="mb-4 flex h-16 w-16 items-center justify-center rounded-2xl bg-amber-500/[0.15]">
						<AlertTriangle class="h-7 w-7 text-amber-400" strokeWidth={2} />
					</div>
					<h2 class="text-lg font-bold text-white">
						{t('upgrade.error.title')}
					</h2>
					{#if upgradeProgress.error}
						<p class="mt-3 rounded-lg bg-amber-500/[0.05] p-3 text-[11px] font-mono text-amber-300/80">
							{upgradeProgress.error}
						</p>
					{/if}
					<p class="mt-4 text-[13px] leading-relaxed text-zinc-400">
						{t('upgrade.error.help')}
					</p>
					<button
						type="button"
						data-testid="upgrade-progress-dismiss"
						class="mt-5 rounded-xl bg-white/[0.04] px-4 py-2 text-xs font-bold text-white hover:bg-white/[0.08]"
						onclick={() => upgradeProgress.reset()}
					>
						{t('action.close')}
					</button>
				</div>
			{/if}
		</div>
	</div>
{/if}
