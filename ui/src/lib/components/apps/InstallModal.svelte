<script lang="ts">
	import { CheckCircle2, AlertCircle, Loader2, Minimize2, X, ArrowRight } from 'lucide-svelte';
	import { fade } from 'svelte/transition';
	import { t } from '$lib/i18n/index.svelte';
	import { Button } from '$lib/components/ui/button';
	import InstallProgressBar from './InstallProgressBar.svelte';
	import LogStreamer from './LogStreamer.svelte';
	import type { InstallPhase } from '$lib/utils/install-phase';

	/**
	 * Shared install-lifecycle modal — Sprint 14 #345.
	 *
	 * Both /apps (Community Install) and /apps/new (Custom App)
	 * render this component with their local phase state. Eliminates
	 * the visual + behavioural divergence that v0.6.7 exposed.
	 *
	 * Phase contract: 'idle' | 'installing' | 'starting' | 'success'
	 * | 'error' | 'timeout'. Pages own the state machine + SSE
	 * subscription; this component owns the rendering.
	 */

	type Phase = 'idle' | 'installing' | 'starting' | 'success' | 'error' | 'timeout';

	let {
		phase = 'idle' as Phase,
		currentPhase = null as InstallPhase | null,
		progress = 0,
		logs = '',
		appTitle = '',
		appIcon = '',
		error = null as string | null,
		portNote = null as string | null,
		minimized = false,
		onMinimize = () => {},
		onCancel = () => {},
		onRetry = () => {},
		onOpen = () => {},
		onStay = () => {},
		onCheckLaunchpad = () => {}
	}: {
		phase?: Phase;
		currentPhase?: InstallPhase | null;
		progress?: number;
		logs?: string;
		appTitle?: string;
		appIcon?: string;
		error?: string | null;
		portNote?: string | null;
		minimized?: boolean;
		onMinimize?: () => void;
		onCancel?: () => void;
		onRetry?: () => void;
		onOpen?: () => void;
		onStay?: () => void;
		onCheckLaunchpad?: () => void;
	} = $props();

	const isInFlight = $derived(phase === 'installing' || phase === 'starting');
	const isTerminal = $derived(phase === 'success' || phase === 'error' || phase === 'timeout');
	const isVisible = $derived(phase !== 'idle' && !minimized);
</script>

