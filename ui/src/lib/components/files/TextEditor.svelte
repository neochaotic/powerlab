<script lang="ts">
	import { readFileContent, updateFileContent, createFileContent } from '$lib/api/files';
	import { toast } from '$lib/stores/toast.svelte';
	import { onMount, onDestroy } from 'svelte';
	import { Save, X, Loader2, FileText } from 'lucide-svelte';
	import { Button } from '$lib/components/ui/button';
	import { fade, scale } from 'svelte/transition';
	import { t } from '$lib/i18n/index.svelte';

	// CodeMirror 6 imports
	import { EditorView, basicSetup } from 'codemirror';
	import { EditorState } from '@codemirror/state';
	import { keymap } from '@codemirror/view';
	import { indentWithTab } from '@codemirror/commands';
	import { yaml } from '@codemirror/lang-yaml';
	import { json } from '@codemirror/lang-json';
	import { javascript } from '@codemirror/lang-javascript';
	import { oneDark } from '@codemirror/theme-one-dark';

	interface Props {
		path: string;
		onClose: () => void;
	}

	let { path, onClose }: Props = $props();

	let content = $state('');
	let loading = $state(true);
	let saving = $state(false);
	let error = $state<string | null>(null);
	// True when this editor was opened for a path that does not exist on
	// disk yet — the very first Save will create the file. The header
	// reflects this so the user knows whether they're editing or creating.
	let isNewFile = $state(false);
	// Tracks unsaved changes so the title shows a • indicator and the
	// close action can prompt for confirmation. Filebrowser does the
	// same; users expect this from any modern editor.
	let isDirty = $state(false);
	// What's on disk right now — compared against the editor's current
	// document on every change to compute isDirty.
	let savedContent = $state('');
	let editorContainer = $state<HTMLDivElement | null>(null);
	// Becomes true the moment we have a successful (or 404 → new) load.
	// A reactive $effect below initializes the CodeMirror instance once
	// BOTH the editorContainer is in the DOM (loading rendered the spinner
	// branch and unmounted, the editor div mounted, bind:this fired) AND
	// readyToInit is true. Without this gate the previous code called
	// initEditor() inside onMount while loading=true, but the editor div
	// only mounts when loading=false — so editorContainer was null and
	// CodeMirror silently never attached, leaving the modal grey.
	let readyToInit = $state(false);
	// CodeMirror view is mutated imperatively (not template-rendered), so a
	// plain `let` is fine. Suppress the runes-warning explicitly.
	// svelte-ignore non_reactive_update
	let view: EditorView | null = null;

	const fileName = $derived(path.split('/').pop() || 'Untitled');

	onMount(async () => {
		try {
			const res = await readFileContent(path);
			if (res.success === 200 || (res as any).success === 0) {
				content = res.data;
				readyToInit = true;
			} else {
				error = res.message || t('editor.failedToLoad');
			}
		} catch (e) {
			// Treat 404 as "open new file" — the editor lets the user type
			// content and the first Save creates the path.
			const apiErr = e as { status?: number; message?: string };
			if (apiErr?.status === 404) {
				isNewFile = true;
				content = '';
				readyToInit = true;
			} else {
				error = apiErr?.message || (e as Error).message;
			}
		} finally {
			loading = false;
		}
	});

	// Reactive init: fires once the editor div is bound (post-render,
	// after loading=false flipped) AND readyToInit is set. Idempotent —
	// guards against re-running if either dep changes after init.
	$effect(() => {
		if (readyToInit && editorContainer && !view) {
			initEditor(content);
		}
	});

	onDestroy(() => {
		if (view) view.destroy();
	});

	function getLanguage(name: string) {
		const ext = name.split('.').pop()?.toLowerCase();
		if (ext === 'yaml' || ext === 'yml') return yaml();
		if (ext === 'json') return json();
		if (ext === 'js' || ext === 'ts') return javascript();
		return [];
	}

	function initEditor(initialContent: string) {
		if (!editorContainer) return;

		savedContent = initialContent;

		const startState = EditorState.create({
			doc: initialContent,
			extensions: [
				basicSetup,
				keymap.of([
					indentWithTab,
					{
						key: 'Mod-s',
						run: () => {
							handleSave();
							return true;
						}
					},
					{
						key: 'Escape',
						run: () => {
							handleCloseRequest();
							return true;
						}
					}
				]),
				// Track dirtiness on every doc change. Compared against
				// savedContent (mutated on successful Save) so the
				// indicator clears as soon as the file is persisted.
				EditorView.updateListener.of((update) => {
					if (update.docChanged) {
						isDirty = update.state.doc.toString() !== savedContent;
					}
				}),
				getLanguage(fileName),
				oneDark,
				EditorView.theme({
					'&': { height: '100%' },
					'.cm-scroller': { overflow: 'auto' }
				})
			]
		});

		view = new EditorView({
			state: startState,
			parent: editorContainer
		});
	}

	async function handleSave() {
		if (!view) return;
		const currentContent = view.state.doc.toString();

		saving = true;
		try {
			// New file → POST (create), existing → PUT (update). Same
			// REST contract filebrowser uses. After the first
			// successful create, flip isNewFile so subsequent saves
			// in the same session use PUT.
			const wasNew = isNewFile;
			const res = wasNew
				? await createFileContent(path, currentContent)
				: await updateFileContent(path, currentContent);
			if (res.success === 200 || (res as any).success === 0) {
				if (wasNew) isNewFile = false;
				savedContent = currentContent;
				isDirty = false;
				toast.success(wasNew ? t('editor.createdToast', { name: fileName }) : t('editor.savedToast', { name: fileName }), 2000);
			} else {
				toast.error(res.message || t('editor.failedToSave'));
			}
		} catch (e) {
			const apiErr = e as { status?: number; message?: string };
			toast.error(apiErr?.message || (e as Error).message);
		} finally {
			saving = false;
		}
	}

	// Close handler — confirms before discarding unsaved changes.
	// Same affordance filebrowser uses: a native confirm() dialog
	// because we don't have a custom modal queue and the user is
	// already inside a modal.
	function handleCloseRequest() {
		if (isDirty) {
			const ok = confirm(t('editor.discardPrompt', { name: fileName }));
			if (!ok) return;
		}
		onClose();
	}
