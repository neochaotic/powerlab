<!--
	InstallProgressBar — single source of truth for the install
	progress UI. Extracted from `routes/apps/+page.svelte` to fix the
	bug where the determinate progress bar only appeared once the
	first `Phase N/M` marker arrived in the SSE log stream — leaving
	the user with no progress feedback during the HTTP POST + early
	SSE seconds. Visible symptom on the user's box during v0.6.5
	testing: "barra de loading aparece somente no final".

	Render rules:
	  - phase ∈ {'installing','starting'} with no `currentPhase` →
	    indeterminate bar (pulse animation) + "Preparing..." label.
	  - phase ∈ {'installing','starting','error','timeout'} with
	    `currentPhase` → determinate bar driven by `progress`.
	  - any other phase → nothing rendered.

	The component is purely visual — it has no SSE / install state
	logic of its own. Callers pass `phase`, `currentPhase`, and
	`progress` and the component handles the rendering.
-->
<script lang="ts">
	import { t } from "$lib/i18n/index.svelte";

	interface PhaseMarker {
		step: number;
		total: number;
		label?: string;
	}

	type InstallPhase =
		| "idle"
		| "confirm"
		| "installing"
		| "starting"
		| "success"
		| "timeout"
		| "error";

	let {
		phase,
		currentPhase,
		progress,
		preparingLabel,
	}: {
		phase: InstallPhase;
		currentPhase: PhaseMarker | null;
		progress: number;
		preparingLabel?: string;
	} = $props();

	// The bar shows during any in-flight or terminal-with-context phase.
	const SHOW_PHASES: InstallPhase[] = [
		"installing",
		"starting",
		"error",
		"timeout",
	];
	const visible = $derived(SHOW_PHASES.includes(phase));

	// Indeterminate mode = "we're working, but no progress signal yet".
	const indeterminate = $derived(!currentPhase);
</script>

{#if visible}
	<div data-testid="install-progress-bar" class="border-b border-white/[0.06] px-3 py-2.5">
		<div class="mb-1.5 flex items-center justify-between gap-2 text-[11px]">
			{#if currentPhase}
				<span data-testid="install-progress-step" class="font-mono font-bold text-emerald-300">
					{currentPhase.step}/{currentPhase.total}
				</span>
				<span class="truncate text-zinc-300">
					{currentPhase.label || t("apps.installLog")}
				</span>
				<span data-testid="install-progress-percent" class="font-mono text-zinc-500 tabular-nums">
					{Math.round(progress * 100)}%
				</span>
			{:else}
				<span data-testid="install-progress-preparing" class="text-zinc-300">
					{preparingLabel ?? t("apps.preparing")}
				</span>
			{/if}
		</div>
		<div class="h-1.5 overflow-hidden rounded-full bg-white/5">
			{#if indeterminate}
				<!-- Indeterminate: animated stripe sliding right→left until
					 the first `Phase N/M` marker arrives. CSS keyframe is in
					 ui/src/app.css under `.install-progress-indeterminate`. -->
				<div
					data-testid="install-progress-fill-indeterminate"
					class="install-progress-indeterminate h-full w-1/3 rounded-full bg-gradient-to-r from-emerald-500 to-emerald-300"
				></div>
			{:else}
				<div
					data-testid="install-progress-fill-determinate"
					class="h-full rounded-full bg-gradient-to-r from-emerald-500 to-emerald-300 transition-[width] duration-500 ease-out"
					style="width: {progress * 100}%"
				></div>
			{/if}
		</div>
	</div>
{/if}
