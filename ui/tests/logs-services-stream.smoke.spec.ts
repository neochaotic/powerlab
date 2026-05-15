import { expect, request as pwRequest } from '@playwright/test';
import { realBackendTest as test, REAL_BACKEND_BASE, loginAndGetToken } from './helpers/real-backend';

// Real-backend smoke for the per-service journald SSE endpoint
// (Sprint 21 PR 3 backend, PR 4 frontend). The mock-driven suite
// cannot validate two things this spec locks:
//
//   1. The endpoint actually exists at /v1/logs/services/{svc}/stream.
//   2. The response is NOT gzip-compressed even when the client
//      advertises Accept-Encoding: gzip. SvelteKit's EventSource +
//      most browsers send `Accept-Encoding: gzip` by default; an
//      upstream gzip middleware that doesn't skip text/event-stream
//      would buffer the response indefinitely (Sprint 18 #384 bug
//      class). Memory: feedback_sse_test_real_browser_headers.
//
// Env vars (whole spec skips without them):
//   POWERLAB_E2E_BASE     — gateway URL (e.g. http://192.168.18.86:8765)
//   POWERLAB_E2E_USER     — login username
//   POWERLAB_E2E_PASSWORD — login password
//
// Run with:
//   POWERLAB_E2E_BASE=http://<host>:8765 \
//     POWERLAB_E2E_USER=<u> POWERLAB_E2E_PASSWORD=<p> \
//     npx playwright test logs-services-stream.smoke.spec.ts

let sharedToken = '';
test.beforeAll(async () => {
	if (!REAL_BACKEND_BASE) return;
	const ctx = await pwRequest.newContext();
	sharedToken = await loginAndGetToken(ctx);
	await ctx.dispose();
});

test('journald stream endpoint is reachable and not gzip-encoded @smoke', async () => {
	const ctx = await pwRequest.newContext();
	try {
		// Send Accept-Encoding: gzip explicitly. If the gateway grew a
		// gzip middleware that ignores text/event-stream, this is the
		// only header configuration that surfaces the bug.
		const resp = await ctx.fetch(
			`${REAL_BACKEND_BASE}/v1/logs/services/gateway/stream`,
			{
				method: 'GET',
				headers: {
					Authorization: `Bearer ${sharedToken}`,
					Accept: 'text/event-stream',
					'Accept-Encoding': 'gzip'
				},
				// Don't follow / wait for the whole stream — first
				// chunk is enough to assert headers + initial frame.
				maxRedirects: 0,
				timeout: 8000
			}
		);

		// Status 200 means the JWT was accepted AND the handler reached
		// the streaming path. 401 here would be the JWT extraction
		// regression class (memory: raw_fetch_in_stores_is_bug_class).
		expect(resp.status()).toBe(200);

		const headers = resp.headers();
		// The hard assertion — Content-Encoding MUST NOT be gzip.
		// gzip-buffered SSE never sends the first event to the
		// browser (the gzip writer waits for >1024 bytes).
		expect(headers['content-encoding'] ?? '').not.toContain('gzip');

		// Content-Type must be text/event-stream (otherwise EventSource
		// rejects the connection with "EventSource's response has a
		// MIME type that is not 'text/event-stream'").
		expect(headers['content-type'] ?? '').toContain('text/event-stream');

		// X-Accel-Buffering=no is the reverse-proxy guard. Without it,
		// nginx/Apache buffer the response and the operator never sees
		// the live stream in a deployed environment.
		expect(headers['x-accel-buffering']).toBe('no');

		// First chunk should contain the `: stream-open` SSE comment
		// the handler emits before invoking journalctl. This proves
		// the server actually flushed something before any subprocess
		// output arrived.
		const body = await resp.text();
		expect(body).toContain(': stream-open');
	} finally {
		await ctx.dispose();
	}
});

test('journald stream rejects invalid service name @smoke', async () => {
	// Allowlist regression lock at the real gateway. If the lint /
	// allowlist regex ever loosens, this spec catches it.
	const ctx = await pwRequest.newContext();
	try {
		const resp = await ctx.fetch(
			`${REAL_BACKEND_BASE}/v1/logs/services/${encodeURIComponent('Gateway')}/stream`,
			{
				method: 'GET',
				headers: {
					Authorization: `Bearer ${sharedToken}`,
					Accept: 'text/event-stream'
				},
				timeout: 5000
			}
		);
		expect(resp.status()).toBe(400);
	} finally {
		await ctx.dispose();
	}
});
