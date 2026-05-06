import { describe, it, expect, vi, beforeEach } from 'vitest';
import { api, setAuthToken, addRequestInterceptor, addResponseInterceptor } from './client';

// Helper to create a standard fetch mock that uses response.text() (as the client does)
function mockFetch(data: unknown, status = 200, contentType = 'application/json') {
	const body = contentType === 'application/json' ? JSON.stringify(data) : String(data);
	return vi.fn().mockResolvedValue({
		ok: status >= 200 && status < 300,
		status,
		statusText: status === 200 ? 'OK' : status === 401 ? 'Unauthorized' : String(status),
		headers: new Headers(contentType ? { 'content-type': contentType } : {}),
		text: () => Promise.resolve(body)
	});
}

function mockFetchError(status: number, message: string) {
	return vi.fn().mockResolvedValue({
		ok: false,
		status,
		statusText: 'Error',
		headers: new Headers({ 'content-type': 'application/json' }),
		text: () => Promise.resolve(JSON.stringify({ message }))
	});
}

describe('API Client', () => {
	beforeEach(() => {
		setAuthToken(null);
		vi.restoreAllMocks();
	});

	// ─── Basic HTTP Methods ───────────────────────────────────────────────

	it('GET: parses JSON response', async () => {
		const mockData = { data: { running: ['casaos-gateway.service'] }, message: '' };
		vi.stubGlobal('fetch', mockFetch(mockData));

		const result = await api.get('/v2/casaos/health/services');

		expect(result).toEqual(mockData);
		expect(fetch).toHaveBeenCalledWith(
			'/v2/casaos/health/services',
			expect.objectContaining({ method: 'GET' })
		);
	});

	it('POST: sends JSON body', async () => {
		vi.stubGlobal('fetch', mockFetch({ message: 'ok' }));

		await api.post('/v2/app_management/compose/test/status', { status: 'start' });

		expect(fetch).toHaveBeenCalledWith(
			'/v2/app_management/compose/test/status',
			expect.objectContaining({
				method: 'POST',
				body: JSON.stringify({ status: 'start' })
			})
		);
	});

	it('PUT: sends JSON body', async () => {
		vi.stubGlobal('fetch', mockFetch({ message: 'ok' }));

		await api.put('/v1/file/name', { old_path: '/a', new_path: '/b' });

		expect(fetch).toHaveBeenCalledWith(
			'/v1/file/name',
			expect.objectContaining({
				method: 'PUT',
				body: JSON.stringify({ old_path: '/a', new_path: '/b' })
			})
		);
	});

	it('PATCH: sends JSON body', async () => {
		vi.stubGlobal('fetch', mockFetch({ message: 'ok' }));

		await api.patch('/v1/test', { key: 'value' });

		expect(fetch).toHaveBeenCalledWith(
			'/v1/test',
			expect.objectContaining({
				method: 'PATCH',
				body: JSON.stringify({ key: 'value' })
			})
		);
	});

	it('DELETE: sends correct method', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn().mockResolvedValue({
				ok: true,
				status: 204,
				statusText: 'No Content',
				headers: new Headers(),
				text: () => Promise.resolve('')
			})
		);

		const result = await api.delete('/v2/message_bus/ysk/123');

		expect(result).toBeUndefined();
		expect(fetch).toHaveBeenCalledWith(
			'/v2/message_bus/ysk/123',
			expect.objectContaining({ method: 'DELETE' })
		);
	});

	it('postYaml: sends YAML content-type', async () => {
		vi.stubGlobal('fetch', mockFetch({ message: 'ok' }));

		await api.postYaml('/v2/app_management/compose', 'services:\n  app:\n    image: nginx');

		expect(fetch).toHaveBeenCalledWith(
			'/v2/app_management/compose',
			expect.objectContaining({
				method: 'POST',
				headers: expect.objectContaining({ 'Content-Type': 'application/yaml' }),
				body: 'services:\n  app:\n    image: nginx'
			})
		);
	});

	// ─── Authentication ───────────────────────────────────────────────────

	it('includes Authorization header when token is set', async () => {
		setAuthToken('Bearer test-jwt-token');
		vi.stubGlobal('fetch', mockFetch({}));

		await api.get('/v2/casaos/health/services');

		expect(fetch).toHaveBeenCalledWith(
			'/v2/casaos/health/services',
			expect.objectContaining({
				headers: expect.objectContaining({ Authorization: 'Bearer test-jwt-token' })
			})
		);
	});

	it('omits Authorization header when token is null', async () => {
		setAuthToken(null);
		vi.stubGlobal('fetch', mockFetch({}));

		await api.get('/v1/sys/utilization');

		const callArgs = (fetch as ReturnType<typeof vi.fn>).mock.calls[0][1] as RequestInit;
		expect((callArgs.headers as Record<string, string>)['Authorization']).toBeUndefined();
	});

	// ─── Error Handling ───────────────────────────────────────────────────

	it('throws ApiError with status and message on 401', async () => {
		vi.stubGlobal('fetch', mockFetchError(401, 'Invalid token'));

		await expect(api.get('/v2/casaos/health/services')).rejects.toMatchObject({
			status: 401,
			message: 'Invalid token'
		});
	});

	it('throws ApiError on 403 Forbidden', async () => {
		vi.stubGlobal('fetch', mockFetchError(403, 'Forbidden'));

		await expect(api.get('/v1/sys/utilization')).rejects.toMatchObject({
			status: 403,
			message: 'Forbidden'
		});
	});

	it('throws ApiError on 500 with raw body when response is not JSON', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn().mockResolvedValue({
				ok: false,
				status: 500,
				statusText: 'Internal Server Error',
				headers: new Headers({ 'content-type': 'text/plain' }),
				text: () => Promise.resolve('Internal Server Error')
			})
		);

		await expect(api.get('/v1/fail')).rejects.toMatchObject({
			status: 500
		});
	});

	it('throws network ApiError (status 0) when fetch throws', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn().mockRejectedValue(new Error('Network failed'))
		);

		// Allow 3 total attempts (1 initial + 2 retries) to exhaust retries
		await expect(api.get('/v1/unreachable')).rejects.toMatchObject({
			status: 0,
			message: expect.stringContaining('Network error')
		});
	});

	// ─── Retry Logic ──────────────────────────────────────────────────────

	it('retries on network error and succeeds on 3rd attempt', async () => {
		let callCount = 0;
		vi.stubGlobal(
			'fetch',
			vi.fn().mockImplementation(() => {
				callCount++;
				if (callCount < 3) throw new Error('Network failed');
				return Promise.resolve({
					ok: true,
					status: 200,
					headers: new Headers({ 'content-type': 'application/json' }),
					text: () => Promise.resolve(JSON.stringify({ data: 'ok' }))
				});
			})
		);

		const result = await api.get('/test');

		expect(result).toEqual({ data: 'ok' });
		expect(callCount).toBe(3);
	});

	it('throws after exhausting all retries', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn().mockRejectedValue(new Error('Always fails'))
		);

		await expect(api.get('/test')).rejects.toMatchObject({ status: 0 });
		// 1 initial + 2 retries = 3 total calls
		expect((fetch as ReturnType<typeof vi.fn>).mock.calls.length).toBe(3);
	});

	// ─── Special Responses ────────────────────────────────────────────────

	it('handles 204 No Content as undefined', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn().mockResolvedValue({
				ok: true,
				status: 204,
				statusText: 'No Content',
				headers: new Headers(),
				text: () => Promise.resolve('')
			})
		);

		const result = await api.delete('/v2/message_bus/ysk/123');

		expect(result).toBeUndefined();
	});

	it('handles empty body (200 with empty text) as undefined', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn().mockResolvedValue({
				ok: true,
				status: 200,
				statusText: 'OK',
				headers: new Headers(),
				text: () => Promise.resolve('')
			})
		);

		const result = await api.get('/v1/test');

		expect(result).toBeUndefined();
	});

	it('returns raw text for non-JSON content-type', async () => {
		const yamlContent = 'services:\n  nginx:\n    image: nginx';
		vi.stubGlobal('fetch', mockFetch(yamlContent, 200, 'application/yaml'));

		const result = await api.get('/v2/app_management/compose/app-id');

		expect(result).toBe(yamlContent);
	});

	it('throws ApiError on invalid JSON in application/json response', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn().mockResolvedValue({
				ok: true,
				status: 200,
				statusText: 'OK',
				headers: new Headers({ 'content-type': 'application/json' }),
				text: () => Promise.resolve('this is not json{{{')
			})
		);

		await expect(api.get('/v1/bad-json')).rejects.toMatchObject({
			status: 500,
			message: expect.stringContaining('Invalid JSON')
		});
	});

	// ─── Interceptors ─────────────────────────────────────────────────────

	it('applies request interceptor to outgoing headers', async () => {
		addRequestInterceptor((config) => ({
			...config,
			headers: { ...(config.headers as Record<string, string>), 'X-Custom': 'test-value' }
		}));
		vi.stubGlobal('fetch', mockFetch({}));

		await api.get('/v1/test');

		expect(fetch).toHaveBeenCalledWith(
			'/v1/test',
			expect.objectContaining({
				headers: expect.objectContaining({ 'X-Custom': 'test-value' })
			})
		);
	});
});
