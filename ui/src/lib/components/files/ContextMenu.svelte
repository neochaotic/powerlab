<script lang="ts">
	import { cn } from '$lib/utils';
	import type { ComponentProps, ComponentType, SvelteComponent } from 'svelte';

	interface MenuItem {
		label: string;
		icon?: ComponentType<SvelteComponent>;
		action: () => void;
		separator?: boolean;
		disabled?: boolean;
		variant?: 'default' | 'danger';
	}

	interface Props {
		items: MenuItem[];
		x: number;
		y: number;
		visible: boolean;
		onClose: () => void;
	}

	let { items, x, y, visible, onClose }: Props = $props();

	// Adjust position to keep menu in viewport
	const style = $derived.by(() => {
		const menuWidth = 200;
		const menuHeight = items.length * 36;
		const adjustedX = x + menuWidth > window.innerWidth ? x - menuWidth : x;
		const adjustedY = y + menuHeight > window.innerHeight ? y - menuHeight : y;
		return `left: ${adjustedX}px; top: ${adjustedY}px;`;
	});

	function handleClick(item: MenuItem) {
		if (item.disabled) return;
		item.action();
		onClose();
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') onClose();
	}
</script>

<svelte:window onkeydown={handleKeydown} />

{#if visible}
	<!-- Backdrop -->
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div
		class="fixed inset-0 z-40"
		onclick={onClose}
		onkeydown={(e) => e.key === 'Escape' && onClose()}
		oncontextmenu={(e) => { e.preventDefault(); onClose(); }}
	></div>

	<!-- Menu -->
	<div
		class="fixed z-50 min-w-[180px] rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-bg-secondary)] p-1 shadow-xl shadow-black/20"
		style={style}
		role="menu"
	>
		{#each items as item}
			{#if item.separator}
				<div class="my-1 h-px bg-[var(--color-border)]"></div>
			{:else}
				<button
					class={cn(
						'flex w-full items-center gap-2 rounded-[var(--radius-sm)] px-3 py-2 text-left text-sm transition-colors',
						item.disabled
							? 'cursor-not-allowed text-[var(--color-text-muted)]'
							: item.variant === 'danger'
								? 'text-[var(--color-danger)] hover:bg-[var(--color-danger)]/10'
								: 'text-[var(--color-text-primary)] hover:bg-[var(--color-bg-tertiary)]'
					)}
					onclick={() => handleClick(item)}
					disabled={item.disabled}
					role="menuitem"
				>
					{#if item.icon}
						{@const Icon = item.icon}
						<Icon class="w-4 h-4" />
					{/if}
					{item.label}
				</button>
			{/if}
		{/each}
	</div>
{/if}
