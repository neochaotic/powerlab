import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';

// Failure-UX contract:
//
// The updater store polls hourly. A single transient failure should
// NOT surface a red error banner — the system is healthy and the
// upgrade-check is a background concern. Only after a streak of
// failures spanning enough wall-clock time does the UI escalate.
//
// Contract pinned here:
//
//   - On the first success since boot: clear errors, remember the
//     timestamp.
//   - On a single failure: don't show an error to the UI, just bump
//     a counter. The store exposes `transientFailure` (no banner) vs
//     `persistentFailure` (banner).
//   - `persistentFailure` is only true when:
//       (a) we have 3+ consecutive failures AND
//       (b) the last known success is older than 6h (OR we never had
//           a success this session).
//   - On a success during a failure streak: counter resets + lastSuccess
//     timestamp updates + both `transientFailure` and
//     `persistentFailure` go false.
//
// The AboutPane reads these flags instead of the raw `.error` string.

import {
	getUpdaterFailureState,
	recordCheckSuccess,
	recordCheckFailure,
	resetUpdaterFailureState
} from './updater-failure-state';

beforeEach(() => {
	resetUpdaterFailureState();
	vi.useFakeTimers();
	vi.setSystemTime(new Date('2026-05-15T00:00:00Z'));
});

afterEach(() => {
	vi.useRealTimers();
});

describe('Updater failure-UX state machine', () => {
	it('starts clean — no transient, no persistent failure', () => {
		const s = getUpdaterFailureState();
		expect(s.transientFailure).toBe(false);
		expect(s.persistentFailure).toBe(false);
		expect(s.consecutiveFailures).toBe(0);
	});

	it('records first success — clears any errors, sets timestamp', () => {
		recordCheckSuccess();
		const s = getUpdaterFailureState();
		expect(s.lastSuccessTs).not.toBeNull();
		expect(s.consecutiveFailures).toBe(0);
		expect(s.transientFailure).toBe(false);
		expect(s.persistentFailure).toBe(false);
	});

	it('single failure → transient only, no banner', () => {
		recordCheckSuccess(); // bootstrap
		recordCheckFailure('timeout');
		const s = getUpdaterFailureState();
		expect(s.consecutiveFailures).toBe(1);
		expect(s.transientFailure).toBe(true);
		expect(s.persistentFailure).toBe(false);
	});

	it('2 failures still not persistent (need 3+)', () => {
		recordCheckSuccess();
		recordCheckFailure('boom');
		recordCheckFailure('boom');
		const s = getUpdaterFailureState();
		expect(s.consecutiveFailures).toBe(2);
		expect(s.persistentFailure).toBe(false);
	});

	it('3 failures but lastSuccess fresh (< 6h) → still NOT persistent', () => {
		recordCheckSuccess();
		// Failures all within 1h of last success.
		vi.setSystemTime(new Date('2026-05-15T00:30:00Z'));
		recordCheckFailure('boom');
		recordCheckFailure('boom');
		recordCheckFailure('boom');
		const s = getUpdaterFailureState();
		expect(s.consecutiveFailures).toBe(3);
		expect(s.persistentFailure).toBe(false);
		expect(s.transientFailure).toBe(true);
	});

	it('3+ failures AND lastSuccess older than 6h → persistent banner', () => {
		recordCheckSuccess();
		vi.setSystemTime(new Date('2026-05-15T07:00:00Z')); // 7h later
		recordCheckFailure('boom');
		recordCheckFailure('boom');
		recordCheckFailure('boom');
		const s = getUpdaterFailureState();
		expect(s.persistentFailure).toBe(true);
		expect(s.transientFailure).toBe(false); // mutually exclusive
	});

	it('never-succeeded session: 3+ failures escalate to persistent', () => {
		// No success ever — lastSuccessTs stays null. 3+ failures escalate.
		recordCheckFailure('boom');
		recordCheckFailure('boom');
		recordCheckFailure('boom');
		const s = getUpdaterFailureState();
		expect(s.persistentFailure).toBe(true);
	});

	it('success during failure streak clears everything', () => {
		recordCheckFailure('boom');
		recordCheckFailure('boom');
		recordCheckFailure('boom');
		recordCheckSuccess();
		const s = getUpdaterFailureState();
		expect(s.consecutiveFailures).toBe(0);
		expect(s.transientFailure).toBe(false);
		expect(s.persistentFailure).toBe(false);
		expect(s.lastSuccessTs).not.toBeNull();
	});

	it('lastCheckedHumanRelative reports time-since', () => {
		recordCheckSuccess();
		vi.setSystemTime(new Date('2026-05-15T02:30:00Z')); // +2.5h
		const s = getUpdaterFailureState();
		// Some "Xh ago" form — exact wording is up to the formatter,
		// but it must not be empty when we have a success on record.
		expect(s.lastCheckedHumanRelative).toBeTruthy();
		expect(s.lastCheckedHumanRelative).toMatch(/\d/);
	});

	it('lastCheckedHumanRelative null when no success ever', () => {
		recordCheckFailure('boom');
		const s = getUpdaterFailureState();
		expect(s.lastCheckedHumanRelative).toBeNull();
	});
});
