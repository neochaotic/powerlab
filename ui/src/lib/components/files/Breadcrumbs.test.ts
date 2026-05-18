import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/svelte';
import Breadcrumbs from './Breadcrumbs.svelte';

(globalThis as unknown as { __APP_VERSION__: string }).__APP_VERSION__ = 'test';

describe('Breadcrumbs', () => {
	it('renders root segment when path is /', () => {
		const onNavigate = vi.fn();
		const { container } = render(Breadcrumbs, { props: { path: '/', onNavigate } });
		// Root nav has the home button — always present
		const buttons = container.querySelectorAll('button');
		expect(buttons.length).toBeGreaterThan(0);
	});

	it('renders one segment per path component', () => {
		const onNavigate = vi.fn();
		render(Breadcrumbs, { props: { path: '/foo/bar/baz', onNavigate } });
		expect(screen.getByText('foo')).toBeTruthy();
		expect(screen.getByText('bar')).toBeTruthy();
		expect(screen.getByText('baz')).toBeTruthy();
	});

	it('clicking root navigates to /', async () => {
		const onNavigate = vi.fn();
		const { container } = render(Breadcrumbs, { props: { path: '/foo/bar', onNavigate } });
		// First button in the nav is root
		const rootBtn = container.querySelector('button');
		await fireEvent.click(rootBtn!);
		expect(onNavigate).toHaveBeenCalledWith('/');
	});

	it('clicking a path segment navigates to the cumulative path', async () => {
		const onNavigate = vi.fn();
		render(Breadcrumbs, { props: { path: '/a/b/c', onNavigate } });
		await fireEvent.click(screen.getByText('b'));
		expect(onNavigate).toHaveBeenCalledWith('/a/b');
	});

	it('ignores empty segments (path with trailing/leading slashes)', () => {
		const onNavigate = vi.fn();
		render(Breadcrumbs, { props: { path: '//foo//', onNavigate } });
		expect(screen.getByText('foo')).toBeTruthy();
	});

	it('applies optional class to the nav element', () => {
		const onNavigate = vi.fn();
		const { container } = render(Breadcrumbs, {
			props: { path: '/x', onNavigate, class: 'mycustom' }
		});
		const nav = container.querySelector('nav');
		expect(nav?.className).toContain('mycustom');
	});
});
