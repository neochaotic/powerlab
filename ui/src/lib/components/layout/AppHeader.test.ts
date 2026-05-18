import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import AppHeader from './AppHeader.svelte';

(globalThis as unknown as { __APP_VERSION__: string }).__APP_VERSION__ = 'test';

describe('AppHeader', () => {
	it('renders the title', () => {
		render(AppHeader, { props: { title: 'My Page' } });
		expect(screen.getByText('My Page')).toBeTruthy();
	});

	it('renders the subtitle when provided', () => {
		render(AppHeader, { props: { title: 'Hello', subtitle: 'world' } });
		expect(screen.getByText('world')).toBeTruthy();
	});

	it('does not render subtitle element when subtitle prop omitted', () => {
		const { container } = render(AppHeader, { props: { title: 'Solo' } });
		// The subtitle <p> is gated by {#if subtitle} — confirm absent
		const paragraphs = container.querySelectorAll('p');
		expect(paragraphs.length).toBe(0);
	});

	it('renders the Back-to-Launchpad link by default (showBack defaults true)', () => {
		const { container } = render(AppHeader, { props: { title: 'X' } });
		const back = container.querySelector('a[href="/"]');
		expect(back).toBeTruthy();
		expect(back?.getAttribute('aria-label')).toBe('Back to Launchpad');
	});

	it('hides the Back link when showBack=false', () => {
		const { container } = render(AppHeader, { props: { title: 'X', showBack: false } });
		const back = container.querySelector('a[href="/"]');
		expect(back).toBeNull();
	});

	it('applies the optional className to the header element', () => {
		const { container } = render(AppHeader, { props: { title: 'X', class: 'custom-class' } });
		const header = container.querySelector('header');
		expect(header?.className).toContain('custom-class');
	});
});
