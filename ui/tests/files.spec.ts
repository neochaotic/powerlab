import { expect, test } from '@playwright/test';
import { installBaselineMocks } from './helpers/mock-backend';

// /files page smoke — verifies the file browser renders even when the
// /v1/file/dirpath endpoint returns an empty payload (catch-all mock).
//
// Per #108 — "list + folder navigation" coverage. The bug-#2 editor
// regression test (TextEditor save) lives in
// ui/src/lib/components/files/TextEditor.test.ts and is enforced
// separately by vitest; this E2E pass is the page-level smoke.

test.describe('/files page', () => {
	test.beforeEach(async ({ page }) => {
		await installBaselineMocks(page);
		// /v1/file/dirpath is the directory-listing endpoint. The
		// catch-all returns {success: 200, data: null}; the page needs
		// at least an empty array to render the empty-state without
		// throwing.
		await page.route('**/v1/file/dirpath**', (route) =>
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({
					success: 200,
					data: { content: [], total: 0, index: 1, size: 100000 }
				})
			})
		);
		await page.route('**/v1/file/get_default_path', (route) =>
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({ success: 200, data: '/' })
			})
		);
	});

	test('renders file browser shell', async ({ page }) => {
		await page.goto('/files');

		// Title contains PowerLab branding.
		await expect(page).toHaveTitle(/PowerLab/i);

		// AppHeader renders the page title bar — check it's present
		// rather than asserting on its exact i18n text.
		const header = page.locator('header').first();
		await expect(header).toBeVisible();
	});

	// Production-fidelity regression for #57. The vitest suite covers
	// `.cm-editor` mount in jsdom but jsdom does not run CodeMirror's
	// real input pipeline, so the production scenario reported on
	// v0.3.0 ("modal opens but text area is inert") needed a real
	// browser to truly disprove. This spec opens the editor through
	// the actual click flow, types into the CodeMirror surface, and
	// asserts the dirty-indicator flips on so we know keyboard input
	// reached CodeMirror's state.
	test('editor accepts keyboard input on first open (#57)', async ({ page }) => {
		await page.route('**/v1/folder?**', (route) =>
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({
					success: 200,
					data: {
						content: [
							{
								name: 'notes.txt',
								path: '/notes.txt',
								size: 11,
								is_dir: false,
								type: 'text/plain',
								modified: new Date().toISOString()
							}
						],
						total: 1,
						index: 1,
						size: 100000
					}
				})
			})
		);
		await page.route('**/v1/file/content**', (route) =>
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({ success: 200, data: 'hello world' })
			})
		);

		await page.goto('/files');

		// Click the row — single-click opens (filebrowser model).
		await page.getByText('notes.txt').click();

		// Editor modal mounts a CodeMirror surface — `.cm-editor` is the
		// library's outer wrapper and `.cm-content` is the actual
		// contenteditable target.
		const cmEditor = page.locator('.cm-editor').first();
		await expect(cmEditor).toBeVisible({ timeout: 5000 });

		const cmContent = page.locator('.cm-content').first();
		await expect(cmContent).toBeVisible();

		// Type via the real keyboard pipeline. If the v0.3.0 regression
		// were back (CodeMirror not attached, contenteditable inert),
		// this would either timeout or leave the doc unchanged.
		await cmContent.click();
		await page.keyboard.type(' edited');

		// The dirty-indicator (•) appears in the header next to the
		// file name once isDirty flips. That's the reactive proof that
		// CodeMirror's updateListener fired in response to typing.
		await expect(page.getByTitle(/unsaved|nao|sin guardar/i)).toBeVisible({ timeout: 2000 });
	});

	// Regression for #66: the Delete button only renders when
	// selection > 0, but the only ways to select were Cmd-click or
	// right-click → both undiscoverable. The row checkbox column
	// already exists; this asserts the header carries a select-all
	// affordance so a plain click + Delete is reachable without
	// chord shortcuts.
	test('header select-all toggles every row (#66)', async ({ page }) => {
		await page.route('**/v1/folder?**', (route) =>
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({
					success: 200,
					data: {
						content: [
							{ name: 'a.txt', path: '/a.txt', size: 1, is_dir: false, type: 'text/plain', modified: new Date().toISOString() },
							{ name: 'b.txt', path: '/b.txt', size: 1, is_dir: false, type: 'text/plain', modified: new Date().toISOString() }
						],
						total: 2,
						index: 1,
						size: 100000
					}
				})
			})
		);

		await page.goto('/files');
		await expect(page.getByText('a.txt')).toBeVisible();

		// Header select-all checkbox is the discoverability fix. It
		// lives in the first <th> and carries an aria-label so a
		// keyboard / screen-reader user can find it.
		const selectAll = page.getByLabel(/select all/i);
		await expect(selectAll).toBeVisible();

		await selectAll.check();

		// Every row checkbox should be checked once select-all toggled.
		const rowCheckboxes = page.locator('tbody input[type="checkbox"]');
		await expect(rowCheckboxes).toHaveCount(2);
		for (let i = 0; i < 2; i++) {
			await expect(rowCheckboxes.nth(i)).toBeChecked();
		}
	});
});
