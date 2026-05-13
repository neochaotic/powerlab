/**
 * Regression test for v0.6.7 "install modal frozen on Preparing" —
 * the third root cause (after the two backend SSE bugs in PR #341).
 *
 * The `$effect` in routes/apps/+page.svelte read AND wrote the same
 * install-state key (`installing[id]`) reactively. Svelte 5's
 * reactivity engine detected the depth-exceeded loop and aborted
 * further updates: the install modal froze on "Preparing" with no
 * buttons responding. The user captured the runtime error in the
 * browser console as `effect_update_depth_exceeded`.
 *
 * Sister regression coverage:
 *   - backend/app-management/service/task_test.go (10 cases: channel)
 *   - backend/app-management/route/v2/task_e2e_test.go (2 cases: wire)
 *   - this file (UI reactivity contract)
 *
 * Together these three test files lock all three legs of the
 * v0.6.7 root-cause trio so the bug class cannot ship past CI again.
 */

import { render } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import { tick } from 'svelte';
import Fixture from './install-mirror.fixture.svelte';

describe('install mirror $effect — reactivity loop regression', () => {
	it('with untrack: one effect run per local-state change', async () => {
		const { component } = render(Fixture, { props: { useUntrack: true } });
		const c = component as unknown as {
			setPhase: (v: string) => void;
			setLogs: (v: string) => void;
			getRuns: () => number;
			getStorePhase: () => string | undefined;
		};

		await tick();
		const initial = c.getRuns();

		c.setPhase('success');
		await tick();
		expect(c.getRuns()).toBe(initial + 1);
		expect(c.getStorePhase()).toBe('success');

		c.setLogs('Phase 1/3\nPhase 2/3\nPhase 3/3\n');
		await tick();
		expect(c.getRuns()).toBe(initial + 2);

		// Hard ceiling: a reactivity loop would balloon this past
		// Svelte's default depth (~10) in a few ticks. Anything
		// under 20 is healthy.
		expect(c.getRuns()).toBeLessThan(20);
	});

	it('without untrack: Svelte 5 reactivity engine catches the loop', async () => {
		// Capture Svelte's runtime error output without polluting
		// the test runner display.
		const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {});
		const consoleWarn = vi.spyOn(console, 'warn').mockImplementation(() => {});

		let renderError: Error | null = null;
		try {
			const { component } = render(Fixture, { props: { useUntrack: false } });
			const c = component as unknown as {
				setPhase: (v: string) => void;
			};

			await tick();
			c.setPhase('a');
			await tick();
			c.setPhase('b');
			await tick();
			c.setPhase('c');
			await tick();
		} catch (e) {
			renderError = e as Error;
		}

		const allCalls = [
			...consoleError.mock.calls.flat().map(String),
			...consoleWarn.mock.calls.flat().map(String)
		];
		const loopDetected =
			(renderError?.message ?? '').includes('effect_update_depth_exceeded') ||
			allCalls.some((s) => s.includes('effect_update_depth_exceeded'));

		consoleError.mockRestore();
		consoleWarn.mockRestore();

		// Svelte 5 catches the loop and surfaces the marker. If
		// future Svelte versions change semantics, this assertion
		// becomes the canary — revisit the fix design.
		expect(loopDetected).toBe(true);
	});
});
