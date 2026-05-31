/**
 * System Telemetry API endpoints.
 *
 * Maps directly to CasaOS Go backend v1 sys routes.
 */

import { api } from './client';
import type { ApiResult } from './files';

export interface CpuInfo {
	percent: number;
	num: number;
	temperature: number;
	power: number;
	model: string;
}

export interface MemInfo {
	total: number;
	available: number;
	used: number;
	usedPercent: number;
	free: number;
}

export interface NetInfo {
	name: string;
	bytesRecv: number;
	bytesSent: number;
	state: string;
	time: number;
}

export interface SystemUtilization {
	cpu: CpuInfo;
	mem: MemInfo;
	net: NetInfo[];
	gpu?: {
		percent: number;
		memoryUsed: number;
		model: string;
		temperature: number;
	};
	netInRate?: number;
	netOutRate?: number;
	os?: {
		hostname: string;
		kernel: string;
		uptime_str: string;
	};
	[key: string]: unknown;
}

/**
 * MountInfo is one filesystem mount in the /v1/sys/disk response's
 * `mounts` array. Mirrors backend/core/model/disks.go::MountInfo.
 * snake_case at the wire matches every other PowerLab Go→JSON
 * surface (ProcessSummary, MemInfo, etc.) and the MCP
 * system://disk description.
 */
export interface MountInfo {
	path: string;
	fs_type: string;
	total: number;
	used: number;
	free: number;
	used_percent: number;
}

/**
 * PhysicalDisk is one entry in /v1/sys/disk's `physical` array —
 * block-device inventory enriched with best-effort SMART metadata
 * (model + serial + temperature + health). On hosts without
 * smartctl the SMART fields are zero / empty by design; an empty
 * `model` means "no SMART data" — same graceful-degrade pattern
 * as system://gpu's empty model = "no GPU detected".
 */
export interface PhysicalDisk {
	name: string;
	model: string;
	serial: string;
	size_bytes: number;
	temperature_c: number;
	health_status: string;
}

/**
 * DisksInfo is the /v1/sys/disk wire shape. Pre-fix the route
 * returned a single root-mount object; the MCP quality audit
 * surfaced the contract mismatch (the system://disk description
 * promised physical + per-mount + SMART) and the route was
 * widened to match. Dashboard consumers iterate `mounts`; the
 * SMART widget reads `physical`.
 */
export interface DisksInfo {
	physical: PhysicalDisk[];
	mounts: MountInfo[];
}

/**
 * StorageDevice mirrors the local-storage Drive struct (see
 * backend/local-storage/model/disk.go). Surfaced by GET /v1/disks
 * — separate from /v1/sys/disk (core's view); kept for screens
 * that need lsblk's tree of children-by-partition.
 */
export interface StorageDevice {
	name: string;
	size: number;
	model: string;
	health: string;
	temperature: number;
	disk_type: string;
	serial: string;
	path: string;
	children_number: number;
	supported: boolean;
}

export interface StorageDeviceList {
	disks: StorageDevice[];
	avail: StorageDevice[];
}

/** Get overall system utilization (CPU, RAM, Net) */
export function getSystemUtilization() {
	return api.get<ApiResult<SystemUtilization>>('/v1/sys/utilization');
}

/**
 * Get physical disks (SMART metadata when available) + per-mount
 * usage from core's /v1/sys/disk. Always-non-null arrays; agents
 * and the dashboard can iterate without null-guards beyond the
 * outer ApiResult envelope.
 */
export function getSystemDisk() {
	return api.get<ApiResult<DisksInfo>>('/v1/sys/disk');
}

/**
 * Get the rich device list (lsblk + smartctl) from local-storage.
 * Includes per-device temperature + SMART health. Closes #255.
 */
export function getStorageDevices() {
	return api.get<ApiResult<StorageDeviceList>>('/v1/disks');
}

/** Get system hardware info (device model, arch).
 * Backend route is /v1/sys/hardware (not /v1/sys/hardware/info — that
 * was a UI-side typo that surfaced 404 in Settings → About). The
 * Swagger annotation in system.go:180 mentions "/sys/hardware/info"
 * but the actual route registration in v1.go:82 is "/hardware". */
export function getHardwareInfo() {
	return api.get<ApiResult<{ drive_model: string; arch: string }>>('/v1/sys/hardware');
}

/** Reboot or shutdown the system */
export function putSystemState(state: 'restart' | 'off') {
	return api.put<ApiResult<string>>(`/v1/sys/state/${state}`);
}
