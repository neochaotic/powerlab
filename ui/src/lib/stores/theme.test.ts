import { describe, it, expect, beforeEach, vi } from 'vitest';

const mockStorage: Record<string, string> = {};
global.localStorage = {
	getItem: vi.fn((key: string) => mockStorage[key] ?? null),
	setItem: vi.fn((key: string, value: string) => {
		mockStorage[key] = value;
	}),
	removeItem: vi.fn((key: string) => {
		delete mockStorage[key];
	}),
	clear: vi.fn(() => {
		for (const k in mockStorage) delete mockStorage[k];
	}),
	length: 0,
	key: (i: number) => Object.keys(mockStorage)[i] ?? null
} as Storage;

beforeEach(() => {
	vi.resetModules();
	localStorage.clear();
	document.documentElement.removeAttribute('data-theme');
});

describe('Theme store', () => {
	it('defaults to dark when localStorage is empty', async () => {
		const { useTheme } = await import('./theme.svelte');
		expect(useTheme().current).toBe('dark');
		expect(useTheme().isDark).toBe(true);
	});

	it('reads "light" from localStorage as initial', async () => {
		localStorage.setItem('powerlab-theme', 'light');
		const { useTheme } = await import('./theme.svelte');
		expect(useTheme().current).toBe('light');
		expect(useTheme().isDark).toBe(false);
	});

	it('reads "dark" from localStorage as initial', async () => {
		localStorage.setItem('powerlab-theme', 'dark');
		const { useTheme } = await import('./theme.svelte');
		expect(useTheme().current).toBe('dark');
	});

	it('ignores invalid stored values, falls back to dark', async () => {
		localStorage.setItem('powerlab-theme', 'neon-pink');
		const { useTheme } = await import('./theme.svelte');
		expect(useTheme().current).toBe('dark');
	});

	it('toggle flips dark → light → dark and persists each step', async () => {
		const { useTheme } = await import('./theme.svelte');
		const theme = useTheme();
		theme.toggle();
		expect(theme.current).toBe('light');
		expect(localStorage.getItem('powerlab-theme')).toBe('light');
		expect(document.documentElement.getAttribute('data-theme')).toBe('light');

		theme.toggle();
		expect(theme.current).toBe('dark');
		expect(localStorage.getItem('powerlab-theme')).toBe('dark');
		expect(document.documentElement.getAttribute('data-theme')).toBe('dark');
	});

	it('set("light") overrides the current theme + persists', async () => {
		const { useTheme } = await import('./theme.svelte');
		const theme = useTheme();
		theme.set('light');
		expect(theme.current).toBe('light');
		expect(localStorage.getItem('powerlab-theme')).toBe('light');
		expect(document.documentElement.getAttribute('data-theme')).toBe('light');
	});

	it('multiple useTheme() consumers share state', async () => {
		const { useTheme } = await import('./theme.svelte');
		const a = useTheme();
		const b = useTheme();
		a.set('light');
		expect(b.current).toBe('light');
	});
});
