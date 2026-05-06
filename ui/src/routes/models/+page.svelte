<script lang="ts">
	import {
		Brain, Sparkles, Cpu, Download, GitBranch, Gauge, Boxes,
		ExternalLink, ArrowRight
	} from 'lucide-svelte';
	import AppHeader from '$lib/components/layout/AppHeader.svelte';
	import { useSystemStore } from '$lib/stores/system.svelte';
	import { onMount, onDestroy } from 'svelte';
	import { goto } from '$app/navigation';

	const store = useSystemStore();

	onMount(() => store.startPolling(2000));
	onDestroy(() => store.stopPolling());

	const u = $derived(store.utilization);
	const hasGPU = $derived.by(() => {
		if (!u || !u.gpu) return false;
		return u.gpu.percent !== undefined && u.gpu.percent >= 0;
	});
	const gpuModel = $derived(u?.gpu?.model ?? null);
	const gpuVramUsedMB = $derived.by(() => {
		const used = u?.gpu?.memoryUsed;
		return used ? Math.round(used) : null;
	});

	// What we already support today (no smoke-and-mirrors):
	const availableNow = [
		{
			icon: Boxes,
			tint: 'emerald',
			title: 'AI tools in the App Store',
			body: 'Install Ollama, ChatGPT-Next-Web, AnythingLLM, Open WebUI, ChatbotUI and more from the curated catalogue. Full Compose runtime, port auto-remap, live install logs.',
			cta: { label: 'Browse the catalogue', href: '/apps' }
		},
		{
			icon: Cpu,
			tint: 'blue',
			title: 'GPU detected automatically',
			body: 'Apple Silicon (via ioreg) and Nvidia (via nvidia-smi) appear as live gauges on the Dashboard. No driver hunting.',
			cta: { label: 'Open Dashboard', href: '/dashboard' }
		},
		{
			icon: Gauge,
			tint: 'violet',
			title: 'Live VRAM and memory telemetry',
			body: 'Watch a 7B model load in real time. Telemetry refreshes every second so you know exactly when to scale down.',
			cta: { label: 'Open Dashboard', href: '/dashboard' }
		}
	];

	// What's on the roadmap (honest about it):
	const roadmap = [
		{
			icon: Download,
			title: 'One-click Ollama pulls',
			body: 'Pick a model from a curated catalogue, click Pull, watch the download bar. No CLI, no SSH.'
		},
		{
			icon: GitBranch,
			title: 'GGUF drag-and-drop import',
			body: 'Drop a .gguf file from your Mac onto this page. PowerLab loads it into the local Ollama or llama.cpp runtime, ready to chat.'
		},
		{
			icon: Sparkles,
			title: 'Side-by-side benchmarks',
			body: 'Run the same prompt against three models. Compare tokens-per-second, VRAM usage, output quality. Find the right model for your hardware.'
		},
		{
			icon: Brain,
			title: 'Quantization presets',
			body: 'Q4_K_M, Q5_K_M, Q8_0 — pick a quantization level that fits your VRAM, PowerLab handles the conversion in the background.'
		}
	];

	function tintClasses(tint: string) {
		const map: Record<string, string> = {
			emerald: 'bg-emerald-500/[0.12] text-emerald-400',
			blue: 'bg-blue-500/[0.12] text-blue-400',
			violet: 'bg-violet-500/[0.12] text-violet-400'
		};
		return map[tint] ?? map.blue;
	}
</script>

<svelte:head>
	<title>Models — PowerLab</title>
</svelte:head>

