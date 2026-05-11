import { render, screen } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import RadialGauge from './RadialGauge.svelte';

describe('RadialGauge', () => {
	it('renders the rounded integer value as %', () => {
		render(RadialGauge, { props: { value: 47.6, label: 'CPU' } });
		expect(screen.getByText('48%')).toBeTruthy();
	});

	it('renders the label inside the SVG', () => {
		render(RadialGauge, { props: { value: 30, label: 'RAM' } });
		expect(screen.getByText('RAM')).toBeTruthy();
	});

	it('renders the sublabel when supplied', () => {
		render(RadialGauge, { props: { value: 30, label: 'RAM', sublabel: '8 / 16 GB' } });
		expect(screen.getByText('8 / 16 GB')).toBeTruthy();
	});

	it('does NOT render the sublabel when omitted', () => {
		const { container } = render(RadialGauge, { props: { value: 30, label: 'RAM' } });
		expect(container.querySelectorAll('p').length).toBe(0);
	});

	it('clamps value below 0 to 0 (no negative fill)', () => {
		const { container } = render(RadialGauge, { props: { value: -10, label: 'X' } });
		// fillLength becomes 0 → the "fill" circle is skipped (fillLength > 0.5 branch)
		const circles = container.querySelectorAll('circle');
		expect(circles.length).toBe(1); // only the track
	});

	it('clamps value above 100 to 100 (no overflow)', () => {
		const { container } = render(RadialGauge, { props: { value: 150, label: 'X' } });
		// Should render: the fill should be present
		expect(container.querySelectorAll('circle').length).toBeGreaterThanOrEqual(2);
	});

	it('honors custom size prop', () => {
		const { container } = render(RadialGauge, { props: { value: 50, label: 'X', size: 200 } });
		expect(container.querySelector('div[style*="width: 200px"]')).toBeTruthy();
	});
});
