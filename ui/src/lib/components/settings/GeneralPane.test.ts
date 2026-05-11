import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import GeneralPane from './GeneralPane.svelte';

type Props = {
	osHostname: string;
	timezone: string;
	onTimezoneChange: (v: string) => void;
	reachableUrl: string;
	currentPort: string;
	portInput: number;
	onPortInputChange: (v: number) => void;
	onRequestPortChange: () => void;
	timezones: readonly string[];
};

const defaultProps: Props = {
	osHostname: 'powerlab-host',
	timezone: 'America/Sao_Paulo',
	onTimezoneChange: vi.fn(),
	reachableUrl: 'http://powerlab.local:8765',
	currentPort: '8765',
	portInput: 8765,
	onPortInputChange: vi.fn(),
	onRequestPortChange: vi.fn(),
	timezones: ['UTC', 'America/Sao_Paulo', 'Europe/Berlin']
};

beforeEach(() => {
	defaultProps.onTimezoneChange = vi.fn();
	defaultProps.onPortInputChange = vi.fn();
	defaultProps.onRequestPortChange = vi.fn();
});

describe('GeneralPane', () => {
	it('renders the OS hostname', () => {
		render(GeneralPane, { props: defaultProps });
		expect(screen.getByText('powerlab-host')).toBeTruthy();
	});

	it('falls back to "Unknown" when osHostname is empty', () => {
		render(GeneralPane, { props: { ...defaultProps, osHostname: '' } });
		expect(screen.getByText('Unknown')).toBeTruthy();
	});

	it('renders the current listen port', () => {
		render(GeneralPane, { props: defaultProps });
		expect(screen.getByText('8765')).toBeTruthy();
	});

	it('disables the Change… button when portInput matches currentPort (string equality)', () => {
		render(GeneralPane, {
			props: { ...defaultProps, currentPort: '8765', portInput: 8765 }
		});
		const btn = screen.getByText('Change…') as HTMLButtonElement;
		expect(btn.disabled).toBe(true);
	});

	it('enables the Change… button when portInput differs from currentPort', () => {
		render(GeneralPane, {
			props: { ...defaultProps, currentPort: '8765', portInput: 9000 }
		});
		const btn = screen.getByText('Change…') as HTMLButtonElement;
		expect(btn.disabled).toBe(false);
	});

	it('calls onPortInputChange when the port input fires oninput', async () => {
		const onPortInputChange = vi.fn();
		render(GeneralPane, { props: { ...defaultProps, onPortInputChange } });
		const input = screen.getByDisplayValue('8765') as HTMLInputElement;
		await fireEvent.input(input, { target: { value: '9090' } });
		expect(onPortInputChange).toHaveBeenCalledWith(9090);
	});

	it('calls onRequestPortChange when Change… is clicked', async () => {
		const onRequestPortChange = vi.fn();
		render(GeneralPane, {
			props: { ...defaultProps, portInput: 9090, onRequestPortChange }
		});
		await fireEvent.click(screen.getByText('Change…'));
		expect(onRequestPortChange).toHaveBeenCalled();
	});

	it('calls onTimezoneChange when the timezone select changes', async () => {
		const onTimezoneChange = vi.fn();
		render(GeneralPane, { props: { ...defaultProps, onTimezoneChange } });
		// Locate the timezone select via its current value
		const selects = document.querySelectorAll('select');
		const tzSelect = Array.from(selects).find(s => (s as HTMLSelectElement).value === 'America/Sao_Paulo') as HTMLSelectElement;
		expect(tzSelect).toBeTruthy();
		await fireEvent.change(tzSelect, { target: { value: 'UTC' } });
		expect(onTimezoneChange).toHaveBeenCalledWith('UTC');
	});

	it('renders the reachable URL inline', () => {
		render(GeneralPane, { props: defaultProps });
		expect(screen.getByText('http://powerlab.local:8765')).toBeTruthy();
	});
});
