<script lang="ts" module>
	/** One conflicting host port and the user's replacement choice. */
	export type PortChoice = {
		original: number;
		chosen: string; // bound to the input
		status: 'free' | 'inuse' | 'invalid' | 'checking';
	};
</script>

<script lang="ts">
	import { Package, AlertCircle, CheckCircle2, Loader2 } from 'lucide-svelte';
	import { Button } from '$lib/components/ui/button';
	import { cn } from '$lib/utils';
	import { t } from '$lib/i18n/index.svelte';
	import type { ComposeAppStoreInfo } from '$lib/api/apps';

	/**
	 * Store-app install confirmation (#295): first-run tips, compatibility
	 * warnings and the port-conflict picker shown before Install.
	 *
	 * The install state machine stays in routes/apps/+page.svelte; this
	 * component only renders it. `portChoices` is bindable because each
	 * port input binds straight to its choice (same as before the split);
	 * validation, auto-pick and install are parent callbacks.
	 */
	interface Props {
		/** App awaiting confirmation; null hides the modal. */
		app: ComposeAppStoreInfo | null;
		/** Localized app title. */
		title: string;
		/** Localized x-casaos.tips.before_install text, '' when absent. */
		beforeInstallTip: string;
		isChecking: boolean;
		warnings: string[];
		hasCriticalWarning: boolean;
		portChoices: PortChoice[];
		portsResolved: boolean;
		onPortInput: () => void;
		onAutoPick: (choice: PortChoice) => void;
		onCancel: () => void;
		onConfirm: () => void;
	}

	let {
		app,
		title,
		beforeInstallTip,
		isChecking,
		warnings,
		hasCriticalWarning,
		portChoices = $bindable(),
		portsResolved,
		onPortInput,
		onAutoPick,
		onCancel,
		onConfirm
	}: Props = $props();
</script>

{#if app}
	<div class="fixed inset-0 z-50 flex items-end justify-center bg-black/50 backdrop-blur-sm sm:items-center">
		<div class="w-full max-w-sm rounded-t-[2rem] border border-white/8 bg-zinc-900 p-6 sm:rounded-2xl">
			<div class="mb-4 flex items-center gap-3">
				{#if app.icon}
					<img src={app.icon} alt="" class="h-12 w-12 rounded-xl" onerror={(e) => { (e.target as HTMLImageElement).style.display='none'; }} />
				{:else}
					<div class="flex h-12 w-12 items-center justify-center rounded-xl bg-zinc-800"><Package class="h-6 w-6 text-zinc-500" /></div>
				{/if}
				<div>
					<p class="font-semibold text-white">{title}</p>
					<p class="text-xs text-zinc-500">{app.developer || app.author}</p>
				</div>
			</div>
			<p class="mb-4 text-sm text-zinc-400">
				{t('apps.pullingImage')}
			</p>

			{#if beforeInstallTip}
				<!-- x-casaos.tips.before_install — initial-password / first-run
					 hints supplied by the app's compose YAML. Surfaced here
					 before the user clicks Install so they know what to
					 grab post-install (admin tokens auto-written to disk,
					 default credentials baked into the image, etc). Without
					 this, half the catalogue is effectively unusable for
					 anyone who hasn't memorised every app's quirks. -->
				<div class="mb-4 rounded-xl border border-amber-500/20 bg-amber-500/[0.06] p-3">
					<div class="flex items-start gap-2">
						<AlertCircle class="h-3.5 w-3.5 shrink-0 mt-0.5 text-amber-400/90" />
						<div class="flex-1 min-w-0">
							<p class="mb-1 text-[10px] font-bold uppercase tracking-widest text-amber-400">{t('apps.firstRunNote')}</p>
							<div class="text-[11px] leading-relaxed text-amber-100/80 whitespace-pre-wrap break-words">{beforeInstallTip}</div>
						</div>
					</div>
				</div>
			{/if}

			{#if isChecking}
				<div class="mb-5 flex items-center gap-2 rounded-xl bg-white/5 p-3 text-xs text-zinc-500">
					<Loader2 class="h-3 w-3 animate-spin" />
					{t('status.loading')}…
				</div>
			{:else}
				{#if warnings.length > 0}
					<div class="mb-3 space-y-2">
						<p class="text-[10px] font-bold uppercase tracking-widest text-zinc-500">{t('apps.compatibilityWarnings')}</p>
						<div class="space-y-1.5">
							{#each warnings as warning}
								<div class="flex items-start gap-2 rounded-xl bg-amber-500/10 p-2.5 text-[11px] leading-tight text-amber-200/80 border border-amber-500/10">
									<AlertCircle class="h-3 w-3 shrink-0 mt-0.5 text-amber-500" />
									<span>{warning}</span>
								</div>
							{/each}
						</div>
					</div>
				{/if}

				{#if portChoices.length > 0}
					<div class="mb-3 space-y-2">
						<p class="text-[10px] font-bold uppercase tracking-widest text-zinc-500">{t('apps.portConflicts')}</p>
						<div class="space-y-2">
							{#each portChoices as choice}
								<div class="flex items-center gap-2.5 rounded-xl border border-white/[0.06] bg-white/[0.02] px-3 py-2.5">
									<span class="font-mono text-xs text-zinc-500 line-through">{choice.original}</span>
									<span class="text-zinc-700">→</span>
									<input
										type="number"
										min="1"
										max="65535"
										bind:value={choice.chosen}
										oninput={onPortInput}
										class={cn(
											'w-20 rounded-md border bg-white/[0.03] px-2 py-1 text-center font-mono text-xs outline-none transition-colors',
											choice.status === 'free' && 'border-emerald-500/30 text-emerald-300 focus:border-emerald-500/60',
											choice.status === 'inuse' && 'border-red-500/40 text-red-300 focus:border-red-500/70',
											choice.status === 'invalid' && 'border-red-500/40 text-red-300',
											choice.status === 'checking' && 'border-white/10 text-zinc-400'
										)}
									/>
									{#if choice.status === 'checking'}
										<Loader2 class="h-3.5 w-3.5 animate-spin text-zinc-500" />
									{:else if choice.status === 'free'}
										<CheckCircle2 class="h-3.5 w-3.5 text-emerald-400" />
									{:else}
										<AlertCircle class="h-3.5 w-3.5 text-red-400" />
									{/if}
									{#if choice.status === 'inuse' || choice.status === 'invalid'}
										<button
											type="button"
											onclick={() => onAutoPick(choice)}
											class="ml-auto rounded-md bg-white/[0.04] px-2 py-1 text-[10px] font-bold uppercase tracking-wider text-zinc-300 transition-colors hover:bg-white/[0.08]"
										>
											Auto
										</button>
									{/if}
								</div>
							{/each}
						</div>
						<p class="px-1 text-[10px] text-zinc-600">
							{portsResolved
								? t('apps.portsFreeDesc')
								: t('apps.editPortsDesc')}
						</p>
					</div>
				{/if}
			{/if}

			<div class="flex gap-2">
				<Button variant="ghost" class="flex-1 rounded-xl" onclick={onCancel}>Cancel</Button>
				<Button
					class={cn(
						'flex-1 rounded-xl font-bold',
						hasCriticalWarning ? 'bg-red-600 text-white hover:bg-red-500' : ''
					)}
					disabled={isChecking || !portsResolved}
					onclick={onConfirm}
				>
					{hasCriticalWarning ? t('apps.installAnyway') : t('action.start')}
				</Button>
			</div>
		</div>
	</div>
{/if}
