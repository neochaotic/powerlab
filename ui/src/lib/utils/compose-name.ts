/**
 * Compose service-name validation.
 *
 * Docker Compose accepts service names matching `[a-z0-9][a-z0-9_.-]*`.
 * The PowerLab Custom App form pre-validates so the user sees an inline
 * error instead of a backend stack trace after Deploy.
 *
 * Issue #240 regression: empty input previously fell back to the literal
 * string `"web"`, deploying the app under an unexpected name and confusing
 * the user. The empty case is now an explicit `'empty'` error.
 */

const NAME_RE = /^[a-z0-9][a-z0-9_.-]*$/;

export type ComposeNameError = 'empty' | 'invalid_chars';

export function validateComposeName(name: string): ComposeNameError | null {
	const v = name?.trim() ?? '';
	if (v === '') return 'empty';
	if (!NAME_RE.test(v)) return 'invalid_chars';
	return null;
}
