<script lang="ts">
	import { onMount } from 'svelte';
	import { ClipboardList, RefreshCw, Database, AlertCircle } from 'lucide-svelte';
	import { cn } from '$lib/utils';
	import {
		getAuditRecent,
		getAuditStats,
		type AuditRecord,
		type AuditStats
	} from '$lib/api/audit';

	// Settings → Audit pane. Surfaces the per-service audit log
	// recorded by the ADR-0033 middleware: who hit what, when,
	// with which status, from which IP.
	//
	// Today the pane reads only the gateway's audit DB (the slice
	// that B1c wired up). When B1c's stdlib follow-up lands and
	// every service has its own DB, this pane gets a service
	// picker.

	let records = $state<AuditRecord[]>([]);
	let stats = $state<AuditStats | null>(null);
	let loading = $state(false);
	let error = $state<string | null>(null);

	// Default limit — matches the backend default. Keeping in sync
	// with `RecentOptions.clamp()` in backend/common/utils/audit/endpoints.go.
	const DEFAULT_LIMIT = 100;
	let limit = $state(DEFAULT_LIMIT);

	async function load(): Promise<void> {
		loading = true;
		error = null;
		try {
			const [rows, s] = await Promise.all([getAuditRecent({ limit }), getAuditStats()]);
			records = rows;
			stats = s;
		} catch (e) {
			// 401 is handled centrally by the onAuthError hook in
			// $lib/api/client (logout + toast); we still want to
			// surface other errors here so operators see them.
			const apiErr = e as { status?: number; message?: string };
			if (apiErr?.status !== 401) {
				error = apiErr?.message ?? String(e);
			}
			records = [];
		} finally {
			loading = false;
		}
	}

	onMount(load);

	// ─── Formatters ─────────────────────────────────────────────
	function formatTs(unixMicros: number): string {
		if (!unixMicros) return '—';
		// JS Date uses ms; backend stores µs → divide by 1000.
		const d = new Date(unixMicros / 1000);
		return d.toLocaleString();
	}

	function formatLatency(us: number): string {
		if (us < 1000) return `${us}µs`;
		if (us < 1_000_000) return `${(us / 1000).toFixed(1)}ms`;
		return `${(us / 1_000_000).toFixed(2)}s`;
	}

	function formatBytes(bytes: number): string {
		if (bytes < 1024) return `${bytes} B`;
		if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
		return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
	}

	function statusTone(status: number): string {
		if (status >= 200 && status < 300) return 'text-emerald-400';
		if (status >= 300 && status < 400) return 'text-blue-400';
		if (status >= 400 && status < 500) return 'text-amber-400';
		return 'text-red-400';
	}
</script>

