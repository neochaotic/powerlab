import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import UninstallAppModal from './UninstallAppModal.svelte';

let onCancel: () => void;
let onConfirm: () => void;
let onDeleteDataChange: (value: boolean) => void;

beforeEach(() => {
	onCancel = vi.fn();
	onConfirm = vi.fn();
	onDeleteDataChange = vi.fn();
});

describe('UninstallAppModal', () => {
	it('renders nothing when open=false', () => {
		const { container } = render(UninstallAppModal, {
			props: { open: false, deleteData: false, onDeleteDataChange, onCancel, onConfirm }
		});
		expect(container.querySelector('.fixed')).toBeNull();
	});

	it('renders Cancel + Uninstall when open=true', () => {
		render(UninstallAppModal, {
			props: { open: true, deleteData: false, onDeleteDataChange, onCancel, onConfirm }
		});
		expect(screen.getByText('Cancel')).toBeTruthy();
		expect(screen.getByRole('button', { name: 'Uninstall' })).toBeTruthy();
	});

	it('reflects deleteData prop in the checkbox checked state', () => {
		render(UninstallAppModal, {
			props: { open: true, deleteData: true, onDeleteDataChange, onCancel, onConfirm }
		});
		const checkbox = screen.getByRole('checkbox') as HTMLInputElement;
		expect(checkbox.checked).toBe(true);
	});

	it('calls onDeleteDataChange when the checkbox toggles', async () => {
		render(UninstallAppModal, {
			props: { open: true, deleteData: false, onDeleteDataChange, onCancel, onConfirm }
		});
		await fireEvent.click(screen.getByRole('checkbox'));
		expect(onDeleteDataChange).toHaveBeenCalledWith(true);
	});

	it('calls onConfirm when Uninstall is clicked', async () => {
		render(UninstallAppModal, {
			props: { open: true, deleteData: false, onDeleteDataChange, onCancel, onConfirm }
		});
		await fireEvent.click(screen.getByRole('button', { name: 'Uninstall' }));
		expect(onConfirm).toHaveBeenCalledTimes(1);
	});

	it('calls onCancel when Cancel is clicked', async () => {
		render(UninstallAppModal, {
			props: { open: true, deleteData: false, onDeleteDataChange, onCancel, onConfirm }
		});
		await fireEvent.click(screen.getByText('Cancel'));
		expect(onCancel).toHaveBeenCalledTimes(1);
	});
});
