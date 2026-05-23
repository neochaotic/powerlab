/**
 * File manager store using Svelte 5 runes.
 *
 * Manages: current path, file list, selection, clipboard, sorting.
 * All data comes from the API — zero local business logic.
 */

import { listDirectory, type FileItem } from '$lib/api/files';

type SortKey = 'name' | 'size' | 'modified';
type SortDir = 'asc' | 'desc';
type ClipboardOp = 'copy' | 'cut' | null;

let currentPath = $state('/DATA');
let files = $state<FileItem[]>([]);
let loading = $state(false);
let error = $state<string | null>(null);
let selectedPaths = $state<Set<string>>(new Set());
let sortBy = $state<SortKey>('name');
let sortDir = $state<SortDir>('asc');
let clipboardPaths = $state<string[]>([]);
let clipboardOp = $state<ClipboardOp>(null);

// Sort files: directories first, then by selected column
const sortedFiles = $derived.by(() => {
	return [...files].sort((a, b) => {
		if (a.is_dir !== b.is_dir) return a.is_dir ? -1 : 1;

		let cmp = 0;
		switch (sortBy) {
			case 'name':
				cmp = a.name.localeCompare(b.name, undefined, { sensitivity: 'base' });
				break;
			case 'size':
				cmp = a.size - b.size;
				break;
			case 'modified':
				cmp = new Date(a.modified).getTime() - new Date(b.modified).getTime();
				break;
		}
		return sortDir === 'asc' ? cmp : -cmp;
	});
});

async function fetchFiles(path?: string) {
	const targetPath = path ?? currentPath;
	loading = true;
	error = null;
	selectedPaths = new Set();

	try {
		const result = await listDirectory(targetPath);
		files = result.data?.content ?? [];
		currentPath = targetPath;
	} catch (e) {
		error = (e as { message?: string })?.message ?? 'Failed to load directory';
		files = [];
	} finally {
		loading = false;
	}
}

function selectFile(path: string, multi: boolean) {
	if (multi) {
		const next = new Set(selectedPaths);
		if (next.has(path)) {
			next.delete(path);
		} else {
			next.add(path);
		}
		selectedPaths = next;
	} else {
		selectedPaths = new Set([path]);
	}
}

function selectAll() {
	selectedPaths = new Set(files.map((f) => f.path));
}

function clearSelection() {
	selectedPaths = new Set();
}

function toggleSort(key: SortKey) {
	if (sortBy === key) {
		sortDir = sortDir === 'asc' ? 'desc' : 'asc';
	} else {
		sortBy = key;
		sortDir = 'asc';
	}
}

function copyToClipboard(paths: string[]) {
	clipboardPaths = paths;
	clipboardOp = 'copy';
}

function cutToClipboard(paths: string[]) {
	clipboardPaths = paths;
	clipboardOp = 'cut';
}

function clearClipboard() {
	clipboardPaths = [];
	clipboardOp = null;
}

export function useFileStore() {
	return {
		get currentPath() { return currentPath; },
		get files() { return sortedFiles; },
		get loading() { return loading; },
		get error() { return error; },
		get selectedPaths() { return selectedPaths; },
		get sortBy() { return sortBy; },
		get sortDir() { return sortDir; },
		get clipboardPaths() { return clipboardPaths; },
		get clipboardOp() { return clipboardOp; },
		get hasClipboard() { return clipboardPaths.length > 0; },
		get selectedCount() { return selectedPaths.size; },
		fetchFiles,
		selectFile,
		selectAll,
		clearSelection,
		toggleSort,
		copyToClipboard,
		cutToClipboard,
		clearClipboard
	};
}
