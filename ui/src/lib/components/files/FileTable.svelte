<script lang="ts">
	import { cn } from '$lib/utils';
	import type { FileItem } from '$lib/api/files';
	import {
		Folder, File, Image, Video, Music, FileText, Archive,
		Code, FileCode, Terminal, Disc, Download, ChevronUp, ChevronDown,
		FolderOpen
	} from 'lucide-svelte';
	import type { ComponentType, SvelteComponent } from 'svelte';

	interface Props {
		files: FileItem[];
		selectedPaths: Set<string>;
		sortBy: 'name' | 'size' | 'modified';
		sortDir: 'asc' | 'desc';
		loading: boolean;
		onSelect: (path: string, multi: boolean) => void;
		onOpen: (item: FileItem) => void;
		onSort: (by: 'name' | 'size' | 'modified') => void;
		onContextMenu: (e: MouseEvent, item: FileItem) => void;
	}

	let {
		files,
		selectedPaths,
		sortBy,
		sortDir,
		loading,
		onSelect,
		onOpen,
		onSort,
		onContextMenu
	}: Props = $props();

	// ── Virtual scrolling ────────────────────────────────────────────────
	const ROW_HEIGHT = 40; // px — must match the <tr> height set below
	const OVERSCAN = 8;    // rows rendered above/below the visible window

	let containerEl = $state<HTMLElement | null>(null);
	let scrollTop = $state(0);
	let containerHeight = $state(600);

	$effect(() => {
		if (!containerEl) return;
		const ro = new ResizeObserver(([entry]) => {
			containerHeight = entry.contentRect.height;
		});
		ro.observe(containerEl);
		return () => ro.disconnect();
	});

	const visibleStart = $derived(Math.max(0, Math.floor(scrollTop / ROW_HEIGHT) - OVERSCAN));
	const visibleEnd = $derived(Math.min(files.length, Math.ceil((scrollTop + containerHeight) / ROW_HEIGHT) + OVERSCAN));
	const visibleFiles = $derived(files.slice(visibleStart, visibleEnd));
	const topPadding = $derived(visibleStart * ROW_HEIGHT);
	const bottomPadding = $derived(Math.max(0, (files.length - visibleEnd) * ROW_HEIGHT));

	// ── Helpers ──────────────────────────────────────────────────────────
	function formatSize(bytes: number): string {
		if (bytes === 0) return '—';
		const units = ['B', 'KB', 'MB', 'GB', 'TB'];
		const i = Math.floor(Math.log(bytes) / Math.log(1024));
		return `${(bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0)} ${units[i]}`;
	}

	function formatDate(dateStr: string): string {
		if (!dateStr) return '—';
		const d = new Date(dateStr);
		return d.toLocaleDateString('en-US', {
			month: 'short', day: 'numeric', year: 'numeric',
			hour: '2-digit', minute: '2-digit'
		});
	}

	function getFileIcon(item: FileItem): ComponentType<SvelteComponent> {
		if (item.is_dir) return Folder;
		const ext = item.name.split('.').pop()?.toLowerCase() ?? '';
		const map: Record<string, ComponentType<SvelteComponent>> = {
			png: Image, jpg: Image, jpeg: Image, gif: Image, svg: Image, webp: Image, bmp: Image,
			mp4: Video, mkv: Video, avi: Video, mov: Video, webm: Video,
			mp3: Music, flac: Music, wav: Music, ogg: Music, aac: Music,
			pdf: FileText, doc: FileText, docx: FileText, txt: FileText, md: FileText,
			zip: Archive, tar: Archive, gz: Archive, rar: Archive, '7z': Archive, xz: Archive,
			js: Code, ts: Code, py: Code, go: Code, rs: Code, java: Code, cpp: Code, c: Code,
			json: FileCode, yaml: FileCode, yml: FileCode, toml: FileCode, xml: FileCode, csv: FileCode,
			sh: Terminal, bash: Terminal, zsh: Terminal,
			iso: Disc, img: Disc,
		};
		return map[ext] ?? File;
	}

	function getIconColor(item: FileItem): string {
		if (item.is_dir) return 'text-amber-400';
		const ext = item.name.split('.').pop()?.toLowerCase() ?? '';
		if (['png','jpg','jpeg','gif','svg','webp'].includes(ext)) return 'text-pink-400';
		if (['mp4','mkv','avi','mov','webm'].includes(ext)) return 'text-purple-400';
		if (['mp3','flac','wav','ogg'].includes(ext)) return 'text-blue-400';
		if (['zip','tar','gz','rar','7z'].includes(ext)) return 'text-orange-400';
		if (['js','ts','py','go','rs'].includes(ext)) return 'text-emerald-400';
		if (['json','yaml','yml','toml'].includes(ext)) return 'text-cyan-400';
		if (['sh','bash','zsh'].includes(ext)) return 'text-zinc-300';
		return 'text-zinc-400';
	}
</script>

<!-- Scroll container owned by FileTable so ResizeObserver has an accurate height -->
<div
	bind:this={containerEl}
	class="h-full overflow-y-auto"
	onscroll={(e) => { scrollTop = (e.currentTarget as HTMLElement).scrollTop; }}
