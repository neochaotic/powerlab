import { describe, it, expect, vi, beforeEach } from 'vitest';
import {
	getAppStoreList,
	getInstalledApps,
	installComposeApp,
	getStoreAppYaml,
	getComposeApp,
	uninstallComposeApp,
	setComposeAppStatus,
	getComposeAppContainers,
	getComposeAppLogs
} from './apps';

function mockJson(data: unknown, status = 200) {
	return vi.fn().mockResolvedValue({
		ok: status >= 200 && status < 300,
		status,
		statusText: 'OK',
		headers: new Headers({ 'content-type': 'application/json' }),
		text: () => Promise.resolve(JSON.stringify(data))
	});
}

function mockText(content: string, contentType = 'application/yaml') {
	return vi.fn().mockResolvedValue({
		ok: true,
		status: 200,
		statusText: 'OK',
		headers: new Headers({ 'content-type': contentType }),
		text: () => Promise.resolve(content)
	});
}

describe('Apps API', () => {
	beforeEach(() => {
		vi.restoreAllMocks();
	});

	// ─── App Store ────────────────────────────────────────────────────────

	it('getAppStoreList: builds URL with no params', async () => {
		vi.stubGlobal('fetch', mockJson({ data: { installed: [], list: {} } }));

		await getAppStoreList();

		expect(fetch).toHaveBeenCalledWith('/v2/app_management/apps', expect.anything());
	});

	it('getAppStoreList: builds URL with all query params', async () => {
		vi.stubGlobal('fetch', mockJson({ data: { installed: [], list: {} } }));

		await getAppStoreList('chat', 'official', true);

		expect(fetch).toHaveBeenCalledWith(
			expect.stringContaining('category=chat'),
			expect.anything()
		);
		expect(fetch).toHaveBeenCalledWith(
			expect.stringContaining('author_type=official'),
			expect.anything()
		);
		expect(fetch).toHaveBeenCalledWith(
			expect.stringContaining('recommend=true'),
			expect.anything()
		);
	});

	it('getAppStoreList: omits undefined optional params', async () => {
		vi.stubGlobal('fetch', mockJson({ data: { installed: [], list: {} } }));

		await getAppStoreList('media');

		const url = (fetch as ReturnType<typeof vi.fn>).mock.calls[0][0] as string;
		expect(url).toContain('category=media');
		expect(url).not.toContain('author_type');
		expect(url).not.toContain('recommend');
	});

	// ─── Installed Apps ───────────────────────────────────────────────────

	it('getInstalledApps: calls correct endpoint', async () => {
		vi.stubGlobal('fetch', mockJson({ data: {}, message: '' }));

		await getInstalledApps();

		expect(fetch).toHaveBeenCalledWith('/v2/app_management/compose', expect.anything());
	});

	// ─── Installation ─────────────────────────────────────────────────────

	it('installComposeApp: sends YAML with correct content-type', async () => {
		vi.stubGlobal('fetch', mockJson({ message: 'ok' }));

		await installComposeApp('services:\n  app:\n    image: nginx');

		expect(fetch).toHaveBeenCalledWith(
			'/v2/app_management/compose',
			expect.objectContaining({
				method: 'POST',
				headers: expect.objectContaining({ 'Content-Type': 'application/yaml' }),
				body: 'services:\n  app:\n    image: nginx'
			})
		);
	});

	it('installComposeApp: sends dry_run query param', async () => {
		vi.stubGlobal('fetch', mockJson({ message: 'ok' }));

		await installComposeApp('services: {}', true);

		expect(fetch).toHaveBeenCalledWith(
			'/v2/app_management/compose?dry_run=true',
			expect.anything()
		);
	});

	it('installComposeApp: no dry_run param when false', async () => {
		vi.stubGlobal('fetch', mockJson({ message: 'ok' }));

		await installComposeApp('services: {}', false);

		const url = (fetch as ReturnType<typeof vi.fn>).mock.calls[0][0] as string;
		expect(url).not.toContain('dry_run');
	});

	// ─── Compose App Operations ───────────────────────────────────────────

	/**
	 * Regression: the install flow previously called getComposeApp (installed-apps
	 * endpoint /compose/{id}) instead of getStoreAppYaml (store endpoint /apps/{id}/compose).
	 * The installed-apps endpoint returns "compose app `X` not found" for apps that
	 * haven't been installed yet, breaking every store installation.
	 */
	it('getStoreAppYaml: calls store endpoint /apps/{id}/compose, NOT /compose/{id}', async () => {
		vi.stubGlobal('fetch', mockText('name: autobrr\nservices:\n  autobrr:\n    image: autobrr/autobrr'));

		const result = await getStoreAppYaml('autobrr');

		expect(fetch).toHaveBeenCalledWith(
			'/v2/app_management/apps/autobrr/compose',
			expect.objectContaining({
				headers: expect.objectContaining({ Accept: 'application/yaml' })
			})
		);
		// Must NOT call the installed-apps endpoint
		const url = (fetch as ReturnType<typeof vi.fn>).mock.calls[0][0] as string;
		expect(url).not.toContain('/compose/autobrr');
		expect(result).toBe('name: autobrr\nservices:\n  autobrr:\n    image: autobrr/autobrr');
	});

	it('getComposeApp: calls installed-apps endpoint /compose/{id} with Accept yaml', async () => {
		vi.stubGlobal('fetch', mockText('services:\n  app:\n    image: nginx'));

		await getComposeApp('syncthing');

		expect(fetch).toHaveBeenCalledWith(
			'/v2/app_management/compose/syncthing',
			expect.objectContaining({
				headers: expect.objectContaining({ Accept: 'application/yaml' })
			})
		);
	});

	it('uninstallComposeApp: sends DELETE with default config', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn().mockResolvedValue({
				ok: true,
				status: 204,
				statusText: 'No Content',
				headers: new Headers(),
				text: () => Promise.resolve('')
			})
		);

		await uninstallComposeApp('syncthing');

		expect(fetch).toHaveBeenCalledWith(
			'/v2/app_management/compose/syncthing?delete_config_folder=false',
			expect.objectContaining({ method: 'DELETE' })
		);
	});

	it('uninstallComposeApp: passes delete_config_folder=true', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn().mockResolvedValue({
				ok: true,
				status: 204,
				statusText: 'No Content',
				headers: new Headers(),
				text: () => Promise.resolve('')
			})
		);

		await uninstallComposeApp('syncthing', true);

		expect(fetch).toHaveBeenCalledWith(
			'/v2/app_management/compose/syncthing?delete_config_folder=true',
			expect.objectContaining({ method: 'DELETE' })
		);
	});

	// ─── Status Control ───────────────────────────────────────────────────

	it('setComposeAppStatus: sends start payload', async () => {
		vi.stubGlobal('fetch', mockJson({ message: 'ok' }));

		await setComposeAppStatus('syncthing', 'start');

		expect(fetch).toHaveBeenCalledWith(
			'/v2/app_management/compose/syncthing/status',
			expect.objectContaining({
				method: 'PUT',
				body: JSON.stringify('start')
			})
		);
	});

	it('setComposeAppStatus: sends stop payload', async () => {
		vi.stubGlobal('fetch', mockJson({ message: 'ok' }));

		await setComposeAppStatus('plex', 'stop');

		expect(fetch).toHaveBeenCalledWith(
			'/v2/app_management/compose/plex/status',
			expect.objectContaining({ body: JSON.stringify('stop') })
		);
	});

	it('setComposeAppStatus: sends restart payload', async () => {
		vi.stubGlobal('fetch', mockJson({ message: 'ok' }));

		await setComposeAppStatus('syncthing', 'restart');

		expect(fetch).toHaveBeenCalledWith(
			'/v2/app_management/compose/syncthing/status',
			expect.objectContaining({ body: JSON.stringify('restart') })
		);
	});

	// ─── Containers & Logs ────────────────────────────────────────────────

	it('getComposeAppContainers: calls correct endpoint', async () => {
		vi.stubGlobal('fetch', mockJson({ data: { main: 'syncthing', containers: {} } }));

		await getComposeAppContainers('syncthing');

		expect(fetch).toHaveBeenCalledWith(
			'/v2/app_management/compose/syncthing/containers',
			expect.anything()
		);
	});

	it('getComposeAppLogs: calls correct endpoint with default lines', async () => {
		vi.stubGlobal('fetch', mockJson({ data: 'log output' }));

		await getComposeAppLogs('syncthing');

		expect(fetch).toHaveBeenCalledWith(
			'/v2/app_management/compose/syncthing/logs?lines=1000',
			expect.anything()
		);
	});

	it('getComposeAppLogs: respects custom lines param', async () => {
		vi.stubGlobal('fetch', mockJson({ data: 'log output' }));

		await getComposeAppLogs('syncthing', 200);

		expect(fetch).toHaveBeenCalledWith(
			'/v2/app_management/compose/syncthing/logs?lines=200',
			expect.anything()
		);
	});

	// ─── Error Handling ───────────────────────────────────────────────────

	it('throws ApiError when app not found (404)', async () => {
		vi.stubGlobal('fetch', mockJson({ message: 'Not found' }, 404));

		await expect(getInstalledApps()).rejects.toMatchObject({
			status: 404,
			message: 'Not found'
		});
	});
});
