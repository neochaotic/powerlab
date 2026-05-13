/**
 * Squeeze a release manifest's `summary` field into a UI-safe blurb
 * for the "Update available" surface in AboutPane / future toasts.
 *
 * Why this exists: the manifest's `summary` field has a 250-char
 * contract (see docs/UPDATE_MANIFEST.md "summary (string, required,
 * ≤ 250 chars)"). Historically that contract was not enforced anywhere,
 * so summaries grew into multi-KB walls of dev notes — the v0.6.6
 * manifest summary that triggered the v0.6.6 retro item carried the
 * Sprint 13 closeout + "Sprint 13 partial framing (preserved for
 * context)" + Sprint 12 framing + Sprint 12 framing (again) + v0.6.2
 * framing, all dumped raw into a single `<span>` in AboutPane.svelte.
 *
 * Rules applied (in order):
 *   1. Strip the literal "Sprint N framing (preserved for context):"
 *      tail — that block-marker is a manifest write antipattern that
 *      keeps inflating each cut.
 *   2. Take only the first paragraph (split on blank-line).
 *   3. Collapse internal single newlines so the blurb renders as one
 *      flowing sentence(s), not a 5-line paragraph.
 *   4. Hard-truncate to MAX_SUMMARY_CHARS on a word boundary, with
 *      an ellipsis suffix. The `truncated` flag tells the UI whether
 *      to show a "Show more" link.
 *
 * The full original lives in `.full` for the expand-inline action;
 * AboutPane keeps a `<details>` (or equivalent) gated on `.truncated`.
 *
 * This is a stopgap until release-manifest.yaml summaries are cut-time
 * enforced to ≤ 250 chars (CI gate landing in the same PR). Even after
 * the gate lands, this function stays so the UI is defensive against a
 * manifest that somehow ships past the gate.
 */

export const MAX_SUMMARY_CHARS = 240;

export interface SummarizedReleaseNotes {
	text: string;
	full: string;
	truncated: boolean;
}

const FRAMING_TAIL_RE = /\s*Sprint \d+(\.\d+)? (partial )?framing \(preserved for context\)[:\s].*$/is;

export function summarizeReleaseNotes(raw: string): SummarizedReleaseNotes {
	const full = raw ?? '';
	const trimmed = full.trim();

	if (trimmed === '') {
		return { text: '', full, truncated: false };
	}

	// Rule 1 — strip the framing tail before we split paragraphs.
	const withoutFraming = trimmed.replace(FRAMING_TAIL_RE, '').trim();
	const framingStripped = withoutFraming !== trimmed;

	// Rule 2 — first paragraph.
	const firstParagraph = withoutFraming.split(/\n\s*\n/)[0] ?? '';

	// Rule 3 — collapse internal whitespace runs into single spaces.
	const flat = firstParagraph.replace(/\s+/g, ' ').trim();

	// Did we drop content along the way?
	let truncated = framingStripped || flat !== withoutFraming.replace(/\s+/g, ' ').trim();

	// Rule 4 — hard length cap, word boundary.
	let text = flat;
	if (text.length > MAX_SUMMARY_CHARS) {
		const slice = text.slice(0, MAX_SUMMARY_CHARS);
		const lastSpace = slice.lastIndexOf(' ');
		const cutAt = lastSpace > MAX_SUMMARY_CHARS / 2 ? lastSpace : MAX_SUMMARY_CHARS;
		text = slice.slice(0, cutAt).trimEnd() + '…';
		truncated = true;
	}

	return { text, full, truncated };
}
