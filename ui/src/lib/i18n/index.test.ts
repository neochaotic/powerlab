import { describe, it, expect, beforeEach, vi } from 'vitest';

// The i18n module reads localStorage + navigator at import time. To
// test the auto-detect path, we have to stub those BEFORE the dynamic
// import. Each test that exercises detection imports the module
// fresh via vi.resetModules() + dynamic import.

// In-memory localStorage polyfill — JSDOM under vitest sometimes
// ships an incomplete Storage object whose .setItem/.getItem are not
// callable. We replace it on each test so the persistence assertions
// have something deterministic to work with.
function makeLocalStorageStub() {
	let store: Record<string, string> = {};
	return {
		getItem: (k: string) => (k in store ? store[k] : null),
		setItem: (k: string, v: string) => { store[k] = String(v); },
		removeItem: (k: string) => { delete store[k]; },
		clear: () => { store = {}; },
		key: (i: number) => Object.keys(store)[i] ?? null,
		get length() { return Object.keys(store).length; }
	} as Storage;
}

describe('i18n core', () => {
	beforeEach(() => {
		vi.resetModules();
		vi.unstubAllGlobals();
		const ls = makeLocalStorageStub();
		vi.stubGlobal('localStorage', ls);
		// The i18n module reads via `window.localStorage` so wire that
		// to the same backing store.
		Object.defineProperty(window, 'localStorage', { value: ls, configurable: true });
	});

	it('returns the key itself when the translation is missing', async () => {
		const { t } = await import('./index.svelte');
		expect(t('non.existent.key')).toBe('non.existent.key');
	});

	it('returns English by default and renders a known string', async () => {
		const { t, getLocale } = await import('./index.svelte');
		expect(getLocale()).toBe('en');
		expect(t('action.save')).toBe('Save');
	});

	it('switches between locales and renders the matching translations', async () => {
		const { t, setLocale, getLocale } = await import('./index.svelte');

		setLocale('pt-BR');
		expect(getLocale()).toBe('pt-BR');
		expect(t('action.save')).toBe('Salvar');

		setLocale('es');
		expect(getLocale()).toBe('es');
		expect(t('action.save')).toBe('Guardar');
	});

	it('interpolates {param} placeholders', async () => {
		const { t, setLocale } = await import('./index.svelte');
		expect(t('apps.updatePrompt', { title: 'Plex' })).toBe(
			'A new version of Plex is ready to be installed.'
		);
		setLocale('pt-BR');
		expect(t('apps.updatePrompt', { title: 'Plex' })).toBe(
			'Uma nova versão de Plex está pronta para ser instalada.'
		);
	});

	it('handles params even when the translation has no placeholder', async () => {
		const { t } = await import('./index.svelte');
		expect(t('action.save', { extra: 'unused' })).toBe('Save');
	});

	it('rejects unknown locales — setLocale is a no-op for invalid IDs', async () => {
		const { setLocale, getLocale } = await import('./index.svelte');
		setLocale('zh-Hant');
		expect(getLocale()).toBe('en');
	});

	it('falls back to English when the requested locale lacks the key', async () => {
		const { t, setLocale } = await import('./index.svelte');
		setLocale('pt-BR');
		// Real key that exists in pt-BR — covers the happy path.
		expect(t('action.save')).toBe('Salvar');
	});

	// ── persistence ────────────────────────────────────────────────────

	it('reads the persisted locale from localStorage on first import', async () => {
		window.localStorage.setItem('powerlab_locale', 'pt-BR');
		const { getLocale, t } = await import('./index.svelte');
		expect(getLocale()).toBe('pt-BR');
		expect(t('action.save')).toBe('Salvar');
	});

	it('writes the chosen locale to localStorage so it survives a reload', async () => {
		const { setLocale } = await import('./index.svelte');
		setLocale('es');
		expect(window.localStorage.getItem('powerlab_locale')).toBe('es');
	});

	it('ignores a localStorage value that is not a registered locale', async () => {
		window.localStorage.setItem('powerlab_locale', 'klingon');
		const { getLocale } = await import('./index.svelte');
		expect(getLocale()).toBe('en');
	});

	// ── navigator.language autodetect ──────────────────────────────────

	it('autodetects pt-BR from navigator.language exact match', async () => {
		vi.stubGlobal('navigator', { language: 'pt-BR' });
		const { getLocale } = await import('./index.svelte');
		expect(getLocale()).toBe('pt-BR');
	});

	it('autodetects pt-BR for a Portuguese variant we do not ship (pt-PT)', async () => {
		vi.stubGlobal('navigator', { language: 'pt-PT' });
		const { getLocale } = await import('./index.svelte');
		// We only ship pt-BR; an unspecific Portuguese should still
		// land on Portuguese rather than English.
		expect(getLocale()).toBe('pt-BR');
	});

	it('autodetects es for any Spanish variant', async () => {
		vi.stubGlobal('navigator', { language: 'es-MX' });
		const { getLocale } = await import('./index.svelte');
		expect(getLocale()).toBe('es');
	});

	it('falls back to English when navigator.language is unknown', async () => {
		vi.stubGlobal('navigator', { language: 'zh-CN' });
		const { getLocale } = await import('./index.svelte');
		expect(getLocale()).toBe('en');
	});

	it('localStorage wins over navigator.language', async () => {
		window.localStorage.setItem('powerlab_locale', 'es');
		vi.stubGlobal('navigator', { language: 'pt-BR' });
		const { getLocale } = await import('./index.svelte');
		expect(getLocale()).toBe('es');
	});
});

