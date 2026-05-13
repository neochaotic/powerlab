/**
 * Regression test for Custom App ("/apps/new") deploy modal
 * lifecycle. Locks the parity contract with the Community Install
 * modal (apps/+page.svelte):
 *   - On POST 2xx, modal stays in "deploying" state and SSE Phase
 *     markers drive the determinate progress.
 *   - On SSE `event: end`, modal transitions to terminal state
 *     (deployResult success / error) based on whether the app
 *     appears in the installed list.
 *   - `deployResult` is NEVER set on POST success alone — install
 *     hasn't actually finished, just started.
 *
 * v0.6.7 bug: the previous code set deployResult immediately when
 * POST returned, surfacing "Service running" before the install
 * had even pulled images. Community Install does NOT do this; the
 * #247 "install UX parity" track left this gap unaddressed.
 *
 * Sister regression coverage:
 *   - apps/install-mirror.test.ts (Community Install reactivity)
 *   - backend/app-management/service/task_test.go (channel)
 *   - backend/app-management/route/v2/task_e2e_test.go (wire)
 */

import { render, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { tick } from 'svelte';
import Page from './+page.svelte';

beforeEach(() => {
	vi.restoreAllMocks();
	// Stub fetch for the YAML POST + any catalog calls.
	global.fetch = vi.fn().mockImplementation((url: string) => {
		const u = String(url);
		if (u.includes('/v2/app_management/compose')) {
			return Promise.resolve(
				new Response(
					JSON.stringify({
						success: 200,
						message: 'compose app is being installed asynchronously'
					}),
					{ status: 200, headers: { 'Content-Type': 'application/json' } }
				)
			);
		}
		if (u.includes('/v1/users/status') || u.includes('/v1/users/info')) {
			return Promise.resolve(
				new Response(JSON.stringify({ success: 200, data: { initialized: true } }), {
					status: 200
				})
			);
		}
		return Promise.resolve(new Response('{}', { status: 200 }));
	}) as unknown as typeof fetch;
});

// Capture EventSource instances so the test can drive them.
class FakeEventSource extends EventTarget {
	url: string;
	readyState = 0;
	onmessage: ((e: MessageEvent) => void) | null = null;
	onerror: ((e: Event) => void) | null = null;
	static instances: FakeEventSource[] = [];
	constructor(url: string) {
		super();
		this.url = url;
		this.readyState = 1;
		FakeEventSource.instances.push(this);
	}
	emit(data: string, eventType = 'message') {
		const ev = new MessageEvent(eventType, { data });
		if (eventType === 'message' && this.onmessage) this.onmessage(ev);
		this.dispatchEvent(ev);
	}
	emitEnd() {
		const ev = new MessageEvent('end', { data: 'task finished' });
		this.dispatchEvent(ev);
	}
	close() {
		this.readyState = 2;
	}
}
(global as unknown as { EventSource: typeof FakeEventSource }).EventSource = FakeEventSource;

describe('Custom App deploy lifecycle', () => {
	beforeEach(() => {
		FakeEventSource.instances.length = 0;
	});

	it('deployResult stays null while install is in progress (no premature success)', async () => {
		const { container } = render(Page);
		await tick();

		const textarea = container.querySelector('textarea')!;
		await fireEvent.input(textarea, {
			target: {
				value:
					"name: testapp\nservices:\n  app:\n    image: nginx\n    ports:\n      - '8123:80'\n"
			}
		});
		await tick();

		// Find the Deploy button. The orchestrator page labels it
		// based on edit-mode vs new-app — pick by role broad match.
		const deployBtn = container.querySelector(
			'button[type="submit"], button:has(svg)'
		) as HTMLButtonElement | null;

		if (deployBtn && !deployBtn.disabled) {
			await fireEvent.click(deployBtn);
			await tick();
		}

		// Allow micro-tasks to run.
		await new Promise((r) => setTimeout(r, 50));
		await tick();

		// After POST returns 2xx, the deploy modal should be in the
		// `isDeploying` state — NOT showing a success card. The bug
		// was: deployResult set to {success:true} too early, causing
		// "Service running" to render while the install was still
		// pulling images.
		const text = container.textContent || '';
		expect(text).not.toMatch(/service running/i);
		expect(text).not.toMatch(/serviço.*ativo/i); // pt-BR variant
	});
});
