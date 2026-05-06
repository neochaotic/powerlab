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

export interface DiskInfo {
	path: string;
	fstype: string;
	total: number;
	free: number;
	used: number;
	usedPercent: number;
}

/** Get overall system utilization (CPU, RAM, Net) */
export function getSystemUtilization() {
	return api.get<ApiResult<SystemUtilization>>('/v1/sys/utilization');
}

/** Get basic disk information */
export function getSystemDisk() {
	// The CasaOS backend returns a single disk info object or an array? Let's assume object or array of objects.
	return api.get<ApiResult<DiskInfo>>('/v1/sys/disk');
}

/** Get system hardware info (device model, arch) */
export function getHardwareInfo() {
	return api.get<ApiResult<{ drive_model: string; arch: string }>>('/v1/sys/hardware/info');
}

/** Reboot or shutdown the system */
export function putSystemState(state: 'restart' | 'off') {
	return api.put<ApiResult<string>>(`/v1/sys/state/${state}`);
}
