/**
 * uuid regression tests — these lock in the behavior that fixes
 * the v0.5.2 → v0.5.3 toast crash:
 *
 *   "Uncaught (in promise) TypeError: crypto.randomUUID is not a function"
 *
 * triggered on every toast.add() call when the page is loaded over
 * an insecure context (http://IP:port, the new default after #130
 * disabled HTTPS). crypto.randomUUID requires a secure context;
 * crypto.getRandomValues does NOT, so we fall back to that and
 * build a v4-shaped UUID by hand.
 */

import { describe, it, expect, vi, afterEach } from 'vitest';
import { generateID } from './uuid';

const UUID_V4_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;

afterEach(() => {
	vi.unstubAllGlobals();
});

describe('generateID — primary path (crypto.randomUUID)', () => {
	it('uses crypto.randomUUID when it is available', () => {
		const stub = vi.fn(() => '11111111-2222-4333-8444-555555555555');
		vi.stubGlobal('crypto', { randomUUID: stub, getRandomValues: () => {} });

		const id = generateID();

		expect(stub).toHaveBeenCalledOnce();
		expect(id).toBe('11111111-2222-4333-8444-555555555555');
	});

	it('returns a v4-shaped UUID', () => {
		// Use the actual jsdom crypto without stubbing — we just
		// want to confirm the shape.
		const id = generateID();
		expect(id).toMatch(UUID_V4_RE);
	});
});

describe('generateID — secure-context fallback (crypto.getRandomValues)', () => {
	it('falls back when crypto.randomUUID is unavailable (non-secure context)', () => {
		const getRandomValues = vi.fn((bytes: Uint8Array) => {
			for (let i = 0; i < bytes.length; i++) bytes[i] = i + 1;
			return bytes;
		});
		// Simulate non-secure-context Chrome: getRandomValues exists,
		// randomUUID does not.
		vi.stubGlobal('crypto', { getRandomValues });

		const id = generateID();

		expect(getRandomValues).toHaveBeenCalledOnce();
		expect(id).toMatch(UUID_V4_RE);
	});

	it('falls back when crypto.randomUUID throws', () => {
		const getRandomValues = vi.fn((bytes: Uint8Array) => {
			for (let i = 0; i < bytes.length; i++) bytes[i] = 0xab;
			return bytes;
		});
		vi.stubGlobal('crypto', {
			randomUUID: () => {
				throw new Error('not allowed in this context');
			},
			getRandomValues
		});

		const id = generateID();
		expect(getRandomValues).toHaveBeenCalledOnce();
		expect(id).toMatch(UUID_V4_RE);
	});
});

describe('generateID — last-resort fallback (Math.random)', () => {
	it('falls back to Math.random when crypto is entirely absent', () => {
		// Note: typeof undefined === "undefined", so vi.stubGlobal
		// to undefined doesn't quite simulate "no crypto at all".
		// We stub to a bare object (no methods) to drive both upper
		// branches into the 3rd fallback.
		vi.stubGlobal('crypto', {});

		const id = generateID();
		expect(id).toMatch(UUID_V4_RE);
	});
});

describe('generateID — uniqueness', () => {
	it('produces different IDs across many calls', () => {
		const seen = new Set<string>();
		for (let i = 0; i < 100; i++) seen.add(generateID());
		expect(seen.size).toBe(100);
	});
});
