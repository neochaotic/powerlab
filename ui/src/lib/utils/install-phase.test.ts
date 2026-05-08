/**
 * Bug #8: install overlay had no visible progress indicator. The
 * backend was already emitting "Phase N/M" markers in the log
 * stream; the UI just wasn't using them. These tests pin the
 * parser's contract so the visual progress bar stays in sync
 * with the backend's step semantics.
 */

import { describe, it, expect } from 'vitest';
import { parseLatestPhase, phaseProgress } from './install-phase';

describe('parseLatestPhase', () => {
	it('returns null when the log buffer is empty', () => {
		expect(parseLatestPhase('')).toBeNull();
	});

	it('returns null when no Phase line has been emitted yet', () => {
		const noisy = '[hash] Pulling fs layer\n[hash] Downloading\n[hash] Verifying\n';
		expect(parseLatestPhase(noisy)).toBeNull();
	});

	it('extracts the step, total, and label from a single phase line', () => {
		expect(parseLatestPhase('Phase 1/3: Pulling image...')).toEqual({
			step: 1,
			total: 3,
			label: 'Pulling image'
		});
	});

	it('returns the LATEST phase when multiple are present', () => {
		const log = `
[hash] Pulling fs layer
Phase 1/3: Pulling image
[hash] Downloading
[hash] Pull complete
Phase 2/3: Creating containers
[hash] Created
		`.trim();
		expect(parseLatestPhase(log)).toEqual({
			step: 2,
			total: 3,
			label: 'Creating containers'
		});
	});

	it('handles weird whitespace between Phase and N/M', () => {
		expect(parseLatestPhase('Phase  2 / 3 : Doing the thing.')).toEqual({
			step: 2,
			total: 3,
			label: 'Doing the thing'
		});
	});

	it('handles a line without a label after the colon', () => {
		expect(parseLatestPhase('Phase 3/3:')).toEqual({
			step: 3,
			total: 3,
			label: ''
		});
	});

	it('ignores malformed phase lines (non-numeric step)', () => {
		expect(parseLatestPhase('Phase x/3: nope')).toBeNull();
	});

	it('ignores malformed phase lines (zero total)', () => {
		expect(parseLatestPhase('Phase 1/0: division by zero')).toBeNull();
	});
});

describe('phaseProgress', () => {
	it('returns 0 for null phase', () => {
		expect(phaseProgress(null)).toBe(0);
	});

	it('returns step/total for a normal phase', () => {
		expect(phaseProgress({ step: 1, total: 3, label: 'a' })).toBeCloseTo(0.333, 2);
		expect(phaseProgress({ step: 2, total: 3, label: 'a' })).toBeCloseTo(0.666, 2);
	});

	it('clamps to 1 when step >= total', () => {
		expect(phaseProgress({ step: 3, total: 3, label: 'done' })).toBe(1);
		expect(phaseProgress({ step: 99, total: 3, label: 'over' })).toBe(1);
	});

	it('returns 0 for a zero-total (defensive)', () => {
		expect(phaseProgress({ step: 1, total: 0, label: '' })).toBe(0);
	});
});
