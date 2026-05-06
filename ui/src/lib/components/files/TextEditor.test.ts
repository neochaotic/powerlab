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
