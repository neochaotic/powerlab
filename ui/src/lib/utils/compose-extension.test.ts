/**
 * Regression lock for the PowerLab compose extension priority chain.
 *
 * ADR-0021 ("Docker label namespace and appdata path") commits PowerLab
 * to writing `x-powerlab:` on new compose docs while continuing to read
 * `x-web:` and `x-casaos:` so existing CasaOS-store apps keep working
 * during the rebrand. Sprint 8 landed the rebrand wave; these tests
 * pin the read priority + the round-trip-preserving write so a future
 * cleanup cannot silently drop legacy keys before the deprecation
 * window ends (issue #201).
 */

import { describe, it, expect } from 'vitest';
import {
	readPowerLabExt,
	writePowerLabExt,
	deletePowerLabExtProperty,
	EXTENSION_KEYS,
	CANONICAL_EXTENSION_KEY
} from './compose-extension';

describe('compose-extension', () => {
	describe('EXTENSION_KEYS', () => {
		it('is ordered x-powerlab → x-web → x-casaos', () => {
			expect(EXTENSION_KEYS).toEqual(['x-powerlab', 'x-web', 'x-casaos']);
		});

		it('canonical key is x-powerlab', () => {
			expect(CANONICAL_EXTENSION_KEY).toBe('x-powerlab');
		});
	});

	describe('readPowerLabExt', () => {
		it('returns null for null/undefined doc', () => {
			expect(readPowerLabExt(null)).toBeNull();
			expect(readPowerLabExt(undefined)).toBeNull();
		});

		it('returns null when no extension key is present', () => {
			expect(readPowerLabExt({ services: {} })).toBeNull();
		});

		it('reads x-powerlab when only x-powerlab is present', () => {
			const doc = { 'x-powerlab': { title: 'A' } };
			expect(readPowerLabExt(doc)).toEqual({ ext: { title: 'A' }, key: 'x-powerlab' });
		});

		it('reads x-web when only x-web is present (legacy alias)', () => {
			const doc = { 'x-web': { title: 'B' } };
			expect(readPowerLabExt(doc)).toEqual({ ext: { title: 'B' }, key: 'x-web' });
		});

		it('reads x-casaos when only x-casaos is present (legacy alias)', () => {
			const doc = { 'x-casaos': { title: 'C' } };
			expect(readPowerLabExt(doc)).toEqual({ ext: { title: 'C' }, key: 'x-casaos' });
		});

		it('prefers x-powerlab over both legacy keys (priority order)', () => {
			const doc = {
				'x-powerlab': { title: 'P' },
				'x-web': { title: 'W' },
				'x-casaos': { title: 'C' }
			};
			expect(readPowerLabExt(doc)).toEqual({ ext: { title: 'P' }, key: 'x-powerlab' });
		});

		it('prefers x-web over x-casaos when x-powerlab is absent', () => {
			const doc = { 'x-web': { title: 'W' }, 'x-casaos': { title: 'C' } };
			expect(readPowerLabExt(doc)).toEqual({ ext: { title: 'W' }, key: 'x-web' });
		});

		it('skips null extension values', () => {
			const doc = { 'x-powerlab': null, 'x-casaos': { title: 'C' } };
			expect(readPowerLabExt(doc)).toEqual({ ext: { title: 'C' }, key: 'x-casaos' });
		});

		it('skips array extension values (must be a plain object)', () => {
			const doc = { 'x-powerlab': ['nope'], 'x-casaos': { title: 'C' } };
			expect(readPowerLabExt(doc)).toEqual({ ext: { title: 'C' }, key: 'x-casaos' });
		});

		it('skips primitive extension values', () => {
			const doc = { 'x-powerlab': 'string', 'x-casaos': { title: 'C' } };
			expect(readPowerLabExt(doc)).toEqual({ ext: { title: 'C' }, key: 'x-casaos' });
		});
	});

	describe('writePowerLabExt', () => {
		it('writes under x-powerlab on a new doc', () => {
			const doc: Record<string, unknown> = {};
			writePowerLabExt(doc, { title: 'X' });
			expect(doc).toEqual({ 'x-powerlab': { title: 'X' } });
		});

		it('preserves x-web when the author used x-web', () => {
			const doc: Record<string, unknown> = { 'x-web': { title: 'old' } };
			writePowerLabExt(doc, { title: 'new' });
			expect(doc).toEqual({ 'x-web': { title: 'new' } });
		});

		it('preserves x-casaos when the author used x-casaos', () => {
			const doc: Record<string, unknown> = { 'x-casaos': { title: 'old' } };
			writePowerLabExt(doc, { title: 'new' });
			expect(doc).toEqual({ 'x-casaos': { title: 'new' } });
			expect(doc['x-powerlab']).toBeUndefined();
		});

		it('returns the same doc reference for chaining', () => {
			const doc: Record<string, unknown> = {};
			const result = writePowerLabExt(doc, { a: 1 });
			expect(result).toBe(doc);
		});
	});

	describe('deletePowerLabExtProperty', () => {
		it('removes a single property from the active key', () => {
			const doc = { 'x-powerlab': { title: 'A', deprecated: true } };
			deletePowerLabExtProperty(doc, 'deprecated');
			expect(doc['x-powerlab']).toEqual({ title: 'A' });
		});

		it('is a no-op when no extension is present', () => {
			const doc: Record<string, unknown> = { services: {} };
			expect(() => deletePowerLabExtProperty(doc, 'foo')).not.toThrow();
			expect(doc).toEqual({ services: {} });
		});

		it('targets x-casaos when only x-casaos is present', () => {
			const doc = { 'x-casaos': { title: 'A', drop: true } };
			deletePowerLabExtProperty(doc, 'drop');
			expect(doc['x-casaos']).toEqual({ title: 'A' });
		});
	});
});
