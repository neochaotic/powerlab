import { render, screen } from '@testing-library/svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import HttpBanner from './HttpBanner.svelte';

describe('HttpBanner', () => {
	beforeEach(() => {
		vi.stubGlobal('location', {
			protocol: 'http:',
			href: 'http://localhost:8765/'
		});
	});

	it('renders when protocol is http:', () => {
		render(HttpBanner);
		expect(screen.getByText(/Connection unencrypted/i)).toBeTruthy();
	});

	it('does not render when protocol is https:', () => {
		vi.stubGlobal('location', {
			protocol: 'https:',
			href: 'https://localhost:8443/'
		});
		render(HttpBanner);
		expect(screen.queryByText(/Connection unencrypted/i)).toBeNull();
	});
});
