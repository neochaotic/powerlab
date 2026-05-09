/**
 * generateID returns a unique-enough identifier for client-only use
 * (toast IDs, transient list keys, etc.). NOT cryptographically
 * secure — do NOT use for tokens, session IDs, or anything that
 * needs unpredictability under adversarial conditions.
 *
 * Why we don't always use crypto.randomUUID(): it is only available
 * in **secure contexts** (HTTPS, localhost, or file://). On a
 * PowerLab home install accessed via http://192.168.x.y:8765 (the
 * v0.5.2+ default after HTTPS gating), the secure-context check
 * fails and `crypto.randomUUID is not a function`. That broke the
 * toast store on every fresh v0.5.2 install before this fallback
 * was added — every toast.add() threw, no toasts visible.
 *
 * Ordered fallbacks:
 *
 *   1. crypto.randomUUID() — preferred, RFC 4122 v4 UUID, available
 *      in any modern browser when the page is loaded over a secure
 *      context.
 *   2. crypto.getRandomValues() — Web Crypto API; available in
 *      every modern browser INCLUDING non-secure contexts. Build a
 *      v4-shaped UUID by hand from 16 random bytes.
 *   3. Math.random() composition — last-resort fallback. Not random
 *      enough for security but fine for unique-enough-in-this-tab
 *      use. Only fires when running under jsdom that doesn't ship
 *      Web Crypto either.
 */
export function generateID(): string {
	// 1. crypto.randomUUID — secure context only
	if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
		try {
			return crypto.randomUUID();
		} catch {
			// fall through to 2 — defensive against future breakage
		}
	}

	// 2. crypto.getRandomValues — Web Crypto, no secure-context requirement
	if (typeof crypto !== 'undefined' && typeof crypto.getRandomValues === 'function') {
		const bytes = new Uint8Array(16);
		crypto.getRandomValues(bytes);
		// RFC 4122 v4: bits 4-7 of byte 6 are the version (4),
		// bits 6-7 of byte 8 are the variant (10).
		bytes[6] = (bytes[6] & 0x0f) | 0x40;
		bytes[8] = (bytes[8] & 0x3f) | 0x80;
		const hex = Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('');
		return (
			hex.slice(0, 8) +
			'-' +
			hex.slice(8, 12) +
			'-' +
			hex.slice(12, 16) +
			'-' +
			hex.slice(16, 20) +
			'-' +
			hex.slice(20, 32)
		);
	}

	// 3. Math.random — true last-resort. Only hit if neither
	// crypto.randomUUID nor crypto.getRandomValues exist (jsdom
	// versions before 22 had this gap).
	const rnd = (n: number) =>
		Array.from({ length: n }, () => Math.floor(Math.random() * 16).toString(16)).join('');
	return `${rnd(8)}-${rnd(4)}-4${rnd(3)}-${'89ab'[Math.floor(Math.random() * 4)]}${rnd(3)}-${rnd(12)}`;
}
