import { expect, request as pwRequest } from '@playwright/test';
import { realBackendTest as test, REAL_BACKEND_BASE, loginAndGetToken } from './helpers/real-backend';

// Real-backend test for the Logs viewer endpoints. The mocked
// LogsPane.test.ts proves the UI handles a fake list; this spec
// proves the gateway actually serves the right files at the right
// paths and the path-traversal hardening holds against live
// requests.

let token = '';

test.beforeAll(async () => {
	if (!REAL_BACKEND_BASE) return;
	const ctx = await pwRequest.newContext();
	token = await loginAndGetToken(ctx);
	await ctx.dispose();
});

test('/v1/logs/files returns the .log entries, newest first @smoke', async ({ request }) => {
	const res = await request.get(`${REAL_BACKEND_BASE}/v1/logs/files`, {
		headers: { Authorization: `Bearer ${token}` }
	});
	expect(res.ok(), `HTTP ${res.status()}: ${await res.text()}`).toBe(true);

	const body = await res.json();
	expect(body).toHaveProperty('data');
	const entries = body.data as Array<{ name: string; size_bytes: number; modified_us: number }>;
	expect(Array.isArray(entries)).toBe(true);

	// Every entry must satisfy the allowlist shape — no archives,
	// no funny chars, just `*.log`.
	for (const e of entries) {
		expect(e.name).toMatch(/^[A-Za-z0-9._-]+\.log$/);
		expect(e.size_bytes).toBeGreaterThanOrEqual(0);
	}

	// Sorted DESC by mtime.
	for (let i = 1; i < entries.length; i++) {
		expect(
			entries[i - 1].modified_us,
			`entry ${i - 1} (${entries[i - 1].name}) older than entry ${i} — sort broken`
		).toBeGreaterThanOrEqual(entries[i].modified_us);
	}
});

test('/v1/logs/files/{name} returns plain text tail with content-type @smoke', async ({ request }) => {
	// Find any existing .log to read. A staging box always has at
	// least app-management.log.
	const listRes = await request.get(`${REAL_BACKEND_BASE}/v1/logs/files`, {
		headers: { Authorization: `Bearer ${token}` }
	});
	const list = (await listRes.json()).data as Array<{ name: string }>;
	if (list.length === 0) {
		test.skip(true, 'no log files on staging to tail');
		return;
	}
	const name = list[0].name;

	const tailRes = await request.get(`${REAL_BACKEND_BASE}/v1/logs/files/${name}`, {
		headers: { Authorization: `Bearer ${token}` }
	});
	expect(tailRes.ok()).toBe(true);

	const ct = tailRes.headers()['content-type'] ?? '';
	expect(ct).toContain('text/plain');
	// X-Log-Size-Bytes mirrors the on-disk size — useful for the UI
	// to show "showing last 200KB of N MB".
	expect(tailRes.headers()['x-log-size-bytes']).toBeDefined();
});

test('path traversal rejected: ../etc/passwd → 400 @smoke', async ({ request }) => {
	const res = await request.get(`${REAL_BACKEND_BASE}/v1/logs/files/../../../etc/passwd`, {
		headers: { Authorization: `Bearer ${token}` }
	});
	// The last-segment-only routing means "passwd" hits the
	// allowlist check (no .log suffix) → 400. Either way it MUST
	// NOT return /etc/passwd contents.
	expect(res.status()).toBeLessThan(500);
	expect(res.status()).not.toBe(200);
	const body = await res.text();
	expect(body.toLowerCase()).not.toContain('root:');
	expect(body.toLowerCase()).not.toContain('/bin/bash');
});

test('non-.log extension rejected: audit.jsonl → 400 @smoke', async ({ request }) => {
	// The audit.jsonl IS in the log dir but NOT exposed by this
	// endpoint — the Audit pane reads it through /v1/audit/recent.
	// Belt+braces: ensure the logs endpoint cannot serve the raw
	// JSONL file.
	const res = await request.get(`${REAL_BACKEND_BASE}/v1/logs/files/audit.jsonl`, {
		headers: { Authorization: `Bearer ${token}` }
	});
	expect(res.status()).toBe(400);
});
