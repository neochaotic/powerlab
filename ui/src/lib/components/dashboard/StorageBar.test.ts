/**
 * Regression tests for StorageBar.
 *
 * Antigravity bug: `healthColor` was `$derived(() => fn)` — the $derived held
 * the arrow function itself, not the string result. The fix is `$derived.by()`.
 *
 * These tests catch the case where `healthColor` is mistakenly used as a function
 * object for the CSS class: the class string would be corrupted ("() => {...}")
 * instead of the expected Tailwind class, failing the assertions below.
 */
import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/svelte';
import StorageBar from './StorageBar.svelte';

describe('StorageBar', () => {
	it('renders with Healthy badge by default', () => {
		const { getByText } = render(StorageBar, { used: 200, total: 1000 });
		expect(getByText('Healthy')).toBeTruthy();
	});

	it('Critical health applies danger color class to badge', () => {
		const { getByText } = render(StorageBar, { used: 950, total: 1000, health: 'Critical' });
		const badge = getByText('Critical');
		// If healthColor were a function object (broken $derived anti-pattern),
		// the class would be "[object Object]" or similar, NOT the expected token.
		expect(badge.className).toContain('bg-[var(--color-danger)]');
	});

	it('Warning health applies warning color class to badge', () => {
		const { getByText } = render(StorageBar, { used: 800, total: 1000, health: 'Warning' });
		const badge = getByText('Warning');
		expect(badge.className).toContain('bg-[var(--color-warning)]');
	});

	it('Healthy status applies accent color class to badge', () => {
		const { getByText } = render(StorageBar, { used: 100, total: 1000, health: 'Healthy' });
		const badge = getByText('Healthy');
		expect(badge.className).toContain('bg-[var(--color-accent)]');
	});

	it('usage bar width reflects the used/total percentage', () => {
		const { container } = render(StorageBar, { used: 250, total: 1000 });
		// The inner progress bar sets width via inline style
		const bar = container.querySelector('[style*="width"]') as HTMLElement;
		expect(bar?.style.width).toBe('25%');
	});

	it('formats bytes correctly in labels', () => {
		const { getByText } = render(StorageBar, { used: 1073741824, total: 10737418240 }); // 1 GB / 10 GB
		expect(getByText(/Used: 1\.0 GB/)).toBeTruthy();
		expect(getByText(/Total: 10\.0 GB/)).toBeTruthy();
	});

	it('handles zero total without dividing by zero', () => {
		expect(() => render(StorageBar, { used: 0, total: 0 })).not.toThrow();
	});
});
