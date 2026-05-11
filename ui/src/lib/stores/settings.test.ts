import { describe, it, expect, beforeEach, vi } from 'vitest';

vi.mock('$lib/api/client', () => ({
	api: {
		get: vi.fn(),
		put: vi.fn()
	}
}));

import { api } from '$lib/api/client';

beforeEach(() => {
	vi.resetModules();
	vi.mocked(api.get).mockReset();
	vi.mocked(api.put).mockReset();
});

describe('Settings store', () => {
	it('initial state: utilization=null, loading=false, error=null', async () => {
		const { useSettingsStore } = await import('./settings.svelte');
		const s = useSettingsStore();
		expect(s.utilization).toBeNull();
		expect(s.loading).toBe(false);
		expect(s.error).toBeNull();
		expect(s.networkInterfaces).toEqual([]);
		expect(s.systemUsers).toEqual([]);
	});

	it('fetchUtilization populates utilization from API response', async () => {
		vi.mocked(api.get).mockResolvedValue({
			data: { cpu: { percent: 22 }, mem: { total: 100, used: 50, free: 50 } }
		} as any);
		const { useSettingsStore } = await import('./settings.svelte');
		const s = useSettingsStore();
		await s.fetchUtilization();
		expect(s.utilization).toMatchObject({ cpu: { percent: 22 } });
	});

	it('fetchUtilization swallows errors (no throw)', async () => {
		vi.mocked(api.get).mockRejectedValue(new Error('500'));
		const { useSettingsStore } = await import('./settings.svelte');
		const s = useSettingsStore();
		await expect(s.fetchUtilization()).resolves.toBeUndefined();
		expect(s.utilization).toBeNull();
	});

	it('fetchHardwareInfo populates hardwareInfo', async () => {
		vi.mocked(api.get).mockResolvedValue({ data: { cpu: 'AMD Ryzen', ram_gb: 32 } } as any);
		const { useSettingsStore } = await import('./settings.svelte');
		const s = useSettingsStore();
		await s.fetchHardwareInfo();
		expect(s.hardwareInfo).toEqual({ cpu: 'AMD Ryzen', ram_gb: 32 });
	});

	it('fetchTimezone populates timezone field from response.timezone', async () => {
		vi.mocked(api.get).mockResolvedValue({ data: { timezone: 'America/Sao_Paulo' } } as any);
		const { useSettingsStore } = await import('./settings.svelte');
		const s = useSettingsStore();
		await s.fetchTimezone();
		expect(s.timezone).toBe('America/Sao_Paulo');
	});

	it('setTimezone PUTs and updates the local store on success', async () => {
		vi.mocked(api.put).mockResolvedValue({} as any);
		const { useSettingsStore } = await import('./settings.svelte');
		const s = useSettingsStore();
		await s.setTimezone('Europe/Berlin');
		expect(api.put).toHaveBeenCalledWith(expect.any(String), { timezone: 'Europe/Berlin' });
		expect(s.timezone).toBe('Europe/Berlin');
		expect(s.loading).toBe(false);
	});

	it('setTimezone surfaces an error string and rethrows on PUT failure', async () => {
		vi.mocked(api.put).mockRejectedValue(new Error('boom'));
		const { useSettingsStore } = await import('./settings.svelte');
		const s = useSettingsStore();
		await expect(s.setTimezone('UTC')).rejects.toThrow('boom');
		expect(s.error).toBe('Failed to update timezone');
		expect(s.loading).toBe(false);
	});

	it('fetchNetworkInterfaces populates interfaces array', async () => {
		vi.mocked(api.get).mockResolvedValue({
			data: [{ name: 'eth0', ip: '10.0.0.1', mac: 'aa', type: 'physical', state: 'UP' }]
		} as any);
		const { useSettingsStore } = await import('./settings.svelte');
		const s = useSettingsStore();
		await s.fetchNetworkInterfaces();
		expect(s.networkInterfaces).toHaveLength(1);
		expect(s.networkInterfaces[0].name).toBe('eth0');
	});

	it('fetchSystemUsers populates users array', async () => {
		vi.mocked(api.get).mockResolvedValue({
			data: [{ username: 'admin', uid: '1000', gid: '1000', home_dir: '/home/admin', shell: '/bin/bash' }]
		} as any);
		const { useSettingsStore } = await import('./settings.svelte');
		const s = useSettingsStore();
		await s.fetchSystemUsers();
		expect(s.systemUsers).toHaveLength(1);
	});
});
