/**
 * Docker & App Management API endpoints.
 *
 * Maps directly to CasaOS Go backend v2 app_management routes.
 * Zero business logic — just sends requests and returns typed responses.
 */

import { api } from './client';

// ─── DTOs (match Go backend OpenAPI exactly) ──────────────────────────

export interface BaseResponse {
	message: string;
}

/** Helper hints surfaced by the UI before/after install. Mirrors
 * CasaOS's `x-casaos.tips` field. `before_install` is a locale map
 * shown above the install confirm button (e.g. "default password is
 * auto-generated to /DATA/AppData/<app>/admin_token.txt — copy it
 * before first login"). `custom` is a post-install string shown on
 * the installed-app drawer (e.g. "set ADMIN_TOKEN env var to
 * override"). Both rendered as markdown so upstream apps that
 * already use bullet lists / code spans keep working. */
export interface AppTips {
	before_install?: Record<string, string>;
	custom?: string;
}

export interface ComposeAppStoreInfo {
	store_app_id: string;
	title: Record<string, string>;
	image: Record<string, string>;
	description: Record<string, string>;
	tagline: Record<string, string>;
	icon: string;
	screenshot_link?: string[];
	thumbnail: string;
	author: string;
	developer: string;
	category: string;
	hostname?: string;
	port_map?: string;
	index?: string;
	main?: string;
	tips?: AppTips;
	is_uncontrolled?: boolean;
	// Provenance block — populated by the umbrel-catalog sync pipeline
	// (#307 Phase 1+). Absent for apps loaded from CasaOS-AppStore /
	// Big-Bear / local-store — the UI falls back to an icon-URL
	// heuristic in `lib/utils/app-source.ts` so every app surfaces
	// SOME source label, never empty.
	source?: {
		catalog: string;
		upstream_id?: string;
		upstream_repo?: string;
		upstream_commit?: string;
		upstream_path?: string;
		transform_version?: string;
		synced_at?: string;
	};
}

export interface ComposeAppWithStoreInfo {
	store_info: ComposeAppStoreInfo;
	compose: unknown; // The raw docker-compose JSON
	status: string; // 'running', 'exited', etc.
	update_available?: boolean;
	is_uncontrolled?: boolean;
}

export interface ComposeAppStoreInfoLists {
	installed: string[];
	list: Record<string, ComposeAppStoreInfo>;
}

export interface ContainerSummary {
	ID: string;
	Names: string[];
	Image: string;
	State: string;
	Status: string;
	Ports: Array<{ PrivatePort: number; PublicPort: number; Type: string }>;
}

export interface ComposeAppContainers {
	main: string;
	containers: Record<string, ContainerSummary>;
}

export interface ApiResult<T> extends BaseResponse {
	data: T;
}

// ─── API Functions ────────────────────────────────────────────────────

/** Get app list from registered app stores */
export function getAppStoreList(category?: string, authorType?: string, recommend?: boolean) {
	const params = new URLSearchParams();
	if (category) params.append('category', category);
	if (authorType) params.append('author_type', authorType);
	if (recommend !== undefined) params.append('recommend', String(recommend));
	
	const qs = params.toString() ? `?${params}` : '';
	return api.get<ApiResult<ComposeAppStoreInfoLists>>(`/v2/app_management/apps${qs}`);
}

/** Get the list of installed compose apps */
export function getInstalledApps() {
	return api.get<ApiResult<Record<string, ComposeAppWithStoreInfo>>>('/v2/app_management/compose');
}

/** Install a compose app from YAML */
export function installComposeApp(yamlContent: string, dryRun = false) {
	return api.postYaml<BaseResponse>(`/v2/app_management/compose${dryRun ? '?dry_run=true' : ''}`, yamlContent);
}

