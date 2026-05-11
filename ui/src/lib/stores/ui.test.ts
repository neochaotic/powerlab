import { describe, it, expect, beforeEach, vi } from 'vitest';

vi.mock('$app/navigation', () => ({
	goto: vi.fn(),
	invalidate: vi.fn(),
	invalidateAll: vi.fn(),
	preloadData: vi.fn(),
	preloadCode: vi.fn(),
	beforeNavigate: vi.fn(),
	afterNavigate: vi.fn(),
	onNavigate: vi.fn(),
	pushState: vi.fn(),
	replaceState: vi.fn()
}));

import { goto } from '$app/navigation';

beforeEach(() => {
	vi.resetModules();
	vi.mocked(goto).mockReset();
});

function setLocation(pathname: string) {
	Object.defineProperty(window, 'location', {
		value: { pathname },
		writable: true,
		configurable: true
	});
}

describe('UI store', () => {
	it('isTerminalOpen starts false', async () => {
		const { ui } = await import('./ui.svelte');
		expect(ui.isTerminalOpen).toBe(false);
	});

	it('openTerminal flips isTerminalOpen to true', async () => {
		const { ui } = await import('./ui.svelte');
		ui.openTerminal();
		expect(ui.isTerminalOpen).toBe(true);
	});

	it('isTerminalOpen setter accepts boolean and reads back', async () => {
		const { ui } = await import('./ui.svelte');
		ui.isTerminalOpen = true;
		expect(ui.isTerminalOpen).toBe(true);
		ui.isTerminalOpen = false;
		expect(ui.isTerminalOpen).toBe(false);
	});

	it('searchTriggered starts at 0', async () => {
		setLocation('/apps');
		const { ui } = await import('./ui.svelte');
		expect(ui.searchTriggered).toBe(0);
	});

	it('openSearch increments searchTriggered counter', async () => {
		setLocation('/apps');
		const { ui } = await import('./ui.svelte');
		const before = ui.searchTriggered;
		ui.openSearch();
		expect(ui.searchTriggered).toBe(before + 1);
		ui.openSearch();
		expect(ui.searchTriggered).toBe(before + 2);
	});

	it('openSearch navigates to /apps when not already there', async () => {
		setLocation('/dashboard');
		const { ui } = await import('./ui.svelte');
		ui.openSearch();
		expect(goto).toHaveBeenCalledWith('/apps');
	});

	it('openSearch does NOT navigate when already on /apps', async () => {
		setLocation('/apps');
		const { ui } = await import('./ui.svelte');
		ui.openSearch();
		expect(goto).not.toHaveBeenCalled();
	});
});
