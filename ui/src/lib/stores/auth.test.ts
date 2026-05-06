import { describe, it, expect, beforeEach, vi } from 'vitest';
import { auth } from './auth.svelte';
import { setAuthToken } from '$lib/api/client';

// Force robust localStorage mock
const mockStorage: Record<string, string> = {};
global.localStorage = {
	getItem: vi.fn((key: string) => mockStorage[key] || null),
	setItem: vi.fn((key: string, value: string) => { mockStorage[key] = value; }),
	removeItem: vi.fn((key: string) => { delete mockStorage[key]; }),
	clear: vi.fn(() => { for (const key in mockStorage) delete mockStorage[key]; }),
	length: 0,
	key: (i: number) => Object.keys(mockStorage)[i] || null
} as any;

// Helper to mock fetch with typed response
function mockFetch(data: any, ok = true, status = 200) {
	vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
		ok,
		status,
		headers: new Headers({ 'content-type': 'application/json' }),
		text: () => Promise.resolve(JSON.stringify(data))
	}));
}

describe('Auth Store', () => {
	beforeEach(() => {
		auth.logout();
		vi.clearAllMocks();
		if (typeof window !== 'undefined') {
			localStorage.clear();
		}
	});

	it('initial state is logged out and initialized', () => {
		expect(auth.isAuthenticated).toBe(false);
		expect(auth.isInitialized).toBe(true);
		expect(auth.user).toBeNull();
	});

	it('login success sets user and token', async () => {
		const mockResponse = {
			data: {
				token: { access_token: 'valid_token' },
				user: { username: 'admin', id: 1 }
			}
		};
		mockFetch(mockResponse);

		const result = await auth.login('admin', 'password123');

		expect(result).toBe('ok');
		expect(auth.isAuthenticated).toBe(true);
		expect(auth.user?.username).toBe('admin');
		expect(localStorage.getItem('powerlab_token')).toBe('valid_token');
	});

	it('login failure returns "invalid" and stays logged out', async () => {
		mockFetch({ message: 'Invalid password' }, false, 401);

		const result = await auth.login('admin', 'wrong');

		expect(result).toBe('invalid');
		expect(auth.isAuthenticated).toBe(false);
		expect(auth.user).toBeNull();
	});

	it('checkStatus updates isInitialized and captures registration key', async () => {
		mockFetch({ data: { initialized: false, key: 'reg_key_abc' } });

		await auth.checkStatus();

		expect(auth.isInitialized).toBe(false);
		expect(auth.registrationKey).toBe('reg_key_abc');
	});

	it('register success performs registration and auto-login', async () => {
		// 1. Mock status to get key
		mockFetch({ data: { initialized: false, key: 'setup_key' } });
		await auth.checkStatus();

		// 2. Mock register success followed by login success
		const mockLoginResponse = {
			data: {
				token: { access_token: 'new_token' },
				user: { username: 'new_admin', id: 1 }
			}
		};

		// Mock the fetch sequence: register (OK), then login (OK)
		const fetchMock = vi.fn()
			.mockResolvedValueOnce({
				ok: true,
				status: 200,
				headers: new Headers({ 'content-type': 'application/json' }),
				text: () => Promise.resolve(JSON.stringify({ message: 'registered' }))
			})
			.mockResolvedValueOnce({
				ok: true,
				status: 200,
				headers: new Headers({ 'content-type': 'application/json' }),
				text: () => Promise.resolve(JSON.stringify(mockLoginResponse))
			});
		
		vi.stubGlobal('fetch', fetchMock);

		const success = await auth.register('new_admin', 'pass123');

		expect(success).toBe(true);
		expect(auth.isAuthenticated).toBe(true);
		expect(auth.user?.username).toBe('new_admin');
		expect(fetchMock).toHaveBeenCalledTimes(2);
	});

	it('logout clears state and storage', () => {
		localStorage.setItem('powerlab_token', 'test_token');
		auth.isAuthenticated = true;

		auth.logout();

		expect(auth.isAuthenticated).toBe(false);
		expect(auth.user).toBeNull();
		expect(localStorage.getItem('powerlab_token')).toBeNull();
	});
});
