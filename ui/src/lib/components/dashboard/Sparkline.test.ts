import { render } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import Sparkline from './Sparkline.svelte';

describe('Sparkline', () => {
	it('renders an empty path when external values has < 2 points', () => {
		const { container } = render(Sparkline, {
			props: { values: [5] }
		});
		const path = container.querySelector('path');
		expect(path?.getAttribute('d')).toBe('');
	});

	it('renders an empty path when external values is empty', () => {
		const { container } = render(Sparkline, {
			props: { values: [] }
		});
		expect(container.querySelector('path')?.getAttribute('d')).toBe('');
	});

	it('renders a non-empty path when external values has ≥ 2 points', () => {
		const { container } = render(Sparkline, {
			props: { values: [10, 20, 30, 40] }
		});
		const d = container.querySelector('path')?.getAttribute('d') ?? '';
		expect(d).toMatch(/^M [\d.]+ [\d.]+( L [\d.]+ [\d.]+)+$/);
	});

	it('honors custom width and height', () => {
		const { container } = render(Sparkline, {
			props: { values: [1, 2, 3], width: 200, height: 60 }
		});
		const svg = container.querySelector('svg');
		expect(svg?.getAttribute('viewBox')).toBe('0 0 200 60');
	});

	it('applies the color prop to the stroke', () => {
		const { container } = render(Sparkline, {
			props: { values: [1, 2], color: '#ff0000' }
		});
		expect(container.querySelector('path')?.getAttribute('stroke')).toBe('#ff0000');
	});

	it('falls back to internal rolling history when only `value` is supplied (no `values`)', () => {
		const { container } = render(Sparkline, { props: { value: 50 } });
		// First render: history has just one entry → path is empty
		expect(container.querySelector('path')?.getAttribute('d')).toBe('');
	});

	it('renders an SVG element when no points are supplied', () => {
		const { container } = render(Sparkline, { props: { values: [] } });
		// Even with no points the SVG shell still renders
		expect(container.querySelector('svg')).toBeTruthy();
	});
});
