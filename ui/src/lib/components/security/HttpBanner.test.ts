import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import HttpBanner from './HttpBanner.svelte';

// goto is the SvelteKit navigation primitive; intercept it so the
// banner-click test doesn't try to actually navigate the test DOM.
const gotoSpy = vi.fn();
vi.mock('$app/navigation', () => ({
	goto: (url: string) => gotoSpy(url)
}));

describe('HttpBanner', () => {
	beforeEach(() => {
		gotoSpy.mockClear();
		// Reset sessionStorage between cases so dismissal in one test
		// doesn't leak into the next.
		try { sessionStorage.clear(); } catch { /* JSDOM may not have it */ }
		vi.stubGlobal('location', {
			protocol: 'http:',
			href: 'http://localhost:8765/'
		});
	});

	it('renders the soft HTTP pill when protocol is http:', () => {
		render(HttpBanner);
		// Soft design: small "HTTP" pill, not a full-width banner.
		expect(screen.getByText('HTTP')).toBeTruthy();
	});

	it('does not render when protocol is https:', () => {
		vi.stubGlobal('location', {
			protocol: 'https:',
			href: 'https://localhost:8443/'
		});
		render(HttpBanner);
		expect(screen.queryByText('HTTP')).toBeNull();
	});

	it('clicking the pill body navigates to the security walkthrough', async () => {
		render(HttpBanner);
		// Find the pill body button (the one with "HTTP" text). The
		// dismiss button has aria-label "Dismiss" so it's distinguishable.
		const pillButton = screen.getByTitle(/click to enable HTTPS/i);
		await fireEvent.click(pillButton);
		expect(gotoSpy).toHaveBeenCalledWith('/settings#security');
	});

	it('clicking dismiss persists the dismissal to sessionStorage', async () => {
		render(HttpBanner);
		const dismiss = screen.getByLabelText('Dismiss');
		await fireEvent.click(dismiss);
		// We don't assert immediate DOM removal because Svelte's
		// fly-out transition keeps the element mounted for ~250ms.
		// Persistence to sessionStorage is the load-bearing
		// behavior — see the next test.
		expect(sessionStorage.getItem('powerlab_http_banner_dismissed')).toBe('1');
	});

	it('respects a prior session dismissal', () => {
		sessionStorage.setItem('powerlab_http_banner_dismissed', '1');
		render(HttpBanner);
		expect(screen.queryByText('HTTP')).toBeNull();
	});
});
