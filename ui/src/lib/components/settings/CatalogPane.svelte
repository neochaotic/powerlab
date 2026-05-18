<script lang="ts">
	import { onMount } from 'svelte';
	import { Store, ShieldAlert, Plus, Trash2, ExternalLink, RefreshCw } from 'lucide-svelte';
	import { cn } from '$lib/utils';
	import {
		listCatalogSources,
		addCatalogSource,
		removeCatalogSource,
		type AppStoreSource
	} from '$lib/api/catalog';

	// Settings → Catalog pane (ADR-0039).
	//
	// PowerLab ships a single curated catalog (id=0, immutable). The
	// operator can add custom sources at their own risk — typed URL,
	// one-time acknowledgement modal, permanent "unaudited" badge.
	// There is NO toggle for an "experimental" passthrough mode —
	// catalogs are first-class registrations, not gated features.
	//
	// Security model (memory: security_is_priority):
	//   - PowerLab applies its hard hook/exports.sh filter to ALL
	//     catalogs at sync time, including operator-added ones.
	//   - PowerLab applies install-time transforms (hostname,
	//     host-substitution, bind-mount chmod, image-skeleton-seed)
	//     to apps from ALL catalogs.
	//   - PowerLab does NOT audit individual app composes from
	//     operator-added catalogs — that's the operator's call.

	let sources = $state<AppStoreSource[]>([]);
	let loading = $state(false);
	let error = $state<string | null>(null);
	let showAddModal = $state(false);
	let newUrl = $state('');
	let adding = $state(false);
	let acknowledgedRisk = $state(false);

	async function load(): Promise<void> {
		loading = true;
		error = null;
		try {
			sources = await listCatalogSources();
		} catch (e) {
			const apiErr = e as { status?: number; message?: string };
			if (apiErr?.status !== 401) {
				error = apiErr?.message ?? String(e);
			}
		} finally {
			loading = false;
		}
	}

	function openAddModal(): void {
		newUrl = '';
		acknowledgedRisk = false;
		showAddModal = true;
	}

	function cancelAdd(): void {
		showAddModal = false;
		newUrl = '';
		acknowledgedRisk = false;
	}

	async function confirmAdd(): Promise<void> {
		const url = newUrl.trim();
		if (!url || !acknowledgedRisk) return;
		adding = true;
		error = null;
		try {
			await addCatalogSource(url);
			showAddModal = false;
			newUrl = '';
			acknowledgedRisk = false;
			// Reload after a brief delay so the async registration
			// has time to surface the new entry.
			setTimeout(load, 1500);
		} catch (e) {
			const apiErr = e as { message?: string };
			error = apiErr?.message ?? String(e);
		} finally {
			adding = false;
		}
	}

	async function removeSource(id: number): Promise<void> {
		if (id === 0) return; // guard: never remove default
		const confirmed = confirm('Remove this catalog source? Apps already installed from it will continue to run.');
		if (!confirmed) return;
		error = null;
		try {
			await removeCatalogSource(id);
			await load();
		} catch (e) {
			const apiErr = e as { message?: string };
			error = apiErr?.message ?? String(e);
		}
	}

	function sourceLabel(s: AppStoreSource): string {
		if (s.id === 0) return 'PowerLab Curated';
		// Pull a readable name from the URL
		try {
			const u = new URL(s.url);
			return u.hostname + u.pathname;
		} catch {
			return s.url;
		}
	}

	function sourceBadge(s: AppStoreSource): { text: string; classes: string } {
		if (s.id === 0) {
			return {
				text: 'Default · Curated',
				classes: 'border-emerald-500/30 bg-emerald-500/[0.08] text-emerald-300'
			};
		}
		return {
			text: 'Unaudited',
			classes: 'border-yellow-500/30 bg-yellow-500/[0.08] text-yellow-300'
		};
	}

	onMount(load);
</script>

