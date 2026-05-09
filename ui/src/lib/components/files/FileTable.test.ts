/**
 * FileTable regression tests.
 *
 * Bug observed during v0.5.0 testing (#116 item 1, second half):
 * users have no discoverable way to select files. The pre-fix
 * click model was filebrowser-style — single-click opens, modifier-
 * click selects — and the only delete affordance is the toolbar
 * button that shows when selectedCount > 0. Without modifier-click
 * (touch devices, less-experienced users), files cannot be selected
 * and the Delete button never appears, making delete effectively
 * impossible.
 *
 * The fix adds an always-visible checkbox column. These tests lock
 * the behavior so a future redesign that drops the checkbox would
 * reintroduce the discoverability gap and fail this suite.
 */

import { describe, it, expect, vi } from 'vitest';
import { render, fireEvent } from '@testing-library/svelte';
import FileTable from './FileTable.svelte';
import type { FileItem } from '$lib/api/files';

// Minimal fixture covering all FileItem fields. The trailing fields
// (sign/thumb/type/date/extensions) are CasaOS legacy carryovers that
// the table doesn't render — they're here only to satisfy the type.
const sample: FileItem[] = [
	{ name: 'photos', path: '/data/photos', is_dir: true, size: 0, modified: '2026-05-01T00:00:00Z',
		sign: '', thumb: '', type: 0, date: '', extensions: null },
	{ name: 'notes.txt', path: '/data/notes.txt', is_dir: false, size: 1024, modified: '2026-05-08T12:00:00Z',
		sign: '', thumb: '', type: 0, date: '', extensions: null }
];

describe('FileTable — discoverable file selection (#116 regression)', () => {
	it('renders one selection checkbox per row', () => {
		const { container } = render(FileTable, {
			files: sample,
			selectedPaths: new Set<string>(),
			sortBy: 'name',
			sortDir: 'asc',
			loading: false,
			onSelect: () => {},
			onOpen: () => {},
			onSort: () => {},
			onContextMenu: () => {}
		});

		// Two files → two row checkboxes (header checkbox is separate).
		const rowCheckboxes = container.querySelectorAll('tbody input[type="checkbox"]');
		expect(rowCheckboxes.length).toBe(2);
	});

	it('clicking a row checkbox calls onSelect with that row path', async () => {
		const onSelect = vi.fn();

		const { container } = render(FileTable, {
			files: sample,
			selectedPaths: new Set<string>(),
			sortBy: 'name',
			sortDir: 'asc',
			loading: false,
			onSelect,
			onOpen: () => {},
			onSort: () => {},
			onContextMenu: () => {}
		});

		const checkboxes = container.querySelectorAll('tbody input[type="checkbox"]');
		// Click the second row's checkbox (notes.txt). Use change event
		// so Svelte reactivity fires the bound handler.
		await fireEvent.click(checkboxes[1] as HTMLInputElement);

		expect(onSelect).toHaveBeenCalled();
		const lastCallArgs = onSelect.mock.calls.at(-1);
		expect(lastCallArgs?.[0]).toBe('/data/notes.txt');
	});

	it('clicking the row checkbox does NOT trigger onOpen (clicks are independent)', async () => {
		const onOpen = vi.fn();
		const onSelect = vi.fn();

		const { container } = render(FileTable, {
			files: sample,
			selectedPaths: new Set<string>(),
			sortBy: 'name',
			sortDir: 'asc',
			loading: false,
			onSelect,
			onOpen,
			onSort: () => {},
			onContextMenu: () => {}
		});

		const checkboxes = container.querySelectorAll('tbody input[type="checkbox"]');
		await fireEvent.click(checkboxes[0] as HTMLInputElement);

		// Selection should fire, but the row's own click handler must
		// NOT propagate up — otherwise selecting also OPENS the file,
		// which is exactly the surprise the click model tries to avoid.
		expect(onSelect).toHaveBeenCalled();
		expect(onOpen).not.toHaveBeenCalled();
	});
});
