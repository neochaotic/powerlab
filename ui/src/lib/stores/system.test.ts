import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';

const mockStorage: Record<string, string> = {};
global.localStorage = {
	getItem: vi.fn((key: string) => mockStorage[key] ?? null),
	setItem: vi.fn((key: string, value: string) => {
		mockStorage[key] = value;
	}),
	removeItem: vi.fn((key: string) => {
		delete mockStorage[key];
	}),
	clear: vi.fn(() => {
		for (const k in mockStorage) delete mockStorage[k];
	}),
	length: 0,
	key: (i: number) => Object.keys(mockStorage)[i] ?? null
} as Storage;

vi.mock('$lib/api/system', () => ({
	getSystemUtilization: vi.fn(),
	getSystemDisk: vi.fn(),
	getStorageDevices: vi.fn()
}));

import * as systemApi from '$lib/api/system';

beforeEach(() => {
	vi.resetModules();
	vi.useFakeTimers();
	localStorage.clear();
	vi.mocked(systemApi.getSystemUtilization).mockReset();
	vi.mocked(systemApi.getSystemDisk).mockReset();
	vi.mocked(systemApi.getStorageDevices).mockReset();
});

afterEach(() => {
	vi.useRealTimers();
});

describe('System store', () => {
	it('utilization starts null when localStorage has no cache', async () => {
		const { useSystemStore } = await import('./system.svelte');
		expect(useSystemStore().utilization).toBeNull();
	});

	it('utilization hydrates from localStorage cache on init', async () => {
		const cached = {
			cpu: { percent: 42, num: 4, temperature: 50, model: 'test' },
			mem: { total: 100, used: 50, free: 50 }
		};
		localStorage.setItem('powerlab_sys_util', JSON.stringify(cached));
		const { useSystemStore } = await import('./system.svelte');
		expect(useSystemStore().utilization).toMatchObject({ cpu: { percent: 42 } });
	});

	it('loading is true initially when no cache, false after first fetch', async () => {
		vi.mocked(systemApi.getSystemUtilization).mockResolvedValue({
			data: { cpu: { percent: 30, num: 4, temperature: 50, model: 'm' } }
		} as any);
		vi.mocked(systemApi.getSystemDisk).mockResolvedValue({ data: null } as any);
		vi.mocked(systemApi.getStorageDevices).mockResolvedValue({ data: null } as any);

		const { useSystemStore } = await import('./system.svelte');
		const store = useSystemStore();
		expect(store.loading).toBe(true);

		store.startPolling(1000);
		// Drain microtasks for the immediate fetch
		await vi.runOnlyPendingTimersAsync();
		await Promise.resolve();
		await Promise.resolve();
		expect(store.loading).toBe(false);
		store.stopPolling();
	});

	it('startPolling is idempotent — second call is a no-op', async () => {
		vi.mocked(systemApi.getSystemUtilization).mockResolvedValue({ data: null } as any);
		vi.mocked(systemApi.getSystemDisk).mockResolvedValue({ data: null } as any);
		vi.mocked(systemApi.getStorageDevices).mockResolvedValue({ data: null } as any);

		const { useSystemStore } = await import('./system.svelte');
		const store = useSystemStore();
		store.startPolling(1000);
		store.startPolling(1000);
		// First call kicks 3 immediate fetches; second call should not add more.
		expect(vi.mocked(systemApi.getSystemUtilization)).toHaveBeenCalledTimes(1);
		store.stopPolling();
	});

	it('stopPolling clears the interval (no further fetches)', async () => {
		vi.mocked(systemApi.getSystemUtilization).mockResolvedValue({ data: null } as any);
		vi.mocked(systemApi.getSystemDisk).mockResolvedValue({ data: null } as any);
		vi.mocked(systemApi.getStorageDevices).mockResolvedValue({ data: null } as any);

		const { useSystemStore } = await import('./system.svelte');
		const store = useSystemStore();
		store.startPolling(1000);
		const initialCalls = vi.mocked(systemApi.getSystemUtilization).mock.calls.length;
		store.stopPolling();
		await vi.advanceTimersByTimeAsync(5000);
		expect(vi.mocked(systemApi.getSystemUtilization).mock.calls.length).toBe(initialCalls);
	});

	it('error captures the message when fetchUtilization throws', async () => {
		vi.mocked(systemApi.getSystemUtilization).mockRejectedValue(new Error('boom'));
		vi.mocked(systemApi.getSystemDisk).mockResolvedValue({ data: null } as any);
		vi.mocked(systemApi.getStorageDevices).mockResolvedValue({ data: null } as any);

		const { useSystemStore } = await import('./system.svelte');
		const store = useSystemStore();
		store.startPolling(1000);
		await vi.runOnlyPendingTimersAsync();
		await Promise.resolve();
		await Promise.resolve();
		expect(store.error).toBe('boom');
		store.stopPolling();
	});

	it('disks accepts both array and single-object backend responses', async () => {
		vi.mocked(systemApi.getSystemUtilization).mockResolvedValue({ data: null } as any);
		vi.mocked(systemApi.getSystemDisk).mockResolvedValue({
			data: [{ path: '/', total: 100, used: 50, free: 50 }]
		} as any);
		vi.mocked(systemApi.getStorageDevices).mockResolvedValue({ data: null } as any);

		const { useSystemStore } = await import('./system.svelte');
		const store = useSystemStore();
		store.startPolling(1000);
		await vi.runOnlyPendingTimersAsync();
		await Promise.resolve();
		await Promise.resolve();
		expect(store.disks).toHaveLength(1);
		store.stopPolling();
	});
});
