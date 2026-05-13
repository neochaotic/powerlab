<!--
	LogStreamer — single source of truth for the live install-log
	display. Sprint 13.2.3.

	Both Community Install (in /apps/+page.svelte) and Custom App
	Build (in /apps/new/+page.svelte) used to render the install
	log pre inline with their own auto-scroll, scrollbar-hiding,
	and label/header rules. The duplication meant every UX tweak
	(scroll-pause, font, theming) had to land twice — and inevitably
	drifted. Extracting here gives both pages one component.

	Behavior:
	  - Renders a fixed-height monospace pre, dark background.
	  - Auto-scrolls to the latest line on every `logs` mutation,
	    UNLESS the user has scrolled up to read prior output
	    (pause-on-manual-scroll). Scrolls resume when the user
	    reaches the bottom again.
	  - Optional header strip with a pulsing dot + label.

	Pure presentational. SSE connection management stays in the
	parent (it owns the EventSource lifecycle, retries, etc.) —
	this component just renders the resulting text.
-->
<script lang="ts">
	import { t } from "$lib/i18n/index.svelte";

	let {
		logs,
		label,
		heightClass = "h-40",
	}: {
		logs: string;
		label?: string;
		heightClass?: string;
	} = $props();

	let preEl: HTMLPreElement | undefined = $state();
	let autoScrollEnabled = $state(true);

	// On every logs change, scroll to bottom IF the user hasn't
	// manually scrolled up. Tracked via `autoScrollEnabled`, set
	// from the onscroll handler (cheap math, no event throttling
	// needed — pre is only as long as install logs grow).
	$effect(() => {
		// Reactive read so $effect re-runs when `logs` changes.
		void logs;
		if (!preEl) return;
		if (!autoScrollEnabled) return;
		requestAnimationFrame(() => {
			if (preEl) preEl.scrollTop = preEl.scrollHeight;
		});
	});

	function onScroll() {
		if (!preEl) return;
		// "At bottom" with 8px tolerance so a 1-pixel rounding
		// doesn't unpause autoscroll while the user is still
		// reading prior lines.
		const atBottom =
			preEl.scrollTop + preEl.clientHeight >= preEl.scrollHeight - 8;
		autoScrollEnabled = atBottom;
	}
</script>

<div data-testid="log-streamer" class="overflow-hidden">
	<div class="flex items-center gap-2 border-b border-white/[0.06] px-3 py-2">
		<div class="h-1.5 w-1.5 animate-pulse rounded-full bg-emerald-500"></div>
		<span class="font-mono text-[10px] text-zinc-500">
			{label ?? t("apps.installLog")}
		</span>
		{#if !autoScrollEnabled}
			<span
				data-testid="log-streamer-paused"
				class="ml-auto rounded-full bg-amber-500/10 px-2 py-0.5 font-mono text-[9px] font-bold uppercase tracking-wider text-amber-400/70"
				title="Scrolled up — autoscroll paused. Scroll to bottom to resume."
			>
				paused
			</span>
		{/if}
	</div>
	<pre
		bind:this={preEl}
		data-testid="log-streamer-pre"
		onscroll={onScroll}
		class="{heightClass} overflow-y-auto p-3 font-mono text-[11px] leading-relaxed text-zinc-400 scrollbar-none"
		style="scrollbar-width:none"
	>{logs}</pre>
</div>
