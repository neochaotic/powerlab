import { render, screen, waitFor } from '@testing-library/svelte';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import DockerUnavailableBanner from './DockerUnavailableBanner.svelte';

// #63: a host without Docker used to show an empty App Store / Launchpad
// with no diagnostic. The banner probes /v2/app_management/docker/system
// (503 when the daemon is missing) and shows install hints.

function mockFetch(status: number, body: unknown) {
	const fn = vi.fn().mockResolvedValue({
		ok: status >= 200 && status < 300,
		status,
		statusText: status === 503 ? 'Service Unavailable' : 'OK',
		headers: new Headers({ 'content-type': 'application/json' }),
		text: () => Promise.resolve(JSON.stringify(body))
	});
	vi.stubGlobal('fetch', fn);
	return fn;
}

describe('DockerUnavailableBanner', () => {
	beforeEach(() => {
		vi.restoreAllMocks();
	});
	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('shows install hints and docs links when the Docker probe returns 503', async () => {
		const fetchFn = mockFetch(503, {
			message: 'Cannot connect to the Docker daemon at unix:///var/run/docker.sock'
		});
		render(DockerUnavailableBanner);

		const banner = await screen.findByTestId('docker-unavailable-banner');
		expect(fetchFn).toHaveBeenCalledWith('/v2/app_management/docker/system', expect.anything());
		expect(banner.getAttribute('role')).toBe('alert');
		expect(banner.textContent).toContain('Docker is not available');
		expect(banner.textContent).toContain('curl -fsSL https://get.docker.com | sudo sh');
		expect(banner.textContent).toContain('sudo systemctl enable --now docker');
		expect(screen.getByTestId('docker-unavailable-reason').textContent).toContain(
			'Cannot connect to the Docker daemon'
		);
		const hrefs = Array.from(banner.querySelectorAll('a')).map((a) => a.getAttribute('href'));
		expect(hrefs).toContain('https://docs.docker.com/engine/install/');
		expect(hrefs).toContain('https://neochaotic.github.io/powerlab/getting-started/install/');
	});

	it('shows the banner for a 500 from app-management too', async () => {
		mockFetch(500, { message: 'boom' });
		render(DockerUnavailableBanner);
		expect(await screen.findByTestId('docker-unavailable-banner')).toBeTruthy();
	});

	it('stays hidden when Docker answers', async () => {
		const fetchFn = mockFetch(200, {
			docker_version: '27.0.1',
			containers_count: 3,
			images_count: 5
		});
		render(DockerUnavailableBanner);
		await waitFor(() => expect(fetchFn).toHaveBeenCalled());
		await new Promise((r) => setTimeout(r, 0));
		expect(screen.queryByTestId('docker-unavailable-banner')).toBeNull();
	});

	it('stays hidden on non-5xx errors (auth, older backend without the route)', async () => {
		const fetchFn = mockFetch(404, { message: 'not found' });
		render(DockerUnavailableBanner);
		await waitFor(() => expect(fetchFn).toHaveBeenCalled());
		await new Promise((r) => setTimeout(r, 0));
		expect(screen.queryByTestId('docker-unavailable-banner')).toBeNull();
	});
});
