/**
 * PowerLab API Endpoints
 *
 * Typed endpoint definitions mapping directly to CasaOS Go backend OpenAPI specs.
 * This is the contract layer — DTOs match backend schemas exactly.
 */

// ─── Base Types ───────────────────────────────────────────────────────

export interface BaseResponse {
	message?: string;
}

export interface SuccessResponse<T> extends BaseResponse {
	data: T;
}

// ─── Health (PowerLab Core: /v2/powerlab-core) ─────────────────────────────────

export interface HealthServices {
	running: string[];
	not_running: string[];
}

export interface HealthPorts {
	tcp: number[];
	udp: number[];
}

// ─── App Management (/v2/app_management) ──────────────────────────────

export interface AppStoreInfo {
	id: number;
	url: string;
}

export interface ComposeAppStatus {
	status: 'running' | 'stopped' | 'starting' | 'stopping' | 'restarting';
}

export interface ContainerSummary {
	id: string;
	name: string;
	image: string;
	state: string;
	status: string;
	ports: string[];
}

// ─── Local Storage (/v2/local_storage) ────────────────────────────────

export interface Mount {
	id?: number;
	mount_point: string;
	fstype?: string;
	source?: string;
	options?: string;
	extended?: Record<string, string>;
}

export interface Merge {
	id?: number;
	fstype?: string;
	mount_point: string;
	source_base_path?: string;
	source_volume_uuids?: string[];
	created_at?: string;
	updated_at?: string;
}

export type MergeStatus = 'initialized' | 'uninitialized' | 'error';

// ─── MessageBus (/v2/message_bus) ─────────────────────────────────────

export interface EventType {
	sourceID: string;
	name: string;
	propertyTypeList: PropertyType[];
}

export interface PropertyType {
	name: string;
	description?: string;
	example?: string;
}

export interface BusEvent {
	sourceID: string;
	name: string;
	uuid?: string;
	properties: Record<string, string>;
	timestamp?: string;
}

export interface YSKCard {
	id: string;
	cardType: 'task' | 'long-notice' | 'short-notice';
	renderType: 'task' | 'list-notice' | 'icon-text-notice' | 'markdown-notice';
	content: YSKCardContent;
}

export interface YSKCardContent {
	titleIcon: string;
	titleText: string;
	bodyProgress?: { label: string; progress: number };
	bodyList?: Array<{ icon?: string; text: string; description?: string }>;
	footerActions?: Array<{ text: string; url?: string }>;
}

// ─── Gateway (/v1/gateway) ────────────────────────────────────────────

export interface GatewayPort {
	port: string;
}

// ─── User Service (/v2/users) ─────────────────────────────────────────

export interface UserEvent {
	event_uuid: string;
	sourceID: string;
	name: string;
	properties: Array<{ name: string; value: string }>;
	timestamp?: string;
}

// ─── API Path Constants ───────────────────────────────────────────────

export const ENDPOINTS = {
	// CasaOS Core
	HEALTH_SERVICES: '/v2/powerlab-core/health/services',
	HEALTH_PORTS: '/v2/powerlab-core/health/ports',
	HEALTH_LOGS: '/v2/powerlab-core/health/logs',
	FILE_UPLOAD: '/v2/powerlab-core/file/upload',
	FILE_TEST: '/v2/powerlab-core/file/test',

	// App Management
	APP_INFO: '/v2/app_management/info',
	APP_STORE_LIST: '/v2/app_management/appstore',
	APP_CATEGORIES: '/v2/app_management/categories',
	APP_STORE_APPS: '/v2/app_management/apps',
	APP_COMPOSE_LIST: '/v2/app_management/compose',
	APP_COMPOSE_DEPLOY: '/v2/app_management/compose',
	APP_COMPOSE_LOGS: (id: string) => `/v2/app_management/compose/${id}/logs`,
	APP_COMPOSE_TASK_LOGS: '/v2/app_management/compose/task/:id/logs',
	APP_GLOBAL_SETTINGS: '/v2/app_management/global',
	APP_WEB_GRID: '/v2/app_management/web/appgrid',

	// Local Storage
	STORAGE_MOUNT: '/v2/local_storage/mount',
	STORAGE_MERGE: '/v2/local_storage/merge',
	STORAGE_MERGE_INIT: '/v2/local_storage/merge/init',

	// MessageBus
	MSG_EVENT_TYPES: '/v2/message_bus/event_type',
	MSG_ACTION_TYPES: '/v2/message_bus/action_type',
	MSG_YSK: '/v2/message_bus/ysk',
	MSG_SOCKET_IO: '/v2/message_bus/socket.io',

	// User Service
	USER_LOGIN: '/v1/users/login',
	USER_REGISTER: '/v1/users/register',
	USER_CURRENT: '/v1/users/current',
	USER_STATUS: '/v1/users/status',
	USER_EVENTS: '/v2/users/events',

	// System (V1)
	SYS_UTILIZATION: '/v1/sys/utilization',
	SYS_HARDWARE: '/v1/sys/hardware',
	// SYS_VERSION (was /v1/sys/version/check) removed in Sprint 5 #203
	// kill #1 — backend route deleted with the rest of the upstream-
	// CasaOS self-update path. PowerLab updater uses
	// /v1/powerlab-update/* (see updater.ts).
	SYS_REBOOT: '/v1/sys/state/restart',
	SYS_SHUTDOWN: '/v1/sys/state/off',
	SYS_TIMEZONE: '/v1/sys/timezone',
	SYS_NETWORK_INTERFACES: '/v1/sys/network/interfaces',
	SYS_USERS: '/v1/sys/users'
} as const;
