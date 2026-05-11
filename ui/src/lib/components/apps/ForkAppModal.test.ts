import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import ForkAppModal from './ForkAppModal.svelte';

let onCancel: ReturnType<typeof vi.fn>;
let onConfirm: ReturnType<typeof vi.fn>;

beforeEach(() => {
	onCancel = vi.fn();
	onConfirm = vi.fn();
});

describe('ForkAppModal', () => {
	it('renders nothing when open=false', () => {
		const { container } = render(ForkAppModal, {
			props: { open: false, onCancel, onConfirm }
		});
		expect(container.querySelector('.fixed')).toBeNull();
	});

	it('renders the modal shell when open=true', () => {
		render(ForkAppModal, { props: { open: true, onCancel, onConfirm } });
		expect(screen.getByText('Cancel')).toBeTruthy();
		expect(screen.getByText('Open Editor')).toBeTruthy();
	});

	it('calls onCancel when Cancel is clicked', async () => {
		render(ForkAppModal, { props: { open: true, onCancel, onConfirm } });
		await fireEvent.click(screen.getByText('Cancel'));
		expect(onCancel).toHaveBeenCalledTimes(1);
		expect(onConfirm).not.toHaveBeenCalled();
	});

	it('calls onConfirm when Open Editor is clicked', async () => {
		render(ForkAppModal, { props: { open: true, onCancel, onConfirm } });
		await fireEvent.click(screen.getByText('Open Editor'));
		expect(onConfirm).toHaveBeenCalledTimes(1);
		expect(onCancel).not.toHaveBeenCalled();
	});
});