<div class="flex h-full flex-col overflow-y-auto p-6 md:p-8">
	<AppHeader title="Models" subtitle="Local AI, with the polish of a real product." />

	<!-- Hero -->
	<div class="relative mt-2 overflow-hidden rounded-3xl border border-blue-400/15 bg-gradient-to-br from-zinc-950 via-zinc-950 to-blue-950/20 p-8">
		<div class="pointer-events-none absolute -right-20 -top-20 h-72 w-72 rounded-full bg-blue-500/[0.10] blur-3xl"></div>
		<div class="pointer-events-none absolute -bottom-20 -left-20 h-72 w-72 rounded-full bg-violet-500/[0.08] blur-3xl"></div>

		<div class="relative grid gap-6 md:grid-cols-[1fr_auto] md:items-center">
			<div>
				<div class="mb-3 inline-flex items-center gap-1.5 rounded-full border border-blue-400/20 bg-blue-400/[0.08] px-2.5 py-1 text-[10px] font-medium uppercase tracking-widest text-blue-300">
					<Sparkles class="h-3 w-3" />
					Pre-release · Models tab in development
				</div>
				<h1 class="bg-gradient-to-br from-white via-white to-blue-200/80 bg-clip-text text-4xl font-bold tracking-tight text-transparent md:text-5xl">
					Run open AI on your own metal.
				</h1>
				<p class="mt-3 max-w-xl text-base leading-relaxed text-zinc-400">
					Today PowerLab runs every popular local AI tool through the App Store. The dedicated Models tab — drag-and-drop GGUF imports, one-click Ollama pulls, benchmarks — is on the way.
				</p>
			</div>

			<!-- GPU pill (only if a GPU was detected) -->
			{#if hasGPU}
				<div class="rounded-2xl border border-white/[0.06] bg-white/[0.02] p-4 text-right md:min-w-[180px]">
					<p class="mb-1 text-[10px] font-medium uppercase tracking-widest text-zinc-500">GPU detected</p>
					<p class="text-sm font-semibold text-white">{gpuModel ?? 'GPU online'}</p>
					{#if gpuVramUsedMB}
						<p class="mt-1 text-[12px] text-zinc-400">{gpuVramUsedMB} MB VRAM in use</p>
					{/if}
				</div>
			{:else}
				<div class="rounded-2xl border border-white/[0.06] bg-white/[0.02] p-4 text-right md:min-w-[180px]">
					<p class="mb-1 text-[10px] font-medium uppercase tracking-widest text-zinc-500">No GPU detected</p>
					<p class="text-sm font-semibold text-white">CPU inference</p>
					<p class="mt-1 text-[12px] text-zinc-400">7B Q4 models still run</p>
				</div>
			{/if}
		</div>
	</div>

	<!-- Available today -->
	<section class="mt-10">
		<h2 class="mb-4 text-[11px] font-bold uppercase tracking-[0.2em] text-zinc-500">
			Available today
		</h2>
		<div class="grid grid-cols-1 gap-3 md:grid-cols-3">
			{#each availableNow as item}
				{@const Icon = item.icon}
				<button
					class="group rounded-2xl border border-white/[0.06] bg-white/[0.02] p-5 text-left transition-all hover:border-white/10 hover:bg-white/[0.04]"
					onclick={() => goto(item.cta.href)}
				>
					<div class={'mb-4 flex h-10 w-10 items-center justify-center rounded-xl ' + tintClasses(item.tint)}>
						<Icon class="h-5 w-5" strokeWidth={2} />
					</div>
					<h3 class="text-sm font-semibold text-white">{item.title}</h3>
					<p class="mt-1.5 text-[12px] leading-relaxed text-zinc-500">{item.body}</p>
					<div class="mt-4 flex items-center gap-1.5 text-[12px] font-medium text-zinc-400 group-hover:text-white">
						{item.cta.label}
						<ArrowRight class="h-3.5 w-3.5 transition-transform group-hover:translate-x-0.5" />
					</div>
				</button>
			{/each}
		</div>
	</section>

	<!-- Roadmap -->
	<section class="mt-10 mb-8">
		<div class="mb-4 flex items-baseline justify-between">
			<h2 class="text-[11px] font-bold uppercase tracking-[0.2em] text-zinc-500">
				On the way
			</h2>
			<a
				href="https://github.com/neochaotic/powerlab/issues"
				target="_blank"
				rel="noopener"
				class="flex items-center gap-1.5 text-[11px] text-zinc-500 transition-colors hover:text-white"
			>
				Request a feature
				<ExternalLink class="h-3 w-3" />
			</a>
		</div>
		<div class="grid grid-cols-1 gap-3 md:grid-cols-2">
			{#each roadmap as item}
				{@const Icon = item.icon}
				<div class="rounded-2xl border border-white/[0.04] bg-white/[0.015] p-5">
					<div class="mb-3 flex items-center gap-3">
						<div class="flex h-9 w-9 items-center justify-center rounded-xl bg-blue-500/[0.08]">
							<Icon class="h-4 w-4 text-blue-400/80" strokeWidth={1.75} />
						</div>
						<h3 class="text-sm font-semibold text-zinc-300">{item.title}</h3>
					</div>
					<p class="text-[12px] leading-relaxed text-zinc-500">{item.body}</p>
				</div>
			{/each}
		</div>
	</section>
</div>
