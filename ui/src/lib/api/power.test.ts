import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
	listPowerLabServices,
	restartPowerLabService,
	rebootHost,
	shutdownHost,
	type ServiceState
} from './power';

// Contract tests for the Power-actions API client (#260). The endpoint
// shapes match backend/core/route/v1/power.go — change one, the other
// must change, and these tests catch the drift.

describe('power API client', () => {
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

	it('listPowerLabServices GETs /v1/sys/services', async () => {
		mockResponse([]);
		await listPowerLabServices();
		const url = (global.fetch as ReturnType<typeof vi.fn>).mock.calls[0][0] as string;
		expect(url).toBe('/v1/sys/services');
	});

	it('listPowerLabServices unwraps data envelope into ServiceState[]', async () => {
		const states: ServiceState[] = [
			{ name: 'powerlab-gateway', active_state: 'active', sub_state: 'running', pid: '1234' },
			{ name: 'powerlab-core', active_state: 'inactive', sub_state: 'dead' }
		];
		mockResponse(states);
		const out = await listPowerLabServices();
		expect(out).toEqual(states);
	});

	it('listPowerLabServices returns [] when backend sends data:null', async () => {
		mockResponse(null);
		const out = await listPowerLabServices();
		expect(out).toEqual([]);
	});

	it('restartPowerLabService POSTs /v1/sys/services/{name}/restart', async () => {
		mockResponse('powerlab-app-management');
		await restartPowerLabService('powerlab-app-management');
		const [url, opts] = (global.fetch as ReturnType<typeof vi.fn>).mock.calls[0];
		expect(url).toBe('/v1/sys/services/powerlab-app-management/restart');
		expect((opts as RequestInit).method).toBe('POST');
	});

	it('restartPowerLabService url-encodes the service name', async () => {
		// Defensive: even though backend validates the name against an
		// allow-list, the client should still encode in case a future
		// change widens what's allowed.
		mockResponse('ok');
		await restartPowerLabService('powerlab-svc with spaces');
		const url = (global.fetch as ReturnType<typeof vi.fn>).mock.calls[0][0] as string;
		expect(url).toBe('/v1/sys/services/powerlab-svc%20with%20spaces/restart');
	});

	it('rebootHost POSTs /v1/sys/host/reboot with {"confirm":true}', async () => {
		mockResponse('host reboot initiated');
		await rebootHost();
		const [url, opts] = (global.fetch as ReturnType<typeof vi.fn>).mock.calls[0];
		expect(url).toBe('/v1/sys/host/reboot');
		expect((opts as RequestInit).method).toBe('POST');
		expect((opts as RequestInit).body).toBe(JSON.stringify({ confirm: true }));
	});

	it('shutdownHost POSTs /v1/sys/host/shutdown with {"confirm":true}', async () => {
		mockResponse('host shutdown initiated');
		await shutdownHost();
		const [url, opts] = (global.fetch as ReturnType<typeof vi.fn>).mock.calls[0];
		expect(url).toBe('/v1/sys/host/shutdown');
		expect((opts as RequestInit).method).toBe('POST');
		expect((opts as RequestInit).body).toBe(JSON.stringify({ confirm: true }));
	});
});
