/**
 * Regression lock for issue #240 (inline name validation).
 *
 * Before #240 the Custom App form silently fell back to `'web'` when the
 * user cleared the name input. Users thought they'd cleared the form
 * and then found an app named `web` deployed. The fix made empty input
 * an explicit error. These tests pin that contract so a future refactor
 * cannot reintroduce the silent fallback.
 */

import { describe, it, expect } from 'vitest';
import { validateComposeName } from './compose-name';

describe('validateComposeName', () => {
	describe('rejects empty input (#240 regression)', () => {
		it('returns "empty" for the empty string', () => {
			expect(validateComposeName('')).toBe('empty');
		});

		it('returns "empty" for whitespace-only input', () => {
			expect(validateComposeName('   ')).toBe('empty');
		});

		it('returns "empty" for a tab character', () => {
			expect(validateComposeName('\t')).toBe('empty');
		});
	});

	describe('rejects characters outside the Docker Compose set', () => {
		it('rejects uppercase', () => {
			expect(validateComposeName('Foo')).toBe('invalid_chars');
		});

		it('rejects spaces', () => {
			expect(validateComposeName('my app')).toBe('invalid_chars');
		});

		it('rejects @ # $ symbols', () => {
			expect(validateComposeName('app@1')).toBe('invalid_chars');
			expect(validateComposeName('app#1')).toBe('invalid_chars');
			expect(validateComposeName('app$1')).toBe('invalid_chars');
		});

		it('rejects names that start with `-`, `_`, or `.`', () => {
			expect(validateComposeName('-foo')).toBe('invalid_chars');
			expect(validateComposeName('_foo')).toBe('invalid_chars');
			expect(validateComposeName('.foo')).toBe('invalid_chars');
		});
	});

	describe('accepts valid Docker Compose service names', () => {
		it('accepts simple lowercase', () => {
			expect(validateComposeName('foo')).toBeNull();
		});

		it('accepts digits and hyphens after the first char', () => {
			expect(validateComposeName('foo-1')).toBeNull();
			expect(validateComposeName('foo_bar')).toBeNull();
			expect(validateComposeName('foo.bar')).toBeNull();
		});

		it('accepts names that start with a digit', () => {
			expect(validateComposeName('1foo')).toBeNull();
		});

		it('trims surrounding whitespace before validating', () => {
			expect(validateComposeName('  nginx  ')).toBeNull();
		});
	});
});
