/**
 * Translation layer for the PowerLab compose extension.
 *
 * Compose YAML authored by PowerLab uses `x-powerlab:` as the canonical
 * extension key. The CasaOS store apps we ship and the broader CasaOS
 * ecosystem still use `x-casaos:` — and there was an intermediate
 * `x-web:` alias used briefly upstream. This module hides those three
 * names from the rest of the UI:
 *
 *   - `readPowerLabExt(yaml)` returns the merged extension regardless
 *     of which alias the author used. Priority order is x-powerlab →
 *     x-web → x-casaos. The first present, non-null map wins.
 *
 *   - `writePowerLabExt(yaml, ext)` writes back to whichever key is
 *     already present on the doc, preserving the author's choice.
 *     If no key is present, the new ext is written under `x-powerlab`
 *     (the canonical key).
 *
 *   - `EXTENSION_KEYS` is the priority-ordered tuple — exported so
 *     code that needs to enumerate the aliases (e.g. the parser
 *     warning in `apps/+page.svelte`) does not have to repeat them.
 *
 * Mirrors `service.LookupAppExtension` in
 * `backend/app-management/service/extension.go`.
 */

export const EXTENSION_KEYS = ['x-powerlab', 'x-web', 'x-casaos'] as const;
export type ExtensionKey = (typeof EXTENSION_KEYS)[number];

export const CANONICAL_EXTENSION_KEY: ExtensionKey = 'x-powerlab';

export type PowerLabExtension = Record<string, unknown>;

/** Returns the first present extension map and the key it was found under. */
export function readPowerLabExt(
	doc: Record<string, unknown> | null | undefined
): { ext: PowerLabExtension; key: ExtensionKey } | null {
	if (!doc) return null;
	for (const key of EXTENSION_KEYS) {
		const v = doc[key];
		if (v && typeof v === 'object' && !Array.isArray(v)) {
			return { ext: v as PowerLabExtension, key };
		}
	}
	return null;
}

/**
 * Writes the extension under the key it already used (if any), or under the
 * canonical key for new docs. Mutates `doc` in place and returns it for
 * chaining.
 */
export function writePowerLabExt(
	doc: Record<string, unknown>,
	ext: PowerLabExtension
): Record<string, unknown> {
	const existing = readPowerLabExt(doc);
	const key = existing?.key ?? CANONICAL_EXTENSION_KEY;
	doc[key] = ext;
	return doc;
}

/** Removes a single property from whichever extension key is present. */
export function deletePowerLabExtProperty(
	doc: Record<string, unknown> | null | undefined,
	property: string
): void {
	const existing = readPowerLabExt(doc);
	if (!existing) return;
	delete (existing.ext as Record<string, unknown>)[property];
}
