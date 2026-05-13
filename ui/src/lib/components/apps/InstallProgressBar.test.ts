import { render, screen } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import InstallProgressBar from './InstallProgressBar.svelte';

// Locks the v0.6.5 regression where the determinate progress bar
// only appeared once the first `Phase N/M` SSE marker arrived,
// leaving the user with no progress feedback during the HTTP POST
// + early SSE phase. Each case below maps to a user-visible state.

describe('InstallProgressBar', () => {
	it("shows indeterminate bar when phase='installing' with no currentPhase (HTTP POST in flight)", () => {
		render(InstallProgressBar, {
			props: { phase: 'installing', currentPhase: null, progress: 0 },
		});
		expect(screen.queryByTestId('install-progress-bar')).toBeTruthy();
		expect(screen.queryByTestId('install-progress-fill-indeterminate')).toBeTruthy();
		expect(screen.queryByTestId('install-progress-fill-determinate')).toBeNull();
		expect(screen.queryByTestId('install-progress-preparing')).toBeTruthy();
	});

	it("shows indeterminate bar when phase='starting' with no currentPhase (SSE just opened)", () => {
		render(InstallProgressBar, {
			props: { phase: 'starting', currentPhase: null, progress: 0 },
		});
		expect(screen.queryByTestId('install-progress-fill-indeterminate')).toBeTruthy();
		expect(screen.queryByTestId('install-progress-fill-determinate')).toBeNull();
	});

	it("switches to determinate bar when currentPhase arrives mid-install", () => {
		render(InstallProgressBar, {
			props: {
				phase: 'starting',
				currentPhase: { step: 2, total: 5, label: 'Pulling images' },
				progress: 0.4,
			},
		});
		expect(screen.queryByTestId('install-progress-fill-determinate')).toBeTruthy();
		expect(screen.queryByTestId('install-progress-fill-indeterminate')).toBeNull();
		expect(screen.queryByTestId('install-progress-step')?.textContent?.trim()).toBe('2/5');
		expect(screen.queryByTestId('install-progress-percent')?.textContent?.trim()).toBe('40%');
	});

	it("renders nothing when phase='idle'", () => {
		render(InstallProgressBar, {
			props: { phase: 'idle', currentPhase: null, progress: 0 },
		});
		expect(screen.queryByTestId('install-progress-bar')).toBeNull();
	});

	it("renders nothing when phase='success' (success state owned by caller)", () => {
		render(InstallProgressBar, {
			props: { phase: 'success', currentPhase: null, progress: 1 },
		});
		expect(screen.queryByTestId('install-progress-bar')).toBeNull();
	});

	it("stays visible during phase='error' if there was progress (shows where install died)", () => {
		render(InstallProgressBar, {
			props: {
				phase: 'error',
				currentPhase: { step: 3, total: 5, label: 'Image pull failed' },
				progress: 0.6,
			},
		});
		expect(screen.queryByTestId('install-progress-bar')).toBeTruthy();
		expect(screen.queryByTestId('install-progress-fill-determinate')).toBeTruthy();
	});

	it("uses custom preparingLabel when provided", () => {
		render(InstallProgressBar, {
			props: {
				phase: 'installing',
				currentPhase: null,
				progress: 0,
				preparingLabel: 'Custom prep text',
			},
		});
		expect(screen.queryByTestId('install-progress-preparing')?.textContent?.trim()).toBe(
			'Custom prep text',
		);
	});
});
