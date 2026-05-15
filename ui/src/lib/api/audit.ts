/**
 * Audit log API client (Sprint 16 #357 B1f).
 *
 * Maps to the read-side endpoints registered by the gateway's
 * management route (see backend/common/utils/audit/endpoints.go):
 *
 *   GET /v1/audit/recent — paginated records, newest first
 *   GET /v1/audit/stats  — row count + oldest/newest + file size
 *
 * Both endpoints sit behind the gateway's JWT middleware; failures
 * with HTTP 401 trigger the centralised onAuthError handler from
 * Sprint 16 C3 (logout + "Session expired" toast).
 */

import { api } from './client';

/**
 * AuditRecord mirrors `backend/common/utils/audit/types.go::Record`.
 * Field names use JSON tag form (snake_case) so the backend's
 * standard `encoding/json` marshal matches without adapters.
 *
 * Nullable fields (user_id, username) arrive as null when the
 * request was loopback / pre-auth.
 *
 * Per ADR-0035 the storage is JSONL — there is no DB-assigned id;
 * `ts_us + request_id` is the natural row identity for UI keying.
 */
export interface AuditRecord {
	/** RFC 3339 timestamp string with microsecond precision. */
	ts: string;
	/** Microseconds since the Unix epoch — sortable integer form. */
	ts_us: number;
	method: string;
	path: string;
	query?: string;
	status: number;
	latency_us: number;
	user_id: number | null;
	username: string | null;
	remote_ip: string;
	request_id?: string;
	/**
	 * Record-type discriminator. Absent / empty → HTTP request audit
	 * (the original record type). "ui_error" → frontend error captured
	 * by the SvelteKit shell and POSTed to /v1/audit/frontend-error.
	 */
	kind?: string;
	/**
	 * Kind-specific fields. For "ui_error":
	 *   { message, stack?, url?, ua?, viewport?: { w, h } }.
	 */
	payload?: Record<string, unknown>;
}

/**
 * AuditStats summarises the audit table: row count, oldest/newest
 * timestamps in microseconds, and on-disk file size. Returned by
 * /v1/audit/stats.
 */
export interface AuditStats {
	row_count: number;
	oldest_unix_us: number;
	newest_unix_us: number;
	file_size_bytes: number;
	path: string;
}

interface Envelope<T> {
	data: T;
	message?: string;
}

/**
 * RecentOptions are the optional query parameters that map 1:1 to
 * the backend's `RecentOptions`. Pass `{}` for the default (newest
 * 100 records, no filters).
 */
export interface RecentOptions {
	/** Cap on records returned. Default 100, max 1000 (backend clamps). */
	limit?: number;
	/** Filter to a specific user_id (omit for all users). */
	userId?: number;
	/** Cursor: only rows with ts > sinceUnixMicros are returned. */
	sinceUnixMicros?: number;
}

/**
 * Fetch the most recent audit records. Newest-first ordering.
 */
export async function getAuditRecent(opts: RecentOptions = {}): Promise<AuditRecord[]> {
	const params = new URLSearchParams();
	if (opts.limit !== undefined) params.set('limit', String(opts.limit));
	if (opts.userId !== undefined) params.set('user_id', String(opts.userId));
	if (opts.sinceUnixMicros !== undefined) params.set('since', String(opts.sinceUnixMicros));

	const qs = params.toString();
	const url = qs ? `/v1/audit/recent?${qs}` : '/v1/audit/recent';
	const res = await api.get<Envelope<AuditRecord[]>>(url);
	return res.data ?? [];
}

/**
 * Fetch the audit table summary (row count, time bounds, file size).
 */
export async function getAuditStats(): Promise<AuditStats> {
	const res = await api.get<Envelope<AuditStats>>('/v1/audit/stats');
	return res.data;
}
