import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import LogStreamer from './LogStreamer.svelte';

// Locks the LogStreamer contract — used in both Community Install
// modal and Custom App build page. Pure presentational: the parent
// owns the SSE EventSource and the `logs` string; this component
// renders + auto-scrolls.

describe('LogStreamer', () => {
	it('renders the supplied logs in a pre element', () => {
		render(LogStreamer, { props: { logs: '[a1b2c3] Pulling image\n[a1b2c3] Extracting\n' } });
		const pre = screen.getByTestId('log-streamer-pre');
		expect(pre.textContent).toContain('Pulling image');
		expect(pre.textContent).toContain('Extracting');
	});

	it('renders empty logs as an empty pre (parent owns the show/hide decision)', () => {
		render(LogStreamer, { props: { logs: '' } });
		const pre = screen.getByTestId('log-streamer-pre');
		expect(pre.textContent).toBe('');
	});

	it('shows custom label when provided', () => {
		render(LogStreamer, { props: { logs: 'x', label: 'Build output' } });
		expect(screen.getByText('Build output')).toBeTruthy();
	});

	it('does NOT show paused tag at first render (autoscroll enabled by default)', () => {
		render(LogStreamer, { props: { logs: 'x\n' } });
		expect(screen.queryByTestId('log-streamer-paused')).toBeNull();
	});

	it('shows paused tag when the user has scrolled away from the bottom', async () => {
		render(LogStreamer, { props: { logs: 'x\n'.repeat(50) } });
		const pre = screen.getByTestId('log-streamer-pre') as HTMLPreElement;

		// Simulate the user scrolling up: scrollTop low, scrollHeight high,
		// clientHeight some — atBottom evaluates false.
		Object.defineProperty(pre, 'scrollTop', { configurable: true, value: 0 });
		Object.defineProperty(pre, 'scrollHeight', { configurable: true, value: 1000 });
		Object.defineProperty(pre, 'clientHeight', { configurable: true, value: 200 });

		await fireEvent.scroll(pre);
		expect(screen.queryByTestId('log-streamer-paused')).not.toBeNull();
	});

	it('resumes autoscroll when the user scrolls back to bottom', async () => {
		render(LogStreamer, { props: { logs: 'x\n'.repeat(50) } });
		const pre = screen.getByTestId('log-streamer-pre') as HTMLPreElement;

		// Pause first
		Object.defineProperty(pre, 'scrollTop', { configurable: true, value: 0 });
		Object.defineProperty(pre, 'scrollHeight', { configurable: true, value: 1000 });
		Object.defineProperty(pre, 'clientHeight', { configurable: true, value: 200 });
		await fireEvent.scroll(pre);
		expect(screen.queryByTestId('log-streamer-paused')).not.toBeNull();

		// Scroll back to bottom — scrollTop + clientHeight covers scrollHeight
		Object.defineProperty(pre, 'scrollTop', { configurable: true, value: 800 });
		await fireEvent.scroll(pre);
		expect(screen.queryByTestId('log-streamer-paused')).toBeNull();
	});

	it('honors the heightClass prop for parent layout control', () => {
		const { container } = render(LogStreamer, { props: { logs: 'x', heightClass: 'h-96' } });
		const pre = container.querySelector('[data-testid="log-streamer-pre"]') as HTMLPreElement;
		expect(pre.className).toContain('h-96');
	});
});