<div class="space-y-6">
	<!-- Header -->
	<header class="flex items-start justify-between gap-4">
		<div>
			<h2 class="text-2xl font-bold text-white">Audit log</h2>
			<p class="mt-1 text-sm text-zinc-400">
				Every authenticated HTTP request to the gateway, recorded with
				method, path, status, user, and IP. Kept per the retention policy
				configured in the backend (default 30 days).
			</p>
		</div>
		<button
			onclick={load}
			disabled={loading}
			class="flex h-9 items-center gap-2 rounded-xl border border-white/10 bg-white/[0.03] px-3 text-xs text-zinc-300 transition-colors hover:border-white/20 hover:text-white disabled:opacity-50"
		>
			<RefreshCw class={cn('h-3.5 w-3.5', loading && 'animate-spin')} />
			Refresh
		</button>
	</header>

	<!-- Stats card -->
	{#if stats}
		<div class="grid grid-cols-1 gap-3 sm:grid-cols-4">
			<div class="rounded-2xl border border-white/[0.06] bg-white/[0.02] p-4">
				<div class="text-[10px] font-medium uppercase tracking-widest text-zinc-500">
					Records
				</div>
				<div class="mt-1 text-2xl font-bold text-white">
					{stats.row_count.toLocaleString()}
				</div>
			</div>
			<div class="rounded-2xl border border-white/[0.06] bg-white/[0.02] p-4">
				<div class="text-[10px] font-medium uppercase tracking-widest text-zinc-500">
					Oldest
				</div>
				<div class="mt-1 text-sm font-medium text-zinc-200">
					{formatTs(stats.oldest_unix_us)}
				</div>
			</div>
			<div class="rounded-2xl border border-white/[0.06] bg-white/[0.02] p-4">
				<div class="text-[10px] font-medium uppercase tracking-widest text-zinc-500">
					Newest
				</div>
				<div class="mt-1 text-sm font-medium text-zinc-200">
					{formatTs(stats.newest_unix_us)}
				</div>
			</div>
			<div class="rounded-2xl border border-white/[0.06] bg-white/[0.02] p-4">
				<div class="flex items-center gap-1.5 text-[10px] font-medium uppercase tracking-widest text-zinc-500">
					<Database class="h-3 w-3" /> On disk
				</div>
				<div class="mt-1 text-sm font-medium text-zinc-200">
					{formatBytes(stats.file_size_bytes)}
				</div>
			</div>
		</div>
	{/if}

	<!-- Error banner -->
	{#if error}
		<div class="flex items-center gap-3 rounded-2xl border border-red-500/20 bg-red-500/[0.05] p-4 text-sm text-red-400">
			<AlertCircle class="h-4 w-4 shrink-0" />
			<span>Could not load audit log: {error}</span>
		</div>
	{/if}

	<!-- Records table -->
	<div class="overflow-hidden rounded-2xl border border-white/[0.06] bg-white/[0.02]">
		<div class="flex items-center justify-between border-b border-white/[0.04] px-4 py-3">
			<div class="flex items-center gap-2 text-sm font-medium text-zinc-200">
				<ClipboardList class="h-4 w-4 text-zinc-400" />
				Recent requests
			</div>
			<div class="text-xs text-zinc-500">Limit: {limit}</div>
		</div>

		{#if loading && records.length === 0}
			<div class="px-4 py-8 text-center text-sm text-zinc-500">Loading…</div>
		{:else if records.length === 0}
			<div class="px-4 py-8 text-center text-sm text-zinc-500">
				No audit records yet. Try clicking around the panel.
			</div>
		{:else}
			<div class="max-h-[60vh] overflow-y-auto custom-scrollbar">
				<table class="w-full text-xs" data-testid="audit-table">
					<thead class="sticky top-0 bg-zinc-950 text-[10px] uppercase tracking-widest text-zinc-500">
						<tr>
							<th class="px-4 py-2 text-left font-medium">Time</th>
							<th class="px-4 py-2 text-left font-medium">Method</th>
							<th class="px-4 py-2 text-left font-medium">Path</th>
							<th class="px-4 py-2 text-right font-medium">Status</th>
							<th class="px-4 py-2 text-right font-medium">Latency</th>
							<th class="px-4 py-2 text-left font-medium">User</th>
							<th class="px-4 py-2 text-left font-medium">IP</th>
						</tr>
					</thead>
					<tbody class="divide-y divide-white/[0.03]">
						{#each records as r (r.id)}
							<tr class="hover:bg-white/[0.02]">
								<td class="whitespace-nowrap px-4 py-2 font-mono text-zinc-400">
									{formatTs(r.ts_unix_us)}
								</td>
								<td class="px-4 py-2 font-mono text-zinc-300">{r.method}</td>
								<td class="max-w-[280px] truncate px-4 py-2 font-mono text-zinc-200" title={r.path}>
									{r.path}
								</td>
								<td class={cn('px-4 py-2 text-right font-mono', statusTone(r.status))}>
									{r.status}
								</td>
								<td class="whitespace-nowrap px-4 py-2 text-right font-mono text-zinc-400">
									{formatLatency(r.latency_us)}
								</td>
								<td class="px-4 py-2 text-zinc-300">
									{r.username ?? (r.user_id !== null ? `#${r.user_id}` : '—')}
								</td>
								<td class="whitespace-nowrap px-4 py-2 font-mono text-zinc-500">
									{r.remote_ip}
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{/if}
	</div>
</div>
