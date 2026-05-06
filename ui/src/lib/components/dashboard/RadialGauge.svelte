<script lang="ts">
	let {
		value = 0,
		label = '',
		sublabel = '',
		color = '#10b981',
		size = 160
	}: {
		value: number;
		label?: string;
		sublabel?: string;
		color?: string;
		size?: number;
	} = $props();

	const cx = 60;
	const cy = 60;
	const r = 44;
	const C = 2 * Math.PI * r;
	// 270° arc (75% of full circle), 90° gap at bottom
	const arcLength = $derived(C * 0.75);
	const gapLength = $derived(C * 0.25);
	const fillLength = $derived(arcLength * Math.max(0, Math.min(1, value / 100)));
</script>

<div class="flex flex-col items-center" style="width: {size}px">
	<svg viewBox="0 0 120 120" width={size} height={size} aria-label="{label}: {value.toFixed(0)}%">
		<!-- Rotate so the 90° gap sits at the bottom center -->
		<g style="transform: rotate(135deg); transform-origin: 60px 60px">
			<!-- Track -->
			<circle
				{cx} {cy} {r}
				fill="none"
				stroke="#27272a"
				stroke-width="10"
				stroke-dasharray="{arcLength} {gapLength}"
				stroke-linecap="round"
			/>
			<!-- Value fill -->
			{#if fillLength > 0.5}
				<circle
					{cx} {cy} {r}
					fill="none"
					stroke={color}
					stroke-width="10"
					stroke-dasharray="{fillLength} {C}"
					stroke-linecap="round"
					class="transition-[stroke-dasharray] duration-300 ease-out"
					style="filter: drop-shadow(0 0 5px {color}66)"
				/>
			{/if}
		</g>
		<!-- Center value -->
		<text
			x="60" y="54"
			text-anchor="middle"
			dominant-baseline="middle"
			fill="white"
			style="font-size: 20px; font-weight: 700; font-family: inherit"
		>{value.toFixed(0)}%</text>
		<!-- Center label -->
		<text
			x="60" y="72"
			text-anchor="middle"
			dominant-baseline="middle"
			fill="#52525b"
			style="font-size: 8.5px; font-weight: 700; font-family: inherit; text-transform: uppercase; letter-spacing: 1.5px"
		>{label}</text>
	</svg>
	{#if sublabel}
		<p class="mt-1 text-[10px] font-medium text-zinc-500">{sublabel}</p>
	{/if}
</div>
