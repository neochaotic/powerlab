/**
 * PowerLab i18n — minimal translation system, JSON-backed.
 *
 * Each locale is a flat key/value JSON file under `locales/`. Adding
 * a new locale is one new file plus one entry in the registry below.
 * Translators contribute by editing JSON, never the host TypeScript.
 *
 * Behavior:
 *
 *   - On first load, picks the locale the user explicitly chose
 *     before (persisted to localStorage) → falls back to a sensible
 *     match for `navigator.language` → falls back to English.
 *   - `setLocale(...)` validates against the registry and writes
 *     localStorage so the choice survives a refresh.
 *   - Missing keys return the key itself. This is a deliberate
 *     debug affordance — a screen showing `dashboard.cpu` instead
 *     of "CPU Usage" makes the gap obvious during code review.
 *   - Vite imports the JSON statically at build time so the locales
 *     are part of the bundle (no runtime fetch, no waterfall).
 *
 * Usage:
 *
 *   import { t, setLocale, getLocale, availableLocales } from '$lib/i18n';
 *   t('dashboard.cpu')                          // "CPU Usage"
 *   t('apps.updatePrompt', { title: 'Plex' })   // string interpolation
 *   setLocale('pt-BR')                          // persists, refresh-safe
 */

import en from './locales/en.json';
import ptBR from './locales/pt-BR.json';
import es from './locales/es.json';

export type LocaleID = 'en' | 'pt-BR' | 'es';

type Bundle = Record<string, string>;
type Registry = Record<LocaleID, Bundle>;

const REGISTRY: Registry = {
	en: en as Bundle,
	'pt-BR': ptBR as Bundle,
	es: es as Bundle
};

const LOCALES: LocaleID[] = Object.keys(REGISTRY) as LocaleID[];
const DEFAULT_LOCALE: LocaleID = 'en';
const STORAGE_KEY = 'powerlab_locale';

// Public list for UI components that render a locale picker. The
// label is the language's own name so users see their language in
// its own script (`Português (Brasil)`, not "Portuguese (Brazil)").
export const availableLocales: { id: LocaleID; label: string }[] = [
	{ id: 'en', label: 'English' },
	{ id: 'pt-BR', label: 'Português (Brasil)' },
	{ id: 'es', label: 'Español' }
];

// detectInitialLocale runs once on module load. Order:
//   1. localStorage value (the user's explicit prior choice)
//   2. navigator.language match — exact, then language-only
//   3. DEFAULT_LOCALE
//
// Defensive: any access to window/localStorage/navigator is wrapped
// because this module also imports under SSR / vitest where those
// globals may be absent.
function detectInitialLocale(): LocaleID {
	if (typeof window === 'undefined') return DEFAULT_LOCALE;

	try {
		const saved = window.localStorage?.getItem(STORAGE_KEY);
		if (saved && (LOCALES as string[]).includes(saved)) {
			return saved as LocaleID;
		}
	} catch {
		// localStorage may be unavailable (private mode, sandbox).
		// Fall through to navigator-based detection.
	}

	const nav = typeof navigator !== 'undefined' ? navigator.language : '';
	if (nav) {
		if ((LOCALES as string[]).includes(nav)) return nav as LocaleID;
		// Language-only match: `pt-PT` → `pt-BR`, `en-US` → `en`.
		// We pick the first registered locale whose primary subtag
		// matches the navigator's primary subtag.
		const primary = nav.split('-')[0]?.toLowerCase();
		if (primary) {
			const fuzzy = LOCALES.find((l) => l.toLowerCase().startsWith(primary));
			if (fuzzy) return fuzzy;
		}
	}

	return DEFAULT_LOCALE;
}

let currentLocale: LocaleID = $state(detectInitialLocale());

export function setLocale(locale: string): void {
	if (!(LOCALES as string[]).includes(locale)) return;
	currentLocale = locale as LocaleID;
	try {
		window.localStorage?.setItem(STORAGE_KEY, locale);
	} catch {
		// localStorage may be unavailable; runtime selection still
		// applies for the session.
	}
}

export function getLocale(): LocaleID {
	return currentLocale;
}

export function t(key: string, params?: Record<string, string | number>): string {
	const bundle = REGISTRY[currentLocale] ?? REGISTRY[DEFAULT_LOCALE];
	let text = bundle[key] ?? REGISTRY[DEFAULT_LOCALE][key] ?? key;
	if (params) {
		for (const [k, v] of Object.entries(params)) {
			text = text.replace(`{${k}}`, String(v));
		}
	}
	return text;
}

export type T = typeof t;
