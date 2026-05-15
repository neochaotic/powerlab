import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/svelte';
import AuditPane from './AuditPane.svelte';

(globalThis as unknown as { __APP_VERSION__: string }).__APP_VERSION__ = 'test';

// Localized mock of the audit API module.
vi.mock('$lib/api/audit', () => ({
	getAuditRecent: vi.fn(),
	getAuditStats: vi.fn()
}));

import { getAuditRecent, getAuditStats } from '$lib/api/audit';

function rec(overrides: Record<string, unknown> = {}) {
	return {
		ts: new Date().toISOString(),
		ts_us: Date.now() * 1000,
		method: 'GET',
		path: '/v1/foo',
		query: '',
		status: 200,
		latency_us: 1234,
		user_id: 1,
		username: 'alice',
		remote_ip: '192.168.1.10',
		request_id: 'req-1',
		...overrides
	};
}

describe('AuditPane', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it('renders stats and rows on mount', async () => {
		vi.mocked(getAuditRecent).mockResolvedValueOnce([rec({ request_id: 'r1', path: '/v1/one' }), rec({ request_id: 'r2', path: '/v1/two' })]);
		vi.mocked(getAuditStats).mockResolvedValueOnce({
			row_count: 47329,
			oldest_unix_us: Date.now() * 1000 - 86_400_000_000,
			newest_unix_us: Date.now() * 1000,
			file_size_bytes: 1024 * 1024 * 12,
			path: '/var/lib/powerlab/gateway/audit.db'
		});

		render(AuditPane);

		await waitFor(() => {
			expect(getAuditRecent).toHaveBeenCalled();
			expect(getAuditStats).toHaveBeenCalled();
		});

		// Stats card shows row count with toLocaleString formatting.
		await waitFor(() => expect(screen.getByText(/47,329/)).toBeTruthy());

		// Both rows rendered.
		await waitFor(() => {
			expect(screen.getByText('/v1/one')).toBeTruthy();
			expect(screen.getByText('/v1/two')).toBeTruthy();
		});
	});

	it('shows "no records" when backend returns empty', async () => {
		vi.mocked(getAuditRecent).mockResolvedValueOnce([]);
		vi.mocked(getAuditStats).mockResolvedValueOnce({
			row_count: 0,
			oldest_unix_us: 0,
			newest_unix_us: 0,
			file_size_bytes: 0,
			path: ''
		});

		render(AuditPane);

		await waitFor(() => expect(screen.getByText(/no audit records yet/i)).toBeTruthy());
	});

	it('surfaces non-401 errors in the error banner', async () => {
		vi.mocked(getAuditRecent).mockRejectedValueOnce({ status: 500, message: 'boom' });
		vi.mocked(getAuditStats).mockRejectedValueOnce({ status: 500, message: 'boom' });

		render(AuditPane);

		await waitFor(() => expect(screen.getByText(/Could not load audit log/i)).toBeTruthy());
	});

	it('suppresses 401 from the banner — onAuthError handles it centrally', async () => {
		vi.mocked(getAuditRecent).mockRejectedValueOnce({ status: 401, message: 'unauthorized' });
		vi.mocked(getAuditStats).mockRejectedValueOnce({ status: 401, message: 'unauthorized' });

		render(AuditPane);

		// Wait for the loader to settle, then assert the banner is NOT there.
		await waitFor(() => expect(screen.queryByText(/Could not load audit log/i)).toBeNull());
	});

	it('renders nullable user_id and username gracefully', async () => {
		vi.mocked(getAuditRecent).mockResolvedValueOnce([
			rec({ user_id: null, username: null, path: '/v1/probe' })
		]);
		vi.mocked(getAuditStats).mockResolvedValueOnce({
			row_count: 1,
			oldest_unix_us: 0,
			newest_unix_us: 0,
			file_size_bytes: 0,
			path: ''
		});

		render(AuditPane);

		await waitFor(() => expect(screen.getByText('/v1/probe')).toBeTruthy());
		// User cell renders the em-dash sentinel for null user.
		expect(screen.getAllByText('—').length).toBeGreaterThan(0);
	});

	it('renders ui_error rows with the bug badge + error message', async () => {
		vi.mocked(getAuditRecent).mockResolvedValueOnce([
			rec({
				kind: 'ui_error',
				method: 'POST',
				path: '/v1/audit/frontend-error',
				status: 202,
				payload: {
					message: "TypeError: Cannot read properties of undefined (reading 'foo')",
					stack: 'at /apps/+page.svelte:42:7\nat onMount',
					url: '/apps',
					ua: 'Mozilla/5.0',
					viewport: { w: 1920, h: 1080 }
				}
			})
		]);
		vi.mocked(getAuditStats).mockResolvedValueOnce({
			row_count: 1,
			oldest_unix_us: 0,
			newest_unix_us: 0,
			file_size_bytes: 0,
			path: ''
		});

		render(AuditPane);

		// The dedicated row renders with the testid + the bug badge.
		const row = await screen.findByTestId('audit-row-ui-error');
		expect(row).toBeTruthy();
		expect(row.textContent).toContain('TypeError');
		expect(row.textContent?.toLowerCase()).toContain('ui error');
	});

	it('toggles the ui_error detail row on click (expand stack)', async () => {
		vi.mocked(getAuditRecent).mockResolvedValueOnce([
			rec({
				kind: 'ui_error',
				status: 202,
				payload: {
					message: 'boom',
					stack: 'at /settings:1:1\nat doStuff'
				}
			})
		]);
		vi.mocked(getAuditStats).mockResolvedValueOnce({
			row_count: 1,
			oldest_unix_us: 0,
			newest_unix_us: 0,
			file_size_bytes: 0,
			path: ''
		});

		render(AuditPane);

		const row = await screen.findByTestId('audit-row-ui-error');
		// Initially collapsed.
		expect(screen.queryByTestId('audit-row-ui-error-detail')).toBeNull();
		row.dispatchEvent(new MouseEvent('click', { bubbles: true }));
		const detail = await screen.findByTestId('audit-row-ui-error-detail');
		expect(detail.textContent).toContain('doStuff');
	});
});
