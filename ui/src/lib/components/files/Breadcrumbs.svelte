<script lang="ts">
	import { cn } from '$lib/utils';

	interface Props {
		path: string;
		onNavigate: (path: string) => void;
		class?: string;
	}

	let { path, onNavigate, class: className }: Props = $props();

	const segments = $derived(() => {
		const parts = path.split('/').filter(Boolean);
		return parts.map((part, i) => ({
			name: part,
			path: '/' + parts.slice(0, i + 1).join('/')
		}));
	});
</script>

<nav
	class={cn(
		'flex items-center gap-1 overflow-x-auto text-sm scrollbar-none',
		className
	)}
	aria-label="File path"
>
	<button
		class="flex shrink-0 items-center gap-1 rounded px-2 py-1 text-[var(--color-text-secondary)] transition-colors hover:bg-[var(--color-bg-tertiary)] hover:text-[var(--color-text-primary)]"
		onclick={() => onNavigate('/')}
	>
		<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
			<path stroke-linecap="round" stroke-linejoin="round" d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6" />
		</svg>
		<span>Root</span>
	</button>

	{#each segments() as segment, i}
		<svg class="h-3 w-3 shrink-0 text-[var(--color-text-muted)]" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
			<path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7" />
		</svg>

		{#if i === segments().length - 1}
			<span class="truncate rounded px-2 py-1 font-medium text-[var(--color-text-primary)]">
				{segment.name}
			</span>
		{:else}
			<button
				class="truncate rounded px-2 py-1 text-[var(--color-text-secondary)] transition-colors hover:bg-[var(--color-bg-tertiary)] hover:text-[var(--color-text-primary)]"
				onclick={() => onNavigate(segment.path)}
			>
				{segment.name}
			</button>
		{/if}
	{/each}
</nav>
