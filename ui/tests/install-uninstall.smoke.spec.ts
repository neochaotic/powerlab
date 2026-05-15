import { expect, request as pwRequest } from '@playwright/test';
import { realBackendTest as test, REAL_BACKEND_BASE, loginAndGetToken } from './helpers/real-backend';

// Real-backend install → SSE stream → finalize → uninstall round-
// trip. Locks the install pipeline end-to-end against the actual
// gateway + app-management. Mocked install-flow.spec.ts proves the
// UI handles a fake SSE stream; this spec proves the BACKEND
// produces the right stream and the install actually lands a
// container.
//
// Uses a uniquely-named, tiny alpine compose so:
//   - we never collide with the user's installed apps
//   - the test cleans up after itself in afterAll (memory
//     feedback_clean_up_planted_test_data — orphaned test installs
//     ambush the user's next real action)
//   - the image is small enough that the pull phase completes in
//     under the default 30s expect timeout

const PROJECT = `pl-smoke-install-${Date.now()}`;
const HIGH_PORT = 28000 + (Date.now() % 1000);
const YAML = `name: ${PROJECT}
services:
  app:
    image: alpine:latest
    command: sleep 300
    ports:
      - "${HIGH_PORT}:8080"
`;

let token = '';

test.beforeAll(async () => {
	if (!REAL_BACKEND_BASE) return;
	const ctx = await pwRequest.newContext();
	token = await loginAndGetToken(ctx);
	await ctx.dispose();
});

test.afterAll(async () => {
	if (!REAL_BACKEND_BASE || !token) return;
	// Best-effort cleanup — uninstall succeeds or 404s on the
	// already-gone case; either way we don't leak a planted install.
	const ctx = await pwRequest.newContext();
	await ctx.delete(`${REAL_BACKEND_BASE}/v2/app_management/compose/${PROJECT}`, {
		headers: { Authorization: `Bearer ${token}` }
	});
	await ctx.dispose();
});

test('install → SSE log stream emits Phase markers → finalize → uninstall @smoke', async ({ request }) => {
	// 1. Install the YAML via the Custom App POST path.
	const installRes = await request.post(`${REAL_BACKEND_BASE}/v2/app_management/compose`, {
		headers: {
			Authorization: `Bearer ${token}`,
			'Content-Type': 'application/yaml'
		},
		data: YAML
	});
	expect(installRes.status(), `install: ${await installRes.text()}`).toBeLessThan(300);

	// 2. SSE log stream must emit at least the Phase 1/3 marker
	// within a reasonable window. Image is alpine:latest (~5 MB);
	// pull completes in seconds on a warm Docker daemon.
	const sseUrl = `${REAL_BACKEND_BASE}/v2/app_management/compose/task/${PROJECT}/logs?token=${encodeURIComponent(token)}`;
	const sseRes = await request.get(sseUrl, { timeout: 10_000 });
	expect(sseRes.ok()).toBe(true);
	// Wire-level lock — Sprint 18 SSE fix regression catch.
	const contentEncoding = sseRes.headers()['content-encoding'] ?? '';
	expect(contentEncoding.toLowerCase()).not.toContain('gzip');
	const contentType = sseRes.headers()['content-type'] ?? '';
	expect(contentType).toContain('text/event-stream');

	const body = await sseRes.text();
	expect(body).toContain('Phase 1/3');

	// 3. Poll until the project appears in the installed-apps list
	// (finalizeDeploy's contract).
	let appeared = false;
	for (let i = 0; i < 30; i++) {
		const listRes = await request.get(`${REAL_BACKEND_BASE}/v2/app_management/compose`, {
			headers: { Authorization: `Bearer ${token}` }
		});
		const list = await listRes.json();
		if (list?.data && PROJECT in list.data) {
			appeared = true;
			break;
		}
		await new Promise((r) => setTimeout(r, 1000));
	}
	expect(appeared, 'project did not appear in installed-apps list within 30s').toBe(true);

	// 4. Uninstall. The backend's tolerant fallback (PR 3) handles
	// any container_name label-strip case — same `compose.alpine`
	// fixture we used here doesn't set container_name, so the
	// happy path applies.
	const uninstallRes = await request.delete(
		`${REAL_BACKEND_BASE}/v2/app_management/compose/${PROJECT}`,
		{ headers: { Authorization: `Bearer ${token}` } }
	);
	expect(uninstallRes.ok(), `uninstall: ${await uninstallRes.text()}`).toBe(true);

	// 5. Project should disappear from the installed-apps list within
	// a few seconds.
	let disappeared = false;
	for (let i = 0; i < 15; i++) {
		const listRes = await request.get(`${REAL_BACKEND_BASE}/v2/app_management/compose`, {
			headers: { Authorization: `Bearer ${token}` }
		});
		const list = await listRes.json();
		if (!list?.data || !(PROJECT in list.data)) {
			disappeared = true;
			break;
		}
		await new Promise((r) => setTimeout(r, 1000));
	}
	expect(disappeared, 'project did not disappear after uninstall within 15s').toBe(true);
});
