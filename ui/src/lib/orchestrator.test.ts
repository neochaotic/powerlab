import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import Page from '../routes/apps/new/+page.svelte';

describe('Orchestrator Page', () => {
	it('should render the custom app builder header and YAML editor', () => {
		const { container } = render(Page);

		expect(screen.getByText(/New Custom App/i)).toBeTruthy();
		expect(screen.getByText(/Custom App Builder/i)).toBeTruthy();

		const textarea = container.querySelector('textarea');
		expect(textarea).toBeTruthy();
		expect(textarea?.value).toContain("version:");
		expect(textarea?.value).toContain("services:");
	});

	it('should have functional Mac controls with aria-labels', () => {
		const { container } = render(Page);
		expect(container.querySelector('[aria-label="Exit to apps"]')).toBeTruthy();
		expect(container.querySelector('[aria-label="Toggle split view"]')).toBeTruthy();
		expect(container.querySelector('[aria-label="Switch to form view"]')).toBeTruthy();
	});

	/**
	 * Regression: orchestrator page was wrapped in root layout's max-w-7xl+padding div,
	 * preventing full-screen rendering. The page must own its own full-height container.
	 */
	it('should have a full-height flex-col root container without width constraints', () => {
		const { container } = render(Page);
		const root = container.firstElementChild as HTMLElement;

		expect(root).toBeTruthy();
		expect(root.className).toContain('h-full');
		expect(root.className).toContain('flex');
		expect(root.className).toContain('flex-col');
		expect(root.className).not.toContain('max-w');
		expect(root.className).not.toContain('px-8');
	});

	it('should render both the YAML editor and the form panel in split mode', () => {
		const { container } = render(Page);

		const textarea = container.querySelector('textarea');
		expect(textarea).toBeTruthy();

		expect(screen.getByText(/Network Settings/i)).toBeTruthy();
		expect(screen.getByText(/Storage & Devices/i)).toBeTruthy();
		expect(screen.getByText(/Execution & Resources/i)).toBeTruthy();
	});

	it('should display the view switcher with Split, Form, and YAML options', () => {
		render(Page);
		expect(screen.getByText('Split')).toBeTruthy();
		expect(screen.getByText('Form')).toBeTruthy();
		expect(screen.getByText('YAML')).toBeTruthy();
	});

	it('should show the Deploy button', () => {
		render(Page);
		expect(screen.getByText('Deploy')).toBeTruthy();
	});
});
