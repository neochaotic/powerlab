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

	// The "without untrack" buggy-pattern case was previously here
	// as a sanity check that Svelte 5's runtime catches the loop.
	// Removed: CI runs hit a hard 5-second timeout because Svelte's
	// infinite-loop detection has timing-dependent behaviour that's
	// reliable on a dev machine but flaky in resource-constrained
	// CI containers. The WITH untrack case above is the actual
	// regression lock — it asserts the fix is in place. The
	// without-case was documentation; documentation that's
	// unreliable is worse than no documentation.
});
