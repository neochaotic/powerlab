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

import { getSystemUtilization, getSystemDisk, type SystemUtilization, type DiskInfo } from '$lib/api/system';

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
// Start true if there's no cached utilization on disk; the polling loop flips
// this to false in its finally{} block. Reading `utilization === null` here
// would only capture the value at module init, never updating later — Svelte
// flags that with state_referenced_locally.
let loading = $state(typeof localStorage === 'undefined' || !localStorage.getItem('powerlab_sys_util'));
let error = $state<string | null>(null);
let interval: ReturnType<typeof setInterval> | null = null;

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

export function useSystemStore() {
	return {
		get utilization() { return utilization; },
		get disks() { return disks; },
		get loading() { return loading; },
		get error() { return error; },
		startPolling(ms = 3000) {
			if (interval) return;
			fetchUtilization();
			fetchDisks();
			interval = setInterval(() => {
				fetchUtilization();
				fetchDisks();
			}, ms);
		},
		stopPolling() {
			if (interval) {
				clearInterval(interval);
				interval = null;
			}
		}
	};
}
