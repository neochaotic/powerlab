import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import type { ComposeAppStoreInfo } from '$lib/api/apps';
import DetailModal from './DetailModal.svelte';

const APP = {
	store_app_id: 'jellyfin',
	title: { en_us: 'Jellyfin' },
	icon: 'https://example.invalid/jellyfin.png',
	developer: 'Jellyfin Team',
	author: 'someone',
	category: 'Media',
	screenshot_link: ['https://example.invalid/s1.png', 'https://example.invalid/s2.png'],
	tips: { custom: 'Set **ADMIN_TOKEN** to override.' }
} as unknown as ComposeAppStoreInfo;

let onClose: () => void;
let onGet: () => void;

function props(overrides: Record<string, unknown> = {}) {
	return {
		app: APP,
		title: 'Jellyfin',
		tagline: 'The free media system',
		description: 'Stream your media anywhere.',
		beforeInstallTip: '',
		onClose,
		onGet,
		...overrides
	};
}

beforeEach(() => {
	onClose = vi.fn();
	onGet = vi.fn();
});

describe('DetailModal', () => {
	it('renders nothing when app is null', () => {
		const { container } = render(DetailModal, { props: props({ app: null }) });
		expect(container.querySelector('.fixed')).toBeNull();
	});

	it('renders header, screenshots, description and custom tip', () => {
		render(DetailModal, { props: props() });
		expect(screen.getByRole('heading', { level: 1 }).textContent).toBe('Jellyfin');
		expect(screen.getByText('Jellyfin Team')).toBeTruthy();
		expect(screen.getByText('The free media system')).toBeTruthy();
		expect(screen.getByText('Media')).toBeTruthy();
		expect(screen.getAllByAltText('Screenshot')).toHaveLength(2);
		expect(screen.getByText('Stream your media anywhere.')).toBeTruthy();
		expect(screen.getByText('First-run note')).toBeTruthy();
		expect(screen.getByText('ADMIN_TOKEN')).toBeTruthy();
		expect(screen.queryByText('Before you install')).toBeNull();
	});

	it('falls back to author / placeholder developer and hides an Unknown tagline', () => {
		render(DetailModal, {
			props: props({
				app: { ...APP, developer: '', author: '', screenshot_link: [], tips: undefined },
				tagline: 'Unknown'
			})
		});
		expect(screen.getByText('Independent Developer')).toBeTruthy();
		expect(screen.queryByText('Unknown')).toBeNull();
		expect(screen.queryByText('Preview')).toBeNull();
		expect(screen.queryByText('First-run note')).toBeNull();
	});

	it('shows the before-install tip when provided', () => {
		render(DetailModal, { props: props({ beforeInstallTip: 'Default password: changeme' }) });
		expect(screen.getByText('Before you install')).toBeTruthy();
		expect(screen.getByText('Default password: changeme')).toBeTruthy();
	});

	it('closes from the backdrop and the close button, not from the sheet', async () => {
		const { container } = render(DetailModal, { props: props() });
		const backdrop = container.querySelector('.fixed') as HTMLElement;
		const sheet = backdrop.firstElementChild as HTMLElement;

		await fireEvent.click(sheet);
		expect(onClose).not.toHaveBeenCalled();

		await fireEvent.click(backdrop);
		expect(onClose).toHaveBeenCalledTimes(1);

		const closeBtn = sheet.querySelector('button') as HTMLButtonElement;
		await fireEvent.click(closeBtn);
		expect(onClose).toHaveBeenCalledTimes(2);
	});

	it('Get calls onGet', async () => {
		render(DetailModal, { props: props() });
		await fireEvent.click(screen.getByText('Get'));
		expect(onGet).toHaveBeenCalledTimes(1);
		expect(onClose).not.toHaveBeenCalled();
	});
});
