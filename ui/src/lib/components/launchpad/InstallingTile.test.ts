import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import InstallingTile from './InstallingTile.svelte';
import type { InstallStateEntry } from '$lib/stores/install-state.svelte';
import type { ComposeAppStoreInfo } from '$lib/api/apps';

const baseEntry: InstallStateEntry = {
	id: 'enclosed',
	storeInfo: {
		store_app_id: 'enclosed',
		title: { en_us: 'Enclosed' },
		icon: 'https://example.com/icon.svg',
		port_map: '8788',
	} as unknown as ComposeAppStoreInfo,
	phase: 'installing',
	currentPhase: null,
	progress: 0,
	logs: '',
	startedAt: Date.now(),
};

describe('InstallingTile', () => {
	it('renders with installing label when phase is in flight', () => {
		render(InstallingTile, { props: { entry: baseEntry } });
		expect(screen.queryByTestId('installing-tile')).toBeTruthy();
		expect(screen.queryByTestId('installing-tile-label')?.textContent?.trim()).toBe('installing');
	});

	it('applies the indeterminate spin class when currentPhase is null', () => {
		render(InstallingTile, { props: { entry: baseEntry } });
		const ring = screen.queryByTestId('installing-tile-ring');
		expect(ring?.classList.contains('installing-tile-ring-spin')).toBe(true);
	});

	it('removes the spin class once currentPhase arrives (transitions to determinate)', () => {
		render(InstallingTile, {
			props: {
				entry: {
					...baseEntry,
					phase: 'starting',
					currentPhase: { step: 2, total: 5, label: 'Pulling' },
					progress: 0.4,
				},
			},
		});
		const ring = screen.queryByTestId('installing-tile-ring');
		expect(ring?.classList.contains('installing-tile-ring-spin')).toBe(false);
	});

	it("arc dasharray reflects progress at 40%", () => {
		render(InstallingTile, {
			props: {
				entry: { ...baseEntry, phase: 'starting', currentPhase: { step: 2, total: 5 }, progress: 0.4 },
			},
		});
		const arc = screen.queryByTestId('installing-tile-progress-arc');
		// circumference = 2π × 26 ≈ 163.36 → 40% ≈ 65.34, gap ≈ 98.02
		const dasharray = arc?.getAttribute('stroke-dasharray') ?? '';
		const [filled, gap] = dasharray.split(' ').map(Number);
		expect(filled).toBeGreaterThan(64);
		expect(filled).toBeLessThan(67);
		expect(filled + gap).toBeGreaterThan(162);
		expect(filled + gap).toBeLessThan(165);
	});

	it('shows success badge when phase=success', () => {
		render(InstallingTile, {
			props: { entry: { ...baseEntry, phase: 'success', progress: 1 } },
		});
		expect(screen.queryByTestId('installing-tile-badge-success')).toBeTruthy();
		expect(screen.queryByTestId('installing-tile-badge-error')).toBeNull();
		expect(screen.queryByTestId('installing-tile-label')?.textContent?.trim()).toBe('installed');
	});

	it('shows error badge when phase=error', () => {
		render(InstallingTile, {
			props: { entry: { ...baseEntry, phase: 'error', error: 'boom' } },
		});
		expect(screen.queryByTestId('installing-tile-badge-error')).toBeTruthy();
		expect(screen.queryByTestId('installing-tile-badge-success')).toBeNull();
		expect(screen.queryByTestId('installing-tile-label')?.textContent?.trim()).toBe('error');
	});

	it('shows error badge when phase=timeout (timeout = error class)', () => {
		render(InstallingTile, {
			props: { entry: { ...baseEntry, phase: 'timeout' } },
		});
		expect(screen.queryByTestId('installing-tile-badge-error')).toBeTruthy();
		expect(screen.queryByTestId('installing-tile-label')?.textContent?.trim()).toBe('timeout');
	});

	it('falls back to id when title is missing', () => {
		render(InstallingTile, {
			props: {
				entry: {
					...baseEntry,
					storeInfo: {} as ComposeAppStoreInfo,
				},
			},
		});
		const tile = screen.queryByTestId('installing-tile');
		expect(tile?.textContent).toContain('enclosed');
	});

	it('fires onclick when the tile button is clicked', async () => {
		const onclick = vi.fn();
		render(InstallingTile, { props: { entry: baseEntry, onclick } });
		const btn = screen.getByRole('button');
		await fireEvent.click(btn);
		expect(onclick).toHaveBeenCalledOnce();
	});
});
