/**
 * Logs API client.
 *
 * Maps to the read-side log-viewer endpoints mounted on the
 * gateway's public mux:
 *
 *   GET /v1/logs/files            — list .log files in /var/log/powerlab/
 *   GET /v1/logs/files/{name}     — read the last N bytes (default 200 KB)
 *
 * Distinct from /v1/audit/* which exposes the HTTP audit JSONL —
 * this client exposes raw service stdout (.log files). Per-service
 * journald streaming with live follow is a separate, bigger
 * feature.
 */

import { api } from './client';

export interface LogFileEntry {
	/** Filename (e.g. "app-management.log"). Never a path. */
	name: string;
	/** File size in bytes. */
	size_bytes: number;
	/** Last-modified timestamp, RFC 3339. */
	modified_ts: string;
	/** Last-modified timestamp, microseconds since Unix epoch. */
	modified_us: number;
}

interface Envelope<T> {
	data: T;
	message?: string;
}

/**
 * List the `.log` files in /var/log/powerlab/. Rotated archives
 * (`.log.gz`) are intentionally excluded by the backend; surface
 * here is whatever .log files are currently active. Sorted newest-
 * first by mtime.
 */
export async function listLogFiles(): Promise<LogFileEntry[]> {
	const res = await api.get<Envelope<LogFileEntry[]>>('/v1/logs/files');
	return res.data;
}

/**
 * Fetch the last `tail` bytes of a log file. Defaults to 200 KB
 * (backend default — keep this in sync). The backend caps the
 * value at 5 MB to prevent a misclick from DoS-ing the gateway.
 *
 * Returns plain text (no envelope) — the backend writes the file
 * bytes directly with Content-Type: text/plain. Filename is
 * validated by the backend against a strict allowlist before any
 * filesystem access (path-traversal hardening).
 */
export async function readLogFile(name: string, tail?: number): Promise<string> {
	const qs = tail ? `?tail=${tail}` : '';
	return api.get<string>(`/v1/logs/files/${encodeURIComponent(name)}${qs}`, {
		// Override the api client's JSON parsing — this endpoint
		// returns raw text, not JSON. Same pattern as the YAML
		// downloads.
		headers: { Accept: 'text/plain' }
	});
}
