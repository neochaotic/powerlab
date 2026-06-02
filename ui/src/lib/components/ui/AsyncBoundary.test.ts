import { render, screen } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import AsyncBoundary from './AsyncBoundary.svelte';

describe('AsyncBoundary precedence', () => {
	// Precedence is the locked contract: error > loading > empty > children.
	// Renderers must NEVER show two of these simultaneously — the operator
	// reading the screen (sighted or assistive) sees exactly one state.

	it('renders error when error is set, regardless of other flags', () => {
		render(AsyncBoundary, {
			props: { error: 'something broke', loading: true, empty: true }
		});
		expect(screen.queryByTestId('async-boundary-error')).toBeTruthy();
		expect(screen.queryByTestId('async-boundary-loading')).toBeNull();
		expect(screen.queryByTestId('async-boundary-empty')).toBeNull();
	});

	it('renders loading when error is null and loading is true', () => {
		render(AsyncBoundary, { props: { loading: true, empty: true } });
		expect(screen.queryByTestId('async-boundary-loading')).toBeTruthy();
		expect(screen.queryByTestId('async-boundary-empty')).toBeNull();
	});

	it('renders empty when error is null, loading is false, empty is true', () => {
		render(AsyncBoundary, { props: { empty: true } });
		expect(screen.queryByTestId('async-boundary-empty')).toBeTruthy();
	});

	it('renders nothing visible when no state flag is set and no children', () => {
		// Important: when none of the states apply, the component renders
		// NOTHING (not an empty placeholder). The caller controls the
		// layout; AsyncBoundary doesn't reserve space.
		const { container } = render(AsyncBoundary, { props: {} });
		expect(container.querySelector('[data-testid^="async-boundary"]')).toBeNull();
	});
});

describe('AsyncBoundary copy + a11y', () => {
	it('uses default loading copy when none provided', () => {
		render(AsyncBoundary, { props: { loading: true } });
		expect(screen.getByTestId('async-boundary-loading').textContent).toContain('Loading…');
	});

	it('uses default empty copy when none provided', () => {
		render(AsyncBoundary, { props: { empty: true } });
		expect(screen.getByTestId('async-boundary-empty').textContent).toContain('Nothing here yet.');
	});

	it('honours custom loadingText', () => {
		render(AsyncBoundary, {
			props: { loading: true, loadingText: 'Fetching catalog sources…' }
		});
		expect(screen.getByTestId('async-boundary-loading').textContent).toContain(
			'Fetching catalog sources…'
		);
	});

	it('honours custom emptyText', () => {
		render(AsyncBoundary, {
			props: { empty: true, emptyText: 'No sources registered.' }
		});
		expect(screen.getByTestId('async-boundary-empty').textContent).toContain(
			'No sources registered.'
		);
	});

	it('marks the error state with role=alert so screen readers announce it', () => {
		render(AsyncBoundary, { props: { error: 'oops' } });
		const err = screen.getByTestId('async-boundary-error');
		expect(err.getAttribute('role')).toBe('alert');
	});

	it('marks the loading state with aria-live=polite + aria-busy=true', () => {
		render(AsyncBoundary, { props: { loading: true } });
		const loading = screen.getByTestId('async-boundary-loading');
		expect(loading.getAttribute('aria-live')).toBe('polite');
		expect(loading.getAttribute('aria-busy')).toBe('true');
	});
});

describe('AsyncBoundary variant', () => {
	// variant="card" (default) gets the rounded-2xl border chrome that
	// matches CatalogPane / PowerPane content cards. variant="inline"
	// drops the border for use inside table cards (AuditPane / LogsPane)
	// where the parent already provides the chrome.

	it('default (variant unset) applies card chrome with rounded-2xl border', () => {
		render(AsyncBoundary, { props: { loading: true } });
		const el = screen.getByTestId('async-boundary-loading');
		expect(el.className).toContain('rounded-2xl');
		expect(el.className).toContain('border-white/[0.06]');
	});

	it('variant=inline applies inline chrome without border', () => {
		render(AsyncBoundary, { props: { loading: true, variant: 'inline' } });
		const el = screen.getByTestId('async-boundary-loading');
		expect(el.className).not.toContain('rounded-2xl');
		expect(el.className).not.toContain('border-white');
		expect(el.className).toContain('px-4');
		expect(el.className).toContain('py-8');
	});

	it('variant=inline applies the same chrome to the empty state', () => {
		render(AsyncBoundary, { props: { empty: true, variant: 'inline' } });
		const el = screen.getByTestId('async-boundary-empty');
		expect(el.className).not.toContain('rounded-2xl');
		expect(el.className).toContain('px-4');
	});

	it('error state ALWAYS uses banner chrome regardless of variant', () => {
		// Error is always rendered as a top-level banner — variant=inline
		// must NOT downgrade the error's visibility chrome.
		render(AsyncBoundary, { props: { error: 'boom', variant: 'inline' } });
		const el = screen.getByTestId('async-boundary-error');
		expect(el.className).toContain('rounded-2xl');
		expect(el.className).toContain('border-red-500');
	});
});
