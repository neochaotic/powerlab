/**
 * #38: FilePreview chooses its previewer from the listing's
 * server-classified `type`, falling back to the extension guess when
 * the backend predates it (`type: 0`).
 */

import { describe, it, expect, vi } from 'vitest';
import { render } from '@testing-library/svelte';
import FilePreview from './FilePreview.svelte';
import type { FileItem } from '$lib/api/files';

function item(name: string, type: string | number): FileItem {
	return {
		name, path: `/data/${name}`, is_dir: false, size: 10, modified: '2026-05-01T00:00:00Z',
		sign: '', thumb: '', type, date: '', extensions: null
	};
}

function renderPreview(i: FileItem) {
	return render(FilePreview, { item: i, onClose: vi.fn(), onOpenEditor: vi.fn() });
}

describe('FilePreview — server type (#38)', () => {
	it('plays an .mkv as video when the server says video', () => {
		const { container } = renderPreview(item('movie.mkv', 'video'));
		expect(container.querySelector('video')).not.toBeNull();
	});

	it('shows the audio player for type audio', () => {
		const { container } = renderPreview(item('song.flac', 'audio'));
		expect(container.querySelector('audio')).not.toBeNull();
	});

	it('shows no preview for an archive', () => {
		const { container, getByText } = renderPreview(item('backup.zip', 'archive'));
		expect(container.querySelector('img, video, audio, embed')).toBeNull();
		expect(getByText('No Preview Available')).toBeTruthy();
	});

	it('falls back to the extension guess for older backends (type 0)', () => {
		const { container } = renderPreview(item('photo.png', 0));
		expect(container.querySelector('img')).not.toBeNull();
	});
});