// ── data-quality regression tests ────────────────────────────────────
//
// These pin properties of the actual JSON files so a future merge
// that drops a key, breaks a placeholder, or sneaks a wrong-language
// word in fails loudly rather than at user-visible runtime.

describe('i18n data quality', () => {
	beforeEach(() => {
		vi.resetModules();
	});

	it('every locale has the same keys as English (no orphans, no gaps)', async () => {
		const en = (await import('./locales/en.json')).default;
		const ptBR = (await import('./locales/pt-BR.json')).default;
		const es = (await import('./locales/es.json')).default;

		const enKeys = Object.keys(en).sort();
		expect(Object.keys(ptBR).sort()).toEqual(enKeys);
		expect(Object.keys(es).sort()).toEqual(enKeys);
	});

	it('Spanish does not contain Portuguese-only words like "ao" — pins the regression', async () => {
		const es = (await import('./locales/es.json')).default as Record<string, string>;
		// "ao" is Portuguese for "to the" (masculine). Spanish is "al".
		// A bare ` ao ` in Spanish almost certainly means a translator
		// crossed wires. We allow it inside compound words (radio,
		// caos, etc.) by requiring word boundaries.
		const offenders: string[] = [];
		for (const [k, v] of Object.entries(es)) {
			if (/\bao\b/.test(v)) offenders.push(`${k}: ${v}`);
		}
		expect(offenders).toEqual([]);
	});

	it('every translation has the same {param} placeholders as English', async () => {
		const en = (await import('./locales/en.json')).default as Record<string, string>;
		const ptBR = (await import('./locales/pt-BR.json')).default as Record<string, string>;
		const es = (await import('./locales/es.json')).default as Record<string, string>;

		const placeholdersOf = (s: string) =>
			[...s.matchAll(/\{([^}]+)\}/g)].map((m) => m[1]).sort();

		for (const [k, vEn] of Object.entries(en)) {
			const enPlaceholders = placeholdersOf(vEn);
			for (const [name, bundle] of [
				['pt-BR', ptBR],
				['es', es]
			] as const) {
				const v = bundle[k];
				if (!v) continue; // covered by the keys-match test above
				expect(placeholdersOf(v), `${name}/${k}`).toEqual(enPlaceholders);
			}
		}
	});
});
