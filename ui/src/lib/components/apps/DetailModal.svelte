<script lang="ts">
	import { X, Package } from 'lucide-svelte';
	import { fade } from 'svelte/transition';
	import { Button } from '$lib/components/ui/button';
	import Markdown from '$lib/components/ui/Markdown.svelte';
	import { t } from '$lib/i18n/index.svelte';
	import type { ComposeAppStoreInfo } from '$lib/api/apps';

	/**
	 * App Store detail sheet (#295): icon, tagline, screenshots,
	 * description, tips and the "Get" button. Localized strings come in
	 * already resolved so this stays presentational; the parent owns the
	 * selection and the install flow.
	 */
	interface Props {
		/** App to show; null hides the modal. */
		app: ComposeAppStoreInfo | null;
		title: string;
		tagline: string;
		description: string;
		/** Localized x-casaos.tips.before_install text, '' when absent. */
		beforeInstallTip: string;
		onClose: () => void;
		onGet: () => void;
	}

	let { app, title, tagline, description, beforeInstallTip, onClose, onGet }: Props = $props();
</script>

{#if app}
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div
		class="fixed inset-0 z-[70] flex items-center justify-center p-4 bg-zinc-950/80 backdrop-blur-md"
		onclick={onClose}
		transition:fade={{ duration: 200 }}
	>
		<!-- svelte-ignore a11y_click_events_have_key_events -->
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div
			class="relative w-full max-w-2xl max-h-[90vh] flex flex-col overflow-hidden rounded-[2.5rem] border border-white/[0.08] bg-zinc-900 shadow-[0_32px_64px_rgba(0,0,0,0.5)]"
			onclick={(e) => e.stopPropagation()}
		>
			<!-- Close button -->
			<button
				class="absolute right-6 top-6 z-10 flex h-10 w-10 items-center justify-center rounded-full bg-white/5 text-zinc-400 backdrop-blur-md transition-all hover:bg-white/10 hover:text-white"
				onclick={onClose}
			>
				<X class="h-5 w-5" />
			</button>

			<div class="flex-1 overflow-y-auto p-8 pt-10 scrollbar-none" style="scrollbar-width: none">
				<!-- Header Section -->
				<div class="flex flex-col items-center text-center sm:flex-row sm:text-left sm:items-start gap-6">
					<div class="h-24 w-24 shrink-0 shadow-2xl">
						{#if app.icon}
							<img src={app.icon} alt={title} class="h-24 w-24 rounded-[2rem] object-contain bg-white/[0.05] border border-white/10" />
						{:else}
							<div class="flex h-24 w-24 items-center justify-center rounded-[2rem] bg-white/[0.05] border border-white/10">
								<Package class="h-10 w-10 text-zinc-500" />
							</div>
						{/if}
					</div>
					<div class="flex-1 pt-1">
						<h1 class="text-2xl font-black tracking-tight text-white">{title}</h1>
						<p class="text-sm font-medium text-emerald-500">{app.developer || app.author || 'Independent Developer'}</p>
						{#if tagline && tagline !== 'Unknown'}
							<p class="mt-2 text-sm leading-relaxed text-zinc-400">{tagline}</p>
						{/if}
						<div class="mt-4 flex flex-wrap justify-center sm:justify-start gap-2">
							{#if app.category}
								<span class="rounded-full bg-white/5 border border-white/10 px-3 py-1 text-[10px] font-bold uppercase tracking-wider text-zinc-400">{app.category}</span>
							{/if}
						</div>
					</div>
				</div>

				<!-- Screenshots Carrousel -->
				{#if app.screenshot_link && app.screenshot_link.length > 0}
					<div class="mt-10 overflow-hidden">
						<h3 class="mb-4 text-xs font-bold uppercase tracking-widest text-zinc-500">{t('apps.preview')}</h3>
						<div class="flex gap-4 overflow-x-auto pb-2 scrollbar-none" style="scrollbar-width: none">
							{#each app.screenshot_link as shot}
								<img src={shot} alt="Screenshot" class="h-48 rounded-2xl border border-white/10 bg-white/[0.02] shadow-lg" />
							{/each}
						</div>
					</div>
				{/if}

				<!-- Description Section -->
				<div class="mt-10">
					<h3 class="mb-4 text-xs font-bold uppercase tracking-widest text-zinc-500">{t('apps.aboutThisApp')}</h3>
					<Markdown content={description} />
				</div>

				{#if app.tips?.custom}
					<!-- x-casaos.tips.custom — post-install hint with config
						 instructions, env-var overrides, default credentials,
						 etc. Rendered as markdown so apps that already use
						 bullet lists / code spans display correctly. -->
					<div class="mt-10">
						<h3 class="mb-4 text-xs font-bold uppercase tracking-widest text-zinc-500">{t('apps.firstRunNote')}</h3>
						<div class="rounded-2xl border border-amber-500/20 bg-amber-500/[0.06] p-5">
							<Markdown content={app.tips.custom} />
						</div>
					</div>
				{/if}

				{#if beforeInstallTip}
					<div class="mt-6">
						<h3 class="mb-4 text-xs font-bold uppercase tracking-widest text-zinc-500">{t('apps.beforeYouInstall')}</h3>
						<div class="rounded-2xl border border-amber-500/20 bg-amber-500/[0.06] p-5 text-sm leading-relaxed text-amber-100/80 whitespace-pre-wrap break-words">{beforeInstallTip}</div>
					</div>
				{/if}
			</div>

			<!-- Footer Action -->
			<div class="border-t border-white/[0.08] bg-white/[0.02] p-6 backdrop-blur-xl">
				<div class="flex items-center justify-between gap-4">
					<div class="hidden sm:block">
						<p class="text-[10px] font-bold uppercase tracking-widest text-zinc-500">{t('apps.openSource')}</p>
						<p class="text-xs text-zinc-400">{t('apps.verifiedInstallation')}</p>
					</div>
					<Button
						class="h-12 w-full sm:w-40 rounded-2xl bg-white text-zinc-950 font-bold hover:bg-emerald-500 hover:text-zinc-950 transition-all shadow-[0_8px_24px_rgba(255,255,255,0.15)] active:scale-95"
						onclick={onGet}
					>
						{t('apps.get')}
					</Button>
				</div>
			</div>
		</div>
	</div>
{/if}
