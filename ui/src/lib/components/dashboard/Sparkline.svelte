<script lang="ts">
	// Two modes:
	//  • `value` (default): caller pushes the latest reading; component maintains
	//    its own rolling history of `maxPoints`.
	//  • `values`: caller owns the full history array — component just renders it
	//    (used when the same series is shared across multiple sparklines).
	let {
		value = 0,
		values = undefined,
		maxPoints = 40,
		color = 'var(--color-info)',
		height = 40,
		width = 100
	}: {
		value?: number;
		values?: number[];
		maxPoints?: number;
		color?: string;
		height?: number;
		width?: number;
	} = $props();

	import { untrack } from 'svelte';

	let internalHistory = $state<number[]>([]);

	$effect(() => {
		// Only auto-track when `values` isn't provided.
		if (values !== undefined) return;
		const val = value;
		untrack(() => {
			const next = internalHistory.length >= maxPoints
				? [...internalHistory.slice(-(maxPoints - 1)), val]
				: [...internalHistory, val];
			internalHistory = next;
		});
	});

	const history = $derived(values ?? internalHistory);

	// Compute SVG path
	const pathData = $derived.by(() => {
		if (history.length < 2) return '';

		const maxVal = Math.max(...history, 1); // Avoid division by 0
		const minVal = Math.min(...history, 0); // Always ground to 0
		const range = maxVal - minVal;
		const stepX = width / (maxPoints - 1);

		let d = '';
		for (let i = 0; i < history.length; i++) {
			const x = i * stepX;
			// Invert Y because SVG origin is top-left
			const y = height - ((history[i] - minVal) / range) * height;
			if (i === 0) d += `M ${x} ${y}`;
			else d += ` L ${x} ${y}`;
		}
		return d;
	});
</script>

<div class="flex flex-col h-full w-full justify-end">
	<svg 
		viewBox="0 0 {width} {height}" 
		class="w-full h-full overflow-visible"
		preserveAspectRatio="none"
	>
		<!-- No CSS transition: SVG `d` doesn't animate via `transition-all` and the
			 forced repaint at every poll added visible jitter. Instant draws are clean. -->
		<path
			d={pathData}
			fill="none"
			stroke={color}
			stroke-width="2"
			stroke-linecap="round"
			stroke-linejoin="round"
		/>
	</svg>
</div>
