<script lang="ts">
	import { onMount } from 'svelte';
	import { FileText, RefreshCw, AlertCircle, Download, Activity } from 'lucide-svelte';
	import { cn } from '$lib/utils';
	import { listLogFiles, readLogFile, type LogFileEntry } from '$lib/api/logs';
	import ServiceLogsTab from './ServiceLogsTab.svelte';
	import AsyncBoundary from '$lib/components/ui/AsyncBoundary.svelte';

	type Tab = 'files' | 'services';
	let activeTab = $state<Tab>('files');

	// Settings → Logs pane. Read-only viewer for the .log files
	// under /var/log/powerlab/. Distinct from the Audit pane (HTTP
	// request audit JSONL) — this exposes raw service stdout
	// (app-management.log, gateway.log, user.log, upgrade.log,
	// etc.). Per-service journald streaming with live follow + tabs
	// + severity coloring is a separate, bigger feature.
	//
	// MVP scope: list + tail of the last 200 KB. Operators can
	// download the truncated view for grep / archive. Rotated
	// .log.gz transparent decompression and live follow are
	// deliberate non-goals.

	let files = $state<LogFileEntry[]>([]);
	let selectedFile = $state<string | null>(null);
	let fileContent = $state<string>('');
	let loading = $state(false);
	let loadingContent = $state(false);
	let error = $state<string | null>(null);

	async function loadFiles(): Promise<void> {
		loading = true;
		error = null;
		try {
			files = await listLogFiles();
		} catch (e) {
			const apiErr = e as { status?: number; message?: string };
			if (apiErr?.status !== 401) {
				error = apiErr?.message ?? String(e);
			}
			files = [];
		} finally {
			loading = false;
		}
	}

	async function loadContent(name: string): Promise<void> {
		selectedFile = name;
		loadingContent = true;
		fileContent = '';
		try {
			fileContent = await readLogFile(name);
		} catch (e) {
			const apiErr = e as { status?: number; message?: string };
			if (apiErr?.status !== 401) {
				fileContent = `Error loading ${name}: ${apiErr?.message ?? String(e)}`;
			}
		} finally {
			loadingContent = false;
		}
	}

	function downloadContent(): void {
		if (!selectedFile || !fileContent) return;
		const blob = new Blob([fileContent], { type: 'text/plain' });
		const url = URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		a.download = `${selectedFile}.tail.txt`;
		a.click();
		URL.revokeObjectURL(url);
	}

	function formatBytes(bytes: number): string {
		if (bytes < 1024) return `${bytes} B`;
		if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
		return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
	}

	function formatTime(unixMicros: number): string {
		if (!unixMicros) return '—';
		return new Date(unixMicros / 1000).toLocaleString();
	}

	onMount(loadFiles);
</script>

