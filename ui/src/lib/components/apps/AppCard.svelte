<script lang="ts">
	import type { ComposeAppWithStoreInfo, ComposeAppStoreInfo } from '$lib/api/apps';
	import { Button } from '$lib/components/ui/button';
	import { cn } from '$lib/utils';
	import { ScrollText, Trash2, Download, Package, Settings, Pencil, Activity } from 'lucide-svelte';
	import { detectAppSource, appSourceLabel, appSourceUpstreamURL } from '$lib/utils/app-source';
	import { goto } from '$app/navigation';

	interface Props {
		app?: ComposeAppWithStoreInfo & { id?: string };
		storeApp?: ComposeAppStoreInfo;
		/** True if the installed app came from the PowerLab store catalog. */
		isPowerLabApp?: boolean;
		onToggleStatus?: (id: string, currentStatus: string) => void;
		onUninstall?: (id: string) => void;
		onInstall?: (storeApp: ComposeAppStoreInfo) => void;
		onOpenLogs?: (id: string) => void;
		onOpenMetrics?: (id: string) => void;
		onEdit?: (id: string) => void;
		class?: string;
	}

	let { app, storeApp, isPowerLabApp = false, onToggleStatus, onUninstall, onInstall, onOpenLogs, onOpenMetrics, onEdit, class: className }: Props = $props();

	const isInstalled = $derived(!!app);
	const info = $derived(app ? app.store_info : storeApp);
	const status = $derived(app?.status ?? 'stopped');
	const isRunning = $derived(status === 'running');
	const appType = $derived.by(() => {
		if (!isInstalled) return null;
		return isPowerLabApp ? 'powerlab' : 'custom';
	});

	function getTitle(titleObj: Record<string, string> | undefined) {
		if (!titleObj) return 'Unknown App';
		return titleObj['en_us'] || titleObj['en_US'] || Object.values(titleObj)[0] || 'Unknown App';
	}

	// Broken image → hide; the fallback slot renders instead
	let iconFailed = $state(false);
</script>

{#if info}
	<div class={cn("group relative flex flex-col overflow-hidden rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-bg-secondary)] p-5 shadow-sm transition-all hover:border-[var(--color-accent)] hover:shadow-md", className)}>

		<!-- Header: Icon & Title -->
		<div class="mb-4 flex items-start gap-4">
			{#if info.icon && !iconFailed}
				<img
					src={info.icon}
					alt={getTitle(info.title)}
					class="h-16 w-16 rounded-[var(--radius-md)] bg-[var(--color-bg-primary)] object-contain shadow-sm"
					onerror={() => { iconFailed = true; }}
				/>
			{:else}
				<div class="flex h-16 w-16 items-center justify-center rounded-[var(--radius-md)] bg-[var(--color-bg-tertiary)]">
					<Package class="h-8 w-8 text-zinc-500" strokeWidth={1.5} />
				</div>
			{/if}

			<div class="flex-1 overflow-hidden">
				<div class="flex items-start justify-between gap-2">
					<h3 class="truncate text-base font-semibold text-[var(--color-text-primary)]">
						{getTitle(info.title)}
					</h3>
					<!-- Status dot + type badge -->
					<div class="flex shrink-0 items-center gap-1.5">
						{#if appType === 'powerlab'}
							<span class="rounded-full border border-emerald-500/20 bg-emerald-500/10 px-1.5 py-0.5 text-[9px] font-bold uppercase tracking-widest text-emerald-400">
								PowerLab
							</span>
						{:else if appType === 'custom'}
							<span class="rounded-full border border-amber-500/20 bg-amber-500/10 px-1.5 py-0.5 text-[9px] font-bold uppercase tracking-widest text-amber-400">
								Custom
							</span>
						{/if}
						{#if isInstalled}
							<div class={cn(
								"h-2.5 w-2.5 rounded-full",
								isRunning
									? "bg-emerald-500 shadow-[0_0_8px_rgba(16,185,129,0.5)]"
									: "bg-zinc-600"
							)}></div>
						{/if}
					</div>
				</div>
				<p class="mt-1 line-clamp-2 text-sm text-[var(--color-text-secondary)]">
					{getTitle(info.tagline)}
				</p>
				<!--
					Source badge — Apple-style discrete: text-only, muted color,
					middle-dot separator on the metadata row. Phase 5 of #307.
					Detection: explicit backend source field wins, else icon-URL
					heuristic, else "store" (never empty).
				-->
				{#if info}
					{@const _src = detectAppSource(info)}
					{@const _href = appSourceUpstreamURL(info)}
					{@const _title = info?.source?.synced_at
						? `From ${appSourceLabel(_src)} catalog · synced ${info.source.synced_at}`
						: `From ${appSourceLabel(_src)} catalog`}
					<p class="mt-1 text-[11px] text-zinc-500">
						{#if info.category}<span>{info.category}</span> ·{' '}{/if}
						{#if _href}
							<a
								href={_href}
								target="_blank"
								rel="noopener noreferrer"
								title={_title}
								class="hover:text-zinc-300 transition-colors"
								onclick={(e) => e.stopPropagation()}
								data-testid="app-source-badge"
							>{appSourceLabel(_src)}</a>
						{:else}
							<span title={_title} data-testid="app-source-badge">{appSourceLabel(_src)}</span>
						{/if}
					</p>
				{/if}
			</div>
		</div>

		<!-- Footer: Actions -->
		<div class="mt-auto flex items-center justify-between border-t border-[var(--color-border)]/50 pt-4">
			{#if isInstalled && app}
				<div class="flex gap-2">
					<Button
						variant={isRunning ? 'outline' : 'default'}
						size="sm"
						class="w-20"
						onclick={() => onToggleStatus?.(app.store_info.store_app_id, status)}
					>
						{isRunning ? 'Stop' : 'Start'}
					</Button>
					{#if isRunning && info.port_map && info.hostname}
						<Button variant="outline" size="sm" onclick={() => window.open(`http://${info.hostname}:${info.port_map}`, '_blank')}>
							Open UI
						</Button>
					{/if}
				</div>

				<div class="flex gap-1 opacity-0 transition-opacity group-hover:opacity-100">
					<Button
						variant="ghost"
						size="icon"
						title={isPowerLabApp ? 'Fork as Custom App' : 'Edit Custom App'}
						onclick={() => onEdit ? onEdit(app.store_info.store_app_id) : goto(`/apps/new?id=${app.store_info.store_app_id}`)}
					>
						{#if isPowerLabApp}
							<Settings class="h-4 w-4" />
						{:else}
							<Pencil class="h-4 w-4" />
						{/if}
					</Button>
					<Button
						variant="ghost"
						size="icon"
						title="View Logs"
						onclick={() => onOpenLogs?.(app.store_info.store_app_id)}
					>
						<ScrollText class="h-4 w-4" />
					</Button>
					<Button
						variant="ghost"
						size="icon"
						title="View Metrics"
						onclick={() => onOpenMetrics?.(app.store_info.store_app_id)}
					>
						<Activity class="h-4 w-4" />
					</Button>
					<Button
						variant="ghost"
						size="icon"
						title="Uninstall"
						class="text-[var(--color-danger)] hover:bg-[var(--color-danger)]/10"
						onclick={() => onUninstall?.(app.store_info.store_app_id)}
					>
						<Trash2 class="h-4 w-4" />
					</Button>
				</div>
			{:else if storeApp}
				<Button variant="default" size="sm" class="w-full gap-2" onclick={() => onInstall?.(storeApp)}>
					<Download class="h-4 w-4" />
					Install
				</Button>
			{/if}
		</div>
	</div>
{/if}
