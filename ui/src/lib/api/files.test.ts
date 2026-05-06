import { describe, it, expect, vi, beforeEach } from 'vitest';
import {
	listDirectory,
	createFolder,
	createFile,
	renamePath,
	deletePaths,
	operateFileOrDir,
	getDirectorySize,
	getDownloadUrl,
	getBatchDownloadUrl,
	readFileContent,
	updateFileContent
} from './files';
import { setAuthToken } from './client';

function mockJson(data: unknown, status = 200) {
	return vi.fn().mockResolvedValue({
		ok: status >= 200 && status < 300,
		status,
		statusText: 'OK',
		headers: new Headers({ 'content-type': 'application/json' }),
		text: () => Promise.resolve(JSON.stringify(data))
	});
}

const mockFileList = {
	success: 200,
	message: 'success',
	data: {
		content: [
			{ name: 'documents', size: 0, is_dir: true, modified: '2024-01-01T00:00:00Z', path: '/DATA/documents', sign: '', thumb: '', type: 0, date: '', extensions: null },
			{ name: 'video.mp4', size: 1024000, is_dir: false, modified: '2024-01-02T00:00:00Z', path: '/DATA/video.mp4', sign: '', thumb: '', type: 0, date: '', extensions: null }
		],
		total: 2,
		index: 1,
		size: 1000
	}
};

