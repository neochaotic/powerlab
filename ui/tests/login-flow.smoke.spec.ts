import { expect } from '@playwright/test';
import { realBackendTest as test, REAL_BACKEND_BASE } from './helpers/real-backend';

// Real-backend login flow. Locks the contract that:
//   - POST /v1/users/login with the configured creds returns a token
//   - GET /v1/users/current with that token returns the username
//   - GET /v2/app_management/compose returns the installed-apps map
//
// What the mock-driven `auth.spec.ts` cannot prove: that the actual
// user-service is reachable through the gateway and accepts the
// token shape the panel uses. v0.6.x has surfaced this exact gap
// multiple times (raw fetch in stores → silent 401, gateway-proxy
// routing breakage, audit endpoints unreachable).
//
// Run:
//   POWERLAB_E2E_BASE=http://192.168.18.86:8765 \
//   POWERLAB_E2E_USER=neochaotic \
//   POWERLAB_E2E_PASSWORD=<pass> \
//   npx playwright test login-flow.smoke.spec.ts

test('login returns a token and the token gates user-current @smoke', async ({ request }) => {
	const user = process.env.POWERLAB_E2E_USER ?? 'admin';
	const pass = process.env.POWERLAB_E2E_PASSWORD ?? '';

	const loginRes = await request.post(`${REAL_BACKEND_BASE}/v1/users/login`, {
		data: { username: user, password: pass },
		headers: { 'Content-Type': 'application/json' }
	});
	expect(loginRes.ok(), `login HTTP ${loginRes.status()}: ${await loginRes.text()}`).toBe(true);

	const body = await loginRes.json();
	const token = body?.data?.token?.access_token;
	expect(token, 'login response missing data.token.access_token').toBeTruthy();
	expect(typeof token).toBe('string');
	expect((token as string).length).toBeGreaterThan(100);

	const currentRes = await request.get(`${REAL_BACKEND_BASE}/v1/users/current`, {
		headers: { Authorization: `Bearer ${token}` }
	});
	expect(currentRes.ok(), `current HTTP ${currentRes.status()}: ${await currentRes.text()}`).toBe(true);
	const current = await currentRes.json();
	expect(current?.data?.username).toBe(user);
});

test('installed-apps list reachable with a valid token @smoke', async ({ request }) => {
	const user = process.env.POWERLAB_E2E_USER ?? 'admin';
	const pass = process.env.POWERLAB_E2E_PASSWORD ?? '';

	const loginRes = await request.post(`${REAL_BACKEND_BASE}/v1/users/login`, {
		data: { username: user, password: pass },
		headers: { 'Content-Type': 'application/json' }
	});
	const token = (await loginRes.json())?.data?.token?.access_token;
	expect(token).toBeTruthy();

	const listRes = await request.get(`${REAL_BACKEND_BASE}/v2/app_management/compose`, {
		headers: { Authorization: `Bearer ${token}` }
	});
	expect(listRes.ok(), `compose list HTTP ${listRes.status()}`).toBe(true);
	const list = await listRes.json();
	// The shape is `data: { <project>: {...} }` — empty when no apps
	// installed, but the field must be present so the panel can iterate.
	expect(list).toHaveProperty('data');
	expect(typeof list.data).toBe('object');
});

test('missing token returns 401 — gateway HTTPJWT actually enforces @smoke', async ({ request }) => {
	// Lock the bug class: a panel call without Authorization MUST
	// be rejected by the gateway. Prior bugs (v0.6.12 audit endpoints
	// unreachable) shipped because the gateway returned the SPA index
	// HTML for an unauthenticated API call instead of 401.
	const res = await request.get(`${REAL_BACKEND_BASE}/v2/app_management/compose`);
	expect(res.status()).toBe(401);

	const ct = res.headers()['content-type'] ?? '';
	expect(
		ct.includes('application/json') || ct.includes('text/plain'),
		`401 should return JSON or plain, not HTML; got ${ct}`
	).toBe(true);
});
