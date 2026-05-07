<script lang="ts">
	import { uploadFileChunk } from '$lib/api/files';
	import { Upload, Loader2 } from 'lucide-svelte';
	import { t } from '$lib/i18n/index.svelte';

	interface Props {
		currentPath: string;
		onUploadComplete: () => void;
		uploadFiles?: (files: FileList | File[]) => void;
	}

	let { currentPath, onUploadComplete, uploadFiles = $bindable() }: Props = $props();
	uploadFiles = (files: FileList | File[]) => handleFiles(files as any);

	let fileInput: HTMLInputElement;
	let uploading = $state(false);
	let progress = $state(0);
	let currentFile = $state('');

	async function handleFiles(files: FileList | null) {
		if (!files || files.length === 0) return;
		uploading = true;

		for (let i = 0; i < files.length; i++) {
			const file = files[i];
			currentFile = file.name;

			// 5MB chunks for reliable uploading
			const CHUNK_SIZE = 5 * 1024 * 1024;
			const totalChunks = Math.max(1, Math.ceil(file.size / CHUNK_SIZE));

			for (let chunkIndex = 0; chunkIndex < totalChunks; chunkIndex++) {
				const start = chunkIndex * CHUNK_SIZE;
				const end = Math.min(start + CHUNK_SIZE, file.size);
				const chunk = file.slice(start, end);
				
				// Create a new File object for the chunk so it retains the name
				const chunkFile = new File([chunk], file.name, { type: file.type });

				try {
					// Backend merges chunks when all are received
					await uploadFileChunk(
						currentPath,
						chunkFile,
						chunkIndex + 1, // 1-based chunk number
						totalChunks,
						file.name // relative path (same as filename for direct uploads)
					);
				} catch (err) {
					console.error(`Failed to upload chunk ${chunkIndex + 1} of ${file.name}`, err);
					// In a production scenario, we could retry the chunk here
				}

				progress = Math.round(((chunkIndex + 1) / totalChunks) * 100);
			}
		}

		uploading = false;
		progress = 0;
		currentFile = '';
		if (fileInput) fileInput.value = ''; // Reset input so same file can be uploaded again
		onUploadComplete();
	}
</script>

<!-- Hidden file input triggered by the button -->
<input
	type="file"
	multiple
	class="hidden"
	bind:this={fileInput}
	onchange={(e) => handleFiles(e.currentTarget.files)}
/>

<button
	onclick={() => fileInput.click()}
	disabled={uploading}
	class="flex h-10 items-center gap-2 rounded-xl border border-white/[0.06] bg-white/[0.02] px-4 text-sm font-medium text-zinc-300 transition-all hover:border-white/10 hover:bg-white/[0.05] hover:text-white disabled:opacity-60"
>
	{#if uploading}
		<Loader2 class="h-4 w-4 animate-spin" />
		{t('files.uploading')}
	{:else}
		<Upload class="h-4 w-4" />
		{t('files.upload')}
	{/if}
</button>

<!-- Floating Progress Widget -->
{#if uploading}
	<div class="fixed bottom-6 right-6 z-50 w-80 rounded-2xl border border-white/[0.06] bg-zinc-950/90 p-4 shadow-[0_24px_48px_-12px_rgba(0,0,0,0.6)] backdrop-blur-xl">
		<div class="mb-2 flex items-center justify-between gap-2 text-sm">
			<span class="truncate font-medium text-white" title={currentFile}>
				{currentFile}
			</span>
			<span class="shrink-0 text-xs font-bold tabular-nums text-emerald-400">{progress}%</span>
		</div>
		<div class="h-1 w-full overflow-hidden rounded-full bg-white/[0.06]">
			<div
				class="h-full rounded-full bg-gradient-to-r from-emerald-400 to-teal-500 transition-[width] duration-200"
				style="width: {progress}%"
			></div>
		</div>
	</div>
{/if}