describe('Files API', () => {
	beforeEach(() => {
		vi.restoreAllMocks();
	});

	// ─── Directory Listing ────────────────────────────────────────────────

	it('listDirectory: builds correct URL with encoded path', async () => {
		vi.stubGlobal('fetch', mockJson(mockFileList));

		await listDirectory('/DATA');

		expect(fetch).toHaveBeenCalledWith(
			expect.stringContaining('/v1/folder?path=%2FDATA'),
			expect.anything()
		);
	});

	it('listDirectory: includes index and size params', async () => {
		vi.stubGlobal('fetch', mockJson(mockFileList));

		await listDirectory('/DATA', 2, 50);

		const url = (fetch as ReturnType<typeof vi.fn>).mock.calls[0][0] as string;
		expect(url).toContain('index=2');
		expect(url).toContain('size=50');
	});

	it('listDirectory: defaults to index=1 and size=1000', async () => {
		vi.stubGlobal('fetch', mockJson(mockFileList));

		await listDirectory('/DATA');

		const url = (fetch as ReturnType<typeof vi.fn>).mock.calls[0][0] as string;
		expect(url).toContain('index=1');
		expect(url).toContain('size=1000');
	});

	it('listDirectory: encodes paths with special characters', async () => {
		vi.stubGlobal('fetch', mockJson(mockFileList));

		await listDirectory('/DATA/my folder/sub dir');

		const url = (fetch as ReturnType<typeof vi.fn>).mock.calls[0][0] as string;
		// URLSearchParams encodes spaces as '+', which is valid for query strings
		const decodedUrl = decodeURIComponent(url.replace(/\+/g, ' '));
		expect(decodedUrl).toContain('/DATA/my folder/sub dir');
	});

	// ─── Create Operations ────────────────────────────────────────────────

	it('createFolder: sends correct payload', async () => {
		vi.stubGlobal('fetch', mockJson({ success: 200, message: '', data: null }));

		await createFolder('/DATA/new-folder');

		expect(fetch).toHaveBeenCalledWith(
			'/v1/folder',
			expect.objectContaining({
				method: 'POST',
				body: JSON.stringify({ path: '/DATA/new-folder' })
			})
		);
	});

	it('createFile: sends correct payload', async () => {
		vi.stubGlobal('fetch', mockJson({ success: 200, message: '', data: null }));

		await createFile('/DATA/new-file.txt');

		expect(fetch).toHaveBeenCalledWith(
			'/v1/file',
			expect.objectContaining({
				method: 'POST',
				body: JSON.stringify({ path: '/DATA/new-file.txt' })
			})
		);
	});

	// ─── Rename ───────────────────────────────────────────────────────────

	it('renamePath: sends old_path and new_path', async () => {
		vi.stubGlobal('fetch', mockJson({ success: 200, message: '', data: null }));

		await renamePath('/DATA/old-name.txt', '/DATA/new-name.txt');

		expect(fetch).toHaveBeenCalledWith(
			'/v1/file/name',
			expect.objectContaining({
				method: 'PUT',
				body: JSON.stringify({ old_path: '/DATA/old-name.txt', new_path: '/DATA/new-name.txt' })
			})
		);
	});

	// ─── Delete ───────────────────────────────────────────────────────────

	it('deletePaths: sends array of paths as JSON body', async () => {
		vi.stubGlobal('fetch', mockJson({ success: 200, message: '', data: null }));

		await deletePaths(['/DATA/file1.txt', '/DATA/file2.txt']);

		expect(fetch).toHaveBeenCalledWith(
			'/v1/batch',
			expect.objectContaining({
				method: 'DELETE',
				body: JSON.stringify(['/DATA/file1.txt', '/DATA/file2.txt'])
			})
		);
	});

	it('deletePaths: sends single path as array', async () => {
		vi.stubGlobal('fetch', mockJson({ success: 200, message: '', data: null }));

		await deletePaths(['/DATA/single.txt']);

		expect(fetch).toHaveBeenCalledWith(
			'/v1/batch',
			expect.objectContaining({ body: JSON.stringify(['/DATA/single.txt']) })
		);
	});

	// ─── Copy & Move ──────────────────────────────────────────────────────

	it('operateFileOrDir (copy): sends correct payload', async () => {
		vi.stubGlobal('fetch', mockJson({ success: 200, message: '', data: null }));

		await operateFileOrDir('copy', '/DATA/dest', [{ from: '/DATA/src/file.txt' }]);

		expect(fetch).toHaveBeenCalledWith(
			'/v1/batch/task',
			expect.objectContaining({
				method: 'POST',
				body: JSON.stringify({ type: 'copy', to: '/DATA/dest', item: [{ from: '/DATA/src/file.txt' }] })
			})
		);
	});

	it('operateFileOrDir (move): sends correct payload', async () => {
		vi.stubGlobal('fetch', mockJson({ success: 200, message: '', data: null }));

		await operateFileOrDir('move', '/DATA/dest', [
			{ from: '/DATA/src/a.txt' },
			{ from: '/DATA/src/b.txt' }
		]);

		expect(fetch).toHaveBeenCalledWith(
			'/v1/batch/task',
			expect.objectContaining({
				body: JSON.stringify({
					type: 'move',
					to: '/DATA/dest',
					item: [{ from: '/DATA/src/a.txt' }, { from: '/DATA/src/b.txt' }]
				})
			})
		);
	});

	// ─── Directory Size ───────────────────────────────────────────────────

	it('getDirectorySize: builds URL with encoded path', async () => {
		vi.stubGlobal('fetch', mockJson({ success: 200, message: '', data: { size: 1024 } }));

		await getDirectorySize('/DATA/documents');

		expect(fetch).toHaveBeenCalledWith(
			expect.stringContaining(`/v1/folder/size?path=${encodeURIComponent('/DATA/documents')}`),
			expect.anything()
		);
	});

	// ─── URL Builders (pure functions, no fetch) ──────────────────────────

	it('getDownloadUrl: builds correct single-file URL when not signed in', () => {
		setAuthToken(null);
		const url = getDownloadUrl('/DATA/test.txt');
		expect(url).toBe('/v1/file?path=%2FDATA%2Ftest.txt');
	});

	it('getDownloadUrl: appends ?token= when signed in (so <video src> can authenticate)', () => {
		// <a href> / <video src> / <img src> can't send Authorization
		// headers. Without a token in the URL, every Files-page download
		// or media preview returns 401 from any non-localhost client.
		setAuthToken('JWT_FOR_DOWNLOAD');
		const url = getDownloadUrl('/DATA/movie.mp4');
		expect(url).toContain('path=%2FDATA%2Fmovie.mp4');
		expect(url).toContain('token=JWT_FOR_DOWNLOAD');
		setAuthToken(null);
	});

	it('getDownloadUrl: encodes special characters in path', () => {
		setAuthToken(null);
		const url = getDownloadUrl('/DATA/my file (1).txt');
		expect(url).toContain(encodeURIComponent('/DATA/my file (1).txt'));
	});

	it('getBatchDownloadUrl: appends ?token= when signed in', () => {
		setAuthToken('JWT_FOR_BATCH');
		const url = getBatchDownloadUrl(['/DATA/a.txt'], 'tar');
		expect(url).toContain('token=JWT_FOR_BATCH');
		setAuthToken(null);
	});

	it('getBatchDownloadUrl: builds URL with zip format', () => {
		const url = getBatchDownloadUrl(['/DATA/a.txt', '/DATA/b.txt'], 'zip');
		expect(url).toContain('/v1/batch?files=');
		expect(url).toContain('format=zip');
	});

	it('getBatchDownloadUrl: defaults to zip format', () => {
		const url = getBatchDownloadUrl(['/DATA/a.txt']);
		expect(url).toContain('format=zip');
	});

	it('getBatchDownloadUrl: supports tar format', () => {
		const url = getBatchDownloadUrl(['/DATA/a.txt'], 'tar');
		expect(url).toContain('format=tar');
	});

	// ─── File Content ─────────────────────────────────────────────────────

	it('readFileContent: builds correct URL', async () => {
		vi.stubGlobal('fetch', mockJson({ success: 200, message: '', data: 'file content' }));

		await readFileContent('/DATA/config.yaml');

		expect(fetch).toHaveBeenCalledWith(
			expect.stringContaining(`/v1/file/content?path=${encodeURIComponent('/DATA/config.yaml')}`),
			expect.anything()
		);
	});

	it('updateFileContent: sends file_path and file_content', async () => {
		vi.stubGlobal('fetch', mockJson({ success: 200, message: '', data: null }));

		await updateFileContent('/DATA/config.yaml', 'key: value');

		expect(fetch).toHaveBeenCalledWith(
			'/v1/file',
			expect.objectContaining({
				method: 'PUT',
				body: JSON.stringify({ file_path: '/DATA/config.yaml', file_content: 'key: value' })
			})
		);
	});

	// ─── Error Handling ───────────────────────────────────────────────────

	it('throws ApiError on 404 not found', async () => {
		vi.stubGlobal('fetch', mockJson({ message: 'Path not found' }, 404));

		await expect(listDirectory('/nonexistent')).rejects.toMatchObject({
			status: 404,
			message: 'Path not found'
		});
	});

	it('throws ApiError on 403 permission denied', async () => {
		vi.stubGlobal('fetch', mockJson({ message: 'Permission denied' }, 403));

		await expect(listDirectory('/root/private')).rejects.toMatchObject({
			status: 403
		});
	});
});
