<script lang="ts">
	import { Pencil } from 'lucide-svelte';
	import { Button } from '$lib/components/ui/button';
	import { t } from '$lib/i18n/index.svelte';

	interface Props {
		open: boolean;
		onCancel: () => void;
		onConfirm: () => void;
	}

	let { open, onCancel, onConfirm }: Props = $props();
</script>

{#if open}
	<div class="fixed inset-0 z-50 flex items-end justify-center bg-black/50 backdrop-blur-sm sm:items-center">
		<div class="w-full max-w-sm rounded-t-[2rem] border border-white/8 bg-zinc-900 p-6 sm:rounded-2xl">
			<div class="mb-2 flex items-center gap-2">
				<Pencil class="h-4 w-4 text-amber-400" />
				<p class="font-semibold text-white">{t('launchpad.forkApp')}</p>
			</div>
			<p class="mb-1 text-sm text-zinc-400">
				{t('launchpad.forkDesc')}
			</p>
			<p class="mb-5 text-xs text-zinc-600">{t('apps.forkUniqueName')}</p>
			<div class="flex gap-2">
				<Button variant="ghost" class="flex-1 rounded-xl" onclick={onCancel}>Cancel</Button>
				<Button class="flex-1 rounded-xl bg-amber-500 text-black hover:bg-amber-400 font-bold" onclick={onConfirm}>Open Editor</Button>
			</div>
		</div>
	</div>
{/if}
