import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import NetworkPane from './NetworkPane.svelte';

type Props = {
	mdnsHostname: string;
	reachableUrl: string;
	copiedKey: string | null;
	onCopy: (text: string, key: string) => void;
	networkInterfaces: Array<{ name: string; state: string; type: string; ip?: string; mac?: string }>;
};

const defaultProps: Props = {
	mdnsHostname: 'powerlab.local',
	reachableUrl: 'http://powerlab.local:8765',
	copiedKey: null,
	onCopy: vi.fn(),
	networkInterfaces: []
};

beforeEach(() => {
	defaultProps.onCopy = vi.fn();
});

describe('NetworkPane', () => {
	it('renders the mDNS hostname and reachable URL', () => {
		render(NetworkPane, { props: defaultProps });
		expect(screen.getByText('powerlab.local')).toBeTruthy();
		expect(screen.getByText('http://powerlab.local:8765')).toBeTruthy();
	});

	it('renders empty state when no interfaces are supplied', () => {
		render(NetworkPane, { props: { ...defaultProps, networkInterfaces: [] } });
		expect(screen.getByText(/No network interfaces detected/i)).toBeTruthy();
	});

	it('lists each interface with name, state, type, ip and mac', () => {
		const interfaces = [
			{ name: 'eth0', state: 'up', type: 'physical', ip: '10.0.0.5', mac: 'aa:bb:cc:dd:ee:ff' },
			{ name: 'docker0', state: 'down', type: 'virtual', ip: '172.17.0.1', mac: '00:11:22:33:44:55' }
		];
		render(NetworkPane, { props: { ...defaultProps, networkInterfaces: interfaces } });
		expect(screen.getByText('eth0')).toBeTruthy();
		expect(screen.getByText('docker0')).toBeTruthy();
		expect(screen.getByText(/10\.0\.0\.5/)).toBeTruthy();
		expect(screen.getByText(/172\.17\.0\.1/)).toBeTruthy();
	});

	it('shows "No IP" fallback for interfaces without an IP', () => {
		const interfaces = [{ name: 'lo', state: 'up', type: 'virtual' }];
		render(NetworkPane, { props: { ...defaultProps, networkInterfaces: interfaces } });
		expect(screen.getByText(/No IP/)).toBeTruthy();
		expect(screen.getByText(/No MAC/)).toBeTruthy();
	});

	it('calls onCopy(url, "mdns") when the URL copy button is clicked', async () => {
		const onCopy = vi.fn();
		render(NetworkPane, { props: { ...defaultProps, onCopy } });
		const copyBtn = screen.getByLabelText('Copy URL');
		await fireEvent.click(copyBtn);
		expect(onCopy).toHaveBeenCalledWith('http://powerlab.local:8765', 'mdns');
	});
});
