import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import AppCard from './AppCard.svelte';
import type { ComposeAppWithStoreInfo, ComposeAppStoreInfo } from '$lib/api/apps';

// Test fixtures cast via `unknown` because the production types
// (ComposeAppWithStoreInfo) require fields like `compose` that
// AppCard doesn't actually read. Locally typing every field would
// duplicate the API contract; the unknown-cast keeps the tests
// readable at the cost of a single explicit acknowledgement.
const storeApp = {
	store_app_id: 'nginx-proxy-manager',
	title: { en_us: 'Nginx Proxy Manager' },
	tagline: { en_us: 'Reverse proxy with web UI' },
	icon: 'https://example.com/icon.png',
	port_map: '81',
	hostname: 'localhost'
} as unknown as ComposeAppStoreInfo;

const installedApp = {
	id: 'nginx-proxy-manager',
	status: 'running',
	store_info: { ...storeApp }
} as unknown as ComposeAppWithStoreInfo & { id: string };

beforeEach(() => {
	vi.restoreAllMocks();
});

describe('AppCard', () => {
	it('renders nothing when neither app nor storeApp is provided', () => {
		const { container } = render(AppCard, { props: {} });
		expect(container.querySelector('.flex')).toBeNull();
	});

	it('renders an Install button for a store app (not installed)', () => {
		const onInstall = vi.fn();
		render(AppCard, { props: { storeApp, onInstall } });
		expect(screen.getByText('Install')).toBeTruthy();
		expect(screen.queryByText('Stop')).toBeNull();
	});

	it('calls onInstall when Install is clicked', async () => {
		const onInstall = vi.fn();
		render(AppCard, { props: { storeApp, onInstall } });
		await fireEvent.click(screen.getByText('Install'));
		expect(onInstall).toHaveBeenCalledWith(storeApp);
	});

	it('renders Stop button when an installed app is running', () => {
		render(AppCard, { props: { app: installedApp } });
		expect(screen.getByText('Stop')).toBeTruthy();
	});

	it('renders Start button when an installed app is stopped', () => {
		render(AppCard, { props: { app: { ...installedApp, status: 'stopped' } } });
		expect(screen.getByText('Start')).toBeTruthy();
	});

	it('shows Open UI when running AND port_map + hostname are both present', () => {
		render(AppCard, { props: { app: installedApp } });
		expect(screen.getByText('Open UI')).toBeTruthy();
	});

	it('hides Open UI when port_map is missing', () => {
		const noPort = {
			...installedApp,
			store_info: { ...installedApp.store_info, port_map: undefined }
		};
		render(AppCard, { props: { app: noPort } });
		expect(screen.queryByText('Open UI')).toBeNull();
	});

	it('hides Open UI when hostname is missing', () => {
		const noHost = {
			...installedApp,
			store_info: { ...installedApp.store_info, hostname: undefined }
		};
		render(AppCard, { props: { app: noHost } });
		expect(screen.queryByText('Open UI')).toBeNull();
	});

	it('hides Open UI when the app is stopped (even with port_map + hostname)', () => {
		render(AppCard, { props: { app: { ...installedApp, status: 'stopped' } } });
		expect(screen.queryByText('Open UI')).toBeNull();
	});

	it('renders PowerLab badge when isPowerLabApp=true', () => {
		render(AppCard, { props: { app: installedApp, isPowerLabApp: true } });
		expect(screen.getByText('PowerLab')).toBeTruthy();
		expect(screen.queryByText('Custom')).toBeNull();
	});

	it('renders Custom badge when installed but isPowerLabApp=false', () => {
		render(AppCard, { props: { app: installedApp, isPowerLabApp: false } });
		expect(screen.getByText('Custom')).toBeTruthy();
		expect(screen.queryByText('PowerLab')).toBeNull();
	});

	it('renders no badge for an uninstalled storeApp tile', () => {
		render(AppCard, { props: { storeApp } });
		expect(screen.queryByText('PowerLab')).toBeNull();
		expect(screen.queryByText('Custom')).toBeNull();
	});

	it('falls back to "Unknown App" when title is missing', () => {
		const bare = {
			...installedApp,
			store_info: { ...installedApp.store_info, title: undefined as never }
		};
		render(AppCard, { props: { app: bare } });
		expect(screen.getByText('Unknown App')).toBeTruthy();
	});

	it('falls back to en_US (uppercase) when en_us is missing', () => {
		const upper = {
			...installedApp,
			store_info: { ...installedApp.store_info, title: { en_US: 'Upper' } }
		};
		render(AppCard, { props: { app: upper } });
		expect(screen.getByText('Upper')).toBeTruthy();
	});

	it('falls back to first available locale when neither en_us nor en_US is present', () => {
		const other = {
			...installedApp,
			store_info: { ...installedApp.store_info, title: { fr_fr: 'Bonjour' } }
		};
		render(AppCard, { props: { app: other } });
		expect(screen.getByText('Bonjour')).toBeTruthy();
	});

	it('shows Package fallback icon when no icon URL is supplied', () => {
		const noIcon = {
			...installedApp,
			store_info: { ...installedApp.store_info, icon: undefined }
		} as unknown as ComposeAppWithStoreInfo & { id: string };
		const { container } = render(AppCard, { props: { app: noIcon } });
		expect(container.querySelector('img')).toBeNull();
	});

	it('calls onToggleStatus with the app id + current status when Start/Stop clicked', async () => {
		const onToggleStatus = vi.fn();
		render(AppCard, { props: { app: installedApp, onToggleStatus } });
		await fireEvent.click(screen.getByText('Stop'));
		expect(onToggleStatus).toHaveBeenCalledWith('nginx-proxy-manager', 'running');
	});

	it('calls onUninstall when the Trash icon button is clicked', async () => {
		const onUninstall = vi.fn();
		render(AppCard, { props: { app: installedApp, onUninstall } });
		await fireEvent.click(screen.getByTitle('Uninstall'));
		expect(onUninstall).toHaveBeenCalledWith('nginx-proxy-manager');
	});

	it('calls onOpenLogs when View Logs is clicked', async () => {
		const onOpenLogs = vi.fn();
		render(AppCard, { props: { app: installedApp, onOpenLogs } });
		await fireEvent.click(screen.getByTitle('View Logs'));
		expect(onOpenLogs).toHaveBeenCalledWith('nginx-proxy-manager');
	});

	it('calls onOpenMetrics when View Metrics is clicked', async () => {
		const onOpenMetrics = vi.fn();
		render(AppCard, { props: { app: installedApp, onOpenMetrics } });
		await fireEvent.click(screen.getByTitle('View Metrics'));
		expect(onOpenMetrics).toHaveBeenCalledWith('nginx-proxy-manager');
	});

	it('calls onEdit (when provided) for a PowerLab app', async () => {
		const onEdit = vi.fn();
		render(AppCard, { props: { app: installedApp, isPowerLabApp: true, onEdit } });
		await fireEvent.click(screen.getByTitle('Fork as Custom App'));
		expect(onEdit).toHaveBeenCalledWith('nginx-proxy-manager');
	});

	describe('source badge (Phase 5 of #307)', () => {
		it('renders the badge with the explicit source.catalog label when present', () => {
			const umbrelApp = {
				...installedApp,
				store_info: {
					...installedApp.store_info,
					source: { catalog: 'umbrel-apps' }
				}
			};
			render(AppCard, { props: { app: umbrelApp } });
			expect(screen.getByTestId('app-source-badge').textContent).toBe('umbrel');
		});

		it('falls back to icon heuristic — casaos icon URL', () => {
			const casaosApp = {
				...installedApp,
				store_info: {
					...installedApp.store_info,
					icon: 'https://cdn.jsdelivr.net/gh/IceWhaleTech/CasaOS-AppStore@main/Apps/AdGuardHome/icon.png',
					source: undefined
				}
			};
			render(AppCard, { props: { app: casaosApp } });
			expect(screen.getByTestId('app-source-badge').textContent).toBe('casaos');
		});

		it('renders generic "store" when no explicit source + no recognized icon', () => {
			const genericApp = {
				...installedApp,
				store_info: {
					...installedApp.store_info,
					icon: 'https://example.com/icon.png',
					source: undefined
				}
			};
			render(AppCard, { props: { app: genericApp } });
			expect(screen.getByTestId('app-source-badge').textContent).toBe('store');
		});

		it('badge is rendered as a clickable link when an upstream URL exists', () => {
			const umbrelApp = {
				...installedApp,
				store_info: {
					...installedApp.store_info,
					source: {
						catalog: 'umbrel-apps',
						upstream_repo: 'https://github.com/getumbrel/umbrel-apps'
					}
				}
			};
			render(AppCard, { props: { app: umbrelApp } });
			const badge = screen.getByTestId('app-source-badge') as HTMLAnchorElement;
			expect(badge.tagName).toBe('A');
			expect(badge.href).toBe('https://github.com/getumbrel/umbrel-apps');
			expect(badge.target).toBe('_blank');
		});

		it('badge tooltip carries synced_at when present', () => {
			const umbrelApp = {
				...installedApp,
				store_info: {
					...installedApp.store_info,
					source: { catalog: 'umbrel-apps', synced_at: '2026-05-12T03:00:40Z' }
				}
			};
			render(AppCard, { props: { app: umbrelApp } });
			const badge = screen.getByTestId('app-source-badge');
			expect(badge.getAttribute('title')).toContain('2026-05-12');
			expect(badge.getAttribute('title')).toContain('umbrel');
		});

		it('badge is wrapped in a span (not a link) when source has no upstream URL', () => {
			const generic = {
				...installedApp,
				store_info: {
					...installedApp.store_info,
					icon: 'https://example.com/x.png',
					source: undefined
				}
			};
			render(AppCard, { props: { app: generic } });
			const badge = screen.getByTestId('app-source-badge');
			expect(badge.tagName).toBe('SPAN');
		});

		it('clicking the source link does NOT bubble up to the card click handler', async () => {
			// Catalog tile may have an outer click handler in production
			// (e.g. open install modal). The source link must NOT trigger it.
			const umbrelApp = {
				...installedApp,
				store_info: {
					...installedApp.store_info,
					source: { catalog: 'umbrel-apps', upstream_repo: 'https://github.com/getumbrel/umbrel-apps' }
				}
			};
			const onToggleStatus = vi.fn();
			render(AppCard, { props: { app: umbrelApp, onToggleStatus } });
			// Prevent the actual navigation
			const badge = screen.getByTestId('app-source-badge') as HTMLAnchorElement;
			badge.addEventListener('click', (e) => e.preventDefault(), { once: true });
			await fireEvent.click(badge);
			expect(onToggleStatus).not.toHaveBeenCalled();
		});
	});
});
