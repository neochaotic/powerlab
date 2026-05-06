/**
 * System Settings store using Svelte 5 runes.
 */

import { api } from '$lib/api/client';
import { ENDPOINTS } from '$lib/api/endpoints';

export interface SystemUtilization {
	cpu: {
		percent: number;
		num: number;
		temperature: number;
		model: string;
	};
	mem: {
		total: number;
		used: number;
		free: number;
	};
	os: {
		hostname: string;
		kernel: string;
		uptime_str: string;
	};
}

export interface NetworkInterface {
	name: string;
	ip: string;
	mac: string;
	type: 'physical' | 'virtual' | 'unknown';
	state: string;
}

export interface SystemUser {
	username: string;
	uid: string;
	gid: string;
	home_dir: string;
	shell: string;
}

let utilization = $state<SystemUtilization | null>(null);
let hardwareInfo = $state<any>(null);
let networkInterfaces = $state<NetworkInterface[]>([]);
let systemUsers = $state<SystemUser[]>([]);
let timezone = $state<string>('');
let loading = $state(false);
let error = $state<string | null>(null);

async function fetchUtilization() {
	try {
		const res = await api.get<any>(ENDPOINTS.SYS_UTILIZATION);
		if (res.data) utilization = res.data;
	} catch (e) {
		console.error('Failed to fetch utilization', e);
	}
}

async function fetchHardwareInfo() {
	try {
		const res = await api.get<any>(ENDPOINTS.SYS_HARDWARE);
		if (res.data) hardwareInfo = res.data;
	} catch (e) {
		console.error('Failed to fetch hardware info', e);
	}
}

async function fetchTimezone() {
	try {
		const res = await api.get<any>(ENDPOINTS.SYS_TIMEZONE);
		if (res.data) timezone = res.data.timezone;
	} catch (e) {
		console.error('Failed to fetch timezone', e);
	}
}

async function setTimezone(newTimezone: string) {
	try {
		loading = true;
		await api.put(ENDPOINTS.SYS_TIMEZONE, { timezone: newTimezone });
		timezone = newTimezone;
		loading = false;
	} catch (e) {
		error = 'Failed to update timezone';
		loading = false;
		throw e;
	}
}

async function fetchNetworkInterfaces() {
	try {
		const res = await api.get<any>(ENDPOINTS.SYS_NETWORK_INTERFACES);
		if (res.data) networkInterfaces = res.data;
	} catch (e) {
		console.error('Failed to fetch network interfaces', e);
	}
}

async function fetchSystemUsers() {
	try {
		const res = await api.get<any>(ENDPOINTS.SYS_USERS);
		if (res.data) systemUsers = res.data;
	} catch (e) {
		console.error('Failed to fetch system users', e);
	}
}

export function useSettingsStore() {
	return {
		get utilization() { return utilization; },
		get hardwareInfo() { return hardwareInfo; },
		get networkInterfaces() { return networkInterfaces; },
		get systemUsers() { return systemUsers; },
		get timezone() { return timezone; },
		get loading() { return loading; },
		get error() { return error; },
		fetchUtilization,
		fetchHardwareInfo,
		fetchTimezone,
		setTimezone,
		fetchNetworkInterfaces,
		fetchSystemUsers
	};
}
