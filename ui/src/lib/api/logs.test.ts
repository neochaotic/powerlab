import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { listLogFiles, readLogFile, type LogFileEntry } from './logs';

// Contract tests for the logs API client. Endpoint shapes match
// backend/gateway/route/logs.go — change one, the other must change.

describe('logs API client', () => {
	let originalFetch: typeof global.fetch;

	beforeEach(() => {
		originalFetch = global.fetch;
	});
	afterEach(() => {
		global.fetch = originalFetch;
	});

	function mockJsonResponse(data: unknown, status = 200) {
		global.fetch = vi.fn().mockResolvedValue({
			ok: status >= 200 && status < 300,
			status,
			statusText: status === 200 ? 'OK' : String(status),
			headers: new Headers({ 'content-type': 'application/json' }),
			text: () => Promise.resolve(JSON.stringify({ data }))
		}) as unknown as typeof fetch;
	}

	function mockTextResponse(body: string, status = 200) {
		global.fetch = vi.fn().mockResolvedValue({
			ok: status >= 200 && status < 300,
			status,
			statusText: status === 200 ? 'OK' : String(status),
			headers: new Headers({ 'content-type': 'text/plain' }),
			text: () => Promise.resolve(body)
		}) as unknown as typeof fetch;
	}

	it('listLogFiles GETs /v1/logs/files', async () => {
		mockJsonResponse([]);
		await listLogFiles();
		const url = (global.fetch as ReturnType<typeof vi.fn>).mock.calls[0][0] as string;
		expect(url).toBe('/v1/logs/files');
	});

	it('listLogFiles unwraps the data envelope into LogFileEntry[]', async () => {
		const entries: LogFileEntry[] = [
			{
				name: 'app-management.log',
				size_bytes: 12345,
				modified_ts: '2026-05-18T00:00:00Z',
				modified_us: 1779062400000000
			},
			{
				name: 'gateway.log',
				size_bytes: 6789,
				modified_ts: '2026-05-18T00:01:00Z',
				modified_us: 1779062460000000
			}
		];
		mockJsonResponse(entries);
		const out = await listLogFiles();
		expect(out).toEqual(entries);
	});

	it('readLogFile GETs /v1/logs/files/{name} with no query when tail omitted', async () => {
		mockTextResponse('log contents');
		await readLogFile('app-management.log');
		const url = (global.fetch as ReturnType<typeof vi.fn>).mock.calls[0][0] as string;
		expect(url).toBe('/v1/logs/files/app-management.log');
	});

	it('readLogFile appends ?tail=N when tail provided', async () => {
		mockTextResponse('tail bytes');
		await readLogFile('app-management.log', 1024);
		const url = (global.fetch as ReturnType<typeof vi.fn>).mock.calls[0][0] as string;
		expect(url).toBe('/v1/logs/files/app-management.log?tail=1024');
	});

	it('readLogFile url-encodes the filename', async () => {
		// Defensive — backend's allow-list rejects path-traversal, but
		// the client should never send an unencoded filename to begin
		// with so a "weird but valid" name (spaces, unicode) doesn't
		// break the URL parser server-side.
		mockTextResponse('contents');
		await readLogFile('app with spaces.log');
		const url = (global.fetch as ReturnType<typeof vi.fn>).mock.calls[0][0] as string;
		expect(url).toBe('/v1/logs/files/app%20with%20spaces.log');
	});

	it('readLogFile returns the raw text body, no envelope unwrap', async () => {
		// Distinct from listLogFiles: this endpoint returns text/plain
		// directly, not JSON-wrapped. If the api client tries to JSON-
		// parse the response we'd lose the log body.
		const body = 'line 1\nline 2\nline 3\n';
		mockTextResponse(body);
		const out = await readLogFile('gateway.log');
		expect(out).toBe(body);
	});

	it('readLogFile passes Accept: text/plain header to override JSON parsing', async () => {
		mockTextResponse('contents');
		await readLogFile('app-management.log');
		const opts = (global.fetch as ReturnType<typeof vi.fn>).mock.calls[0][1] as RequestInit;
		expect((opts.headers as Record<string, string>)?.Accept).toBe('text/plain');
	});
});