<div class="space-y-6">
	<header class="flex items-start justify-between gap-4">
		<div>
			<h2 class="text-2xl font-bold text-white">Logs</h2>
			<p class="mt-1 text-sm text-zinc-400">
				On-disk service logs from <code class="text-zinc-300">/var/log/powerlab/</code>.
				Shows the last 200 KB of each file — enough for recent troubleshooting
				without freezing the panel. The Audit pane is the read surface for HTTP
				request audit; this one exposes raw service output.
			</p>
		</div>
		{#if activeTab === 'files'}
			<button
				onclick={loadFiles}
				disabled={loading}
				class="flex h-9 items-center gap-2 rounded-xl border border-white/10 bg-white/[0.03] px-3 text-xs text-zinc-300 transition-colors hover:border-white/20 hover:text-white disabled:opacity-50"
			>
				<RefreshCw class={cn('h-3.5 w-3.5', loading && 'animate-spin')} />
				Refresh
			</button>
		{/if}
	</header>

	<!-- Tab switcher: file tail (default) vs live per-service journald. -->
	<div class="flex gap-1 border-b border-white/[0.06]" data-testid="logs-tabs">
		<button
			onclick={() => (activeTab = 'files')}
			data-testid="logs-tab-files"
			class={cn(
				'flex items-center gap-2 px-4 py-2 text-sm transition-colors',
				activeTab === 'files'
					? 'border-b-2 border-emerald-400 text-white'
					: 'text-zinc-400 hover:text-zinc-200'
			)}
		>
			<FileText class="h-3.5 w-3.5" />
			Files
		</button>
		<button
			onclick={() => (activeTab = 'services')}
			data-testid="logs-tab-services"
			class={cn(
				'flex items-center gap-2 px-4 py-2 text-sm transition-colors',
				activeTab === 'services'
					? 'border-b-2 border-emerald-400 text-white'
					: 'text-zinc-400 hover:text-zinc-200'
			)}
		>
			<Activity class="h-3.5 w-3.5" />
			Live (per-service)
		</button>
	</div>

	{#if activeTab === 'files'}
	{#if error}
		<div
			class="flex items-center gap-3 rounded-2xl border border-red-500/20 bg-red-500/[0.05] p-4 text-sm text-red-400"
		>
			<AlertCircle class="h-4 w-4 shrink-0" />
			<span>Could not load logs: {error}</span>
		</div>
	{/if}

	<div class="grid grid-cols-1 gap-4 md:grid-cols-[260px_1fr]">
		<!-- File list -->
		<div
			class="overflow-hidden rounded-2xl border border-white/[0.06] bg-white/[0.02]"
			data-testid="logs-file-list"
		>
			<div class="flex items-center gap-2 border-b border-white/[0.04] px-4 py-3 text-sm font-medium text-zinc-200">
				<FileText class="h-4 w-4 text-zinc-400" />
				Files
			</div>
			<AsyncBoundary
				variant="inline"
				loading={loading && files.length === 0}
				empty={files.length === 0}
				emptyText="No .log files found in /var/log/powerlab/"
			>
				<ul class="divide-y divide-white/[0.03]">
					{#each files as f (f.name)}
						<li>
							<button
								onclick={() => loadContent(f.name)}
								data-testid="logs-file-{f.name}"
								class={cn(
									'w-full px-4 py-3 text-left transition-colors hover:bg-white/[0.03]',
									selectedFile === f.name && 'bg-white/[0.06]'
								)}
							>
								<div class="truncate font-mono text-xs text-zinc-200">{f.name}</div>
								<div class="mt-0.5 flex items-center gap-2 text-[10px] text-zinc-500">
									<span>{formatBytes(f.size_bytes)}</span>
									<span>·</span>
									<span class="truncate">{formatTime(f.modified_us)}</span>
								</div>
							</button>
						</li>
					{/each}
				</ul>
			</AsyncBoundary>
		</div>

		<!-- Content viewer -->
		<div
			class="overflow-hidden rounded-2xl border border-white/[0.06] bg-white/[0.02]"
			data-testid="logs-content"
		>
			<div class="flex items-center justify-between border-b border-white/[0.04] px-4 py-3">
				<div class="font-mono text-xs text-zinc-300">
					{selectedFile ?? 'Select a file →'}
				</div>
				{#if selectedFile && fileContent}
					<button
						onclick={downloadContent}
						class="flex h-7 items-center gap-1.5 rounded-md border border-white/10 bg-white/[0.03] px-2 text-[10px] text-zinc-300 transition-colors hover:border-white/20 hover:text-white"
						data-testid="logs-download"
					>
						<Download class="h-3 w-3" />
						Download
					</button>
				{/if}
			</div>
			<div class="max-h-[60vh] overflow-y-auto custom-scrollbar">
				{#if !selectedFile}
					<div class="px-4 py-8 text-center text-sm text-zinc-500">
						Pick a file from the left to view its tail.
					</div>
				{:else if loadingContent}
					<div class="px-4 py-8 text-center text-sm text-zinc-500">Loading…</div>
				{:else}
					<pre
						class="whitespace-pre-wrap break-all px-4 py-3 font-mono text-[11px] leading-relaxed text-zinc-300"
						data-testid="logs-content-pre"
					>{fileContent}</pre>
				{/if}
			</div>
		</div>
	</div>
	{:else}
		<ServiceLogsTab />
	{/if}
</div>
