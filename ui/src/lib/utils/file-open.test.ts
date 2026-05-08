/**
 * Bug #2 regression: clicking a file with an extension outside the
 * old narrow whitelist (`.txt`, `.md`, etc) opened a new browser tab
 * instead of the editor. The fix changes the decision to a positive
 * default — anything not previewable and not too large goes to the
 * editor.
 *
 * These tests pin down the new contract:
 *   - dotfiles, code files, config files → edit
 *   - images / video / audio / pdf       → preview
 *   - directories                        → navigate
 *   - large files                        → too-large
 */

import { describe, it, expect } from 'vitest';
import {
	decideOpenAction,
	isPreviewable,
	EDITOR_SIZE_LIMIT,
	type FileLike
} from './file-open';

const f = (overrides: Partial<FileLike>): FileLike => ({
	is_dir: false,
	name: 'unset',
	size: 1024,
	...overrides
});

describe('decideOpenAction', () => {
	it('navigates into directories', () => {
		expect(decideOpenAction(f({ is_dir: true, name: 'src' }))).toEqual({
			kind: 'navigate'
		});
	});

	describe('extensions that previously fell through to window.open (bug #2)', () => {
		it.each([
			'main.py',
			'server.go',
			'lib.rs',
			'app.rb',
			'index.php',
			'pom.xml',
			'pyproject.toml',
			'.env',
			'.gitignore',
			'.dockerignore',
			'.zshrc',
			'requirements.txt',
			'docker-compose.yaml',
			'Dockerfile',
			'Makefile',
			'package-lock.json',
			'config.json5',
			'styles.scss',
			'README'
		])('routes %s to the editor', (name) => {
			expect(decideOpenAction(f({ name }))).toEqual({ kind: 'edit' });
		});
	});

	describe('previewable media goes to the side pane', () => {
		it.each([
			'photo.jpg',
			'icon.PNG',
			'logo.svg',
			'demo.webm',
			'song.flac',
			'manual.pdf'
		])('routes %s to preview', (name) => {
			expect(decideOpenAction(f({ name }))).toEqual({ kind: 'preview' });
		});
	});

	it('refuses to load files larger than EDITOR_SIZE_LIMIT in the editor', () => {
		expect(
			decideOpenAction(f({ name: 'huge.log', size: EDITOR_SIZE_LIMIT + 1 }))
		).toEqual({ kind: 'too-large' });
	});

	it('routes a file exactly at the size limit to the editor (boundary)', () => {
		expect(
			decideOpenAction(f({ name: 'right-on-edge.log', size: EDITOR_SIZE_LIMIT }))
		).toEqual({ kind: 'edit' });
	});

	it('previewable files bypass the size limit (videos can be huge but stream)', () => {
		expect(
			decideOpenAction(f({ name: 'movie.mkv', size: EDITOR_SIZE_LIMIT * 100 }))
		).toEqual({ kind: 'preview' });
	});
});

describe('isPreviewable', () => {
	it('returns false for unknown extensions', () => {
		expect(isPreviewable('something.xyz')).toBe(false);
	});

	it('handles uppercase extensions', () => {
		expect(isPreviewable('Photo.JPG')).toBe(true);
	});

	it('returns false for files without an extension', () => {
		expect(isPreviewable('Makefile')).toBe(false);
	});

	it('returns false for filenames that end with a dot', () => {
		expect(isPreviewable('weird.')).toBe(false);
	});
});
