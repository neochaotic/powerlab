<script lang="ts">
	import { useFileStore } from '$lib/stores/files.svelte';
	import { renamePath, deletePaths, createFolder, createFile, operateFileOrDir, getDownloadUrl } from '$lib/api/files';
	import type { FileItem } from '$lib/api/files';
	import Breadcrumbs from '$lib/components/files/Breadcrumbs.svelte';
	import FileTable from '$lib/components/files/FileTable.svelte';
	import ContextMenu from '$lib/components/files/ContextMenu.svelte';
	import Uploader from '$lib/components/files/Uploader.svelte';
	import TextEditor from '$lib/components/files/TextEditor.svelte';
	import FilePreview from '$lib/components/files/FilePreview.svelte';
	import { Button } from '$lib/components/ui/button';
	import { FolderOpen, Download, Copy, Scissors, ClipboardPaste, Pen, Trash2, Plus, Upload, RefreshCw, FilePlus, FolderPlus } from 'lucide-svelte';
	import { onMount } from 'svelte';
	import AppHeader from '$lib/components/layout/AppHeader.svelte';

	const store = useFileStore();

	// Context menu state
	let ctxVisible = $state(false);
	let ctxX = $state(0);
	let ctxY = $state(0);
	let ctxItem = $state<FileItem | null>(null);

	// Rename dialog state
	let renaming = $state(false);
	let renameValue = $state('');

	// New folder dialog state
	let creatingFolder = $state(false);
	let newFolderName = $state('');
	let creatingFile = $state(false);
	let newFileName = $state('');

	function focusOnMount(el: HTMLInputElement) {
		queueMicrotask(() => el.focus());
	}

	// Delete confirmation state
	let confirmingDelete = $state(false);
	let pendingDeletePaths = $state<string[]>([]);

	// Text editor state
	let editingPath = $state<string | null>(null);

	// Preview state
	let previewItem = $derived.by(() => {
		if (store.selectedCount !== 1) return null;
		const path = Array.from(store.selectedPaths)[0];
		const item = store.files.find(f => f.path === path);
		return (item && !item.is_dir) ? item : null;
	});

	// Drag & Drop state
	let isDragging = $state(false);
	let uploadFilesFn = $state<(files: FileList | File[]) => void>();

	onMount(() => {
		store.fetchFiles();
	});

	function handleNavigate(path: string) {
		store.fetchFiles(path);
	}

	function handleOpen(item: FileItem) {
		if (item.is_dir) {
			store.fetchFiles(item.path);
		} else if (isEditable(item.name)) {
			editingPath = item.path;
		} else if (isPreviewable(item.name)) {
			// Single-select the file so the FilePreview side panel opens.
			if (!store.selectedPaths.has(item.path)) {
				store.selectFile(item.path, false);
			}
		} else {
			window.open(getDownloadUrl(item.path), '_blank');
		}
	}

	function isEditable(name: string): boolean {
		const ext = name.split('.').pop()?.toLowerCase();
		const editable = ['txt', 'md', 'yaml', 'yml', 'json', 'conf', 'log', 'sh', 'js', 'ts', 'css', 'html', 'json', 'ini', 'xml', 'dockerfile'];
		return !!ext && editable.includes(ext);
	}

	function isPreviewable(name: string): boolean {
		const ext = name.split('.').pop()?.toLowerCase();
		const previewable = ['jpg', 'jpeg', 'png', 'gif', 'webp', 'svg', 'mp4', 'webm', 'mov', 'mp3', 'flac', 'wav', 'ogg', 'm4a', 'aac', 'pdf'];
		return !!ext && previewable.includes(ext);
	}

	function handleContextMenu(e: MouseEvent, item: FileItem) {
		ctxItem = item;
		ctxX = e.clientX;
		ctxY = e.clientY;
		ctxVisible = true;
		if (!store.selectedPaths.has(item.path)) {
			store.selectFile(item.path, false);
		}
	}

	function startRename() {
		if (!ctxItem) return;
		renameValue = ctxItem.name;
		renaming = true;
	}

	async function confirmRename() {
		if (!ctxItem || !renameValue.trim()) return;
		const dir = ctxItem.path.substring(0, ctxItem.path.lastIndexOf('/'));
		await renamePath(ctxItem.path, `${dir}/${renameValue.trim()}`);
		renaming = false;
		store.fetchFiles();
	}

	function requestDelete() {
		const paths = Array.from(store.selectedPaths);
		if (paths.length === 0) return;
		pendingDeletePaths = paths;
		confirmingDelete = true;
	}

	async function executeDelete() {
		confirmingDelete = false;
		await deletePaths(pendingDeletePaths);
		pendingDeletePaths = [];
		store.fetchFiles();
	}

	async function handleNewFolder() {
		if (!newFolderName.trim()) return;
		await createFolder(`${store.currentPath}/${newFolderName.trim()}`);
		creatingFolder = false;
		newFolderName = '';
		store.fetchFiles();
	}

	async function handleNewFile() {
		const name = newFileName.trim();
		if (!name) return;
		const path = `${store.currentPath}/${name}`;
		try {
			await createFile(path);
			creatingFile = false;
			newFileName = '';
			await store.fetchFiles();
			// Open the new file directly in the editor so the user can start typing.
			editingPath = path;
		} catch (e) {
			console.error('Failed to create file', e);
		}
	}

	function handleDragOver(e: DragEvent) {
		e.preventDefault();
		isDragging = true;
	}

	function handleDragLeave(e: DragEvent) {
		e.preventDefault();
		// Only set to false if leaving the window or a specific root area
		if (e.clientX <= 0 || e.clientY <= 0 || e.clientX >= window.innerWidth || e.clientY >= window.innerHeight) {
			isDragging = false;
		}
	}

	function handleDrop(e: DragEvent) {
		e.preventDefault();
		isDragging = false;
		if (e.dataTransfer?.files && e.dataTransfer.files.length > 0) {
			uploadFilesFn?.(e.dataTransfer.files);
		}
	}

	async function handlePaste() {
		if (!store.hasClipboard || !store.clipboardOp) return;
		const apiOp = store.clipboardOp === 'cut' ? 'move' : 'copy';
		await operateFileOrDir(
			apiOp,
			store.currentPath,
			store.clipboardPaths.map((p) => ({ from: p }))
		);
		if (store.clipboardOp === 'cut') store.clearClipboard();
		store.fetchFiles();
	}

	const contextMenuItems = $derived.by(() => {
		if (!ctxItem) return [];
		const isMulti = store.selectedCount > 1;
		const canEdit = !ctxItem.is_dir && isEditable(ctxItem.name);

		return [
			{ label: ctxItem.is_dir ? 'Open' : (canEdit ? 'Edit' : 'Download'), icon: ctxItem.is_dir ? FolderOpen : (canEdit ? Pen : Download), action: () => handleOpen(ctxItem!) },
			{ label: '', separator: true, action: () => {} },
			{ label: 'Copy', icon: Copy, action: () => store.copyToClipboard(Array.from(store.selectedPaths)) },
			{ label: 'Cut', icon: Scissors, action: () => store.cutToClipboard(Array.from(store.selectedPaths)) },
			{ label: 'Paste', icon: ClipboardPaste, action: handlePaste, disabled: !store.hasClipboard },
			{ label: '', separator: true, action: () => {} },
			{ label: 'Rename', icon: Pen, action: startRename, disabled: isMulti },
			{ label: `Delete${isMulti ? ` (${store.selectedCount})` : ''}`, icon: Trash2, action: requestDelete, variant: 'danger' as const },
		];
	});
