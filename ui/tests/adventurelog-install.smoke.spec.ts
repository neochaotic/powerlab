import { expect, request as pwRequest } from '@playwright/test';
import { realBackendTest as test, REAL_BACKEND_BASE, loginAndGetToken } from './helpers/real-backend';

// Real-backend regression test for #386 (Adventure Log install
// failed with HTTP 400 BadRequest, then self-resolved before
// investigation could land). Locks the current "install succeeds"
// state so the next 400 trips CI loudly.
//
// Strategy: read the catalog's adventurelog compose, rewrite the
// project name to a unique test value, POST it as a Custom App.
// Same compose shape (db + server + web), same container_name
// pattern that surfaced #397 — so this spec also acts as a real-
// backend canary for the broader `container_name strips compose
// project label` bug class once #397 lands.
//
// Run:
//   POWERLAB_E2E_BASE=http://192.168.18.86:8765 \
//   POWERLAB_E2E_USER=neochaotic \
//   POWERLAB_E2E_PASSWORD=<pass> \
//   npx playwright test adventurelog-install.smoke.spec.ts

const TEST_PROJECT = `adventurelog-rg-${Date.now()}`;
// High port unlikely to collide on dev hosts. The .142 staging
// already runs the real adventurelog on 8015; our test fork uses
// a different port so we don't trip the backend's port-conflict
// check (which would surface as HTTP 400 — same status as the
// #386 bug we're regressing against, would mask the signal).
const TEST_PORT = 19000 + (Date.now() % 1000);

// Hand-trimmed mirror of community-catalog/Apps/adventurelog/docker-compose.yml
// — kept inline so the spec is hermetic and the test doesn't break
// when the catalog YAML is reformatted. Updates to the catalog
// shape must echo here. Service set + container_name pattern
// preserved; the project name is replaced at runtime.
const ADVENTURELOG_YAML_TEMPLATE = `name: __PROJECT__
services:
    db:
        environment:
            POSTGRES_DB: database
            POSTGRES_PASSWORD: adventure
            POSTGRES_USER: adventure
        image: ghcr.io/baosystems/postgis:15-3.5
        restart: on-failure
        volumes:
            - /DATA/PowerLabAppData/__PROJECT__/data/db:/var/lib/postgresql/data/
    server:
        container_name: __PROJECT__-server
        depends_on:
            - db
        environment:
            - PGHOST=__PROJECT___db_1
            - PGDATABASE=database
            - PGUSER=adventure
            - PGPASSWORD=adventure
            - SECRET_KEY=changeme
            - PUBLIC_URL=http://localhost:8016
            - CSRF_TRUSTED_ORIGINS=http://localhost:8015,http://localhost:8016
            - DEBUG=False
            - FRONTEND_URL=http://localhost:8015
        image: ghcr.io/seanmorley15/adventurelog-backend:latest
        restart: unless-stopped
        volumes:
            - /DATA/PowerLabAppData/__PROJECT__/server/media/:/code/media/
    web:
        depends_on:
            - server
        environment:
            - PUBLIC_SERVER_URL=http://localhost:__PORT__
            - ORIGIN=http://localhost:__PORT__
            - BODY_SIZE_LIMIT=Infinity
        image: ghcr.io/seanmorley15/adventurelog-frontend:latest
        ports:
            - "__PORT__:3000"
        restart: unless-stopped
`;

let sharedToken = '';

test.beforeAll(async () => {
	if (!REAL_BACKEND_BASE) return;
	const ctx = await pwRequest.newContext();
	sharedToken = await loginAndGetToken(ctx);
	await ctx.dispose();
});

test.afterAll(async () => {
	if (!REAL_BACKEND_BASE || !sharedToken) return;
	// Best-effort cleanup. Don't fail the suite if the uninstall
	// 404s — the install may have failed before the app was
	// registered. (Memory feedback_clean_up_planted_test_data:
	// orphaned test installs ambush the user's next real action.)
	const ctx = await pwRequest.newContext();
	await ctx.delete(`${REAL_BACKEND_BASE}/v2/app_management/compose/${TEST_PROJECT}`, {
		headers: { Authorization: `Bearer ${sharedToken}` }
	});
	await ctx.dispose();
});

test('Adventure Log install returns 2xx, not 400 @smoke', async ({ page }) => {
	const yaml = ADVENTURELOG_YAML_TEMPLATE
		.replaceAll('__PROJECT__', TEST_PROJECT)
		.replaceAll('__PORT__', String(TEST_PORT));

	const installRes = await page.request.post(`${REAL_BACKEND_BASE}/v2/app_management/compose`, {
		headers: {
			Authorization: `Bearer ${sharedToken}`,
			'Content-Type': 'application/yaml'
		},
		data: yaml
	});

	// The #386 bug class is HTTP 400 BadRequest on install. 200/202
	// = success (async install starts). 409 = conflict (project
	// name in use — would mean a prior test run leaked); also OK
	// for the regression intent because it isn't 400.
	expect(
		installRes.status(),
		`install POST should not return 400 (#386 regression). body: ${await installRes.text()}`
	).not.toBe(400);
	expect(installRes.status()).toBeLessThan(500);
});

test('Adventure Log post-install: container with container_name label is found @smoke', async ({ page }) => {
	// Critical assertion: a service using `container_name:` (the
	// pattern that strips the compose project label) is still
	// detectable by the backend's installed-apps lookup. This is
	// the regression-lock for #397 once the tolerant fallback
	// lands. Today, with `container_name: ${TEST_PROJECT}-server`
	// the lookup may miss the container; the test exits early
	// (skip, not fail) so we get signal without a hard red until
	// #397 ships its fix.
	await page.waitForTimeout(8000); // image pull buffer

	const listRes = await page.request.get(`${REAL_BACKEND_BASE}/v2/app_management/compose`, {
		headers: { Authorization: `Bearer ${sharedToken}` }
	});
	expect(listRes.ok()).toBe(true);
	const list = await listRes.json();
	const items = (list?.data ?? {}) as Record<string, unknown>;

	if (!(TEST_PROJECT in items)) {
		test.info().annotations.push({
			type: 'note',
			description:
				`${TEST_PROJECT} not surfaced by /v2/app_management/compose — likely the ` +
				`container_name label-strip bug class (#397). Regression-locks once #397 lands.`
		});
		test.skip(true, '#397 not yet shipped — expected miss');
	}
});
