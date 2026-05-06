<script lang="ts">
	import { cn } from '$lib/utils';
	import type { Snippet } from 'svelte';
	import type { HTMLButtonAttributes } from 'svelte/elements';

	type Variant = 'default' | 'destructive' | 'outline' | 'secondary' | 'ghost' | 'link';
	type Size = 'default' | 'sm' | 'lg' | 'icon';

	interface Props extends HTMLButtonAttributes {
		variant?: Variant;
		size?: Size;
		children: Snippet;
		class?: string;
	}

	let { variant = 'default', size = 'default', children, class: className, ...restProps }: Props =
		$props();

	const variantStyles: Record<Variant, string> = {
		default: 'bg-[var(--color-accent)] text-white hover:bg-[var(--color-accent-hover)]',
		destructive: 'bg-[var(--color-danger)] text-white hover:bg-[var(--color-danger-hover)]',
		outline:
			'border border-[var(--color-border)] bg-transparent hover:bg-[var(--color-bg-tertiary)] text-[var(--color-text-primary)]',
		secondary:
			'bg-[var(--color-bg-tertiary)] text-[var(--color-text-primary)] hover:bg-[var(--color-bg-elevated)]',
		ghost: 'hover:bg-[var(--color-bg-tertiary)] text-[var(--color-text-primary)]',
		link: 'text-[var(--color-accent)] underline-offset-4 hover:underline'
	};

	const sizeStyles: Record<Size, string> = {
		default: 'h-10 px-4 py-2',
		sm: 'h-9 rounded-md px-3',
		lg: 'h-11 rounded-md px-8',
		icon: 'h-10 w-10'
	};
</script>

<button
	class={cn(
		'inline-flex items-center justify-center whitespace-nowrap rounded-[var(--radius-md)] text-sm font-medium transition-colors duration-[var(--transition-fast)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent)] focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50',
		variantStyles[variant],
		sizeStyles[size],
		className
	)}
	{...restProps}
>
	{@render children()}
</button>
