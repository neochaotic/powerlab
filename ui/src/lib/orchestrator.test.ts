import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import Page from '../routes/apps/new/+page.svelte';

// Custom-app page now uses a one-way form (read derived view from
// YAML, edits emit new YAML via onChange). The 3-tab view switcher
// (Split / Form / YAML) is back; YAML stays the sole source of
// truth. Tests below assert the page shell + tabs + editor render.

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

	it('renders the YAML editor and the form panel in split mode', () => {
		const { container } = render(Page);

		expect(container.querySelector('textarea[data-testid="yaml-editor"]')).toBeTruthy();
		expect(container.querySelector('#service-name')).toBeTruthy();
		expect(screen.getByText(/Network Settings/i)).toBeTruthy();
		expect(screen.getByText(/Storage & Devices/i)).toBeTruthy();
		expect(screen.getByText(/Execution & Resources/i)).toBeTruthy();
	});

	it('displays the view switcher with Split, Form, and YAML options', () => {
		render(Page);
		expect(screen.getByText('Split')).toBeTruthy();
		expect(screen.getByText('Form')).toBeTruthy();
		expect(screen.getByText('YAML')).toBeTruthy();
	});

	it('shows the Deploy button', () => {
		render(Page);
		expect(screen.getByTestId('deploy-button')).toBeTruthy();
	});
});
