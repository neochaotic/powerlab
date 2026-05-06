<script lang="ts">
	import type { FileItem } from '$lib/api/files';
	import { getDownloadUrl, readFileContent } from '$lib/api/files';
	import { onMount } from 'svelte';
	import { X, FileText, FileDigit, ExternalLink, Loader2, Music } from 'lucide-svelte';
	import { Button } from '$lib/components/ui/button';
	import { fade, fly } from 'svelte/transition';
	import { formatSize } from '$lib/utils/format';

	interface Props {
		item: FileItem;
		onClose: () => void;
		onOpenEditor: (path: string) => void;
	}

	let { item, onClose, onOpenEditor }: Props = $props();

	let textPreview = $state('');
	let loadingText = $state(false);

	const ext = $derived(item.name.split('.').pop()?.toLowerCase() || '');
	const isImage = $derived(['jpg', 'jpeg', 'png', 'gif', 'webp', 'svg'].includes(ext));
	const isVideo = $derived(['mp4', 'webm', 'mov'].includes(ext));
	const isAudio = $derived(['mp3', 'flac', 'wav', 'ogg', 'm4a', 'aac'].includes(ext));
	const isPdf = $derived(ext === 'pdf');
	const isText = $derived(['txt', 'md', 'yaml', 'yml', 'json', 'conf', 'log', 'sh', 'js', 'ts', 'css', 'html', 'ini', 'xml', 'dockerfile'].includes(ext));

	onMount(async () => {
		if (isText) {
			loadingText = true;
			try {
				const res = await readFileContent(item.path);
				if (res.success === 200 || (res as any).success === 0) {
					// Show first 100 lines
					textPreview = res.data.split('\n').slice(0, 100).join('\n');
				}
			} catch (e) {
				console.error(e);
			} finally {
				loadingText = false;
			}
		}
	});

	const downloadUrl = $derived(getDownloadUrl(item.path));
</script>

<div 
	class="fixed inset-y-0 right-0 z-40 w-full max-w-[400px] border-l border-white/10 bg-zinc-900/95 shadow-2xl backdrop-blur-xl flex flex-col"
	transition:fly={{ x: 400, duration: 300 }}
>
	<!-- Header -->
	<div class="flex items-center justify-between p-6 border-b border-white/5">
		<h2 class="text-sm font-bold text-white uppercase tracking-widest">Preview</h2>
		<button onclick={onClose} class="text-zinc-500 hover:text-white transition-colors">
			<X class="h-5 w-5" />
		</button>
	</div>

	<div class="flex-1 overflow-y-auto p-6">
		<!-- Main Preview Area -->
		<div class="mb-8 flex aspect-video w-full items-center justify-center rounded-2xl bg-black/40 border border-white/5 overflow-hidden">
			{#if isImage}
				<img src={downloadUrl} alt={item.name} class="h-full w-full object-contain" loading="lazy" />
			{:else if isVideo}
				<video src={downloadUrl} controls preload="metadata" class="h-full w-full">
					<track kind="captions" />
				</video>
			{:else if isAudio}
				<div class="flex h-full w-full flex-col items-center justify-center gap-4 p-6">
					<div class="flex h-20 w-20 items-center justify-center rounded-2xl bg-blue-500/[0.12]">
						<Music class="h-10 w-10 text-blue-400" strokeWidth={1.5} />
					</div>
					<audio src={downloadUrl} controls preload="metadata" class="w-full"></audio>
				</div>
			{:else if isPdf}
				<embed src={downloadUrl} type="application/pdf" class="h-full w-full" />
			{:else if isText}
				<div class="h-full w-full p-4 overflow-hidden font-mono text-[10px] text-zinc-400 whitespace-pre leading-relaxed">
					{#if loadingText}
						<div class="flex h-full items-center justify-center">
							<Loader2 class="h-5 w-5 animate-spin text-zinc-600" />
						</div>
					{:else}
						{textPreview}
						{#if textPreview.length > 0}
							<div class="absolute inset-x-0 bottom-0 h-20 bg-gradient-to-t from-black/60 to-transparent"></div>
						{/if}
					{/if}
				</div>
			{:else}
				<div class="flex flex-col items-center gap-3 text-zinc-600">
					<FileDigit class="h-12 w-12 opacity-20" />
					<span class="text-[10px] font-bold uppercase tracking-widest">No Preview Available</span>
				</div>
			{/if}
		</div>

		<!-- File Info -->
		<div class="space-y-6">
			<div>
				<h3 class="mb-1 text-lg font-bold text-white break-all">{item.name}</h3>
				<p class="text-[10px] font-medium text-zinc-500 uppercase tracking-widest break-all">{item.path}</p>
			</div>

			<div class="grid grid-cols-2 gap-4">
				<div class="rounded-xl bg-white/5 p-3">
					<p class="text-[9px] font-bold text-zinc-500 uppercase tracking-widest mb-1">Size</p>
					<p class="text-sm font-medium text-white">{formatSize(item.size)}</p>
				</div>
				<div class="rounded-xl bg-white/5 p-3">
					<p class="text-[9px] font-bold text-zinc-500 uppercase tracking-widest mb-1">Modified</p>
					<p class="text-sm font-medium text-white">{new Date(item.modified).toLocaleDateString()}</p>
				</div>
			</div>

			<div class="flex flex-col gap-2 pt-4">
				{#if isText}
					<Button class="w-full rounded-xl bg-emerald-500 text-zinc-950 hover:bg-emerald-400 font-bold" onclick={() => onOpenEditor(item.path)}>
						<FileText class="mr-2 h-4 w-4" /> Open Editor
					</Button>
				{/if}
				<a
					href={downloadUrl}
					target="_blank"
					rel="noopener"
					class="flex w-full items-center justify-center rounded-xl border border-white/5 bg-white/5 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-white/10"
				>
					<ExternalLink class="mr-2 h-4 w-4" /> Open in New Tab
				</a>
			</div>
		</div>
	</div>
</div>
