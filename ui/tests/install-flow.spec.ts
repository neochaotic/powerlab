import { expect, test } from '@playwright/test';
import { installBaselineMocks } from './helpers/mock-backend';

// Install-flow E2E (#344 / Sprint 14 #265). Mandatory pre-tag gate.
//
// Locks the v0.6.7 → v0.6.8 bug class: install modal stuck on
// "Preparing" forever because (1) Task.Subscribe emitted a buffer as
// a single multi-line SSE message that EventSource silently dropped,
// (2) the channel never closed so `event: end` never fired, and (3)
// the InstallState mirror $effect read+wrote the same key and
// triggered effect_update_depth_exceeded.
//
// The fix landed across three places: backend SSE splitting, channel
// close-after-finish, and the untrack mirror pattern in /apps. This
// test exercises the full FE side so any regression in that wiring
// trips a CI red.
//
// Strategy: drive the Custom App deploy flow (/apps/new) because it
// owns its own YAML input — no catalog mock needed. Stub the POST
// deploy + the SSE task-logs stream with canned phases, then assert
// the InstallModal reaches its Success terminal state and never logs
// `effect_update_depth_exceeded`.

const SSE_BODY = [
	'data: Phase 1/3: Pulling images...',
	'',
	'data: Pulled myapp:latest',
	'',
	'data: Phase 2/3: Creating containers...',
	'',
	'data: Phase 3/3: Starting containers...',
	'',
	'data: Installation completed successfully!',
	'',
	'event: end',
	'data:',
	'',
	''
].join('\n');

test.describe('install-flow E2E', () => {
	test.beforeEach(async ({ page }) => {
		await installBaselineMocks(page);
	});

	test('Custom App deploy → modal opens → terminal Success', async ({ page }) => {
		const consoleErrors: string[] = [];
		page.on('console', (msg) => {
			if (msg.type() === 'error') consoleErrors.push(msg.text());
		});

		// SSE first — more-specific glob wins under Playwright's
		// last-handler-wins resolution.
		await page.route('**/v2/app_management/compose/task/**/logs**', (route) =>
			route.fulfill({
				status: 200,
				headers: {
					'Content-Type': 'text/event-stream',
					'Cache-Control': 'no-cache',
					Connection: 'keep-alive'
				},
				body: SSE_BODY
			})
		);

		// One handler covers POST (deploy start) AND GET (installed
		// list polled by finalizeDeploy). route.fulfill delivers the
		// SSE body atomically, but EventSource parses multiple
		// `data: …\n\n` segments as discrete events — the same wire
		// format the backend emits and what v0.6.7 mishandled.
		await page.route('**/v2/app_management/compose', (route) => {
			const method = route.request().method();
			if (method === 'POST') {
				return route.fulfill({
					status: 200,
					contentType: 'application/json',
					body: JSON.stringify({ message: 'compose app is being installed asynchronously' })
				});
			}
			return route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({
					data: {
						'my-app': {
							store_info: { id: 'my-app', title: { en_us: 'My App' } },
							status: { running: true }
						}
					}
				})
			});
		});

		await page.goto('/apps/new');

		// Default YAML defines a `web` service; the shared InstallModal
		// owns the modal-side rendering.
		const deployBtn = page.getByRole('button', { name: /^deploy$/i });
		await expect(deployBtn).toBeVisible();

		// Native click in browser context — Playwright's .click() can
		// mis-route when a parent transform stacking-context (Svelte's
		// <main> in /apps/new applies transform: translateZ) shifts
		// hit-testing to a different element. .click() on the DOM node
		// always lands on the target.
		await deployBtn.evaluate((b) => (b as HTMLButtonElement).click());

		// Terminal Success: the SSE `event: end` fires finalizeDeploy(),
		// which polls the installed-app list (we returned `web` present)
		// and flips phase → 'success'. The 'install-modal-open' testid
		// is unique to the Success state.
		//
		// We don't assert each intermediate phase: route.fulfill delivers
		// the SSE body atomically, so the modal races through 'starting'
		// → 'success' faster than a 5s poll. The Success terminal proves
		// the full pipeline (POST + SSE multi-line parse + event:end +
		// finalizeDeploy) worked end-to-end.
		await expect(page.getByTestId('install-modal-open')).toBeVisible({ timeout: 15_000 });

		// Regression-lock the v0.6.7 effect-loop signature. If this
		// pops back in any form (mirror update, derived feedback, etc.)
		// the engine shouts "effect_update_depth_exceeded" and the
		// modal freezes; we want that to fail CI loudly.
		const loopErrors = consoleErrors.filter((e) => /effect_update_depth_exceeded/i.test(e));
		expect(loopErrors, `Svelte 5 reactivity loop returned: ${loopErrors.join(' | ')}`).toEqual([]);
	});
});
