/**
 * System telemetry store using Svelte 5 runes.
 *
 * Manages real-time CPU, RAM, GPU, network, and OS metrics via polling.
 * Applies Exponential Moving Average (EMA) to CPU to reduce visual noise.
 * Computes per-second network rates from cumulative byte counters.
 *
 * Design notes:
 * - Module-level state = intentional singleton (one poller across the app).
 * - EMA is computed on a COPY of the API response — never mutates the raw data.
 * - localStorage cache prevents skeleton flash on first load; stale data is
 *   always replaced by the first successful poll.
 */

import {
	getSystemUtilization,
	getSystemDisk,
	getStorageDevices,
	type SystemUtilization,
	type DiskInfo,
	type StorageDevice
} from '$lib/api/system';

let utilization = $state<SystemUtilization | null>((() => {
	if (typeof localStorage === 'undefined') return null;
	try {
		const cached = localStorage.getItem('powerlab_sys_util');
		return cached ? (JSON.parse(cached) as SystemUtilization) : null;
	} catch {
		return null;
	}
})());

let disks = $state<DiskInfo[]>([]);
// Storage devices (rich shape from /v1/disks: model, temperature,
// SMART health). Polled at a slower cadence than utilization — SMART
// + temperature don't change second-by-second and smartctl is a
// non-trivial syscall. See fetchStorageDevices.
let storageDevices = $state<StorageDevice[]>([]);
// Start true if there's no cached utilization on disk; the polling loop flips
// this to false in its finally{} block. Reading `utilization === null` here
// would only capture the value at module init, never updating later — Svelte
// flags that with state_referenced_locally.
let loading = $state(typeof localStorage === 'undefined' || !localStorage.getItem('powerlab_sys_util'));
let error = $state<string | null>(null);
let interval: ReturnType<typeof setInterval> | null = null;

// Refcount of active consumers (#453). Each useSystemStore() facade has
// its own consumer identity; the shared interval runs while >= 1 is
// active and ticks at the SMALLEST requested ms across all consumers.
//
// Why a Map<symbol, number> instead of a plain counter: we need to know
// which intervals each consumer requested so the active tick rate
// reflects min() across them. A bare counter would just track liveness.
const consumers = new Map<symbol, number>();

// EMA state — smooths instantaneous CPU spikes over ~10 samples
let emaCpu: number | null = null;
const EMA_ALPHA = 0.1;

// Network rate state — derived from cumulative byte counters
let prevNetIn = 0;
let prevNetOut = 0;
let prevNetTime = 0;

async function fetchUtilization() {
	try {
		const res = await getSystemUtilization();
		if (!res.data) return;

		const now = Date.now();

		// Work on a shallow copy so the raw API response is never mutated
		const data: SystemUtilization = { ...res.data };

		// Apply EMA to CPU on the copy, not the original
		if (data.cpu) {
			const raw = data.cpu.percent;
			emaCpu = emaCpu === null ? raw : raw * EMA_ALPHA + emaCpu * (1 - EMA_ALPHA);
			data.cpu = { ...data.cpu, percent: emaCpu };
		}

		// Compute per-second network rates from delta bytes
		if (data.net) {
			const currentIn = data.net.reduce((acc, n) => acc + n.bytesRecv, 0);
			const currentOut = data.net.reduce((acc, n) => acc + n.bytesSent, 0);

			if (prevNetTime > 0) {
				const deltaSec = (now - prevNetTime) / 1000;
				data.netInRate = Math.max(0, (currentIn - prevNetIn) / deltaSec);
				data.netOutRate = Math.max(0, (currentOut - prevNetOut) / deltaSec);
			} else {
				data.netInRate = 0;
				data.netOutRate = 0;
			}

			prevNetIn = currentIn;
			prevNetOut = currentOut;
			prevNetTime = now;
		}

		utilization = data;
		error = null;

		// Cache the processed data (with EMA + rates) for skeleton-flash prevention
		try {
			if (typeof localStorage !== 'undefined') {
				localStorage.setItem('powerlab_sys_util', JSON.stringify(data));
			}
		} catch {
			// localStorage quota exceeded — non-fatal
		}
	} catch (e) {
		error = (e as Error).message ?? 'Failed to load system metrics';
	} finally {
		loading = false;
	}
}

async function fetchDisks() {
	try {
		const res = await getSystemDisk();
		if (!res.data) return;
		// Backend returns either a single DiskInfo or an array
		disks = Array.isArray(res.data) ? res.data : [res.data];
	} catch {
		// Disk errors are non-fatal — don't clobber the main error state
	}
}

async function fetchStorageDevices() {
	try {
		const res = await getStorageDevices();
		if (!res.data || !Array.isArray(res.data.disks)) return;
		storageDevices = res.data.disks;
	} catch {
		// Device-list errors are non-fatal — Linux-only feature, macOS dev
		// + local-storage-unavailable hosts return errors that mustn't
		// clobber the main store state.
	}
}

function tick(): void {
	fetchUtilization();
	fetchDisks();
	// Storage devices are polled every Nth utilization tick rather
	// than every tick — smartctl is slow + values don't change
	// second-by-second.
	if (storageDevicesPollCount % 10 === 0) {
		fetchStorageDevices();
	}
	storageDevicesPollCount++;
}

// rearm rebuilds the global interval to match the current consumer set.
// Called whenever a consumer joins or leaves so the active tick rate
// reflects min(ms) across active consumers. Clears the interval entirely
// when no consumers remain — that is the contract that closes #453.
function rearm(): void {
	if (interval) {
		clearInterval(interval);
		interval = null;
	}
	if (consumers.size === 0) return;
	const ms = Math.min(...consumers.values());
	interval = setInterval(tick, ms);
}

export function useSystemStore() {
	// Each facade is unique so duplicate startPolling on the SAME
	// facade is a no-op (matches the legacy idempotency contract) AND
	// stopPolling from ONE facade does not affect OTHERS (the bug fix).
	const id = Symbol('system-store-consumer');
	let started = false;
	return {
		get utilization() { return utilization; },
		get disks() { return disks; },
		get storageDevices() { return storageDevices; },
		get loading() { return loading; },
		get error() { return error; },
		startPolling(ms = 3000) {
			if (started) return;
			const isFirstConsumer = consumers.size === 0;
			consumers.set(id, ms);
			started = true;
			rearm();
			// Eager fetch on the FIRST consumer only — avoids the
			// skeleton-flash. Subsequent consumers piggyback on the
			// existing interval's regular tick. If the interval picked
			// a smaller ms because of this new consumer (rearm above),
			// the next tick lands within that smaller window anyway.
			if (isFirstConsumer) {
				fetchUtilization();
				fetchDisks();
				fetchStorageDevices();
			}
		},
		stopPolling() {
			if (!started) return;
			consumers.delete(id);
			started = false;
			rearm();
		}
	};
}

let storageDevicesPollCount = 0;
