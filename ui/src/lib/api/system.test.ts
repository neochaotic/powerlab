import { describe, it, expect, vi, beforeEach } from 'vitest';
import {
	getSystemUtilization,
	getSystemDisk,
	getHardwareInfo,
	putSystemState
} from './system';

function mockJson(data: unknown, status = 200) {
	return vi.fn().mockResolvedValue({
		ok: status >= 200 && status < 300,
		status,
		statusText: 'OK',
		headers: new Headers({ 'content-type': 'application/json' }),
		text: () => Promise.resolve(JSON.stringify(data))
	});
}

const mockUtilization = {
	success: 200,
	message: '',
	data: {
		cpu: { percent: 45.2, num: 10, temperature: 55, power: 0, model: 'Apple M5' },
		mem: { total: 25769803776, available: 8589934592, used: 17179869184, usedPercent: 66.7, free: 8589934592 },
		net: [{ name: 'en0', bytesRecv: 1024000, bytesSent: 512000, state: 'up', time: 1000 }]
	}
};

describe('System API', () => {
	beforeEach(() => {
		vi.restoreAllMocks();
	});

	// ─── Utilization ──────────────────────────────────────────────────────

	it('getSystemUtilization: calls correct endpoint', async () => {
		vi.stubGlobal('fetch', mockJson(mockUtilization));

		await getSystemUtilization();

		expect(fetch).toHaveBeenCalledWith('/v1/sys/utilization', expect.anything());
	});

	it('getSystemUtilization: returns typed utilization data', async () => {
		vi.stubGlobal('fetch', mockJson(mockUtilization));

		const result = await getSystemUtilization();

		expect(result).toEqual(mockUtilization);
		expect(result.data.cpu.percent).toBe(45.2);
		expect(result.data.cpu.num).toBe(10);
		expect(result.data.mem.usedPercent).toBe(66.7);
		expect(result.data.net).toHaveLength(1);
	});

	it('getSystemUtilization: handles GPU data when present', async () => {
		const withGpu = {
			...mockUtilization,
			data: {
				...mockUtilization.data,
				gpu: { percent: 12.5, memoryUsed: 2048, model: 'Apple M5 GPU', temperature: 40 }
			}
		};
		vi.stubGlobal('fetch', mockJson(withGpu));

		const result = await getSystemUtilization();

		expect(result.data.gpu).toBeDefined();
		expect(result.data.gpu?.model).toBe('Apple M5 GPU');
		expect(result.data.gpu?.percent).toBe(12.5);
	});

	it('getSystemUtilization: handles missing GPU (undefined)', async () => {
		vi.stubGlobal('fetch', mockJson(mockUtilization));

		const result = await getSystemUtilization();

		expect(result.data.gpu).toBeUndefined();
	});

	it('getSystemUtilization: throws on 500 server error', async () => {
		vi.stubGlobal('fetch', mockJson({ message: 'Internal server error' }, 500));

		await expect(getSystemUtilization()).rejects.toMatchObject({ status: 500 });
	});

	// ─── Disk ─────────────────────────────────────────────────────────────

	it('getSystemDisk: calls correct endpoint', async () => {
		vi.stubGlobal('fetch', mockJson({ success: 200, message: '', data: {} }));

		await getSystemDisk();

		expect(fetch).toHaveBeenCalledWith('/v1/sys/disk', expect.anything());
	});

	it('getSystemDisk: uses GET method', async () => {
		vi.stubGlobal('fetch', mockJson({ success: 200, message: '', data: {} }));

		await getSystemDisk();

		expect(fetch).toHaveBeenCalledWith(
			'/v1/sys/disk',
			expect.objectContaining({ method: 'GET' })
		);
	});

	// ─── Hardware Info ────────────────────────────────────────────────────

	it('getHardwareInfo: calls correct endpoint', async () => {
		vi.stubGlobal(
			'fetch',
			mockJson({ success: 200, message: '', data: { drive_model: 'Apple M5', arch: 'arm64' } })
		);

		await getHardwareInfo();

		expect(fetch).toHaveBeenCalledWith('/v1/sys/hardware/info', expect.anything());
	});

	it('getHardwareInfo: returns drive_model and arch', async () => {
		vi.stubGlobal(
			'fetch',
			mockJson({ success: 200, message: '', data: { drive_model: 'Apple M5', arch: 'arm64' } })
		);

		const result = await getHardwareInfo();

		expect(result.data.drive_model).toBe('Apple M5');
		expect(result.data.arch).toBe('arm64');
	});

	// ─── System State ─────────────────────────────────────────────────────

	it('putSystemState (restart): calls correct endpoint with PUT', async () => {
		vi.stubGlobal('fetch', mockJson({ success: 200, message: 'ok', data: '' }));

		await putSystemState('restart');

		expect(fetch).toHaveBeenCalledWith(
			'/v1/sys/state/restart',
			expect.objectContaining({ method: 'PUT' })
		);
	});

	it('putSystemState (off): calls correct endpoint', async () => {
		vi.stubGlobal('fetch', mockJson({ success: 200, message: 'ok', data: '' }));

		await putSystemState('off');

		expect(fetch).toHaveBeenCalledWith('/v1/sys/state/off', expect.anything());
	});

	it('putSystemState: throws on 401 unauthorized', async () => {
		vi.stubGlobal('fetch', mockJson({ message: 'Unauthorized' }, 401));

		await expect(putSystemState('restart')).rejects.toMatchObject({ status: 401 });
	});
});
