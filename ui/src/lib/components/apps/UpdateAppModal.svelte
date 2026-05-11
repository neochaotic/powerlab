<script lang="ts">
	import { ArrowUpCircle } from 'lucide-svelte';
	import { Button } from '$lib/components/ui/button';
	import { fade, scale } from 'svelte/transition';
	import { t } from '$lib/i18n/index.svelte';

	interface AppToUpdate {
		id: string;
		store_info: {
			title?: unknown;
			image?: { en_us?: string };
			thumbnail?: string;
		};
	}

	interface Props {
		app: AppToUpdate | null;
		formattedSize: string;
		title: string;
		onCancel: () => void;
		onConfirm: () => void;
	}

	let { app, formattedSize, title, onCancel, onConfirm }: Props = $props();
</script>

{#if app}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4" transition:fade={{ duration: 200 }}>
		<div class="relative w-full max-w-md rounded-3xl border border-white/10 bg-zinc-950 p-8 shadow-2xl overflow-hidden" transition:scale={{ duration: 300, start: 0.95 }}>
			<div class="absolute inset-0 bg-gradient-to-b from-emerald-500/5 to-transparent pointer-events-none"></div>

			<div class="relative flex flex-col items-center text-center">
				<div class="mb-6 flex h-20 w-20 items-center justify-center rounded-3xl bg-emerald-500 text-zinc-950 shadow-[0_0_30px_rgba(16,185,129,0.3)]">
					<ArrowUpCircle class="h-10 w-10" />
				</div>

				<h2 class="mb-2 text-2xl font-black tracking-tight text-white">{t('apps.updateAvailable')}</h2>
				<p class="mb-8 text-sm font-medium text-zinc-400">
					{t('apps.updatePrompt', { title })}
				</p>

				<div class="mb-10 w-full space-y-3 rounded-2xl bg-white/[0.03] border border-white/5 p-4">
					<div class="flex items-center justify-between">
						<span class="text-[10px] font-bold text-zinc-500 uppercase tracking-widest">{t('apps.type')}</span>
						<span class="text-xs font-bold text-emerald-400">{t('apps.rollingUpdate')}</span>
					</div>
					<div class="h-px bg-white/5"></div>
					<div class="flex items-center justify-between">
						<span class="text-[10px] font-bold text-zinc-600 uppercase tracking-[0.2em]">{formattedSize}</span>
						<span class="text-xs font-mono text-zinc-400 truncate max-w-[200px]">{app.store_info.image?.en_us || app.store_info.thumbnail || 'latest'}</span>
					</div>
				</div>

				<div class="flex w-full gap-3">
					<Button
						variant="ghost"
						class="flex-1 rounded-2xl h-12 text-zinc-500 hover:text-white hover:bg-white/5 font-bold"
						onclick={onCancel}
					>
						Cancel
					</Button>
					<Button
						class="flex-1 rounded-2xl h-12 bg-emerald-500 text-zinc-950 hover:bg-emerald-400 font-black shadow-[0_0_20px_rgba(16,185,129,0.2)]"
						onclick={onConfirm}
					>
						{t('apps.updateNow')}
					</Button>
				</div>
			</div>
		</div>
	</div>
{/if}
