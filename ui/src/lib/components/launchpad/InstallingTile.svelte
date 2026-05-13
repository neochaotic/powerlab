<!--
	InstallingTile — iOS-style "icon is loading" launchpad tile.
	Sprint 13.2.2.

	When the user closes the install modal without canceling, the
	install keeps running. The launchpad renders one of these tiles
	in the position the app will occupy when finished. As progress
	signal arrives via SSE, the circular progress ring fills around
	the icon — same pattern iOS uses for AppStore downloads.

	Visual layers (bottom→top):
	  1. Faint icon (50% opacity until install completes)
	  2. Circular progress ring around the icon — indeterminate
	     spinner before first `Phase N/M` marker, determinate
	     fill once `currentPhase` is set.
	  3. Status badge bottom-right: small spinner / checkmark / X
	     depending on phase.

	Click behavior: opens the install modal back in foreground
	(handled by parent — emitted as the `onclick` callback).
-->
<script lang="ts">
	import { Boxes, X as XIcon, CheckCircle2 } from 'lucide-svelte';
	import type { InstallStateEntry } from '$lib/stores/install-state.svelte';

	let { entry, onclick }: { entry: InstallStateEntry; onclick?: () => void } = $props();

	// Indeterminate when no Phase N/M signal has arrived yet —
	// shows a spinning ring instead of a determinate arc.
	const indeterminate = $derived(entry.currentPhase === null);

	// Stroke-dasharray-based arc. Circle circumference is
	// 2πr where r = 26 → 163.36. We render the arc with
	// `stroke-dasharray: <filled> <gap>` so it visually wraps.
	const RADIUS = 26;
	const CIRCUMFERENCE = 2 * Math.PI * RADIUS; // ~163.36

	const filled = $derived(
		indeterminate
			? CIRCUMFERENCE * 0.25 // 25% arc for the indeterminate spinner
			: CIRCUMFERENCE * Math.min(1, Math.max(0, entry.progress)),
	);
	const gap = $derived(CIRCUMFERENCE - filled);

	function getTitle(): string {
		const t = entry.storeInfo?.title;
		if (typeof t === 'string') return t;
		if (t && typeof t === 'object') {
			return t.en_us || t['en_us'] || Object.values(t)[0] || entry.id;
		}
		return entry.id;
	}
</script>

<div data-testid="installing-tile" data-app-id={entry.id} class="relative flex flex-col items-center gap-3">
	<button
		class="relative flex h-16 w-16 items-center justify-center rounded-[1.5rem] border border-white/[0.06] bg-white/[0.04] shadow-sm overflow-hidden transition-transform duration-200 hover:scale-[1.05]"
		{onclick}
		aria-label="Installing {getTitle()}"
	>
		<!-- Icon (faded while installing) -->
		{#if entry.storeInfo?.icon}
			<img
				src={entry.storeInfo.icon}
				alt=""
				class="h-10 w-10 object-contain opacity-50"
				draggable="false"
				onerror={(e) => {
					(e.currentTarget as HTMLImageElement).style.display = 'none';
				}}
			/>
		{:else}
			<div class="flex h-10 w-10 items-center justify-center opacity-50">
				<Boxes class="h-7 w-7 text-sky-400" strokeWidth={1.5} />
			</div>
		{/if}

		<!-- Progress ring — rotates when indeterminate, fills when
			 determinate. Sits above the icon via absolute positioning. -->
		<svg
			data-testid="installing-tile-ring"
			class="absolute inset-0 h-full w-full"
			class:installing-tile-ring-spin={indeterminate}
			viewBox="0 0 64 64"
			role="presentation"
		>
			<!-- Track (full circle, faint) -->
			<circle
				cx="32"
				cy="32"
				r={RADIUS}
				fill="none"
				stroke="rgba(255,255,255,0.06)"
				stroke-width="3"
			/>
			<!-- Progress arc (emerald) — rotated -90deg so the arc
				 starts at 12-o-clock per the iOS convention. -->
			<circle
				data-testid="installing-tile-progress-arc"
				cx="32"
				cy="32"
				r={RADIUS}
				fill="none"
				stroke="#10b981"
				stroke-width="3"
				stroke-linecap="round"
				stroke-dasharray="{filled} {gap}"
				transform="rotate(-90 32 32)"
				class="transition-[stroke-dasharray] duration-500 ease-out"
			/>
		</svg>

		<!-- Status badge (bottom-right) -->
		{#if entry.phase === 'success'}
			<div data-testid="installing-tile-badge-success" class="absolute -bottom-1 -right-1 flex h-5 w-5 items-center justify-center rounded-full bg-emerald-500 ring-2 ring-zinc-900">
				<CheckCircle2 class="h-3 w-3 text-white" strokeWidth={2.5} />
			</div>
		{:else if entry.phase === 'error' || entry.phase === 'timeout'}
			<div data-testid="installing-tile-badge-error" class="absolute -bottom-1 -right-1 flex h-5 w-5 items-center justify-center rounded-full bg-red-600 ring-2 ring-zinc-900">
				<XIcon class="h-3 w-3 text-white" strokeWidth={2.5} />
			</div>
		{/if}
	</button>

	<div class="flex w-full flex-col items-center gap-0.5">
		<span
			class="w-full truncate text-center text-[10px] font-semibold text-zinc-500"
			title={getTitle()}
		>
			{getTitle()}
		</span>
		<span
			data-testid="installing-tile-label"
			class="text-[7px] font-bold uppercase tracking-[0.15em] text-emerald-500/70"
		>
			{#if entry.phase === 'success'}installed{:else if entry.phase === 'error'}error{:else if entry.phase === 'timeout'}timeout{:else}installing{/if}
		</span>
	</div>
</div>

<style>
	.installing-tile-ring-spin {
		animation: installing-tile-spin 1.4s linear infinite;
	}

	@keyframes installing-tile-spin {
		to {
			transform: rotate(360deg);
		}
	}
</style>
