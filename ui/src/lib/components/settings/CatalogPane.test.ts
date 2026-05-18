import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/svelte';
import CatalogPane from './CatalogPane.svelte';

(globalThis as unknown as { __APP_VERSION__: string }).__APP_VERSION__ = 'test';

// Mock the catalog API module so the pane's onMount load() is
// deterministic without hitting a real backend.
vi.mock('$lib/api/catalog', () => ({
	listCatalogSources: vi.fn(),
	addCatalogSource: vi.fn(),
	removeCatalogSource: vi.fn()
}));

import {
	listCatalogSources,
	addCatalogSource,
	removeCatalogSource
} from '$lib/api/catalog';

const DEFAULT_SOURCE = {
	id: 0,
	url: '/var/lib/powerlab/community-catalog',
	store_root: '/var/lib/powerlab/community-catalog'
};

const OPERATOR_SOURCE = {
	id: 1,
	url: 'https://github.com/operator/custom-catalog',
	store_root: '/var/lib/powerlab/appstore/github.com/abc123'
};

describe('CatalogPane', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it('renders the default PowerLab Curated source with Curated badge', async () => {
		vi.mocked(listCatalogSources).mockResolvedValueOnce([DEFAULT_SOURCE]);
		const { container } = render(CatalogPane);
		await waitFor(() => {
			expect(listCatalogSources).toHaveBeenCalled();
		});
		// Wait for the default-source row to render with the curated badge.
		await waitFor(() => {
			const row = container.querySelector('[data-testid="catalog-source-0"]');
			expect(row).toBeTruthy();
			expect(row?.textContent).toContain('Default');
			expect(row?.textContent).toContain('Curated');
		});
	});

	it('renders operator-added sources with Unaudited badge', async () => {
		vi.mocked(listCatalogSources).mockResolvedValueOnce([DEFAULT_SOURCE, OPERATOR_SOURCE]);
		const { container } = render(CatalogPane);
		await waitFor(() => {
			const row = container.querySelector('[data-testid="catalog-source-1"]');
			expect(row).toBeTruthy();
			expect(row?.textContent).toContain('Unaudited');
		});
	});

	it('hides Remove button on the default source (id=0)', async () => {
		vi.mocked(listCatalogSources).mockResolvedValueOnce([DEFAULT_SOURCE]);
		const { container } = render(CatalogPane);
		await waitFor(() => {
			expect(screen.getByText(/PowerLab Curated/)).toBeTruthy();
		});
		// The default-source row must NOT contain a remove button
		const removeBtn = container.querySelector('[data-testid="catalog-source-0-remove"]');
		expect(removeBtn).toBeNull();
	});

	it('shows Remove button on operator-added sources', async () => {
		vi.mocked(listCatalogSources).mockResolvedValueOnce([DEFAULT_SOURCE, OPERATOR_SOURCE]);
		const { container } = render(CatalogPane);
		await waitFor(() => {
			expect(container.querySelector('[data-testid="catalog-source-1-remove"]')).toBeTruthy();
		});
	});

	it('shows operator-added count in header copy when ≥1 custom source', async () => {
		vi.mocked(listCatalogSources).mockResolvedValueOnce([DEFAULT_SOURCE, OPERATOR_SOURCE]);
		const { container } = render(CatalogPane);
		await waitFor(() => {
			// Look for "operator-added source" wording in header text
			const header = container.querySelector('header');
			expect(header?.textContent).toMatch(/operator-added source/i);
		});
	});

	it('opens the add-source modal when "Add custom catalog source" is clicked', async () => {
		vi.mocked(listCatalogSources).mockResolvedValueOnce([DEFAULT_SOURCE]);
		const { container } = render(CatalogPane);
		await waitFor(() => {
			expect(screen.getByText(/PowerLab Curated/)).toBeTruthy();
		});
		const addBtn = container.querySelector('[data-testid="catalog-add"]');
		expect(addBtn).toBeTruthy();
		await fireEvent.click(addBtn!);
		await waitFor(() => {
			expect(container.querySelector('[data-testid="catalog-add-modal"]')).toBeTruthy();
		});
	});

	it('Add-source confirm button is disabled until acknowledgement checked', async () => {
		vi.mocked(listCatalogSources).mockResolvedValueOnce([DEFAULT_SOURCE]);
		const { container } = render(CatalogPane);
		await waitFor(() => expect(container.querySelector('[data-testid="catalog-source-0"]')).toBeTruthy());
		await fireEvent.click(container.querySelector('[data-testid="catalog-add"]')!);
		await waitFor(() => container.querySelector('[data-testid="catalog-add-modal"]'));

		const urlInput = container.querySelector('[data-testid="catalog-add-url"]') as HTMLInputElement;
		const confirmBtn = container.querySelector('[data-testid="catalog-add-confirm"]') as HTMLButtonElement;
		const ackCheckbox = container.querySelector('[data-testid="catalog-add-ack"]') as HTMLInputElement;

		// Confirm disabled by default
		expect(confirmBtn.disabled).toBe(true);

		// Fill URL alone — still disabled (needs ack)
		await fireEvent.input(urlInput, { target: { value: 'https://example.com/repo' } });
		expect(confirmBtn.disabled).toBe(true);

		// Tick ack — enabled
		await fireEvent.click(ackCheckbox);
		expect(confirmBtn.disabled).toBe(false);
	});

	it('confirmAdd calls addCatalogSource with the typed URL', async () => {
		vi.mocked(listCatalogSources).mockResolvedValue([DEFAULT_SOURCE]);
		vi.mocked(addCatalogSource).mockResolvedValueOnce(undefined);

		const { container } = render(CatalogPane);
		await waitFor(() => expect(container.querySelector('[data-testid="catalog-source-0"]')).toBeTruthy());
		await fireEvent.click(container.querySelector('[data-testid="catalog-add"]')!);

		const urlInput = container.querySelector('[data-testid="catalog-add-url"]') as HTMLInputElement;
		const ackCheckbox = container.querySelector('[data-testid="catalog-add-ack"]') as HTMLInputElement;

		await fireEvent.input(urlInput, { target: { value: 'https://github.com/o/r' } });
		await fireEvent.click(ackCheckbox);
		await fireEvent.click(container.querySelector('[data-testid="catalog-add-confirm"]')!);

		await waitFor(() => {
			expect(addCatalogSource).toHaveBeenCalledWith('https://github.com/o/r');
		});
	});

	it('Refresh button re-fetches catalog sources', async () => {
		vi.mocked(listCatalogSources).mockResolvedValue([DEFAULT_SOURCE]);
		const { container } = render(CatalogPane);
		await waitFor(() => expect(container.querySelector('[data-testid="catalog-source-0"]')).toBeTruthy());
		expect(listCatalogSources).toHaveBeenCalledTimes(1);

		const refreshBtn = container.querySelector('[data-testid="catalog-refresh"]');
		await fireEvent.click(refreshBtn!);

		await waitFor(() => {
			expect(listCatalogSources).toHaveBeenCalledTimes(2);
		});
	});
});
