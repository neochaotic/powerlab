<script lang="ts">
	import { Check, Copy } from 'lucide-svelte';
	import { t } from '$lib/i18n/index.svelte';

	interface Props {
		storagePath: string;
		copiedKey: string | null;
		onCopy: (text: string, key: string) => void;
	}

	let { storagePath, copiedKey, onCopy }: Props = $props();
</script>

<div>
	<header class="mb-8">
		<h1 class="text-2xl font-bold tracking-tight text-white">Apps</h1>
		<p class="mt-1 text-sm text-zinc-500">Where app data lives and where store apps come from.</p>
	</header>

	<!-- Storage path -->
	<section class="mb-8">
		<h3 class="mb-3 text-[10px] font-bold uppercase tracking-[0.2em] text-zinc-500">Storage</h3>
		<div class="rounded-2xl border border-white/5 bg-white/[0.02] p-5">
			<p class="text-sm font-medium text-white">App data directory</p>
			<p class="mt-0.5 text-xs text-zinc-500">Bind-mount root for installed apps. Volume sources prefixed with <code class="rounded bg-white/5 px-1 py-0.5 font-mono text-[10px] text-zinc-300">/DATA</code> are remapped here.</p>
			<div class="mt-3 flex items-center gap-2 rounded-lg bg-black/30 px-3 py-2 font-mono text-[12px] text-zinc-300">
				<span class="flex-1 truncate">{storagePath}</span>
				<button
					class="flex h-6 w-6 items-center justify-center rounded-md text-zinc-500 transition-colors hover:bg-white/5 hover:text-white"
					onclick={() => onCopy(storagePath, 'storage')}
					aria-label="Copy path"
				>
					{#if copiedKey === 'storage'}
						<Check class="h-3.5 w-3.5 text-emerald-400" />
					{:else}
						<Copy class="h-3.5 w-3.5" />
					{/if}
				</button>
			</div>
			<p class="mt-3 text-[11px] text-zinc-600">Configured at startup via <span class="text-zinc-400">app-management.conf</span>. UI editing planned.</p>
		</div>
	</section>

	<!-- App sources -->
	<section>
		<h3 class="mb-3 text-[10px] font-bold uppercase tracking-[0.2em] text-zinc-500">App store sources</h3>
		<div class="overflow-hidden rounded-2xl border border-white/5 bg-white/[0.02] divide-y divide-white/5">
			<div class="px-5 py-4">
				<p class="text-sm font-medium text-white">Local store</p>
				<p class="mt-0.5 truncate font-mono text-[11px] text-zinc-500">/Users/.../powerlab/store</p>
			</div>
			<div class="px-5 py-4">
				<p class="text-sm font-medium text-white">{t('settings.communityCatalog')}</p>
				<p class="mt-0.5 truncate font-mono text-[11px] text-zinc-500">cdn.jsdelivr.net/.../CasaOS-AppStore@gh-pages</p>
			</div>
			<div class="px-5 py-4">
				<p class="text-sm font-medium text-white">Big-Bear catalog</p>
				<p class="mt-0.5 truncate font-mono text-[11px] text-zinc-500">github.com/bigbeartechworld/big-bear-casaos</p>
			</div>
		</div>
		<p class="mt-2 text-[11px] text-zinc-600">Edit sources via <span class="text-zinc-400">app-management.conf</span>. UI editing planned.</p>
	</section>
</div>
