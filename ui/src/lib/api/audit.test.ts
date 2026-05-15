import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { getAuditRecent, getAuditStats } from './audit';

// Contract tests for the audit API client. The endpoint shapes
// match backend/common/utils/audit/endpoints.go — change one,
// the other must change, and these tests will catch the drift.

describe('audit API client', () => {
	let originalFetch: typeof global.fetch;

	beforeEach(() => {
		originalFetch = global.fetch;
	});
	afterEach(() => {
		global.fetch = originalFetch;
	});

	function mockResponse(data: unknown, status = 200) {
		global.fetch = vi.fn().mockResolvedValue({
			ok: status >= 200 && status < 300,
			status,
			statusText: status === 200 ? 'OK' : String(status),
			headers: new Headers({ 'content-type': 'application/json' }),
			text: () => Promise.resolve(JSON.stringify({ data }))
		}) as unknown as typeof fetch;
	}

	it('getAuditRecent defaults: no query params when opts empty', async () => {
		mockResponse([]);
		await getAuditRecent();
		const url = (global.fetch as ReturnType<typeof vi.fn>).mock.calls[0][0] as string;
		expect(url).toBe('/v1/audit/recent');
	});

	it('getAuditRecent passes limit / user_id / since as query params', async () => {
		mockResponse([]);
		await getAuditRecent({ limit: 50, userId: 7, sinceUnixMicros: 1234 });
		const url = (global.fetch as ReturnType<typeof vi.fn>).mock.calls[0][0] as string;
		expect(url).toContain('limit=50');
		expect(url).toContain('user_id=7');
		expect(url).toContain('since=1234');
	});

	it('getAuditRecent returns [] when backend sends data:null', async () => {
		mockResponse(null);
		const rows = await getAuditRecent();
		expect(rows).toEqual([]);
	});

	it('getAuditRecent unwraps the data envelope into the array', async () => {
		const rec = {
			ts: '2026-05-14T01:00:00.000000Z',
			ts_us: 1234,
			method: 'GET',
			path: '/v1/foo',
			query: '',
			status: 200,
			latency_us: 42,
			user_id: 1,
			username: 'alice',
			remote_ip: '192.168.1.10',
			request_id: 'r1'
		};
		mockResponse([rec]);
		const rows = await getAuditRecent();
		expect(rows).toHaveLength(1);
		expect(rows[0].path).toBe('/v1/foo');
	});

	it('getAuditStats unwraps the data envelope', async () => {
		const stats = {
			row_count: 100,
			oldest_unix_us: 1000000,
			newest_unix_us: 2000000,
			file_size_bytes: 4096,
			path: '/var/lib/powerlab/gateway/audit.db'
		};
		mockResponse(stats);
		const result = await getAuditStats();
		expect(result.row_count).toBe(100);
		expect(result.file_size_bytes).toBe(4096);
	});

	it('getAuditRecent calls /v1/audit/stats endpoint (path contract)', async () => {
		mockResponse({ row_count: 0, oldest_unix_us: 0, newest_unix_us: 0, file_size_bytes: 0, path: '' });
		await getAuditStats();
		const url = (global.fetch as ReturnType<typeof vi.fn>).mock.calls[0][0] as string;
		expect(url).toBe('/v1/audit/stats');
	});
});
