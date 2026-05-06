<script lang="ts">
	import { HardDrive } from 'lucide-svelte';
	import { cn } from '$lib/utils';

	let {
		used = 0,
		total = 0,
		label = 'Storage',
		health = 'Healthy'
	}: {
		used: number;
		total: number;
		label?: string;
		health?: 'Healthy' | 'Warning' | 'Critical';
	} = $props();

	const percent = $derived(total > 0 ? (used / total) * 100 : 0);
	
	const healthColor = $derived.by(() => {
		if (health === 'Critical') return 'bg-[var(--color-danger)] text-white';
		if (health === 'Warning') return 'bg-[var(--color-warning)] text-white';
		return 'bg-[var(--color-accent)] text-white';
	});

	function formatSize(bytes: number): string {
		if (bytes === 0) return '0 B';
		const units = ['B', 'KB', 'MB', 'GB', 'TB'];
		const i = Math.floor(Math.log(bytes) / Math.log(1024));
		return `${(bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0)} ${units[i]}`;
	}
</script>

<div class="flex items-center gap-4">
	<div class="flex h-12 w-12 shrink-0 items-center justify-center rounded-lg bg-[var(--color-bg-tertiary)] text-[var(--color-text-secondary)]">
		<HardDrive class="h-6 w-6" />
	</div>
	
	<div class="flex flex-1 flex-col justify-center">
		<div class="mb-1 flex items-center justify-between">
			<span class="font-medium text-[var(--color-text-primary)]">{label}</span>
			<span class={cn("rounded px-1.5 py-0.5 text-[10px] font-bold uppercase tracking-wider", healthColor)}>
				{health}
			</span>
		</div>
		
		<div class="flex items-center justify-between text-xs text-[var(--color-text-muted)]">
			<span>Used: {formatSize(used)}</span>
			<span>Total: {formatSize(total)}</span>
		</div>
		
		<div class="mt-2 h-2 w-full overflow-hidden rounded-full bg-[var(--color-bg-tertiary)]">
			<div
				class="h-full bg-[var(--color-info)] transition-[width] duration-300 ease-out"
				style="width: {percent}%"
			></div>
		</div>
	</div>
</div>
