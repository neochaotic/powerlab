/**
 * Preview class for a file in the Files page (#38).
 *
 * The backend now classifies each listing entry (`type` = video /
 * audio / image / text / pdf / archive / blob) so the preview drawer
 * no longer has to guess. Older backends sent `type: 0` (a CasaOS
 * leftover that was never set), so when the field is missing or not a
 * known class we fall back to the extension guess FilePreview used
 * before.
 */

export type FileKind = 'video' | 'audio' | 'image' | 'text' | 'pdf' | 'archive' | 'blob';

const KINDS = new Set<string>(['video', 'audio', 'image', 'text', 'pdf', 'archive', 'blob']);

export interface TypedFileLike {
	name: string;
	type?: unknown;
}

/** The pre-#38 client-side guess, kept for older backends. */
export function guessKindFromName(name: string): FileKind {
	const ext = name.split('.').pop()?.toLowerCase() || '';
	if (['jpg', 'jpeg', 'png', 'gif', 'webp', 'svg'].includes(ext)) return 'image';
	if (['mp4', 'webm', 'mov'].includes(ext)) return 'video';
	if (['mp3', 'flac', 'wav', 'ogg', 'm4a', 'aac'].includes(ext)) return 'audio';
	if (ext === 'pdf') return 'pdf';
	if (
		['txt', 'md', 'yaml', 'yml', 'json', 'conf', 'log', 'sh', 'js', 'ts', 'css', 'html', 'ini', 'xml', 'dockerfile'].includes(ext)
	) {
		return 'text';
	}
	return 'blob';
}

/** Server classification when present, else the extension guess. */
export function fileKind(item: TypedFileLike): FileKind {
	if (typeof item.type === 'string' && KINDS.has(item.type)) {
		return item.type as FileKind;
	}
	return guessKindFromName(item.name);
}
