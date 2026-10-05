/**
 * #38: FilePreview picks its previewer from the server's `type` field,
 * falling back to the old extension guess when talking to an older
 * backend (which sent `type: 0`).
 */

import { describe, it, expect } from 'vitest';
import { fileKind, guessKindFromName } from './file-type';

describe('fileKind', () => {
	it('uses the server type when it is a known class', () => {
		expect(fileKind({ name: 'movie.mkv', type: 'video' })).toBe('video');
		expect(fileKind({ name: 'song.flac', type: 'audio' })).toBe('audio');
		expect(fileKind({ name: 'backup.tar.gz', type: 'archive' })).toBe('archive');
		expect(fileKind({ name: 'main.go', type: 'text' })).toBe('text');
	});

	it('trusts the server over the extension', () => {
		// Server knows .mkv is video; the old client guess said blob.
		expect(guessKindFromName('movie.mkv')).toBe('blob');
		expect(fileKind({ name: 'movie.mkv', type: 'video' })).toBe('video');
		expect(fileKind({ name: 'x.png', type: 'blob' })).toBe('blob');
	});

	it('falls back to the extension guess for older backends', () => {
		expect(fileKind({ name: 'photo.png', type: 0 })).toBe('image');
		expect(fileKind({ name: 'clip.mp4' })).toBe('video');
		expect(fileKind({ name: 'track.mp3', type: '' })).toBe('audio');
		expect(fileKind({ name: 'doc.pdf', type: 'application/pdf' })).toBe('pdf');
		expect(fileKind({ name: 'notes.md', type: null })).toBe('text');
		expect(fileKind({ name: 'firmware.bin', type: 0 })).toBe('blob');
	});
});
