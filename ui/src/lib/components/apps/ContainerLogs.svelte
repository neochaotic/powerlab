<script lang="ts">
	import { getComposeAppLogs } from '$lib/api/apps';
	import { Button } from '$lib/components/ui/button';
	import { ScrollText, Download, Search, ArrowDown, Terminal } from 'lucide-svelte';
	import { onMount, onDestroy, untrack } from 'svelte';
	import { cn } from '$lib/utils';

	interface Props {
		appId: string;
		onClose: () => void;
	}

	let { appId, onClose }: Props = $props();

	let logs = $state('');
	let filterTerm = $state('');
	let follow = $state(true);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let logContainer = $state<HTMLPreElement>();
	let interval: ReturnType<typeof setInterval>;

	const lines = $derived(logs.split('\n').filter(l => l.trim() !== ''));
	const lineCount = $derived(lines.length);

	const filteredLogs = $derived.by(() => {
		if (!filterTerm) return logs;
		return logs.split('\n')
			.filter(line => line.toLowerCase().includes(filterTerm.toLowerCase()))
			.join('\n');
	});

	async function fetchLogs() {
		try {
			const res = await getComposeAppLogs(appId, 1000); // Fetch more lines for better experience
			logs = res.data || 'No logs available.';
			error = null;
		} catch (e) {
			error = (e as Error).message ?? 'Failed to load logs';
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		if (follow && logContainer && logs) {
			untrack(() => {
				if (logContainer) logContainer.scrollTop = logContainer.scrollHeight;
			});
		}
	});

	onMount(() => {
		fetchLogs();
		interval = setInterval(fetchLogs, 3000);
	});

	onDestroy(() => {
		clearInterval(interval);
	});

	function downloadLogs() {
		const blob = new Blob([logs], { type: 'text/plain' });
		const url = URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		a.download = `${appId}.log`;
		a.click();
		URL.revokeObjectURL(url);
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') onClose();
	}
</script>

<svelte:window onkeydown={handleKeydown} />

<div class="fixed inset-0 z-50 flex flex-col bg-black/80 backdrop-blur-sm p-4 md:p-8">
	<div class="mx-auto flex h-full w-full max-w-5xl flex-col overflow-hidden rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[#0d1117] shadow-2xl">
		
		<!-- Header -->
		<div class="flex flex-col border-b border-white/10 bg-[#161b22]">
			<div class="flex items-center justify-between px-4 py-2.5">
				<div class="flex items-center gap-3">
					<div class="flex items-center gap-2 rounded-lg bg-emerald-500/10 px-2 py-1 text-emerald-400">
						<Terminal class="h-3.5 w-3.5" />
						<span class="font-mono text-[11px] font-bold uppercase tracking-wider">{appId}</span>
					</div>
					<div class="flex items-center gap-1.5 rounded-full bg-white/[0.03] px-2.5 py-1">
						<span class="h-1.5 w-1.5 rounded-full bg-zinc-500 animate-pulse"></span>
						<span class="text-[10px] font-bold text-zinc-500">{lineCount.toLocaleString()} lines</span>
					</div>
				</div>
				<div class="flex items-center gap-2">
					<Button 
						variant="ghost" 
						size="sm" 
						class={cn("h-8 gap-2 text-[11px] font-bold transition-all", follow ? "text-emerald-400 bg-emerald-500/10" : "text-zinc-500 hover:text-zinc-300")}
						onclick={() => follow = !follow}
					>
						<ArrowDown class={cn("h-3.5 w-3.5 transition-transform", follow ? "translate-y-0.5" : "")} />
						Follow
					</Button>
					<div class="h-4 w-px bg-white/5 mx-1"></div>
					<Button variant="ghost" size="sm" class="h-8 gap-2 text-[11px] font-bold text-zinc-400 hover:text-white" onclick={downloadLogs}>
						<Download class="h-3.5 w-3.5" />
						Export
					</Button>
					<Button variant="ghost" size="icon" class="h-8 w-8 text-zinc-500 hover:bg-red-500/20 hover:text-red-400" onclick={onClose}>
						✕
					</Button>
				</div>
			</div>
			
			<!-- Filter Bar -->
			<div class="flex items-center border-t border-white/5 bg-black/20 px-4 py-1.5">
				<Search class="h-3 w-3 text-zinc-600" />
				<input 
					bind:value={filterTerm}
					placeholder="Filter logs..."
					class="flex-1 bg-transparent px-3 py-1 font-mono text-[11px] text-zinc-300 placeholder:text-zinc-600 focus:outline-none"
				/>
				{#if filterTerm}
					<button class="text-[10px] text-zinc-600 hover:text-zinc-400" onclick={() => filterTerm = ''}>Clear</button>
				{/if}
			</div>
		</div>

		<!-- Log Output -->
		<div class="relative flex-1 overflow-hidden bg-[#0d1117]">
			{#if loading && !logs}
				<div class="flex h-full items-center justify-center font-mono text-sm text-zinc-600">
					<div class="flex items-center gap-3">
						<div class="h-4 w-4 animate-spin rounded-full border-2 border-emerald-500 border-t-transparent"></div>
						Streaming logs...
					</div>
				</div>
			{:else if error}
				<div class="flex h-full flex-col items-center justify-center gap-2 p-4 text-center">
					<div class="rounded-full bg-red-500/10 p-3 text-red-500">
						<ScrollText class="h-6 w-6" />
					</div>
					<p class="font-mono text-sm text-red-400">{error}</p>
					<Button variant="outline" size="sm" class="mt-2 border-white/10" onclick={fetchLogs}>Try Again</Button>
				</div>
			{:else if logs.trim() === 'No logs available.' || !logs}
				<div class="flex h-full flex-col items-center justify-center gap-4 text-center">
					<div class="rounded-2xl bg-white/[0.03] p-4 border border-white/5">
						<ScrollText class="h-8 w-8 text-zinc-600" strokeWidth={1.5} />
					</div>
					<div class="space-y-1">
						<p class="text-sm font-bold text-white">No logs yet</p>
						<p class="text-[11px] font-medium text-zinc-500 max-w-[200px]">
							Start the container to see its real-time output here.
						</p>
					</div>
				</div>
			{:else}
				<pre
					bind:this={logContainer}
					class="h-full w-full overflow-auto p-4 font-mono text-[13px] leading-relaxed text-[#c9d1d9] whitespace-pre selection:bg-emerald-500/20 selection:text-white"
				>{filteredLogs}</pre>
			{/if}
		</div>
	</div>
</div>
