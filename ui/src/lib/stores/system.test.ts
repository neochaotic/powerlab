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

	// ─── Refcount semantics (#453) ────────────────────────────────────
	// Locks the contract that fixes the sidebar-telemetry-pauses-on-route-
	// change bug: stopPolling from ONE consumer must not kill the interval
	// for OTHER consumers. Repro flow under the old (buggy) semantics:
	//   Sidebar mounts → startPolling(1000)
	//   Launchpad page mounts → startPolling(2000)  (early-return, no-op)
	//   Launchpad unmounts → stopPolling() — KILLED the sidebar's interval
	//   Settings mounts (no startPolling) → telemetry frozen
	// Refcount fix: each useSystemStore() facade has its own `started`
	// flag + a unique consumer id; stopPolling only clears the interval
	// when the LAST consumer goes away.

	it('refcount: stopPolling from one consumer keeps interval alive for another', async () => {
		vi.mocked(systemApi.getSystemUtilization).mockResolvedValue({ data: null } as any);
		vi.mocked(systemApi.getSystemDisk).mockResolvedValue({ data: null } as any);
		vi.mocked(systemApi.getStorageDevices).mockResolvedValue({ data: null } as any);

		const { useSystemStore } = await import('./system.svelte');
		const sidebar = useSystemStore();
		const launchpad = useSystemStore();

		sidebar.startPolling(1000);
		launchpad.startPolling(2000);
		const beforeUnmount = vi.mocked(systemApi.getSystemUtilization).mock.calls.length;

		launchpad.stopPolling(); // Launchpad navigates away

		// Interval MUST keep firing for the sidebar after launchpad unmount.
		await vi.advanceTimersByTimeAsync(3500);
		const afterUnmount = vi.mocked(systemApi.getSystemUtilization).mock.calls.length;
		expect(afterUnmount).toBeGreaterThan(beforeUnmount);

		sidebar.stopPolling();
	});

	it('refcount: interval clears only when the LAST consumer stops', async () => {
		vi.mocked(systemApi.getSystemUtilization).mockResolvedValue({ data: null } as any);
		vi.mocked(systemApi.getSystemDisk).mockResolvedValue({ data: null } as any);
		vi.mocked(systemApi.getStorageDevices).mockResolvedValue({ data: null } as any);

		const { useSystemStore } = await import('./system.svelte');
		const a = useSystemStore();
		const b = useSystemStore();
		a.startPolling(1000);
		b.startPolling(1000);
		a.stopPolling();
		b.stopPolling();

		const baseline = vi.mocked(systemApi.getSystemUtilization).mock.calls.length;
		await vi.advanceTimersByTimeAsync(5000);
		expect(vi.mocked(systemApi.getSystemUtilization).mock.calls.length).toBe(baseline);
	});

	it('refcount: duplicate stopPolling on same facade is idempotent', async () => {
		// Sidebar.svelte currently has two onDestroy blocks both calling
		// store.stopPolling() (refactor artifact). Under refcount semantics
		// the second call MUST NOT decrement past zero or affect other
		// consumers.
		vi.mocked(systemApi.getSystemUtilization).mockResolvedValue({ data: null } as any);
		vi.mocked(systemApi.getSystemDisk).mockResolvedValue({ data: null } as any);
		vi.mocked(systemApi.getStorageDevices).mockResolvedValue({ data: null } as any);

		const { useSystemStore } = await import('./system.svelte');
		const sidebar = useSystemStore();
		const launchpad = useSystemStore();

		sidebar.startPolling(1000);
		launchpad.startPolling(1000);

		sidebar.stopPolling();
		sidebar.stopPolling(); // duplicate (the Sidebar.svelte double-onDestroy bug)

		const before = vi.mocked(systemApi.getSystemUtilization).mock.calls.length;
		await vi.advanceTimersByTimeAsync(3500);
		// Launchpad still polling → fetch count should grow despite the
		// duplicate stopPolling on sidebar.
		expect(vi.mocked(systemApi.getSystemUtilization).mock.calls.length).toBeGreaterThan(before);

		launchpad.stopPolling();
	});

	it('refcount: interval picks the SMALLEST requested ms across consumers', async () => {
		vi.mocked(systemApi.getSystemUtilization).mockResolvedValue({ data: null } as any);
		vi.mocked(systemApi.getSystemDisk).mockResolvedValue({ data: null } as any);
		vi.mocked(systemApi.getStorageDevices).mockResolvedValue({ data: null } as any);

		const { useSystemStore } = await import('./system.svelte');
		const slow = useSystemStore();
		const fast = useSystemStore();

		slow.startPolling(5000);
		const baselineSlow = vi.mocked(systemApi.getSystemUtilization).mock.calls.length;

		fast.startPolling(1000); // a faster consumer arrives
		await vi.advanceTimersByTimeAsync(1500);
		expect(
			vi.mocked(systemApi.getSystemUtilization).mock.calls.length
		).toBeGreaterThan(baselineSlow);

		slow.stopPolling();
		fast.stopPolling();
	});

	it('disks + physicalDisks split out the {physical, mounts} payload', async () => {
		// /v1/sys/disk now returns the rich DisksInfo shape (physical
		// block devices + per-mount usage). The store splits the two
		// arrays into separate reactive slots so the StorageBar widget
		// iterates `disks` (mounts) and the SMART card iterates
		// `physicalDisks` without re-fetching.
		vi.mocked(systemApi.getSystemUtilization).mockResolvedValue({ data: null } as any);
		vi.mocked(systemApi.getSystemDisk).mockResolvedValue({
			data: {
				physical: [
					{
						name: '/dev/sda',
						model: 'Samsung SSD 870',
						serial: 'S5R',
						size_bytes: 1_000_000_000_000,
						temperature_c: 39,
						health_status: 'PASSED'
					}
				],
				mounts: [
					{ path: '/', fs_type: 'ext4', total: 100, used: 50, free: 50, used_percent: 50 }
				]
			}
		} as any);
		vi.mocked(systemApi.getStorageDevices).mockResolvedValue({ data: null } as any);

		const { useSystemStore } = await import('./system.svelte');
		const store = useSystemStore();
		store.startPolling(1000);
		await vi.runOnlyPendingTimersAsync();
		await Promise.resolve();
		await Promise.resolve();
		expect(store.disks).toHaveLength(1);
		expect(store.disks[0].used_percent).toBe(50);
		expect(store.physicalDisks).toHaveLength(1);
		expect(store.physicalDisks[0].health_status).toBe('PASSED');
		store.stopPolling();
	});

	it('disks defaults to [] when the payload arrives with no mounts/physical arrays', async () => {
		// Defensive: an old core version in front of a new UI returns
		// the legacy single-object shape. We don't render that — both
		// arrays stay empty (no crash, no broken render).
		vi.mocked(systemApi.getSystemUtilization).mockResolvedValue({ data: null } as any);
		vi.mocked(systemApi.getSystemDisk).mockResolvedValue({
			data: { path: '/', total: 100, used: 50, free: 50, usedPercent: 50 }
		} as any);
		vi.mocked(systemApi.getStorageDevices).mockResolvedValue({ data: null } as any);

		const { useSystemStore } = await import('./system.svelte');
		const store = useSystemStore();
		store.startPolling(1000);
		await vi.runOnlyPendingTimersAsync();
		await Promise.resolve();
		await Promise.resolve();
		expect(store.disks).toHaveLength(0);
		expect(store.physicalDisks).toHaveLength(0);
		store.stopPolling();
	});
});
