import { expect, request as pwRequest } from '@playwright/test';
import { realBackendTest as test, REAL_BACKEND_BASE, loginAndGetToken } from './helpers/real-backend';

// Real-backend extension of the audit-pane smoke. The base spec
// (audit-pane.smoke.spec.ts) proves the pane renders against live
// data; this spec proves the ui_error capture pipeline lands in
// the same data:
//
//   POST /v1/audit/frontend-error (the endpoint the SvelteKit
//   shell's window.onerror hook calls)
//   → audit Recorder
//   → JSONL on disk + ring buffer
//   → /v1/audit/recent returns the ui_error record
//   → AuditPane renders the Bug icon row
//
// Mock-driven AuditPane.test.ts already locks the render contract;
// this spec locks the WIRE path that the mock fakes out.

let token = '';

test.beforeAll(async () => {
	if (!REAL_BACKEND_BASE) return;
	const ctx = await pwRequest.newContext();
	token = await loginAndGetToken(ctx);
	await ctx.dispose();
});

test('POST /v1/audit/frontend-error → record appears in /v1/audit/recent @smoke', async ({ request }) => {
	const sentinel = `pl-smoke-uierror-${Date.now()}`;

	const postRes = await request.post(`${REAL_BACKEND_BASE}/v1/audit/frontend-error`, {
		headers: {
			Authorization: `Bearer ${token}`,
			'Content-Type': 'application/json'
		},
		data: {
			message: `Sentinel: ${sentinel}`,
			stack: 'at /tests/audit-ui-error.smoke.spec.ts:1:1',
			url: '/tests',
			ua: 'Playwright smoke'
		}
	});
	expect(postRes.status(), `POST: ${await postRes.text()}`).toBe(202);

	// Async recorder — give it up to 2s to flush the JSONL line and
	// publish into the ring buffer. The recorder's MaxLatency is
	// 200ms; we poll every 200ms with a generous ceiling.
	let found = false;
	for (let i = 0; i < 10; i++) {
		const listRes = await request.get(`${REAL_BACKEND_BASE}/v1/audit/recent?limit=50`, {
			headers: { Authorization: `Bearer ${token}` }
		});
		expect(listRes.ok()).toBe(true);
		const list = await listRes.json();
		const records = (list?.data ?? []) as Array<{
			kind?: string;
			payload?: Record<string, unknown>;
		}>;
		// The sentinel ensures we don't false-match another run's
		// record sitting in the ring buffer.
		if (
			records.some(
				(r) => r.kind === 'ui_error' && (r.payload?.message as string | undefined)?.includes(sentinel)
			)
		) {
			found = true;
			break;
		}
		await new Promise((r) => setTimeout(r, 200));
	}

	expect(found, `sentinel ${sentinel} did not appear in /v1/audit/recent within 2s`).toBe(true);
});

// Note: a "real-backend UI render" version of this test was
// considered but dropped — it's flaky because the AuditPane
// fetches on mount with no auto-refresh, and timing between the
// POST + recorder flush + page navigation is racy without a
// dedicated test hook. The render contract is locked by the
// mocked vitest (AuditPane.test.ts: "renders ui_error rows with
// the bug badge"); this file locks the wire contract that the
// mock pretends. Combined coverage is the right shape — both
// halves of the pipeline are pinned.
