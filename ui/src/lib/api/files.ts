/**
 * File System API endpoints.
 *
 * Maps directly to CasaOS Go backend v1 file routes.
 * Zero business logic — just sends requests and returns typed responses.
 */

import { api } from './client';

// ─── DTOs (match Go backend ObjResp / FsListResp exactly) ─────────────

export interface FileItem {
	name: string;
	size: number;
	is_dir: boolean;
	modified: string;
	sign: string;
	thumb: string;
	type: number;
	path: string;
	date: string;
	extensions: Record<string, unknown> | null;
}

export interface FileListResponse {
	content: FileItem[];
	total: number;
	index: number;
	size: number;
}

export interface ApiResult<T> {
	success: number;
	message: string;
	data: T;
}

// ─── API Functions ────────────────────────────────────────────────────

/** List directory contents */
export function listDirectory(path: string, index = 1, size = 1000) {
	const params = new URLSearchParams({ path, index: String(index), size: String(size) });
	return api.get<ApiResult<FileListResponse>>(`/v1/folder?${params}`);
}

/** Create a new folder */
export function createFolder(path: string) {
	return api.post<ApiResult<null>>('/v1/folder', { path });
}

/** Create a new file */
export function createFile(path: string) {
	return api.post<ApiResult<null>>('/v1/file', { path });
}

/** Rename a file or folder */
export function renamePath(oldPath: string, newPath: string) {
	return api.put<ApiResult<null>>('/v1/file/name', { old_path: oldPath, new_path: newPath });
}

/** Delete files/folders (batch) */
export function deletePaths(paths: string[]) {
	return api.delete<ApiResult<null>>('/v1/batch', {
		body: JSON.stringify(paths),
		headers: { 'Content-Type': 'application/json' }
	});
}

/** Move or copy files */
export function operateFileOrDir(type: 'move' | 'copy', to: string, items: Array<{ from: string }>) {
	return api.post<ApiResult<null>>('/v1/batch/task', {
		type,
		to,
		item: items.map((i) => ({ from: i.from }))
	});
}

/** Get directory size */
export function getDirectorySize(path: string) {
	return api.get<ApiResult<{ size: number }>>(`/v1/folder/size?path=${encodeURIComponent(path)}`);
}

/** Download a single file. Returned URL is suitable for an <a> href, an
 * <img>/<video>/<audio> src, or a window.open(). Authentication rides on
 * the HttpOnly `access_token` cookie set at login, which the browser
 * sends automatically on these same-origin GETs — so the JWT no longer
 * appears in the URL, browser history, or access logs (#35). The gateway
 * still accepts `?token=` as a fallback for older sessions, but we no
 * longer emit it. */
export function getDownloadUrl(path: string): string {
	return `/v1/file?path=${encodeURIComponent(path)}`;
}

/** Download multiple files as archive — authenticates via the HttpOnly
 * cookie, same as getDownloadUrl (#35). */
export function getBatchDownloadUrl(files: string[], format: 'zip' | 'tar' | 'targz' = 'zip'): string {
	return `/v1/batch?files=${files.map(encodeURIComponent).join(',')}&format=${format}`;
}

/** Default starting path for the Files page. The backend prefers
 * `<os-user-home>/PowerLab/` when the JWT is from a real OS account
 * (PAM / dscl), and falls back to `/var/lib/powerlab/files` for
 * SetupWizard users with no OS home dir. The UI calls this on
 * Files-page mount instead of dropping the user into `/DATA` (which
 * doesn't exist on dev hosts) or `/` (hostile root listing). */
export function getDefaultFilesPath() {
	return api.get<ApiResult<{ path: string; source: 'os-home' | 'system-fallback' }>>('/v1/file/home');
}

/** Read file content (text) */
export function readFileContent(path: string) {
	return api.get<ApiResult<string>>(`/v1/file/content?path=${encodeURIComponent(path)}`);
}

/** Update an EXISTING file's content. Returns 404 if the file does not
 * exist — caller should use createFileContent for new files. Mirrors
 * filebrowser's POST=create vs PUT=update split. */
export function updateFileContent(filePath: string, fileContent: string) {
	return api.put<ApiResult<null>>('/v1/file', { file_path: filePath, file_content: fileContent });
}

/** Create a NEW file with the given content. Returns 409 if the file
 * already exists (pass override=true to replace). The editor's
 * "Save as new" flow calls this; the legacy "+ New File" button
 * (which sends empty content) also goes here via createFile(). */
export function createFileContent(filePath: string, fileContent: string, override = false) {
	const q = override ? '?override=true' : '';
	return api.post<ApiResult<null>>(`/v1/file${q}`, { file_path: filePath, file_content: fileContent });
}

/** Upload file chunk */
export function uploadFileChunk(
	path: string,
	file: File,
	chunkNumber: number,
	totalChunks: number,
	relativePath: string
) {
	const formData = new FormData();
	formData.append('file', file);
	formData.append('path', path);
	formData.append('chunkNumber', String(chunkNumber));
	formData.append('totalChunks', String(totalChunks));
	formData.append('filename', file.name);
	formData.append('relativePath', relativePath);
	return api.upload<ApiResult<null>>('/v1/file/upload', formData);
}