>
	<table class="w-full border-collapse text-sm">
		<thead class="sticky top-0 z-10">
			<tr class="border-b border-[var(--color-border)] bg-[var(--color-bg-secondary)]">
				<th class="w-8 px-3 py-2.5"></th>
				<th class="px-3 py-2.5 text-left">
					<button
						class="flex items-center gap-1 font-medium text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]"
						onclick={() => onSort('name')}
					>
						Name
						{#if sortBy === 'name'}
							{#if sortDir === 'asc'}<ChevronUp class="h-3 w-3" />{:else}<ChevronDown class="h-3 w-3" />{/if}
						{/if}
					</button>
				</th>
				<th class="w-28 px-3 py-2.5 text-right">
					<button
						class="flex items-center justify-end gap-1 w-full font-medium text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]"
						onclick={() => onSort('size')}
					>
						Size
						{#if sortBy === 'size'}
							{#if sortDir === 'asc'}<ChevronUp class="h-3 w-3" />{:else}<ChevronDown class="h-3 w-3" />{/if}
						{/if}
					</button>
				</th>
				<th class="w-44 px-3 py-2.5 text-right">
					<button
						class="flex items-center justify-end gap-1 w-full font-medium text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]"
						onclick={() => onSort('modified')}
					>
						Modified
						{#if sortBy === 'modified'}
							{#if sortDir === 'asc'}<ChevronUp class="h-3 w-3" />{:else}<ChevronDown class="h-3 w-3" />{/if}
						{/if}
					</button>
				</th>
			</tr>
		</thead>

		<tbody>
			{#if loading}
				{#each Array(10) as _}
					<tr class="border-b border-[var(--color-border)]/50" style="height: {ROW_HEIGHT}px">
						<td class="px-3"><div class="h-4 w-4 animate-pulse rounded bg-[var(--color-bg-tertiary)]"></div></td>
						<td class="px-3"><div class="h-3 w-48 animate-pulse rounded bg-[var(--color-bg-tertiary)]"></div></td>
						<td class="px-3"><div class="ml-auto h-3 w-14 animate-pulse rounded bg-[var(--color-bg-tertiary)]"></div></td>
						<td class="px-3"><div class="ml-auto h-3 w-28 animate-pulse rounded bg-[var(--color-bg-tertiary)]"></div></td>
					</tr>
				{/each}
			{:else if files.length === 0}
				<tr>
					<td colspan="4" class="px-3 py-24 text-center">
						<div class="mx-auto flex max-w-[280px] flex-col items-center gap-3">
							<div class="flex h-12 w-12 items-center justify-center rounded-2xl bg-emerald-500/[0.08]">
								<FolderOpen class="h-6 w-6 text-emerald-400/80" strokeWidth={1.5} />
							</div>
							<div class="space-y-1">
								<h4 class="text-sm font-medium text-zinc-300">No files in this folder</h4>
								<p class="text-[11px] leading-relaxed text-zinc-500">
									Drop files here or use the Upload button above.
								</p>
							</div>
						</div>
					</td>
				</tr>
			{:else}
				<!-- Top virtual spacer -->
				{#if topPadding > 0}
					<tr aria-hidden="true" style="height: {topPadding}px"><td colspan="4"></td></tr>
				{/if}

				{#each visibleFiles as item (item.path)}
					{@const IconComponent = getFileIcon(item)}
					{@const iconColor = getIconColor(item)}
					<tr
						style="height: {ROW_HEIGHT}px"
						class={cn(
							'cursor-pointer border-b border-[var(--color-border)]/30 transition-colors duration-75',
							selectedPaths.has(item.path)
								? 'bg-[var(--color-accent)]/10'
								: 'hover:bg-[var(--color-bg-tertiary)]/50'
						)}
						onclick={(e) => onSelect(item.path, e.metaKey || e.ctrlKey)}
						ondblclick={() => onOpen(item)}
						oncontextmenu={(e: MouseEvent) => { e.preventDefault(); onContextMenu(e, item); }}
						tabindex="0"
						onkeydown={(e) => e.key === 'Enter' && onOpen(item)}
					>
						<td class="px-3 text-center">
							<IconComponent class="h-4 w-4 {iconColor}" />
						</td>
						<td class="px-3 font-medium text-[var(--color-text-primary)] max-w-0">
							<span class="block truncate" title={item.name}>{item.name}</span>
						</td>
						<td class="px-3 text-right text-[var(--color-text-muted)] tabular-nums text-xs">
							{item.is_dir ? '—' : formatSize(item.size)}
						</td>
						<td class="px-3 text-right text-[var(--color-text-muted)] tabular-nums text-xs">
							{formatDate(item.modified)}
						</td>
					</tr>
				{/each}

				<!-- Bottom virtual spacer -->
				{#if bottomPadding > 0}
					<tr aria-hidden="true" style="height: {bottomPadding}px"><td colspan="4"></td></tr>
				{/if}
			{/if}
		</tbody>
	</table>
</div>
