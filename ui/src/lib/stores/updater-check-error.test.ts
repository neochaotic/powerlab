import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';

// When the user clicks "Check now" and the request fails, the UI must
// say WHY — an HTTP 401 (re-auth) is a different problem than a network
// timeout, and both are different from a 500 on the gateway. Before this
// fix a failed manual check fell through to the silent "transient"
// branch, rendering "Background check did not complete" with no reason —
// which a user reads as "no update available". A user-initiated check
// always surfaces its outcome; only background polls stay quiet on a
// single blip.

import { describeCheckFailure } from './updater-failure-state';

describe('describeCheckFailure', () => {
	it('names the HTTP status for a 401 and hints at re-auth', () => {
		const msg = describeCheckFailure({ status: 401, message: 'unauthorized' });
		expect(msg).toContain('401');
		expect(msg.toLowerCase()).toMatch(/sign in|session|author/);
	});

	it('describes a network error (status 0) as unreachable, not "HTTP 0"', () => {
		const msg = describeCheckFailure({ status: 0, message: 'Network error' });
		expect(msg).not.toContain('HTTP 0');
		expect(msg.toLowerCase()).toMatch(/unreachable|network|connect/);
	});

	it('names the HTTP status for a 5xx gateway error', () => {
		const msg = describeCheckFailure({ status: 503, message: 'unavailable' });
		expect(msg).toContain('503');
	});

	it('falls back to the error message for a plain Error', () => {
		expect(describeCheckFailure(new Error('boom'))).toContain('boom');
	});

	it('returns a generic message for an unknown throwable', () => {
		expect(describeCheckFailure(null)).toBeTruthy();
		expect(describeCheckFailure(undefined)).toBeTruthy();
	});
});

// ── store-level surfacing ──────────────────────────────────────────
// The store must remember whether the most recent check was user-
// initiated, so the AboutPane can choose between the loud (manual) and
// quiet (background) failure paths.

vi.mock('$lib/api/updater', () => ({
	checkForUpdate: vi.fn(),
	preflightUpdate: vi.fn(),
	installUpdate: vi.fn(),
	getUpgradeStatus: vi.fn()
}));

import { checkForUpdate } from '$lib/api/updater';
import { updaterStore } from './updater.svelte';
import { resetUpdaterFailureState } from './updater-failure-state';

beforeEach(() => {
	resetUpdaterFailureState();
	updaterStore.check = null;
	updaterStore.error = null;
	vi.mocked(checkForUpdate).mockReset();
});

afterEach(() => {
	vi.restoreAllMocks();
});

describe('Updater store — user-initiated check failure surfacing', () => {
	it('a manual check that fails sets a status-bearing error AND flags it manual', async () => {
		vi.mocked(checkForUpdate).mockRejectedValueOnce({
			status: 401,
			message: 'unauthorized'
		});

		await updaterStore.refresh(true);

		expect(updaterStore.check).toBeNull();
		expect(updaterStore.error).toContain('401');
		expect(updaterStore.lastCheckFailedManually).toBe(true);
	});

	it('a background poll that fails does NOT flag a manual failure', async () => {
		vi.mocked(checkForUpdate).mockRejectedValueOnce({
			status: 0,
			message: 'Network error'
		});

		await updaterStore.refresh(); // no arg → background

		expect(updaterStore.lastCheckFailedManually).toBe(false);
	});

	it('clears the stale manual-failure flag while a re-check is in flight', async () => {
		// First manual check fails → flag set, red line shown.
		vi.mocked(checkForUpdate).mockRejectedValueOnce({
			status: 401,
			message: 'unauthorized'
		});
		await updaterStore.refresh(true);
		expect(updaterStore.lastCheckFailedManually).toBe(true);

		// User clicks "Check now" again. While the request is in flight,
		// `error` is null — so the flag must NOT still be true, or the
		// AboutPane renders "Update check failed — ." (empty reason)
		// under a "Checking…" button.
		let resolveCheck: (v: { current: string; decision: 'up_to_date' }) => void;
		vi.mocked(checkForUpdate).mockReturnValueOnce(
			new Promise((res) => {
				resolveCheck = res;
			})
		);
		const inflight = updaterStore.refresh(true);

		expect(updaterStore.loading).toBe(true);
		expect(updaterStore.error).toBeNull();
		expect(updaterStore.lastCheckFailedManually).toBe(false);

		resolveCheck!({ current: '0.7.4', decision: 'up_to_date' });
		await inflight;
		expect(updaterStore.lastCheckFailedManually).toBe(false);
	});

	it('a successful manual check clears the manual-failure flag and error', async () => {
		vi.mocked(checkForUpdate).mockRejectedValueOnce({
			status: 401,
			message: 'unauthorized'
		});
		await updaterStore.refresh(true);
		expect(updaterStore.lastCheckFailedManually).toBe(true);

		vi.mocked(checkForUpdate).mockResolvedValueOnce({
			current: '0.7.4',
			decision: 'up_to_date'
		});
		await updaterStore.refresh(true);

		expect(updaterStore.error).toBeNull();
		expect(updaterStore.lastCheckFailedManually).toBe(false);
		expect(updaterStore.check?.decision).toBe('up_to_date');
	});
});
