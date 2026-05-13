<script lang="ts">
	// Test fixture for install-mirror.test.ts — exercises the exact
	// $effect pattern that froze the install modal in v0.6.7. Exports
	// test hooks (setPhase, setLogs, getRuns, getStorePhase) so the
	// test can drive state changes without DOM interaction.
	import { untrack } from 'svelte';

	type Props = { useUntrack: boolean };
	let { useUntrack }: Props = $props();

	// Local reactive state — equivalent of installPhase / installLogs
	// in routes/apps/+page.svelte.
	let localPhase = $state('starting');
	let localLogs = $state('');

	// Shared store — equivalent of installState's `installing` record.
	const store = $state<Record<string, { phase: string; logs: string }>>({});
	store['test-app'] = { phase: 'installing', logs: '' };

	function storeGet(id: string) {
		return store[id] ?? null;
	}
	function storeUpdate(id: string, patch: { phase?: string; logs?: string }) {
		const cur = store[id];
		if (!cur) return;
		store[id] = { ...cur, ...patch };
	}

	// Plain (non-reactive) counter — the test reads it via getRuns()
	// after each setPhase/setLogs call. Putting this in $state would
	// itself create a self-loop when incremented from inside the
	// effect; we use a closure variable so the test instrumentation
	// doesn't muddy the reactivity contract under test.
	// svelte-ignore non_reactive_update
	let runs = 0;

	$effect(() => {
		// Snapshot the local reactive deps. These are the signals the
		// effect SHOULD react to.
		const phase = localPhase;
		const logs = localLogs;
		runs += 1;

		if (useUntrack) {
			// FIXED pattern from PR #341: store read + write inside
			// untrack so the write does not re-trigger this effect.
			untrack(() => {
				if (!storeGet('test-app')) return;
				storeUpdate('test-app', { phase, logs });
			});
		} else {
			// BUGGY v0.6.7 pattern: reads and writes the same
			// reactive store key in the same effect → loop.
			if (!storeGet('test-app')) return;
			storeUpdate('test-app', { phase, logs });
		}
	});

	// Hooks exposed to the test runner.
	export function setPhase(v: string) {
		localPhase = v;
	}
	export function setLogs(v: string) {
		localLogs = v;
	}
	export function getRuns(): number {
		return runs;
	}
	export function getStorePhase(): string | undefined {
		return store['test-app']?.phase;
	}
</script>

<div data-testid="install-mirror-fixture">
	phase={localPhase} logs={localLogs} runs={runs} storePhase={store['test-app']?.phase}
</div>