<div class="space-y-6" data-testid="catalog-pane">
	<header class="flex items-start justify-between gap-4">
		<div>
			<h2 class="text-2xl font-bold text-white">Catalog</h2>
			<p class="mt-1 text-sm text-zinc-400">
				Sources PowerLab pulls app listings from. The PowerLab Curated
				catalog ships with every release and is reviewed per app. You
				can add your own sources below — those are NOT audited by
				PowerLab.
			</p>
		</div>
		<button
			onclick={load}
			disabled={loading}
			class="flex h-9 items-center gap-2 rounded-xl border border-white/10 bg-white/[0.03] px-3 text-xs text-zinc-300 transition-colors hover:border-white/20 hover:text-white disabled:opacity-50"
			data-testid="catalog-refresh"
		>
			<RefreshCw class={cn('h-3.5 w-3.5', loading && 'animate-spin')} />
			Refresh
		</button>
	</header>

	{#if error}
		<div class="rounded-2xl border border-red-500/20 bg-red-500/[0.05] p-4 text-sm text-red-400">
			{error}
		</div>
	{/if}

	<!-- Sources list -->
	<div class="space-y-2" data-testid="catalog-sources">
		{#if loading && sources.length === 0}
			<div class="rounded-2xl border border-white/[0.06] bg-white/[0.02] p-6 text-center text-sm text-zinc-500">
				Loading…
			</div>
		{:else if sources.length === 0}
			<div class="rounded-2xl border border-white/[0.06] bg-white/[0.02] p-6 text-center text-sm text-zinc-500">
				No catalog sources registered. The default catalog should have
				appeared automatically; try Refresh.
			</div>
		{:else}
			{#each sources as s (s.id)}
				{@const badge = sourceBadge(s)}
				<div
					class="flex items-start justify-between gap-3 rounded-2xl border border-white/[0.06] bg-white/[0.02] p-4"
					data-testid="catalog-source-{s.id}"
				>
					<div class="flex flex-1 items-start gap-3">
						<Store class="mt-1 h-4 w-4 shrink-0 text-zinc-400" />
						<div class="min-w-0">
							<div class="flex items-center gap-2 flex-wrap">
								<span class="text-sm font-medium text-zinc-200">{sourceLabel(s)}</span>
								<span class={cn('rounded-full border px-2 py-0.5 text-[10px]', badge.classes)}>
									{badge.text}
								</span>
							</div>
							<div class="mt-1 truncate font-mono text-[10px] text-zinc-500" title={s.url}>
								{s.url}
							</div>
							{#if s.store_root && s.store_root !== s.url}
								<div class="mt-0.5 truncate font-mono text-[10px] text-zinc-600">
									→ {s.store_root}
								</div>
							{/if}
						</div>
					</div>
					{#if s.id !== 0}
						<button
							onclick={() => removeSource(s.id)}
							class="flex h-7 items-center gap-1.5 rounded-md border border-red-500/20 bg-red-500/[0.05] px-2 text-[10px] text-red-300 transition-colors hover:border-red-500/40 hover:bg-red-500/[0.10]"
							data-testid="catalog-source-{s.id}-remove"
						>
							<Trash2 class="h-3 w-3" />
							Remove
						</button>
					{/if}
				</div>
			{/each}
		{/if}
	</div>

	<button
		onclick={openAddModal}
		class="flex w-full items-center justify-center gap-2 rounded-2xl border border-dashed border-white/10 bg-white/[0.02] p-4 text-sm text-zinc-400 transition-colors hover:border-white/20 hover:bg-white/[0.04] hover:text-zinc-200"
		data-testid="catalog-add"
	>
		<Plus class="h-4 w-4" />
		Add custom catalog source
	</button>
</div>

<!-- Add-source modal — typed URL + one-time acknowledgement gate -->
{#if showAddModal}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
		role="dialog"
		aria-modal="true"
		aria-labelledby="catalog-add-title"
	>
		<div
			class="w-full max-w-lg rounded-2xl border border-yellow-500/30 bg-zinc-950 p-6 shadow-2xl"
			data-testid="catalog-add-modal"
		>
			<div class="flex items-start gap-3">
				<ShieldAlert class="mt-0.5 h-6 w-6 shrink-0 text-yellow-400" />
				<div class="flex-1">
					<h3 id="catalog-add-title" class="text-lg font-semibold text-white">
						Add custom catalog source
					</h3>
					<div class="mt-3 space-y-2 text-sm leading-relaxed text-zinc-300">
						<p>
							PowerLab does <strong class="text-yellow-300">not audit</strong>
							catalogs added here. Apps from this source will be installable
							alongside the PowerLab Curated set with an
							<strong>"Unaudited"</strong> badge throughout the store.
						</p>
						<p>
							PowerLab still enforces its safety floor: apps shipping
							<code class="text-zinc-400">hooks/</code> or
							<code class="text-zinc-400">exports.sh</code> are filtered out
							regardless of source. PowerLab will never execute upstream
							bash.
						</p>
					</div>

					<label class="mt-5 block">
						<span class="text-xs font-medium text-zinc-400">Catalog URL or path</span>
						<input
							type="text"
							bind:value={newUrl}
							placeholder="https://github.com/owner/repo or /absolute/path"
							class="mt-1 w-full rounded-lg border border-white/10 bg-zinc-900 px-3 py-2 font-mono text-sm text-zinc-200 placeholder-zinc-600 focus:border-yellow-500/40 focus:outline-none"
							data-testid="catalog-add-url"
						/>
					</label>

					<label class="mt-4 flex items-start gap-2 cursor-pointer">
						<input
							type="checkbox"
							bind:checked={acknowledgedRisk}
							class="mt-0.5 h-4 w-4"
							data-testid="catalog-add-ack"
						/>
						<span class="text-xs leading-relaxed text-zinc-400">
							I understand PowerLab does not audit this catalog. I take
							responsibility for the apps I install from it.
						</span>
					</label>
				</div>
			</div>

			<div class="mt-6 flex justify-end gap-2">
				<button
					onclick={cancelAdd}
					class="rounded-xl border border-white/10 bg-white/[0.03] px-4 py-2 text-sm text-zinc-300 transition-colors hover:border-white/20 hover:text-white"
					data-testid="catalog-add-cancel"
				>
					Cancel
				</button>
				<button
					onclick={confirmAdd}
					disabled={!newUrl.trim() || !acknowledgedRisk || adding}
					class="rounded-xl border border-yellow-500/40 bg-yellow-500/20 px-4 py-2 text-sm font-medium text-yellow-200 transition-colors hover:border-yellow-500/60 hover:bg-yellow-500/30 disabled:cursor-not-allowed disabled:opacity-50"
					data-testid="catalog-add-confirm"
				>
					{adding ? 'Adding…' : 'Add source'}
				</button>
			</div>
		</div>
	</div>
{/if}
