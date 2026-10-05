import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import type { ComposeAppStoreInfo } from '$lib/api/apps';
import InstallConfirmModal, { type PortChoice } from './InstallConfirmModal.svelte';

const APP = {
	store_app_id: 'jellyfin',
	title: { en_us: 'Jellyfin' },
	icon: '',
	developer: 'Jellyfin Team',
	author: 'someone'
} as unknown as ComposeAppStoreInfo;

let onPortInput: () => void;
let onAutoPick: (c: PortChoice) => void;
let onCancel: () => void;
let onConfirm: () => void;

function props(overrides: Record<string, unknown> = {}) {
	return {
		app: APP,
		title: 'Jellyfin',
		beforeInstallTip: '',
		isChecking: false,
		warnings: [] as string[],
		hasCriticalWarning: false,
		portChoices: [] as PortChoice[],
		portsResolved: true,
		onPortInput,
		onAutoPick,
		onCancel,
		onConfirm,
		...overrides
	};
}

beforeEach(() => {
	onPortInput = vi.fn();
	onAutoPick = vi.fn();
	onCancel = vi.fn();
	onConfirm = vi.fn();
});

describe('InstallConfirmModal', () => {
	it('renders nothing when app is null', () => {
		const { container } = render(InstallConfirmModal, { props: props({ app: null }) });
		expect(container.querySelector('.fixed')).toBeNull();
	});

	it('shows title, developer and the first-run tip', () => {
		render(InstallConfirmModal, {
			props: props({ beforeInstallTip: 'Admin token is in /DATA/AppData/x/token.txt' })
		});
		expect(screen.getByText('Jellyfin')).toBeTruthy();
		expect(screen.getByText('Jellyfin Team')).toBeTruthy();
		expect(screen.getByText('First-run note')).toBeTruthy();
		expect(screen.getByText('Admin token is in /DATA/AppData/x/token.txt')).toBeTruthy();
	});

	it('hides the tip block when there is no tip', () => {
		render(InstallConfirmModal, { props: props() });
		expect(screen.queryByText('First-run note')).toBeNull();
	});

	it('shows a loading row and disables Start while checking', () => {
		render(InstallConfirmModal, {
			props: props({ isChecking: true, warnings: ['should be hidden'] })
		});
		expect(screen.getByText(/Loading/)).toBeTruthy();
		expect(screen.queryByText('should be hidden')).toBeNull();
		expect((screen.getByText('Start').closest('button') as HTMLButtonElement).disabled).toBe(true);
	});

	it('lists compatibility warnings and switches to Install Anyway on a critical one', () => {
		render(InstallConfirmModal, {
			props: props({ warnings: ['Critical: privileged mode'], hasCriticalWarning: true })
		});
		expect(screen.getByText('Compatibility Warnings')).toBeTruthy();
		expect(screen.getByText('Critical: privileged mode')).toBeTruthy();
		expect(screen.getByText('Install Anyway')).toBeTruthy();
		expect(screen.queryByText('Start')).toBeNull();
	});

	it('renders port choices, binds edits and forwards Auto', async () => {
		const choices: PortChoice[] = [
			{ original: 8096, chosen: '8097', status: 'free' },
			{ original: 8920, chosen: '8920', status: 'inuse' }
		];
		render(InstallConfirmModal, {
			props: props({ portChoices: choices, portsResolved: false })
		});
		expect(screen.getByText('Port Conflicts')).toBeTruthy();
		expect(screen.getByText('8096')).toBeTruthy();
		expect(screen.getByText(/Edit the highlighted ports/)).toBeTruthy();

		const inputs = screen.getAllByRole('spinbutton') as HTMLInputElement[];
		expect(inputs.map((i) => i.value)).toEqual(['8097', '8920']);

		await fireEvent.input(inputs[0], { target: { value: '9000' } });
		expect(onPortInput).toHaveBeenCalledTimes(1);
		expect(Number(choices[0].chosen)).toBe(9000);

		// Auto only appears on the conflicting row.
		const autos = screen.getAllByText('Auto');
		expect(autos).toHaveLength(1);
		await fireEvent.click(autos[0]);
		expect(onAutoPick).toHaveBeenCalledWith(choices[1]);

		expect((screen.getByText('Start').closest('button') as HTMLButtonElement).disabled).toBe(true);
	});

	it('says ports are free once resolved', () => {
		render(InstallConfirmModal, {
			props: props({ portChoices: [{ original: 80, chosen: '8081', status: 'free' }] })
		});
		expect(screen.getByText(/All chosen ports are free/)).toBeTruthy();
	});

	it('Cancel and Start call their callbacks', async () => {
		render(InstallConfirmModal, { props: props() });
		await fireEvent.click(screen.getByText('Cancel'));
		expect(onCancel).toHaveBeenCalledTimes(1);
		await fireEvent.click(screen.getByText('Start'));
		expect(onConfirm).toHaveBeenCalledTimes(1);
	});
});
