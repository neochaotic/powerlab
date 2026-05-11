import { render, screen } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import MiniProgress from './MiniProgress.svelte';

describe('MiniProgress', () => {
	it('renders label and sublabel', () => {
		render(MiniProgress, { props: { value: 50, label: 'CPU', sublabel: '4 cores' } });
		expect(screen.getByText('CPU')).toBeTruthy();
		expect(screen.getByText('4 cores')).toBeTruthy();
	});

	it('applies critical color when status is critical', () => {
		const { container } = render(MiniProgress, {
			props: { value: 95, label: 'CPU', sublabel: 'high', status: 'critical' }
		});
		expect(container.querySelector('.bg-red-500')).toBeTruthy();
	});

	it('applies warning color when status is warning', () => {
		const { container } = render(MiniProgress, {
			props: { value: 80, label: 'CPU', sublabel: 'med', status: 'warning' }
		});
		expect(container.querySelector('.bg-yellow-500')).toBeTruthy();
	});

	it('uses colorClass when status is normal (default)', () => {
		const { container } = render(MiniProgress, {
			props: { value: 30, label: 'CPU', sublabel: 'low', colorClass: 'bg-blue-500' }
		});
		expect(container.querySelector('.bg-blue-500')).toBeTruthy();
	});

	it('renders width based on value', () => {
		const { container } = render(MiniProgress, {
			props: { value: 42, label: 'CPU', sublabel: 'x' }
		});
		const bar = container.querySelector('[style*="width: 42%"]');
		expect(bar).toBeTruthy();
	});

	it('does not render an icon when none is supplied', () => {
		const { container } = render(MiniProgress, {
			props: { value: 10, label: 'CPU', sublabel: 'x' }
		});
		// No icon container — just the label span
		expect(container.querySelectorAll('svg').length).toBe(0);
	});
});
