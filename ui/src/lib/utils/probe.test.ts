import { describe, it, expect, vi } from 'vitest';
import { probePortReachable } from './probe';

describe('probePortReachable', () => {
	it('returns true on a 200 response', async () => {
		const fetchImpl = vi.fn().mockResolvedValue(new Response('pong', { status: 200 }));
		const ok = await probePortReachable(new URL('http://localhost:8765/foo'), { fetchImpl });
		expect(ok).toBe(true);
		expect(fetchImpl).toHaveBeenCalledWith(
			'http://localhost:8765/ping',
			expect.objectContaining({ method: 'GET', mode: 'cors' })
		);
	});

	it('returns false on a 404 response — non-2xx is not reachable', async () => {
		const fetchImpl = vi.fn().mockResolvedValue(new Response('', { status: 404 }));
		expect(await probePortReachable(new URL('http://localhost:8765/'), { fetchImpl })).toBe(false);
	});

	it('returns false on a 500 response — server errored', async () => {
		const fetchImpl = vi.fn().mockResolvedValue(new Response('', { status: 500 }));
		expect(await probePortReachable(new URL('http://localhost:8765/'), { fetchImpl })).toBe(false);
	});

	it('returns false when fetch rejects (connection refused / TLS error / DNS)', async () => {
		const fetchImpl = vi.fn().mockRejectedValue(new TypeError('Failed to fetch'));
		expect(await probePortReachable(new URL('http://does-not-exist:9999/'), { fetchImpl })).toBe(false);
	});

	it('returns false when fetch is aborted by timeout', async () => {
		const fetchImpl = vi.fn().mockImplementation(() => Promise.reject(new DOMException('aborted', 'AbortError')));
		expect(await probePortReachable(new URL('http://slow-host:9999/'), { fetchImpl })).toBe(false);
	});

	it('rewrites the path to /ping regardless of the input URL path', async () => {
		const fetchImpl = vi.fn().mockResolvedValue(new Response('', { status: 200 }));
		await probePortReachable(new URL('https://gateway.local:8443/settings#security'), { fetchImpl });
		const calledWith = fetchImpl.mock.calls[0][0];
		expect(calledWith).toBe('https://gateway.local:8443/ping');
	});

	it('honors a custom pingPath when given', async () => {
		const fetchImpl = vi.fn().mockResolvedValue(new Response('', { status: 200 }));
		await probePortReachable(new URL('http://h:8/foo'), { fetchImpl, pingPath: '/healthz' });
		expect(fetchImpl.mock.calls[0][0]).toBe('http://h:8/healthz');
	});

	it('preserves the port from the input URL — bug regression for port-change flow', async () => {
		const fetchImpl = vi.fn().mockResolvedValue(new Response('', { status: 200 }));
		// User is on :8765, changes port to 8775 — probe must hit 8775,
		// not the page's current port. Same hostname, different port.
		await probePortReachable(new URL('http://192.168.1.42:8775/settings'), { fetchImpl });
		expect(fetchImpl.mock.calls[0][0]).toBe('http://192.168.1.42:8775/ping');
	});

	it('does not throw — always resolves with a boolean', async () => {
		const fetchImpl = vi.fn().mockImplementation(() => {
			throw new Error('synchronous explosion inside fetch');
		});
		// Must not propagate the throw to the caller. The whole
		// point of the helper is "did the probe answer cleanly?",
		// answered as a boolean.
		const result = await probePortReachable(new URL('http://x/'), { fetchImpl });
		expect(result).toBe(false);
	});
});
