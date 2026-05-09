/**
 * TextEditor regression tests.
 *
 * Bug shipped on 2026-05-06: I changed GET /v1/file/content to return
 * 404 for missing files (good — proper REST), but client.ts throws
 * on every non-2xx, and TextEditor's onMount caught the throw and
 * showed "Could not open file" with no editor surface. The user
 * could not type a NEW file in the editor at all — the very flow
 * they reported as broken.
 *
 * The API-level E2E (scripts/test-linux-e2e.sh) only verified
 * `PUT /v1/file` succeeds for missing paths; it never exercised
 * the browser flow that opens the editor first. THIS is the test
 * that should have existed and would have caught the regression.
 *
 * What this asserts:
 *   - 200 response → editor renders existing content
 *   - 404 response → editor opens in "new file" mode (empty,
 *     no error, "New" badge visible) so the user can type and Save
 *   - Other errors (500, network) → error surface, no editor
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, waitFor } from '@testing-library/svelte';
import TextEditor from './TextEditor.svelte';

function mockResponse(body: unknown, status = 200) {
	return vi.fn().mockResolvedValue({
		ok: status >= 200 && status < 300,
		status,
		statusText: 'OK',
		headers: new Headers({ 'content-type': 'application/json' }),
		text: () => Promise.resolve(JSON.stringify(body))
	});
}

describe('TextEditor — open existing file (200)', () => {
	beforeEach(() => vi.restoreAllMocks());

	it('reads content and shows the path with no "New" badge', async () => {
		vi.stubGlobal(
			'fetch',
			mockResponse({ success: 200, message: 'ok', data: 'hello world' })
		);

		const { findByText, queryByText } = render(TextEditor, {
			path: '/tmp/existing.txt',
			onClose: () => {}
		});

		await findByText('existing.txt');
		expect(queryByText('New')).toBeNull();
	});
});

describe('TextEditor — open NEW file (404)', () => {
	beforeEach(() => vi.restoreAllMocks());

	it('does NOT show error; shows "New" badge so the user can start typing', async () => {
		// 404 from the backend triggers an exception in client.ts
		// (status not in 2xx range). TextEditor must catch that and
		// switch to "new file" mode rather than blocking with a
		// "Could not open file" error.
		vi.stubGlobal(
			'fetch',
			mockResponse({ success: 60001, message: 'file does not exist' }, 404)
		);

		const { findByText, queryByText } = render(TextEditor, {
			path: '/tmp/brand-new.txt',
			onClose: () => {}
		});

		// "New" badge appears — proves the new-file branch ran.
		await findByText('New');

		// And the "Could not open file" error surface must NOT appear.
		expect(queryByText('Could not open file')).toBeNull();
	});
});

describe('TextEditor — CodeMirror actually mounts (race-condition regression)', () => {
	beforeEach(() => vi.restoreAllMocks());

	// Bug shipped on 2026-05-06: initEditor() was called inside
	// onMount BEFORE the spinner branch unmounted and the editor
	// div bound to editorContainer. CodeMirror's `parent:
	// editorContainer` was null, the early-return ran, and the
	// modal stayed grey forever — modal opens, no typing, Save
	// disabled. User: "consigo criar o arquivo mas dai editar
	// nao funciona e nem salvar".
	it('mounts a CodeMirror surface that responds to typing (.cm-editor in DOM)', async () => {
		vi.stubGlobal(
			'fetch',
			mockResponse({ success: 200, message: 'ok', data: 'initial content' })
		);

		const { container, findByText } = render(TextEditor, {
			path: '/tmp/code-mirror-mount.txt',
			onClose: () => {}
		});

		// Wait for the file name to render — proves loading completed
		// and the body branch (which contains the editor div) flipped.
		await findByText('code-mirror-mount.txt');

		// CodeMirror injects a `.cm-editor` element inside the bound
		// container when initEditor runs successfully. If the race
		// condition comes back, this query returns null and the test
		// fails — fast feedback before the modal ever ships grey.
		await waitFor(
			() => {
				const cmEditor = container.querySelector('.cm-editor');
				expect(cmEditor).toBeTruthy();
			},
			{ timeout: 1500 }
		);
	});
});

describe('TextEditor — backend 500 still surfaces error', () => {
	beforeEach(() => vi.restoreAllMocks());

	it('shows the error UI for genuine server failures', async () => {
		vi.stubGlobal(
			'fetch',
			mockResponse({ message: 'database unreachable' }, 500)
		);

		const { findByText, queryByText } = render(TextEditor, {
			path: '/tmp/some.txt',
			onClose: () => {}
		});

		await findByText('Could not open file');
		expect(queryByText('New')).toBeNull();
	});
});

describe('TextEditor — save success emits toast (#116 regression)', () => {
	beforeEach(() => vi.restoreAllMocks());

	// Bug observed during v0.5.0 testing (#116 item 1): user saves a
	// text file in the editor, the save persists to disk, but no
	// success toast appears. This test was missing — that absence is
	// exactly how the regression got through.
	//
	// Tests the new-file path (404 → POST), where the Save button is
	// enabled from the start (no need to fight CodeMirror to flip
	// isDirty). The save handler's success branch is the same code
	// for create and update — exercising create gives us coverage of
	// the toast call without jsdom-vs-CodeMirror friction.
	it('calls toast.success with "Created {name}" after a successful save', async () => {
		// First fetch (GET) returns 404 → editor opens in new-file
		// mode. Second fetch (POST /v1/file) returns success.
		let fetchCallCount = 0;
		const fetchMock = vi.fn(async (_url: unknown, opts?: RequestInit) => {
			fetchCallCount++;
			const method = opts?.method || 'GET';
			if (method === 'GET') {
				return {
					ok: false,
					status: 404,
					statusText: 'Not Found',
					headers: new Headers({ 'content-type': 'application/json' }),
					text: () => Promise.resolve(JSON.stringify({
						success: 60001,
						message: 'file does not exist'
					}))
				} as Response;
			}
			// POST or PUT — successful save.
			return {
				ok: true,
				status: 200,
				statusText: 'OK',
				headers: new Headers({ 'content-type': 'application/json' }),
				text: () => Promise.resolve(JSON.stringify({
					success: 200,
					message: 'ok',
					data: null
				}))
			} as Response;
		});
		vi.stubGlobal('fetch', fetchMock);

		const { toast } = await import('$lib/stores/toast.svelte');
		const successSpy = vi.spyOn(toast, 'success');

		const { findByText, getByRole } = render(TextEditor, {
			path: '/tmp/brand-new-save.txt',
			onClose: () => {}
		});

		// Wait for new-file mode — proves load completed in 404 branch
		// and Save button is therefore enabled (isNewFile=true).
		await findByText('New');

		// Save button is now active. Click.
		const saveBtn = getByRole('button', {
			name: /save|salvar|guardar|create|criar|crear/i
		});
		saveBtn.click();

		// Toast should fire with "Created {name}" / "Criado {name}".
		// Assert success() was invoked with the file name embedded.
		await waitFor(
			() => {
				expect(successSpy).toHaveBeenCalled();
				const lastCall = successSpy.mock.calls.at(-1);
				expect(lastCall?.[0]).toContain('brand-new-save.txt');
			},
			{ timeout: 1500 }
		);

		// Sanity: at least 2 fetch calls (initial GET + the save).
		expect(fetchCallCount).toBeGreaterThanOrEqual(2);
	});

	// Same regression class but exercising the existing-file PUT path.
	// SKIPPED in vitest because jsdom does not run CodeMirror's input
	// pipeline reliably enough to flip isDirty. The PUT save path is
	// covered by the Playwright suite (#108) once the Files-page
	// per-area tests land — until then, the create-flow test above
	// is the regression guard, since the success branch in handleSave
	// is symmetric between create and update (both call
	// toast.success(...)). A bug specific to the PUT path would be
	// in the API response handling, not the toast emission.
	it.skip('calls toast.success on PUT save of an existing file', async () => {
		const fetchMock = vi.fn(async (_url: unknown, opts?: RequestInit) => {
			const method = opts?.method || 'GET';
			const body = method === 'PUT'
				? { success: 200, message: 'ok', data: null }
				: { success: 200, message: 'ok', data: 'starting content' };
			return {
				ok: true,
				status: 200,
				statusText: 'OK',
				headers: new Headers({ 'content-type': 'application/json' }),
				text: () => Promise.resolve(JSON.stringify(body))
			} as Response;
		});
		vi.stubGlobal('fetch', fetchMock);

		const { toast } = await import('$lib/stores/toast.svelte');
		const successSpy = vi.spyOn(toast, 'success');

		const { container, findByText } = render(TextEditor, {
			path: '/tmp/edit-existing.txt',
			onClose: () => {}
		});

		await findByText('edit-existing.txt');
		await waitFor(
			() => {
				expect(container.querySelector('.cm-editor')).toBeTruthy();
			},
			{ timeout: 1500 }
		);

		// Reach into the rendered DOM to find the CodeMirror EditorView
		// instance via the cmView property the library exposes on
		// .cm-content. Dispatching a transaction simulates a real edit
		// and triggers updateListener → isDirty=true.
		const cmContent = container.querySelector('.cm-content') as
			| (HTMLElement & { cmView?: { view?: unknown } })
			| null;
		if (cmContent?.cmView?.view) {
			const view = cmContent.cmView.view as {
				dispatch: (tr: { changes: { from: number; insert: string } }) => void;
				state: { doc: { length: number } };
			};
			view.dispatch({
				changes: { from: view.state.doc.length, insert: 'X' }
			});
		}

		await waitFor(
			() => {
				expect(successSpy).not.toHaveBeenCalled(); // not yet — we haven't clicked save
				const btn = container.querySelector('button.bg-emerald-500');
				expect(btn).toBeTruthy();
				expect(btn).not.toHaveAttribute('disabled');
			},
			{ timeout: 1500 }
		);

		// Click the green Save button.
		const saveBtn = container.querySelector('button.bg-emerald-500') as HTMLButtonElement;
		saveBtn.click();

		await waitFor(
			() => {
				expect(successSpy).toHaveBeenCalled();
				const lastCall = successSpy.mock.calls.at(-1);
				expect(lastCall?.[0]).toContain('edit-existing.txt');
			},
			{ timeout: 1500 }
		);
	});
});