</script>

<div 
	class="fixed inset-0 z-[100] flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm"
	transition:fade={{ duration: 200 }}
>
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div class="absolute inset-0" onclick={handleCloseRequest}></div>

	<div
		class="relative flex h-[90vh] w-full max-w-5xl flex-col rounded-3xl border border-white/10 bg-zinc-950 shadow-2xl overflow-hidden"
		transition:scale={{ duration: 300, start: 0.95 }}
	>
		<!-- Header -->
		<div class="flex items-center justify-between border-b border-white/5 bg-zinc-900/50 px-6 py-4">
			<div class="flex items-center gap-3">
				<div class="flex h-10 w-10 items-center justify-center rounded-xl bg-white/5 text-zinc-400">
					<FileText class="h-5 w-5" />
				</div>
				<div>
					<h2 class="text-sm font-bold text-white truncate max-w-[300px] flex items-center gap-1.5">
						<span>{fileName}</span>
						{#if isDirty}<span class="text-amber-400" title={t('editor.unsavedChanges')}>•</span>{/if}
						{#if isNewFile}<span class="rounded-full bg-emerald-500/15 px-2 py-0.5 text-[9px] font-bold text-emerald-400 uppercase tracking-wider">{t('editor.newFileBadge')}</span>{/if}
					</h2>
					<p class="text-[10px] font-medium text-zinc-500 uppercase tracking-widest">{path}</p>
				</div>
			</div>

			<div class="flex items-center gap-2">
				<Button
					variant="ghost"
					size="sm"
					onclick={handleCloseRequest}
					class="text-zinc-400 hover:text-white"
				>
					{t('action.cancel')}
				</Button>
				<Button
					size="sm"
					onclick={handleSave}
					disabled={loading || saving || (!isDirty && !isNewFile)}
					class="bg-emerald-500 text-zinc-950 hover:bg-emerald-400 disabled:opacity-40 disabled:cursor-not-allowed font-bold px-5"
				>
					{#if saving}
						<Loader2 class="mr-2 h-3.5 w-3.5 animate-spin" />
						{t('editor.saving')}
					{:else if isNewFile}
						<Save class="mr-2 h-3.5 w-3.5" />
						{t('files.create')}
					{:else}
						<Save class="mr-2 h-3.5 w-3.5" />
						{t('action.save')}
					{/if}
				</Button>
			</div>
		</div>

		<!-- Editor Area -->
		<div class="relative flex-1 bg-[#282c34] overflow-hidden">
			{#if loading}
				<div class="flex h-full items-center justify-center bg-zinc-950">
					<div class="flex flex-col items-center gap-3">
						<Loader2 class="h-8 w-8 animate-spin text-emerald-500" />
						<span class="text-[10px] font-bold uppercase tracking-widest text-zinc-600">{t('editor.readingFile')}</span>
					</div>
				</div>
			{:else if error}
				<div class="flex h-full flex-col items-center justify-center bg-zinc-950 p-8 text-center">
					<div class="mb-4 rounded-full bg-red-500/10 p-4 text-red-500">
						<X class="h-8 w-8" />
					</div>
					<h3 class="text-lg font-bold text-white">{t('editor.couldNotOpenFile')}</h3>
					<p class="mt-1 text-sm text-zinc-500 max-w-sm">{error}</p>
					<Button variant="outline" class="mt-6 border-white/10" onclick={onClose}>{t('editor.closeEditor')}</Button>
				</div>
			{:else}
				<div bind:this={editorContainer} class="h-full w-full"></div>
			{/if}
		</div>

		<!-- Footer / Status Bar -->
		<div class="flex items-center justify-between border-t border-white/5 bg-zinc-900/30 px-6 py-2">
			<div class="flex items-center gap-4 text-[10px] font-bold uppercase tracking-widest text-zinc-600">
				<span>UTF-8</span>
				<span>{view?.state.doc.lines || 0} {t('editor.lines')}</span>
				<span>{view?.state.doc.length || 0} {t('editor.chars')}</span>
			</div>
			{#if error}
				 <span class="text-[10px] font-bold text-red-500 uppercase tracking-widest">Error: {error}</span>
			{/if}
		</div>
	</div>
</div>

<style>
	:global(.cm-editor) {
		height: 100%;
		outline: none !important;
	}
	:global(.cm-gutters) {
		background-color: transparent !important;
		border-right: 1px solid rgba(255,255,255,0.05) !important;
		color: #5c6370 !important;
	}
</style>
