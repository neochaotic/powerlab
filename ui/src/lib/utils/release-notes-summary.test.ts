import { describe, it, expect } from 'vitest';
import { summarizeReleaseNotes, MAX_SUMMARY_CHARS } from './release-notes-summary';

describe('summarizeReleaseNotes', () => {
	it('returns short summaries unchanged + truncated=false', () => {
		const out = summarizeReleaseNotes('Quick fix for the install bar.');
		expect(out.text).toBe('Quick fix for the install bar.');
		expect(out.truncated).toBe(false);
	});

	it('returns first paragraph when there are multiple paragraphs', () => {
		const raw = 'Sprint 13 — install UX parity.\n\nSprint 12 framing (preserved for context):\nblah blah blah blah';
		const out = summarizeReleaseNotes(raw);
		expect(out.text).toBe('Sprint 13 — install UX parity.');
		expect(out.truncated).toBe(true);
	});

	it('strips the noisy "framing (preserved for context)" tail explicitly', () => {
		const raw = 'New install UX. Fixed two runtime bugs. Sprint 12 framing (preserved for context): old stuff';
		const out = summarizeReleaseNotes(raw);
		expect(out.text).toBe('New install UX. Fixed two runtime bugs.');
		expect(out.truncated).toBe(true);
	});

	it('hard-truncates first paragraph past MAX_SUMMARY_CHARS with ellipsis', () => {
		const longLine = 'a'.repeat(MAX_SUMMARY_CHARS + 50);
		const out = summarizeReleaseNotes(longLine);
		expect(out.text.length).toBeLessThanOrEqual(MAX_SUMMARY_CHARS + 1);
		expect(out.text.endsWith('…')).toBe(true);
		expect(out.truncated).toBe(true);
	});

	it('truncates on word boundary when possible', () => {
		const raw = 'word '.repeat(80); // 5 chars * 80 = 400 chars
		const out = summarizeReleaseNotes(raw);
		expect(out.text.endsWith('…')).toBe(true);
		// Must not end with a partial word (last non-ellipsis char should be space or letter, but word should be whole)
		const beforeEllipsis = out.text.slice(0, -1).trimEnd();
		expect(beforeEllipsis.split(' ').pop()).toBe('word');
	});

	it('treats empty input as empty output, not truncated', () => {
		const out = summarizeReleaseNotes('');
		expect(out.text).toBe('');
		expect(out.truncated).toBe(false);
	});

	it('treats whitespace-only input as empty output', () => {
		const out = summarizeReleaseNotes('   \n\n  ');
		expect(out.text).toBe('');
		expect(out.truncated).toBe(false);
	});

	it('collapses internal line breaks in the first paragraph for clean rendering', () => {
		const raw = 'Sprint 13 closeout.\nUX polish included.\n\nLater stuff...';
		const out = summarizeReleaseNotes(raw);
		expect(out.text).toBe('Sprint 13 closeout. UX polish included.');
		expect(out.truncated).toBe(true);
	});

	it('exposes the full original via .full property for the expand action', () => {
		const raw = 'First.\n\nSecond paragraph.';
		const out = summarizeReleaseNotes(raw);
		expect(out.full).toBe(raw);
	});
});
