<script lang="ts">
	import { fly } from 'svelte/transition';
	import { CheckCircle, XCircle, AlertTriangle, Info, X } from 'lucide-svelte';
	import { toast, type Toast } from '$lib/stores/toast.svelte';
	import { cn } from '$lib/utils';

	const icons = {
		success: CheckCircle,
		error: XCircle,
		warning: AlertTriangle,
		info: Info
	} as const;

	// Colored accent (left bar + icon) over a neutral glass background. Less
	// shouty than full-color tinted backgrounds, plus the message text stays
	// white for readability regardless of toast type.
	const accent: Record<Toast['type'], string> = {
		success: 'before:bg-emerald-400 text-emerald-300',
		error:   'before:bg-red-400 text-red-300',
		warning: 'before:bg-amber-400 text-amber-300',
		info:    'before:bg-blue-400 text-blue-300'
	};
</script>

<div
	class="pointer-events-none fixed bottom-6 right-6 z-50 flex flex-col items-end gap-2"
	aria-live="polite"
	aria-label="Notifications"
>
	{#each toast.toasts as t (t.id)}
		{@const Icon = icons[t.type]}
		<div
			class={cn(
				'pointer-events-auto relative flex max-w-sm items-start gap-3 overflow-hidden rounded-xl border border-white/[0.06] bg-zinc-950/80 px-4 py-3 shadow-[0_18px_40px_-12px_rgba(0,0,0,0.6)] backdrop-blur-xl',
				"before:absolute before:left-0 before:top-0 before:h-full before:w-[3px] before:content-['']",
				accent[t.type]
			)}
			in:fly={{ x: 40, duration: 250 }}
			out:fly={{ x: 40, duration: 200 }}
			role="alert"
		>
			<Icon class="mt-0.5 h-4 w-4 shrink-0" />
			<p class="flex-1 text-sm font-medium leading-snug text-white">{t.message}</p>
			<button
				onclick={() => toast.dismiss(t.id)}
				class="ml-1 shrink-0 text-zinc-500 transition-colors hover:text-white"
				aria-label="Dismiss notification"
			>
				<X class="h-3.5 w-3.5" />
			</button>
		</div>
	{/each}
</div>
