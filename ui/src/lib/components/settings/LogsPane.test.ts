import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/svelte';
import LogsPane from './LogsPane.svelte';

(globalThis as unknown as { __APP_VERSION__: string }).__APP_VERSION__ = 'test';

// Mock the logs API module so the component exercises its
// rendering logic without hitting the network.
vi.mock('$lib/api/logs', () => ({
	listLogFiles: vi.fn(),
	readLogFile: vi.fn()
}));

import { listLogFiles, readLogFile } from '$lib/api/logs';

function entry(overrides: Record<string, unknown> = {}) {
	return {
		name: 'app-management.log',
		size_bytes: 12345,
		modified_ts: new Date().toISOString(),
		modified_us: Date.now() * 1000,
		...overrides
	};
}

describe('LogsPane', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it('renders the file list on mount', async () => {
		vi.mocked(listLogFiles).mockResolvedValueOnce([
			entry({ name: 'app-management.log', size_bytes: 1024 * 1024 }),
			entry({ name: 'gateway.log', size_bytes: 64 * 1024 })
		]);

		render(LogsPane);

		await waitFor(() => expect(listLogFiles).toHaveBeenCalled());
		expect(await screen.findByText('app-management.log')).toBeTruthy();
		expect(await screen.findByText('gateway.log')).toBeTruthy();
	});

	it('shows the empty state when the backend returns no files', async () => {
		vi.mocked(listLogFiles).mockResolvedValueOnce([]);

		render(LogsPane);

		await waitFor(() =>
			expect(screen.getByText(/no .log files found/i)).toBeTruthy()
		);
	});

	it('surfaces non-401 errors as a banner', async () => {
		vi.mocked(listLogFiles).mockRejectedValueOnce({ status: 500, message: 'boom' });

		render(LogsPane);

		await waitFor(() =>
			expect(screen.getByText(/Could not load logs/i)).toBeTruthy()
		);
	});

	it('suppresses 401 from the banner (auth store handles it centrally)', async () => {
		vi.mocked(listLogFiles).mockRejectedValueOnce({ status: 401, message: 'unauthorized' });

		render(LogsPane);

		// Wait for the listLogFiles call to settle.
		await waitFor(() => expect(listLogFiles).toHaveBeenCalled());
		expect(screen.queryByText(/Could not load logs/i)).toBeNull();
	});

	it('loads file content when a file is clicked', async () => {
		vi.mocked(listLogFiles).mockResolvedValueOnce([
			entry({ name: 'gateway.log' })
		]);
		vi.mocked(readLogFile).mockResolvedValueOnce(
			'2026-05-15T18:00:00Z INFO gateway started\n'
		);

		render(LogsPane);

		const fileButton = await screen.findByTestId('logs-file-gateway.log');
		await fireEvent.click(fileButton);

		await waitFor(() => expect(readLogFile).toHaveBeenCalledWith('gateway.log'));
		const pre = await screen.findByTestId('logs-content-pre');
		expect(pre.textContent).toContain('gateway started');
	});

	it('renders the download button once content is loaded', async () => {
		vi.mocked(listLogFiles).mockResolvedValueOnce([entry({ name: 'gateway.log' })]);
		vi.mocked(readLogFile).mockResolvedValueOnce('content here');

		render(LogsPane);

		// Download is hidden before a file is selected.
		expect(screen.queryByTestId('logs-download')).toBeNull();

		const fileButton = await screen.findByTestId('logs-file-gateway.log');
		await fireEvent.click(fileButton);

		await waitFor(() => expect(screen.getByTestId('logs-download')).toBeTruthy());
	});

	it('shows the placeholder until a file is selected', async () => {
		vi.mocked(listLogFiles).mockResolvedValueOnce([entry()]);

		render(LogsPane);

		await waitFor(() => expect(screen.getByText(/Pick a file from the left/i)).toBeTruthy());
	});

	it('renders newest-first ordering as the backend returned them (no client re-sort)', async () => {
		// Backend already sorts by mtime DESC. The component should not
		// re-sort — verify by inspecting DOM order.
		vi.mocked(listLogFiles).mockResolvedValueOnce([
			entry({ name: 'newer.log', modified_us: 2000 }),
			entry({ name: 'older.log', modified_us: 1000 })
		]);

		const { container } = render(LogsPane);

		await waitFor(() => expect(screen.getByText('newer.log')).toBeTruthy());
		// Match ONLY the per-file buttons (data-testid="logs-file-<name>");
		// the wrapping list container has data-testid="logs-file-list" — exclude it.
		const buttons = Array.from(
			container.querySelectorAll('[data-testid^="logs-file-"]')
		).filter((el) => el.getAttribute('data-testid') !== 'logs-file-list');
		expect(buttons.length).toBe(2);
		expect(buttons[0].getAttribute('data-testid')).toBe('logs-file-newer.log');
		expect(buttons[1].getAttribute('data-testid')).toBe('logs-file-older.log');
	});
});
