/**
 * Theme store using Svelte 5 runes.
 * Dark mode is the default. Persists preference to localStorage.
 */

type Theme = 'dark' | 'light';

const STORAGE_KEY = 'powerlab-theme';

function getInitialTheme(): Theme {
	if (typeof window === 'undefined') return 'dark';
	const stored = localStorage.getItem(STORAGE_KEY);
	if (stored === 'light' || stored === 'dark') return stored;
	return 'dark'; // Default to dark per CLAUDE.MD directive
}

let theme = $state<Theme>(getInitialTheme());

export function useTheme() {
	function toggle() {
		theme = theme === 'dark' ? 'light' : 'dark';
		if (typeof window !== 'undefined') {
			localStorage.setItem(STORAGE_KEY, theme);
			document.documentElement.setAttribute('data-theme', theme);
		}
	}

	function set(t: Theme) {
		theme = t;
		if (typeof window !== 'undefined') {
			localStorage.setItem(STORAGE_KEY, theme);
			document.documentElement.setAttribute('data-theme', theme);
		}
	}

	return {
		get current() {
			return theme;
		},
		get isDark() {
			return theme === 'dark';
		},
		toggle,
		set
	};
}
