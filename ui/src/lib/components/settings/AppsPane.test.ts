import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import AppsPane from './AppsPane.svelte';

describe('AppsPane', () => {
	let onCopy: ReturnType<typeof vi.fn>;

	beforeEach(() => {
		onCopy = vi.fn();
	});

	it('renders the configured storage path', () => {
		render(AppsPane, {
			props: { storagePath: '/data/powerlab/apps', copiedKey: null, onCopy }
		});
		expect(screen.getByText('/data/powerlab/apps')).toBeTruthy();
	});

	it('calls onCopy(path, "storage") when the copy button is clicked', async () => {
		render(AppsPane, {
			props: { storagePath: '/data/powerlab/apps', copiedKey: null, onCopy }
		});
		const copyBtn = screen.getByLabelText('Copy path');
		await fireEvent.click(copyBtn);
		expect(onCopy).toHaveBeenCalledWith('/data/powerlab/apps', 'storage');
	});

	it('renders the local store and community catalog source rows', () => {
		render(AppsPane, {
			props: { storagePath: '/data/powerlab/apps', copiedKey: null, onCopy }
		});
		expect(screen.getByText('Local store')).toBeTruthy();
		expect(screen.getByText('Big-Bear catalog')).toBeTruthy();
	});

	it('shows the checkmark instead of the copy icon when copiedKey matches', () => {
		const { container } = render(AppsPane, {
			props: { storagePath: '/data/powerlab/apps', copiedKey: 'storage', onCopy }
		});
		// Lucide icons render as <svg> — checking the lucide-check class is the
		// most reliable assertion without coupling to internal class names.
		expect(container.querySelector('.text-emerald-400')).toBeTruthy();
	});
});
