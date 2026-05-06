import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { getGatewayPort, setGatewayPort } from './gateway';

// Helper: stub global fetch to return a controlled CasaOS envelope.
// Mirrors the pattern in client.test.ts — the HTTP client reads
// response.text() and parses, so the mock must expose .text(), NOT
// .json(). See CLAUDE.md "Critical mock pattern" notes from the
// review session.
function mockFetch(body: unknown, ok = true, status = 200) {
	vi.stubGlobal(
		'fetch',
		vi.fn().mockResolvedValue({
			ok,
			status,
			headers: new Headers({ 'content-type': 'application/json' }),
			text: () => Promise.resolve(JSON.stringify(body))
		})
	);
}

describe('gateway API', () => {
	beforeEach(() => {
		vi.restoreAllMocks();
	});
	afterEach(() => {
		vi.unstubAllGlobals();
	});

	describe('getGatewayPort', () => {
		it('returns the port from a CasaOS-shaped envelope', async () => {
			mockFetch({ success: 200, message: 'ok', data: '8765' });
			const port = await getGatewayPort();
			expect(port).toBe('8765');
		});

		it('returns empty string when data is missing', async () => {
			mockFetch({ success: 200, message: 'ok' });
			const port = await getGatewayPort();
			expect(port).toBe('');
		});
	});

	describe('setGatewayPort', () => {
		it('rejects non-integer ports without a network call', async () => {
			const fetchSpy = vi.fn();
			vi.stubGlobal('fetch', fetchSpy);
			await expect(setGatewayPort(80.5 as unknown as number)).rejects.toThrow(/out of range/);
			expect(fetchSpy).not.toHaveBeenCalled();
		});

		it('rejects port 0 client-side without hitting the backend', async () => {
			const fetchSpy = vi.fn();
			vi.stubGlobal('fetch', fetchSpy);
			await expect(setGatewayPort(0)).rejects.toThrow(/out of range/);
			expect(fetchSpy).not.toHaveBeenCalled();
		});

		it('rejects port 65536 client-side', async () => {
			const fetchSpy = vi.fn();
			vi.stubGlobal('fetch', fetchSpy);
			await expect(setGatewayPort(65536)).rejects.toThrow(/out of range/);
			expect(fetchSpy).not.toHaveBeenCalled();
		});

		it('rejects negative ports', async () => {
			const fetchSpy = vi.fn();
			vi.stubGlobal('fetch', fetchSpy);
			await expect(setGatewayPort(-1)).rejects.toThrow(/out of range/);
			expect(fetchSpy).not.toHaveBeenCalled();
		});

		it('sends the port as a string in the request body', async () => {
			let capturedBody: string | null = null;
			vi.stubGlobal(
				'fetch',
				vi.fn().mockImplementation((_url, init) => {
					capturedBody = init?.body ?? null;
					return Promise.resolve({
						ok: true,
						status: 200,
						headers: new Headers({ 'content-type': 'application/json' }),
						text: () => Promise.resolve(JSON.stringify({ success: 200, message: 'ok' }))
					});
				})
			);
			await setGatewayPort(9000);
			expect(capturedBody).toBeTruthy();
			expect(JSON.parse(capturedBody!)).toEqual({ port: '9000' });
		});

		it('accepts the boundary ports 1 and 65535', async () => {
			mockFetch({ success: 200, message: 'ok' });
			await expect(setGatewayPort(1)).resolves.not.toThrow();
			await expect(setGatewayPort(65535)).resolves.not.toThrow();
		});
	});
});
