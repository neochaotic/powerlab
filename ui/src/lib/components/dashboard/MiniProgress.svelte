<script lang="ts">
	import { cn } from '$lib/utils';

	let {
		value = 0,
		label,
		sublabel,
		colorClass = 'bg-emerald-500',
		icon: Icon = null,
		status = 'normal'
	} = $props<{
		value: number;
		label: string;
		sublabel: string;
		colorClass?: string;
		icon?: any;
		status?: 'normal' | 'warning' | 'critical';
	}>();

	const statusColor = $derived(
		status === 'critical' ? 'bg-red-500 shadow-[0_0_8px_rgba(239,68,68,0.5)]' : 
		status === 'warning' ? 'bg-yellow-500 shadow-[0_0_8px_rgba(234,179,8,0.5)]' : 
		colorClass
	);
</script>

<div class="flex flex-col gap-1.5">
	<div class="flex items-center justify-between text-xs font-medium">
		<div class="flex items-center gap-1.5 text-zinc-300">
			{#if Icon}
				<Icon class="h-3.5 w-3.5 text-zinc-500" />
			{/if}
			<span class="tracking-wide">{label}</span>
		</div>
		<span class="text-zinc-500">{sublabel}</span>
	</div>
	
	<!-- Progress Bar Track -->
	<!-- transition limited to `width` and short enough (200ms) to settle BEFORE
		 the next telemetry poll. Previously used `transition-all duration-1000`
		 which kept the bar perpetually animating since polls arrive every 1s. -->
	<div class="h-1.5 w-full overflow-hidden rounded-full bg-zinc-900/50">
		<div
			class={cn("h-full rounded-full transition-[width] duration-200 ease-out", statusColor)}
			style="width: {value}%"
		></div>
	</div>
</div>
