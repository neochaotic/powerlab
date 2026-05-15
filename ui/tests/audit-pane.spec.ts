import { expect, test } from '@playwright/test';
import { installBaselineMocks } from './helpers/mock-backend';

// AuditPane E2E (Sprint 16 #357 B1f). Locks the contract that the
// pane reads /v1/audit/recent + /v1/audit/stats and renders the
// stats card + the records table without errors. Catches:
//   - Wire shape drift between backend Record/StatsResult and the
//     UI's audit.ts types.
//   - Settings sidebar regression dropping the Audit nav entry.
//   - Render regression hiding the table or the empty state.

const FIXED_NOW_MICROS = 1_700_000_000_000_000; // arbitrary, stable

const FIXTURE_RECORDS = [
	{
		id: 1,
		ts_unix_us: FIXED_NOW_MICROS,
		method: 'GET',
		path: '/v1/audit/playwright-fixture',
		query: '',
		status: 200,
		latency_us: 1234,
		user_id: 1,
		username: 'alice',
		remote_ip: '192.168.1.10',
		request_id: 'pw-1'
	},
	{
		id: 2,
		ts_unix_us: FIXED_NOW_MICROS - 60_000_000,
		method: 'POST',
		path: '/v1/audit/another-row',
		query: '',
		status: 401,
		latency_us: 567,
		user_id: null,
		username: null,
		remote_ip: 'loopback',
		request_id: ''
	}
];

const FIXTURE_STATS = {
	row_count: 47329,
	oldest_unix_us: FIXED_NOW_MICROS - 30 * 24 * 60 * 60 * 1_000_000,
	newest_unix_us: FIXED_NOW_MICROS,
	file_size_bytes: 12 * 1024 * 1024,
	path: '/var/lib/powerlab/gateway/audit.db'
};

test.describe('Settings → Audit pane E2E', () => {
	test.beforeEach(async ({ page }) => {
		await installBaselineMocks(page);

		// Stub the two endpoints the pane consumes.
		await page.route('**/v1/audit/recent**', (route) =>
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({ data: FIXTURE_RECORDS })
			})
		);
		await page.route('**/v1/audit/stats', (route) =>
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({ data: FIXTURE_STATS })
			})
		);
	});

	test('Audit nav entry mounts pane → stats + table render', async ({ page }) => {
		const consoleErrors: string[] = [];
		page.on('console', (m) => {
			if (m.type() === 'error') consoleErrors.push(m.text());
		});

		// Deep-link to the audit section so we don't depend on
		// click-driving the sidebar (which has its own rendering
		// tests). The pane's onMount fires the two fetches.
		await page.goto('/settings#audit');

		// Stats card row count (toLocaleString → "47,329")
		await expect(page.getByText('47,329')).toBeVisible({ timeout: 10_000 });

		// Records table mounts with both fixture rows
		const tbl = page.getByTestId('audit-table');
		await expect(tbl).toBeVisible();
		await expect(tbl.getByText('/v1/audit/playwright-fixture')).toBeVisible();
		await expect(tbl.getByText('/v1/audit/another-row')).toBeVisible();

		// Status colors: 200 green, 401 amber — render at least the
		// numbers so a regression that drops the status column is
		// caught.
		await expect(tbl.getByText('200', { exact: true })).toBeVisible();
		await expect(tbl.getByText('401', { exact: true })).toBeVisible();

		// Loopback row renders the literal "loopback" sentinel from
		// the backend (ADR-0033 PII). If the UI ever munges it back to
		// 127.0.0.1, this catches.
		await expect(tbl.getByText('loopback')).toBeVisible();

		// Null user fallback: row 2 has user_id=null/username=null.
		// The pane renders the em-dash sentinel — at least one cell
		// in the body should have it.
		await expect(tbl.getByText('—').first()).toBeVisible();

		// No console errors during render — rules out reactive
		// effects throwing on the new code path.
		expect(consoleErrors, `console errors during render: ${consoleErrors.join(' | ')}`).toEqual([]);
	});

	test('empty backend → "no records" empty state', async ({ page }) => {
		// Override the recent fixture to empty for this test.
		await page.route('**/v1/audit/recent**', (route) =>
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({ data: [] })
			})
		);
		await page.route('**/v1/audit/stats', (route) =>
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({
					data: { ...FIXTURE_STATS, row_count: 0 }
				})
			})
		);

		await page.goto('/settings#audit');
		await expect(page.getByText(/no audit records yet/i)).toBeVisible({ timeout: 10_000 });
	});
});