</script>

<svelte:head>
	<title>Files — PowerLab</title>
</svelte:head>

<div
	class="flex h-full flex-col p-6 md:p-8 relative"
	role="region"
	aria-label="Files area — drop files to upload"
	ondragover={handleDragOver}
	ondragleave={handleDragLeave}
	ondrop={handleDrop}
>
	<AppHeader title="Files" subtitle="Browse and manage system files">
		{#snippet actions()}
			<button
				class="flex h-10 items-center gap-2 rounded-xl border border-white/[0.06] bg-white/[0.02] px-4 text-sm font-medium text-zinc-300 transition-all hover:border-white/10 hover:bg-white/[0.05] hover:text-white"
				onclick={() => { creatingFile = true; newFileName = ''; }}
			>
				<FilePlus class="h-4 w-4" /> New File
			</button>
			<button
				class="flex h-10 items-center gap-2 rounded-xl border border-white/[0.06] bg-white/[0.02] px-4 text-sm font-medium text-zinc-300 transition-all hover:border-white/10 hover:bg-white/[0.05] hover:text-white"
				onclick={() => { creatingFolder = true; newFolderName = ''; }}
			>
				<FolderPlus class="h-4 w-4" /> New Folder
			</button>
			<Uploader currentPath={store.currentPath} onUploadComplete={() => store.fetchFiles()} bind:uploadFiles={uploadFilesFn} />
			<button
				class="flex h-10 w-10 items-center justify-center rounded-xl border border-white/[0.06] bg-white/[0.02] text-zinc-400 transition-all hover:border-white/10 hover:bg-white/[0.05] hover:text-white"
				onclick={() => store.fetchFiles()}
				aria-label="Refresh"
				title="Refresh"
			>
				<RefreshCw class="h-4 w-4" />
			</button>
		{/snippet}
	</AppHeader>

	<!-- Breadcrumbs & selection bar -->
	<div class="mb-6 flex items-center justify-between gap-4 rounded-2xl border border-white/[0.06] bg-zinc-950/40 px-4 py-2.5 backdrop-blur-xl">
		<Breadcrumbs path={store.currentPath} onNavigate={handleNavigate} />

		{#if store.selectedCount > 0}
			<div class="flex shrink-0 items-center gap-1.5 rounded-full border border-emerald-400/20 bg-emerald-400/[0.08] px-2.5 py-0.5 text-[10px] font-bold uppercase tracking-widest text-emerald-300">
				<span class="h-1 w-1 rounded-full bg-emerald-400"></span>
				{store.selectedCount} selected
			</div>
		{/if}
	</div>

	<!-- Error -->
	{#if store.error}
		<div class="mx-4 mt-3 rounded-[var(--radius-md)] bg-[var(--color-danger)]/10 px-4 py-3 text-sm text-[var(--color-danger)]">
			{store.error}
		</div>
	{/if}

	<!-- File Table — FileTable owns its own scroll container for virtual scrolling -->
	<div class="flex-1 overflow-hidden">
		<FileTable
			files={store.files}
			selectedPaths={store.selectedPaths}
			sortBy={store.sortBy}
			sortDir={store.sortDir}
			loading={store.loading}
			onSelect={(path, multi) => store.selectFile(path, multi)}
			onOpen={handleOpen}
			onSort={(by) => store.toggleSort(by)}
			onContextMenu={handleContextMenu}
		/>
	</div>

	<!-- Context Menu -->
	<ContextMenu
		items={contextMenuItems}
		x={ctxX}
		y={ctxY}
		visible={ctxVisible}
		onClose={() => { ctxVisible = false; }}
	/>

	<!-- Delete Confirmation Dialog -->
	{#if confirmingDelete}
		<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
			<div class="w-96 rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-bg-secondary)] p-6">
				<h3 class="mb-2 text-base font-semibold text-[var(--color-text-primary)]">Confirm Delete</h3>
				<p class="mb-6 text-sm text-[var(--color-text-secondary)]">
					Delete {pendingDeletePaths.length} item{pendingDeletePaths.length !== 1 ? 's' : ''}? This action cannot be undone.
				</p>
				<div class="flex justify-end gap-2">
					<Button variant="ghost" size="sm" onclick={() => { confirmingDelete = false; pendingDeletePaths = []; }}>Cancel</Button>
					<Button size="sm" class="bg-[var(--color-danger)] hover:bg-[var(--color-danger-hover)] text-white" onclick={executeDelete}>
						Delete
					</Button>
				</div>
			</div>
		</div>
	{/if}

	<!-- Rename Dialog -->
	{#if renaming}
		<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
			<div class="w-96 rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-bg-secondary)] p-6">
				<h3 class="mb-4 text-lg font-semibold text-[var(--color-text-primary)]">Rename</h3>
				<input
					type="text"
					bind:value={renameValue}
					class="w-full rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-bg-primary)] px-3 py-2 text-sm text-[var(--color-text-primary)] outline-none focus:border-[var(--color-accent)]"
					onkeydown={(e) => e.key === 'Enter' && confirmRename()}
				/>
				<div class="mt-4 flex justify-end gap-2">
					<Button variant="ghost" size="sm" onclick={() => { renaming = false; }}>Cancel</Button>
					<Button size="sm" onclick={confirmRename}>Rename</Button>
				</div>
			</div>
		</div>
	{/if}

	<!-- New Folder Dialog -->
	{#if creatingFolder}
		<!-- svelte-ignore a11y_click_events_have_key_events -->
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div
			class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm"
			onclick={(e) => { if (e.target === e.currentTarget) creatingFolder = false; }}
		>
			<div
				class="w-96 rounded-2xl border border-white/[0.08] bg-zinc-950/95 p-6 text-left shadow-[0_32px_64px_-12px_rgba(0,0,0,0.7)] backdrop-blur-xl"
				role="dialog"
				aria-label="New Folder"
			>
				<div class="mb-4 flex items-center gap-3">
					<div class="flex h-10 w-10 items-center justify-center rounded-xl bg-amber-500/[0.12]">
						<FolderPlus class="h-5 w-5 text-amber-400" strokeWidth={1.75} />
					</div>
					<h3 class="text-base font-semibold text-white">New Folder</h3>
				</div>
				<input
					type="text"
					bind:value={newFolderName}
					placeholder="Folder name"
					class="w-full rounded-xl border border-white/[0.08] bg-white/[0.02] px-4 py-2.5 text-sm text-white placeholder:text-zinc-600 outline-none transition-colors focus:border-amber-400/50 focus:bg-white/[0.04]"
					onkeydown={(e) => { if (e.key === 'Enter') handleNewFolder(); if (e.key === 'Escape') creatingFolder = false; }}
					use:focusOnMount
				/>
				<p class="mt-2 text-[11px] text-zinc-500">Will be created in {store.currentPath}</p>
				<div class="mt-4 flex justify-end gap-2">
					<button
						class="rounded-lg px-3 py-1.5 text-sm text-zinc-400 transition-colors hover:bg-white/[0.04] hover:text-white"
						onclick={() => { creatingFolder = false; }}
					>Cancel</button>
					<button
						class="rounded-lg bg-amber-500 px-3 py-1.5 text-sm font-medium text-zinc-950 transition-colors hover:bg-amber-400 disabled:opacity-50"
						onclick={handleNewFolder}
						disabled={!newFolderName.trim()}
					>Create</button>
				</div>
			</div>
		</div>
	{/if}

	<!-- New File Dialog -->
	{#if creatingFile}
		<!-- svelte-ignore a11y_click_events_have_key_events -->
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div
			class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm"
			onclick={(e) => { if (e.target === e.currentTarget) creatingFile = false; }}
		>
			<div
				class="w-96 rounded-2xl border border-white/[0.08] bg-zinc-950/95 p-6 text-left shadow-[0_32px_64px_-12px_rgba(0,0,0,0.7)] backdrop-blur-xl"
				role="dialog"
				aria-label="New File"
			>
				<div class="mb-4 flex items-center gap-3">
					<div class="flex h-10 w-10 items-center justify-center rounded-xl bg-emerald-500/[0.12]">
						<FilePlus class="h-5 w-5 text-emerald-400" strokeWidth={1.75} />
					</div>
					<h3 class="text-base font-semibold text-white">New File</h3>
				</div>
				<input
					type="text"
					bind:value={newFileName}
					placeholder="filename.txt"
					class="w-full rounded-xl border border-white/[0.08] bg-white/[0.02] px-4 py-2.5 text-sm text-white placeholder:text-zinc-600 outline-none transition-colors focus:border-emerald-400/50 focus:bg-white/[0.04]"
					onkeydown={(e) => { if (e.key === 'Enter') handleNewFile(); if (e.key === 'Escape') creatingFile = false; }}
					use:focusOnMount
				/>
				<p class="mt-2 text-[11px] text-zinc-500">Will be created in {store.currentPath}</p>
				<div class="mt-4 flex justify-end gap-2">
					<button
						class="rounded-lg px-3 py-1.5 text-sm text-zinc-400 transition-colors hover:bg-white/[0.04] hover:text-white"
						onclick={() => { creatingFile = false; }}
					>Cancel</button>
					<button
						class="rounded-lg bg-emerald-500 px-3 py-1.5 text-sm font-medium text-zinc-950 transition-colors hover:bg-emerald-400 disabled:opacity-50"
						onclick={handleNewFile}
						disabled={!newFileName.trim()}
					>Create &amp; Open</button>
				</div>
			</div>
		</div>
	{/if}

	{#if editingPath}
		<TextEditor path={editingPath} onClose={() => { editingPath = null; store.fetchFiles(); }} />
	{/if}

	{#if previewItem}
		<FilePreview 
			item={previewItem} 
			onClose={() => store.selectFile(previewItem!.path, false)} 
			onOpenEditor={(path) => { editingPath = path; }}
		/>
	{/if}

	<!-- Drag overlay -->
	{#if isDragging}
		<div class="fixed inset-0 z-50 flex items-center justify-center bg-emerald-500/10 backdrop-blur-[2px] pointer-events-none">
			<div class="m-8 flex h-[calc(100%-4rem)] w-[calc(100%-4rem)] flex-col items-center justify-center rounded-[2rem] border-4 border-dashed border-emerald-500/50 bg-emerald-500/5">
				<div class="flex h-20 w-20 items-center justify-center rounded-3xl bg-emerald-500 text-zinc-950 shadow-[0_0_30px_rgba(16,185,129,0.4)]">
					<Upload class="h-10 w-10" />
				</div>
				<p class="mt-6 text-xl font-bold text-emerald-400">Drop files to upload</p>
				<p class="mt-2 text-sm text-emerald-500/60 font-medium">to {store.currentPath}</p>
			</div>
		</div>
	{/if}
</div>
