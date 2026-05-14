/**
 * InstallModal — shared install lifecycle modal (Sprint 14 #345).
 *
 * Replaces the two divergent modals in:
 *   - routes/apps/+page.svelte (Community Install)
 *   - routes/apps/new/+page.svelte (Custom App)
 *
 * Both pages now render <InstallModal> with their local state. The
 * component owns the phase rendering, button placement, and ghost-
 * tile-friendly minimize behavior. Eliminates the divergence
 * surfaced in v0.6.7/v0.6.8 (different visual modals, different
 * lifecycle handling for Custom App).
 *
 * The lifecycle CONTRACT is one of five phases:
 *   - installing  : pre-SSE, indeterminate Preparing
 *   - starting    : SSE open, may or may not have currentPhase
 *   - success     : terminal happy path
 *   - error       : terminal error
 *   - timeout     : SSE wedged > 10 min
 *
 * Plus minimized=true hides the modal but the install continues
 * (ghost tile from #330 takes over).
 */

import { render, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import InstallModal from './InstallModal.svelte';

const baseProps = {
	phase: 'installing' as const,
	currentPhase: null,
	progress: 0,
	logs: '',
	appTitle: 'Test App',
	appIcon: '',
	error: null as string | null,
	portNote: null as string | null,
	minimized: false,
	onMinimize: vi.fn(),
	onCancel: vi.fn(),
	onRetry: vi.fn(),
	onOpen: vi.fn(),
	onStay: vi.fn(),
	onCheckLaunchpad: vi.fn()
};

describe('InstallModal', () => {
	it('renders nothing when minimized=true', () => {
		const { container } = render(InstallModal, {
			props: { ...baseProps, minimized: true }
		});
		expect(container.querySelector('[data-testid="install-modal"]')).toBeNull();
	});

	it('renders Preparing indeterminate state when phase=installing and no currentPhase', () => {
		const { container } = render(InstallModal, {
			props: { ...baseProps, phase: 'installing', currentPhase: null }
		});
		expect(container.querySelector('[data-testid="install-modal"]')).toBeTruthy();
		// The indeterminate fill testid comes from InstallProgressBar (#329).
		expect(
			container.querySelector('[data-testid="install-progress-fill-indeterminate"]')
		).toBeTruthy();
	});

	it('renders determinate progress when currentPhase is set', () => {
		const { container } = render(InstallModal, {
			props: {
				...baseProps,
				phase: 'starting',
				currentPhase: { step: 2, total: 3, label: 'Creating containers' },
				progress: 0.66
			}
		});
		expect(
			container.querySelector('[data-testid="install-progress-fill-determinate"]')
		).toBeTruthy();
	});

	it('renders success card when phase=success', () => {
		const { container } = render(InstallModal, {
			props: { ...baseProps, phase: 'success' }
		});
		const text = container.textContent || '';
		// Should expose "Open" and "Stay" actions OR a success indicator.
		expect(text.toLowerCase()).toMatch(/running|ativo|executando|open|launchpad/);
	});

	it('renders error card with message when phase=error', () => {
		const { container } = render(InstallModal, {
			props: { ...baseProps, phase: 'error', error: 'port 8000 in use' }
		});
		const text = container.textContent || '';
		expect(text).toContain('port 8000 in use');
	});

	it('renders timeout card when phase=timeout', () => {
		const { container } = render(InstallModal, {
			props: { ...baseProps, phase: 'timeout' }
		});
		const text = container.textContent || '';
		expect(text.toLowerCase()).toMatch(/longer|launchpad|background|tempo/);
	});

	it('renders LogStreamer with non-empty logs prop', () => {
		const { container } = render(InstallModal, {
			props: {
				...baseProps,
				phase: 'starting',
				logs: 'Phase 1/3: Pulling images...\nPhase 2/3: Creating containers...'
			}
		});
		expect(container.querySelector('[data-testid="log-streamer"]')).toBeTruthy();
	});

	it('calls onMinimize when minimize button clicked (during installing)', async () => {
		const onMinimize = vi.fn();
		const { container } = render(InstallModal, {
			props: { ...baseProps, phase: 'starting', onMinimize }
		});
		const btn = container.querySelector('[data-testid="install-modal-minimize"]');
		expect(btn).toBeTruthy();
		await fireEvent.click(btn!);
		expect(onMinimize).toHaveBeenCalledTimes(1);
	});

	it('calls onCancel when Cancel button clicked (during installing)', async () => {
		const onCancel = vi.fn();
		const { container } = render(InstallModal, {
			props: { ...baseProps, phase: 'installing', onCancel }
		});
		const btn = container.querySelector('[data-testid="install-modal-cancel"]');
		expect(btn).toBeTruthy();
		await fireEvent.click(btn!);
		expect(onCancel).toHaveBeenCalledTimes(1);
	});

	it('calls onOpen when Open Launchpad button clicked (after success)', async () => {
		const onOpen = vi.fn();
		const { container } = render(InstallModal, {
			props: { ...baseProps, phase: 'success', onOpen }
		});
		const btn = container.querySelector('[data-testid="install-modal-open"]');
		expect(btn).toBeTruthy();
		await fireEvent.click(btn!);
		expect(onOpen).toHaveBeenCalledTimes(1);
	});

	it('calls onRetry when Retry button clicked (after error)', async () => {
		const onRetry = vi.fn();
		const { container } = render(InstallModal, {
			props: { ...baseProps, phase: 'error', error: 'boom', onRetry }
		});
		const btn = container.querySelector('[data-testid="install-modal-retry"]');
		expect(btn).toBeTruthy();
		await fireEvent.click(btn!);
		expect(onRetry).toHaveBeenCalledTimes(1);
	});

	it('shows portNote on success when provided', () => {
		const { container } = render(InstallModal, {
			props: {
				...baseProps,
				phase: 'success',
				portNote: 'Running on port 8001 — port 8000 was already in use.'
			}
		});
		expect(container.textContent).toContain('8001');
	});

	it('does NOT show minimize button on terminal phases (success/error/timeout)', () => {
		for (const phase of ['success', 'error', 'timeout'] as const) {
			const { container } = render(InstallModal, {
				props: { ...baseProps, phase, error: phase === 'error' ? 'x' : null }
			});
			expect(container.querySelector('[data-testid="install-modal-minimize"]')).toBeNull();
		}
	});
});
