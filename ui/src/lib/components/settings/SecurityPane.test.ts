import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import SecurityPane from './SecurityPane.svelte';
import type { OS } from '$lib/utils/os';

type CertFormat = 'mobileconfig' | 'crt' | 'cer';
type Props = {
	activeSecurityTab: OS;
	onTabChange: (tab: OS) => void;
	isTestingConnection: boolean;
	isHttpsSelfSigned: boolean;
	onDownloadCA: (format: CertFormat) => void;
	onOpenHttpDownload: (format: CertFormat) => void;
	onTestHttpsConnection: () => void;
	onResetTrust: () => void;
	onConfirmRotateCA: () => void;
};

const defaultProps: Props = {
	activeSecurityTab: 'macos',
	onTabChange: vi.fn(),
	isTestingConnection: false,
	isHttpsSelfSigned: false,
	onDownloadCA: vi.fn(),
	onOpenHttpDownload: vi.fn(),
	onTestHttpsConnection: vi.fn(),
	onResetTrust: vi.fn(),
	onConfirmRotateCA: vi.fn()
};

beforeEach(() => {
	defaultProps.onTabChange = vi.fn();
	defaultProps.onDownloadCA = vi.fn();
	defaultProps.onOpenHttpDownload = vi.fn();
	defaultProps.onTestHttpsConnection = vi.fn();
	defaultProps.onResetTrust = vi.fn();
	defaultProps.onConfirmRotateCA = vi.fn();
});

describe('SecurityPane', () => {
	it('renders the Security header', () => {
		render(SecurityPane, { props: defaultProps });
		expect(screen.getByText('Security')).toBeTruthy();
	});

	it('renders without throwing for each OS tab value', () => {
		for (const tab of ['macos', 'windows', 'linux', 'ios', 'android'] as const) {
			expect(() =>
				render(SecurityPane, { props: { ...defaultProps, activeSecurityTab: tab } })
			).not.toThrow();
		}
	});

	it('renders without throwing when HTTPS is self-signed', () => {
		expect(() =>
			render(SecurityPane, { props: { ...defaultProps, isHttpsSelfSigned: true } })
		).not.toThrow();
	});

	it('renders without throwing while a connection test is in flight', () => {
		expect(() =>
			render(SecurityPane, { props: { ...defaultProps, isTestingConnection: true } })
		).not.toThrow();
	});
});
