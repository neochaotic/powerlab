/**
 * Install-phase parser.
 *
 * The compose install backend streams log lines like:
 *   Phase 1/3: Pulling image...
 *   Phase 2/3: Creating containers...
 *   Phase 3/3: Starting...
 *
 * Bug #8: the launchpad's install overlay shows the raw log stream
 * but no visual progress bar — users couldn't tell whether phase 2
 * was 0% or 99% along, and the wall of `[hash] Extracting` lines
 * blends together. We parse the latest "Phase N/M" line into a
 * structured step and render a stepped progress indicator on top
 * of the log box. Pure function so unit tests pin the behavior.
 */

export interface InstallPhase {
	step: number;
	total: number;
	label: string;
}

const PHASE_RE = /Phase\s+(\d+)\s*\/\s*(\d+)\s*[:.]?\s*([^\n\r]*)/i;

/**
 * Extract the most recent phase from a log buffer. Returns null
 * when no phase line has been emitted yet (SSE still streaming
 * pre-pull noise).
 *
 * Scans backwards for cheap "newest match wins" semantics — the
 * log buffer can grow to thousands of lines and we don't need to
 * parse them all.
 */
export function parseLatestPhase(logs: string): InstallPhase | null {
	if (!logs) return null;
	const lines = logs.split(/\r?\n/);
	for (let i = lines.length - 1; i >= 0; i--) {
		const m = PHASE_RE.exec(lines[i]);
		if (!m) continue;
		const step = parseInt(m[1], 10);
		const total = parseInt(m[2], 10);
		if (!Number.isFinite(step) || !Number.isFinite(total) || total <= 0) continue;
		return {
			step,
			total,
			label: (m[3] ?? '').trim().replace(/\.+$/, '')
		};
	}
	return null;
}

/**
 * Compute a 0..1 progress ratio. Pre-Phase-1 → 0; after the last
 * phase emits it stays at 1 (the terminal state — success or error
 * — replaces the bar with a different surface, so this never
 * displays as "100% but still working").
 */
export function phaseProgress(phase: InstallPhase | null): number {
	if (!phase) return 0;
	if (phase.total <= 0) return 0;
	return Math.min(1, Math.max(0, phase.step / phase.total));
}
