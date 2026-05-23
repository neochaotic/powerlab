import { describe, it, expect, vi, beforeEach } from 'vitest';

// Mock the API module the store imports.
vi.mock('$lib/api/files', () => ({
	listDirectory: vi.fn()
}));

import { listDirectory } from '$lib/api/files';
import { useFileStore } from './files.svelte';

describe('files store — scope-aware fallback (#36)', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it('falls back to the scope root when a directory is outside the file scope', async () => {
		// Out-of-scope path 403s with the scope root in `raw.data`; the
		// scope root itself lists fine.
		vi.mocked(listDirectory).mockImplementation(async (path: string) => {
			if (path === '/etc') {
				throw { status: 403, message: 'path is outside the permitted file scope', raw: { data: '/DATA' } };
			}
			return { success: 0, data: { content: [{ name: 'AppData', path: '/DATA/AppData', is_dir: true }] } } as never;
		});

		const store = useFileStore();
		await store.fetchFiles('/etc');

		expect(store.currentPath).toBe('/DATA'); // redirected to the scope root
		expect(store.error).toBeNull(); // no broken "outside scope" screen
		expect(vi.mocked(listDirectory)).toHaveBeenCalledWith('/etc');
		expect(vi.mocked(listDirectory)).toHaveBeenCalledWith('/DATA');
	});

	it('surfaces a non-scope error normally (no retry loop)', async () => {
		vi.mocked(listDirectory).mockRejectedValue({ status: 500, message: 'boom' });
		const store = useFileStore();
		await store.fetchFiles('/DATA/x');
		expect(store.error).toBe('boom');
		// Only the one call — must not retry on a non-scope error.
		expect(vi.mocked(listDirectory)).toHaveBeenCalledTimes(1);
	});
});