/**
 * Apply settings to an EXISTING compose app from YAML.
 *
 * Backend's `applyComposeAppSettings` (PUT /v2/app_management/compose/{id})
 * has skip-self port-conflict logic — the app's own currently-published
 * ports are excluded from the in-use list before validation. The
 * `installComposeApp` POST path does NOT have this skip-self logic, so
 * re-deploying an existing app via POST falsely fails with "ports in
 * use". The orchestrator routes here when the URL carries `?id=X` and
 * `fork` is not set. Closes #65.
 */
export function applyComposeAppSettings(id: string, yamlContent: string, dryRun = false) {
	const path = `/v2/app_management/compose/${encodeURIComponent(id)}${dryRun ? '?dry_run=true' : ''}`;
	return api.putYaml<BaseResponse>(path, yamlContent);
}

/** Get compose YAML from the app store catalog (for install) */
export function getStoreAppYaml(id: string) {
	return api.get<string>(`/v2/app_management/apps/${id}/compose`, {
		headers: { 'Accept': 'application/yaml' }
	});
}

/** Get compose details (interpolated YAML) of a locally installed app */
export function getComposeApp(id: string) {
	return api.get<string>(`/v2/app_management/compose/${id}`, {
		headers: { 'Accept': 'application/yaml' }
	});
}

/** Uninstall a compose app */
export function uninstallComposeApp(id: string, deleteConfigFolder = false) {
	return api.delete<BaseResponse>(`/v2/app_management/compose/${id}?delete_config_folder=${deleteConfigFolder}`);
}

/** Update a compose app (pulls new image and restarts) */
export function updateComposeApp(id: string, force = false) {
	return api.patch<BaseResponse>(`/v2/app_management/compose/${id}${force ? '?force=true' : ''}`);
}

/** Start/restart/stop a compose app.
 * Backend's OpenAPI schema declares the body as a bare JSON string ("stop"),
 * not an object ({ status: "stop" }). Sending an object trips the OpenAPI
 * validator with "value is not one of the allowed values" and the call
 * silently fails — no UI error since stores swallow the throw. */
export function setComposeAppStatus(id: string, status: 'start' | 'stop' | 'restart') {
	return api.put<BaseResponse>(`/v2/app_management/compose/${id}/status`, status);
}

/** Get the list of containers of a compose app */
export function getComposeAppContainers(id: string) {
	return api.get<ApiResult<ComposeAppContainers>>(`/v2/app_management/compose/${id}/containers`);
}

/** Get the logs of a compose app */
export function getComposeAppLogs(id: string, lines = 1000) {
	return api.get<ApiResult<string>>(`/v2/app_management/compose/${id}/logs?lines=${lines}`);
}

export interface ComposeAppStats {
	cpu_percent: number;
	mem_used_bytes: number;
	mem_limit_bytes: number;
	net_rx: number;
	net_tx: number;
}

/** Get metrics/stats for a compose app */
export function getComposeAppStats(id: string) {
	return api.get<ApiResult<ComposeAppStats>>(`/v2/app_management/compose/${id}/stats`);
}

/** Cross-platform port availability probe used by the Custom App Builder. */
export interface PortCheckResult {
	data: Record<string, boolean>;        // "8080" → true if available
	suggestions: Record<string, number>;  // "8080" → 8081 (next free port)
}
export function checkPorts(ports: Array<number | string>, proto: 'tcp' | 'udp' = 'tcp') {
	const list = ports.map(String).filter(Boolean).join(',');
	if (!list) return Promise.resolve({ data: {}, suggestions: {} } as PortCheckResult);
	return api.get<PortCheckResult>(
		`/v2/app_management/ports/check?ports=${encodeURIComponent(list)}&proto=${proto}`
	);
}

export interface DiskUsage {
	bytes: number;
}

/** Get disk usage for a compose app */
export function getComposeAppDiskUsage(id: string) {
	return api.get<ApiResult<DiskUsage>>(`/v2/app_management/compose/${id}/disk-usage`);
}

/** Get app management configuration (storage paths, etc) */
export function getAppManagementConfig() {
	return api.get<AppManagementConfig>('/v2/app_management/config');
}

export interface AppManagementConfig {
	storage_path: string;
	apps_path: string;
}
