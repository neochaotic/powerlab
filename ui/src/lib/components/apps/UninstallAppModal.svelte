<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { t } from '$lib/i18n/index.svelte';

	interface Props {
		open: boolean;
		deleteData: boolean;
		onDeleteDataChange: (value: boolean) => void;
		onCancel: () => void;
		onConfirm: () => void;
	}

	let { open, deleteData, onDeleteDataChange, onCancel, onConfirm }: Props = $props();
</script>

{#if open}
	<div class="fixed inset-0 z-50 flex items-end justify-center bg-black/50 backdrop-blur-sm sm:items-center">
		<div class="w-full max-w-sm rounded-t-[2rem] border border-white/8 bg-zinc-900 p-6 sm:rounded-2xl">
			<p class="mb-1 font-semibold text-white">{t('apps.uninstall')}</p>
			<p class="mb-4 text-sm text-zinc-400">{t('apps.removeAppDesc')}</p>

			<label class="mb-6 flex cursor-pointer items-center gap-3 rounded-xl border border-white/5 bg-white/[0.02] p-4 transition-colors hover:bg-white/[0.04]">
				<input
					type="checkbox"
					checked={deleteData}
					onchange={(e) => onDeleteDataChange(e.currentTarget.checked)}
					class="h-4 w-4 rounded border-zinc-700 bg-zinc-800 text-red-600 focus:ring-red-600 focus:ring-offset-zinc-900"
				/>
				<div class="flex flex-col">
					<span class="text-xs font-bold text-white uppercase tracking-wider">{t('apps.deleteData')}</span>
					<span class="text-[10px] text-zinc-500">{t('apps.deleteAppDataDesc')}</span>
				</div>
			</label>

			<div class="flex gap-2">
				<Button variant="ghost" class="flex-1 rounded-xl" onclick={onCancel}>Cancel</Button>
				<Button class="flex-1 rounded-xl bg-red-600 text-white hover:bg-red-500 font-bold" onclick={onConfirm}>
					Uninstall
				</Button>
			</div>
		</div>
	</div>
{/if}
