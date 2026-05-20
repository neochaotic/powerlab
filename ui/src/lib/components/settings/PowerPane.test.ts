import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/svelte';
import PowerPane from './PowerPane.svelte';

(globalThis as unknown as { __APP_VERSION__: string }).__APP_VERSION__ = 'test';

vi.mock('$lib/api/power', () => ({
	listPowerLabServices: vi.fn(),
	restartPowerLabService: vi.fn(),
	getServicesPreflight: vi.fn(),
	rebootHost: vi.fn(),
	shutdownHost: vi.fn()
}));

import {
	listPowerLabServices,
	restartPowerLabService,
	getServicesPreflight,
	rebootHost,
	shutdownHost
} from '$lib/api/power';

const SERVICES = [
	{ name: 'powerlab-gateway', active_state: 'active', sub_state: 'running', pid: '1001' },
	{ name: 'powerlab-app-management', active_state: 'active', sub_state: 'running', pid: '1002' },
	{ name: 'powerlab-core', active_state: 'inactive', sub_state: 'dead' },
	{ name: 'powerlab-local-storage', active_state: 'failed', sub_state: 'failed' }
];

describe('PowerPane', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		// Avoid the in-component setInterval polling racing with tests.
		vi.useFakeTimers({ shouldAdvanceTime: true });
		// localStorage starts clean — shutdown opt-in defaults to disabled.
		// Some envs mock localStorage without a .clear() — fall back to
		// per-key removal.
		try {
			localStorage.removeItem('powerlab.power.shutdown_enabled');
		} catch {
			// localStorage unavailable — tests will just see default state
		}
	});

	afterEach(() => {
		vi.useRealTimers();
	});

	it('renders all PowerLab services on mount', async () => {
		vi.mocked(listPowerLabServices).mockResolvedValue(SERVICES);
		render(PowerPane);
		await waitFor(() => {
			expect(listPowerLabServices).toHaveBeenCalled();
		});
		await waitFor(() => {
			expect(screen.getByText('powerlab-gateway')).toBeTruthy();
			expect(screen.getByText('powerlab-core')).toBeTruthy();
			expect(screen.getByText('powerlab-local-storage')).toBeTruthy();
		});
	});

	it('shows active/inactive/failed badges per service state', async () => {
		vi.mocked(listPowerLabServices).mockResolvedValue(SERVICES);
		const { container } = render(PowerPane);
		await waitFor(() => screen.getByText('powerlab-gateway'));

		const row = container.querySelector('[data-testid="service-row-powerlab-local-storage"]');
		expect(row?.textContent?.toLowerCase()).toContain('failed');
	});

	it('shows pid for running services, hides for stopped', async () => {
		vi.mocked(listPowerLabServices).mockResolvedValue(SERVICES);
		const { container } = render(PowerPane);
		await waitFor(() => screen.getByText('powerlab-gateway'));

		const gatewayRow = container.querySelector('[data-testid="service-row-powerlab-gateway"]');
		expect(gatewayRow?.textContent).toContain('pid 1001');

		const coreRow = container.querySelector('[data-testid="service-row-powerlab-core"]');
		expect(coreRow?.textContent).not.toContain('pid');
	});

	it('Restart button calls restartPowerLabService with the service name', async () => {
		vi.mocked(listPowerLabServices).mockResolvedValue(SERVICES);
		vi.mocked(restartPowerLabService).mockResolvedValue(undefined);

		const { container } = render(PowerPane);
		await waitFor(() => screen.getByText('powerlab-app-management'));

		const row = container.querySelector('[data-testid="service-row-powerlab-app-management"]');
		const restartBtn = row?.querySelector('[data-testid="service-restart-btn"]') as HTMLButtonElement;
		await fireEvent.click(restartBtn);

		await waitFor(() => {
			expect(restartPowerLabService).toHaveBeenCalledWith('powerlab-app-management');
		});
	});

	it('Reboot button opens modal, not direct backend call', async () => {
		vi.mocked(listPowerLabServices).mockResolvedValue(SERVICES);
		const { container } = render(PowerPane);
		await waitFor(() => screen.getByText('powerlab-gateway'));

		await fireEvent.click(container.querySelector('[data-testid="host-reboot-btn"]')!);
		await waitFor(() => {
			expect(container.querySelector('[data-testid="host-reboot-modal"]')).toBeTruthy();
		});
		// Backend not called yet — modal intercepted
		expect(rebootHost).not.toHaveBeenCalled();
	});

	it('Reboot confirm disabled until ack checked', async () => {
		vi.mocked(listPowerLabServices).mockResolvedValue(SERVICES);
		const { container } = render(PowerPane);
		await waitFor(() => screen.getByText('powerlab-gateway'));

		await fireEvent.click(container.querySelector('[data-testid="host-reboot-btn"]')!);
		await waitFor(() => container.querySelector('[data-testid="host-reboot-modal"]'));

		const confirmBtn = container.querySelector('[data-testid="host-reboot-confirm"]') as HTMLButtonElement;
		expect(confirmBtn.disabled).toBe(true);

		const ack = container.querySelector('[data-testid="host-reboot-ack"]') as HTMLInputElement;
		await fireEvent.click(ack);
		expect(confirmBtn.disabled).toBe(false);
	});

	it('Reboot confirm fires rebootHost after ack', async () => {
		vi.mocked(listPowerLabServices).mockResolvedValue(SERVICES);
		vi.mocked(rebootHost).mockResolvedValue(undefined);

		const { container } = render(PowerPane);
		await waitFor(() => screen.getByText('powerlab-gateway'));
		await fireEvent.click(container.querySelector('[data-testid="host-reboot-btn"]')!);
		await waitFor(() => container.querySelector('[data-testid="host-reboot-modal"]'));
		await fireEvent.click(container.querySelector('[data-testid="host-reboot-ack"]')!);
		await fireEvent.click(container.querySelector('[data-testid="host-reboot-confirm"]')!);

		await waitFor(() => {
			expect(rebootHost).toHaveBeenCalled();
		});
	});

	// Shutdown opt-in tests skipped in vitest: localStorage in the test
	// env is a partial mock without setItem. The opt-in flow is fully
	// covered by Playwright E2E in tests/power-pane.spec.ts (PR #468)
	// where the browser provides a real localStorage. Document the gap
	// here so future maintainers don't re-add broken tests.

	it('Refresh button re-fetches services', async () => {
		vi.mocked(listPowerLabServices).mockResolvedValue(SERVICES);
		const { container } = render(PowerPane);
		await waitFor(() => screen.getByText('powerlab-gateway'));
		expect(listPowerLabServices).toHaveBeenCalledTimes(1);

		await fireEvent.click(container.querySelector('[data-testid="power-refresh"]')!);
		await waitFor(() => {
			expect(listPowerLabServices).toHaveBeenCalledTimes(2);
		});
	});

	it('Gateway Restart opens warning modal instead of calling API directly', async () => {
		vi.mocked(listPowerLabServices).mockResolvedValue(SERVICES);
		vi.mocked(getServicesPreflight).mockResolvedValue([]);

		const { container } = render(PowerPane);
		await waitFor(() => screen.getByText('powerlab-gateway'));

		const gatewayRow = container.querySelector('[data-testid="service-row-powerlab-gateway"]');
		const restartBtn = gatewayRow?.querySelector('[data-testid="service-restart-btn"]') as HTMLButtonElement;
		await fireEvent.click(restartBtn);

		await waitFor(() => {
			expect(container.querySelector('[data-testid="gateway-restart-modal"]')).toBeTruthy();
		});
		expect(restartPowerLabService).not.toHaveBeenCalled();
	});

	it('Gateway restart modal cancel does not call restart API', async () => {
		vi.mocked(listPowerLabServices).mockResolvedValue(SERVICES);
		vi.mocked(getServicesPreflight).mockResolvedValue([]);

		const { container } = render(PowerPane);
		await waitFor(() => screen.getByText('powerlab-gateway'));

		const gatewayRow = container.querySelector('[data-testid="service-row-powerlab-gateway"]');
		await fireEvent.click(gatewayRow?.querySelector('[data-testid="service-restart-btn"]')!);
		await waitFor(() => container.querySelector('[data-testid="gateway-restart-modal"]'));

		await fireEvent.click(container.querySelector('[data-testid="gateway-restart-cancel"]')!);

		await waitFor(() => {
			expect(container.querySelector('[data-testid="gateway-restart-modal"]')).toBeNull();
		});
		expect(restartPowerLabService).not.toHaveBeenCalled();
	});

	it('Gateway restart modal confirm calls restartPowerLabService for gateway', async () => {
		vi.mocked(listPowerLabServices).mockResolvedValue(SERVICES);
		vi.mocked(getServicesPreflight).mockResolvedValue([]);
		vi.mocked(restartPowerLabService).mockResolvedValue(undefined);

		const { container } = render(PowerPane);
		await waitFor(() => screen.getByText('powerlab-gateway'));

		const gatewayRow = container.querySelector('[data-testid="service-row-powerlab-gateway"]');
		await fireEvent.click(gatewayRow?.querySelector('[data-testid="service-restart-btn"]')!);
		await waitFor(() => container.querySelector('[data-testid="gateway-restart-modal"]'));

		await fireEvent.click(container.querySelector('[data-testid="gateway-restart-confirm"]')!);

		await waitFor(() => {
			expect(restartPowerLabService).toHaveBeenCalledWith('powerlab-gateway');
		});
	});
});
