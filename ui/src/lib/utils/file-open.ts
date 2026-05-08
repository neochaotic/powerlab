/**
 * Decide what should happen when the user clicks/double-clicks a
 * file row in the Files page.
 *
 * Pulled out of routes/files/+page.svelte for testability and to
 * fix bug #2 (v0.3.1): the previous handler had a narrow editable-
 * extensions whitelist (`.txt`, `.md`, `.yaml`, etc) — anything
 * outside it (e.g. `.py`, `.go`, `.toml`, `.env`, dotfiles) fell
 * through to `window.open(downloadUrl, '_blank')`, opening the file
 * in a new browser tab. Users expected the editor.
 *
 * The new contract:
 *   - directory  → navigate
 *   - previewable (image/video/audio/pdf) → preview pane
 *   - too large (> EDITOR_SIZE_LIMIT)     → download via right-click
 *   - everything else                     → editor
 *
 * Editing is the safe default for unknown text-y files. Users who
 * want a download still have right-click → Download in the context
 * menu. Real binaries are blocked by the size limit, not extension
 * sniffing — fewer surprises than maintaining a binary blacklist
 * that's always one extension behind.
 */

export interface FileLike {
	is_dir: boolean;
	name: string;
	size: number;
}

export type OpenAction =
	| { kind: 'navigate' }
	| { kind: 'preview' }
	| { kind: 'edit' }
	| { kind: 'too-large' };

// 10 MB. CodeMirror gracefully handles 1 MB but starts feeling slow
// past 5 MB; 10 MB is a sane "this is probably a binary log" cutoff.
// Keep in sync with the toast copy.
export const EDITOR_SIZE_LIMIT = 10 * 1024 * 1024;

const PREVIEWABLE_EXTS = new Set([
	'jpg', 'jpeg', 'png', 'gif', 'webp', 'svg',
	'mp4', 'webm', 'mov', 'mkv',
	'mp3', 'flac', 'wav', 'ogg', 'm4a', 'aac',
	'pdf'
]);

function getExt(name: string): string | undefined {
	// Special case: filenames that BEGIN with a dot but have no further
	// extension (".gitignore", ".env") — treat the part after the dot
	// as the "extension" so we can route them like text files. The
	// decision below will fall through to 'edit' for these, which is
	// what the user wants when they click a dotfile.
	if (name.startsWith('.') && !name.slice(1).includes('.')) {
		return name.slice(1).toLowerCase();
	}
	const idx = name.lastIndexOf('.');
	if (idx < 0 || idx === name.length - 1) return undefined;
	return name.slice(idx + 1).toLowerCase();
}

export function isPreviewable(name: string): boolean {
	const ext = getExt(name);
	return !!ext && PREVIEWABLE_EXTS.has(ext);
}

export function decideOpenAction(file: FileLike): OpenAction {
	if (file.is_dir) return { kind: 'navigate' };
	if (isPreviewable(file.name)) return { kind: 'preview' };
	if (file.size > EDITOR_SIZE_LIMIT) return { kind: 'too-large' };
	return { kind: 'edit' };
}
