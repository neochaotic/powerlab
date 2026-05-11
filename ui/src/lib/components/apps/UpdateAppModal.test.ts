import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import UpdateAppModal from './UpdateAppModal.svelte';

let onCancel: () => void;
let onConfirm: () => void;

const sampleApp = {
	id: 'nginx-proxy-manager',
	store_info: {
		title: 'Nginx Proxy Manager',
		image: { en_us: 'jc21/nginx-proxy-manager:latest' },
		thumbnail: 'https://example.com/icon.png'
	}
};

beforeEach(() => {
	onCancel = vi.fn();
	onConfirm = vi.fn();
});

describe('UpdateAppModal', () => {
	it('renders nothing when app is null', () => {
		const { container } = render(UpdateAppModal, {
			props: { app: null, formattedSize: '120 MB', title: 'NPM', onCancel, onConfirm }
		});
		expect(container.querySelector('.fixed')).toBeNull();
	});

	it('renders the modal when app is supplied', () => {
		render(UpdateAppModal, {
			props: { app: sampleApp, formattedSize: '120 MB', title: 'NPM', onCancel, onConfirm }
		});
		expect(screen.getByText('Cancel')).toBeTruthy();
	});

	it('shows the image reference from store_info.image.en_us', () => {
		render(UpdateAppModal, {
			props: { app: sampleApp, formattedSize: '120 MB', title: 'NPM', onCancel, onConfirm }
		});
		expect(screen.getByText('jc21/nginx-proxy-manager:latest')).toBeTruthy();
	});

	it('falls back to thumbnail when image.en_us is missing', () => {
		const partial = {
			id: 'foo',
			store_info: { title: 'Foo', thumbnail: 'fallback-tag' }
		};
		render(UpdateAppModal, {
			props: { app: partial, formattedSize: '5 MB', title: 'Foo', onCancel, onConfirm }
		});
		expect(screen.getByText('fallback-tag')).toBeTruthy();
	});

	it('falls back to "latest" when both image and thumbnail are missing', () => {
		const bare = { id: 'bar', store_info: {} };
		render(UpdateAppModal, {
			props: { app: bare, formattedSize: '1 MB', title: 'Bar', onCancel, onConfirm }
		});
		expect(screen.getByText('latest')).toBeTruthy();
	});

	it('renders the formattedSize prop', () => {
		render(UpdateAppModal, {
			props: { app: sampleApp, formattedSize: '247 MB', title: 'NPM', onCancel, onConfirm }
		});
		expect(screen.getByText('247 MB')).toBeTruthy();
	});

	it('calls onCancel when Cancel is clicked', async () => {
		render(UpdateAppModal, {
			props: { app: sampleApp, formattedSize: '120 MB', title: 'NPM', onCancel, onConfirm }
		});
		await fireEvent.click(screen.getByText('Cancel'));
		expect(onCancel).toHaveBeenCalledTimes(1);
	});
});
