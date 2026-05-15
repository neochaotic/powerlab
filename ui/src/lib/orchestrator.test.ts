import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import Page from '../routes/apps/new/+page.svelte';

// Custom-app page is YAML-first (no bidirectional ComposeForm).
// Tests below assert the page shell + YAML editor + read-only preview
// render. The form-era cases were removed alongside ComposeForm
// itself; YAML round-trip / shape robustness lives in
// YAMLPreview.test.ts.

describe('Orchestrator Page', () => {
	it('renders the custom app builder header and YAML editor', () => {
		const { container } = render(Page);

		expect(screen.getByText(/New Custom App/i)).toBeTruthy();
		expect(screen.getByText(/Custom App Builder/i)).toBeTruthy();

		const textarea = container.querySelector('textarea[data-testid="yaml-editor"]');
		expect(textarea).toBeTruthy();
		expect((textarea as HTMLTextAreaElement).value).toContain('services:');
	});

	it('exposes the Exit to apps Mac control with an aria-label', () => {
		const { container } = render(Page);
		expect(container.querySelector('[aria-label="Exit to apps"]')).toBeTruthy();
	});

	it('owns a full-height flex-col root container with no width constraint', () => {
		const { container } = render(Page);
		const root = container.firstElementChild as HTMLElement;

		expect(root).toBeTruthy();
		expect(root.className).toContain('h-full');
		expect(root.className).toContain('flex');
		expect(root.className).toContain('flex-col');
		expect(root.className).not.toContain('max-w');
	});

	it('renders the YAML editor and the YAMLPreview panel side-by-side', () => {
		const { container } = render(Page);

		expect(container.querySelector('textarea[data-testid="yaml-editor"]')).toBeTruthy();
		expect(container.querySelector('[data-testid="yaml-preview"]')).toBeTruthy();
	});

	it('shows the Deploy button', () => {
		render(Page);
		expect(screen.getByTestId('deploy-button')).toBeTruthy();
	});
});
