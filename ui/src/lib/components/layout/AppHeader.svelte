<script lang="ts">
	import type { Snippet } from 'svelte';
	import { ArrowLeft } from 'lucide-svelte';
	import { cn } from '$lib/utils';

	interface Props {
		title: string;
		subtitle?: string;
		showBack?: boolean;
		class?: string;
		actions?: Snippet;
	}

	let { title, subtitle, showBack = true, class: className, actions }: Props = $props();
</script>

<header class={cn('mb-10 flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between', className)}>
	<div class="flex items-center gap-3 sm:gap-4">
		{#if showBack}
			<a
				href="/"
				class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl border border-white/[0.06] bg-white/[0.02] text-zinc-400 transition-all hover:-translate-x-0.5 hover:border-white/10 hover:bg-white/[0.05] hover:text-white"
				title="Back to Launchpad"
				aria-label="Back to Launchpad"
			>
				<ArrowLeft class="h-4 w-4" />
			</a>
		{/if}
		<div class="min-w-0">
			<h1 class="truncate bg-gradient-to-br from-white via-white to-zinc-400 bg-clip-text text-2xl font-black tracking-tight text-transparent sm:text-3xl">{title}</h1>
			{#if subtitle}
				<p class="mt-1 truncate text-xs font-medium text-zinc-500">{subtitle}</p>
			{/if}
		</div>
	</div>

	{#if actions}
		<div class="flex items-center gap-2">
			{@render actions()}
		</div>
	{/if}
</header>