{#if isVisible}
	<div
		data-testid="install-modal"
		class="fixed inset-0 z-[80] flex items-center justify-center bg-zinc-950/85 backdrop-blur-md p-4"
		transition:fade={{ duration: 150 }}
	>
		<div
			class="relative w-full max-w-md max-h-[90vh] flex flex-col overflow-hidden rounded-3xl border border-white/[0.08] bg-zinc-900 shadow-[0_32px_64px_rgba(0,0,0,0.5)]"
		>
			<!-- Minimize button (top-right) — only during in-flight phases. -->
			{#if isInFlight}
				<button
					data-testid="install-modal-minimize"
					class="absolute right-6 top-6 z-10 flex h-9 w-9 items-center justify-center rounded-full border border-white/10 bg-white/[0.03] text-zinc-400 transition-colors hover:border-white/20 hover:bg-white/[0.06] hover:text-white"
					onclick={onMinimize}
					aria-label="Minimize"
					title="Minimize — install continues in background (see launchpad)"
				>
					<Minimize2 class="h-4 w-4" />
				</button>
			{/if}

			<div class="flex flex-1 flex-col items-center justify-center gap-5 px-6 py-10 text-center">
				<!-- App icon + state ring -->
				<div class="relative h-20 w-20 shrink-0">
					{#if appIcon}
						<img src={appIcon} alt="" class="h-20 w-20 rounded-[20px] shadow-2xl object-contain bg-white/[0.04]" />
					{:else}
						<div class="flex h-20 w-20 items-center justify-center rounded-[20px] bg-zinc-800">
							<Loader2 class="h-8 w-8 animate-spin text-zinc-500" />
						</div>
					{/if}
					{#if phase === 'success'}
						<div class="absolute -bottom-2 -right-2 flex h-8 w-8 items-center justify-center rounded-full bg-emerald-500 shadow-lg ring-4 ring-zinc-900">
							<CheckCircle2 class="h-5 w-5 text-zinc-950" />
						</div>
					{:else if phase === 'error'}
						<div class="absolute -bottom-2 -right-2 flex h-8 w-8 items-center justify-center rounded-full bg-red-600 shadow-lg ring-4 ring-zinc-900">
							<X class="h-5 w-5 text-white" />
						</div>
					{/if}
				</div>

				<!-- Status text -->
				<div class="shrink-0">
					<p class="text-lg font-bold text-white">
						{#if phase === 'success'}
							{appTitle} {t('apps.appRunning')}
						{:else if phase === 'error'}
							{t('apps.error')}
						{:else if phase === 'timeout'}
							{t('apps.takingLonger')}
						{:else}
							{t('apps.installingApp')} {appTitle}…
						{/if}
					</p>
					{#if phase === 'installing'}
						<p class="mt-1 text-sm text-zinc-500">{t('apps.preparingInstallation')}</p>
					{:else if phase === 'starting'}
						<p class="mt-1 text-sm text-zinc-500">{t('apps.pullingImage')}</p>
					{:else if phase === 'success'}
						<p class="mt-1 text-sm text-zinc-500">{t('apps.appRunningDesc')}</p>
						{#if portNote}
							<p class="mt-2 inline-flex items-center gap-1.5 rounded-full bg-emerald-500/10 px-2.5 py-0.5 text-[11px] font-medium text-emerald-400">
								{portNote}
							</p>
						{/if}
					{:else if phase === 'timeout'}
						<p class="mt-1 text-sm text-zinc-400">{t('apps.installInBackground')}</p>
					{:else if phase === 'error' && error}
						<p class="mt-2 max-w-xs rounded-xl bg-red-950/50 px-4 py-2 text-xs leading-relaxed text-red-400">{error}</p>
					{/if}
				</div>

				<!-- Progress + log -->
				{#if isInFlight || phase === 'error' || phase === 'timeout'}
					<div class="w-full overflow-hidden rounded-2xl border border-white/[0.06] bg-black/40">
						<InstallProgressBar {phase} {currentPhase} {progress} />
						{#if logs}
							<LogStreamer {logs} />
						{/if}
					</div>
				{/if}

				<!-- Action buttons -->
				<div class="flex w-full max-w-xs shrink-0 gap-2">
					{#if phase === 'success'}
						<Button
							data-testid="install-modal-open"
							class="flex-1 rounded-xl bg-emerald-500 text-black font-bold hover:bg-emerald-400"
							onclick={onOpen}
						>
							{t('apps.backToLaunchpad')}
							<ArrowRight class="h-3.5 w-3.5 ml-1" />
						</Button>
						<Button
							variant="outline"
							class="flex-1 rounded-xl border-white/10"
							onclick={onStay}
						>
							{t('apps.stayInStore')}
						</Button>
					{:else if phase === 'error'}
						<Button
							variant="ghost"
							class="flex-1 rounded-xl"
							onclick={onCancel}
						>
							{t('action.cancel')}
						</Button>
						<Button
							data-testid="install-modal-retry"
							class="flex-1 rounded-xl"
							onclick={onRetry}
						>
							{t('apps.retry')}
						</Button>
					{:else if phase === 'timeout'}
						<Button
							class="flex-1 rounded-xl"
							onclick={onCheckLaunchpad}
						>
							{t('apps.checkLaunchpad')}
						</Button>
					{:else}
						<Button
							data-testid="install-modal-cancel"
							variant="ghost"
							class="flex-1 rounded-xl text-zinc-600"
							onclick={onCancel}
						>
							{t('action.cancel')}
						</Button>
					{/if}
				</div>
			</div>
		</div>
	</div>
{/if}
