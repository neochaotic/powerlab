import { browser, dev } from '$app/environment';
import { api, setAuthToken } from '$lib/api/client';
import { ENDPOINTS, type SuccessResponse } from '$lib/api/endpoints';

interface User {
	username: string;
	avatar?: string;
	id?: number;
}

interface LoginData {
	token: {
		access_token: string;
		refresh_token: string;
		expires_at: number;
	};
	user: User;
}

// Guard against environments where localStorage is missing or only mocked
// after module load (vitest 4 + jsdom can leave `localStorage` defined but
// without methods at module-eval time).
function _safeGet(key: string): string | null {
	try {
		if (typeof localStorage === 'undefined') return null;
		if (typeof localStorage.getItem !== 'function') return null;
		return localStorage.getItem(key);
	} catch { return null; }
}

const _initialUserRaw = browser ? _safeGet('powerlab_user') : null;

export const auth = $state({
	isAuthenticated: !!(browser && _safeGet('powerlab_token')),
	isInitialized: true,
	registrationKey: '',
	user: _initialUserRaw ? JSON.parse(_initialUserRaw) : null as User | null,
	isLoading: false,

	async checkStatus() {
		try {
			const response = await api.get<SuccessResponse<{ initialized: boolean; key?: string }>>(ENDPOINTS.USER_STATUS);
			this.isInitialized = response.data.initialized;
			this.registrationKey = response.data.key || '';
			return this.isInitialized;
		} catch (error) {
			console.error('Failed to check initialization status:', error);
			return true;
		}
	},

	async register(username: string, password: string) {
		// Race-fix: SetupWizard may submit before checkStatus() has resolved.
		// In that case registrationKey is empty — fetch it on-demand instead
		// of failing silently.
		if (!this.registrationKey) {
			await this.checkStatus();
		}
		if (!this.registrationKey) {
			console.error('No registration key available — system may already be initialized');
			return false;
		}

		this.isLoading = true;
		try {
			await api.post(ENDPOINTS.USER_REGISTER, {
				username,
				password,
				key: this.registrationKey
			});

			// After registration, immediately log in. The wizard only cares
			// whether registration+login succeeded — boolean is enough.
			return (await this.login(username, password)) === 'ok';
		} catch (error) {
			console.error('Registration failed:', error);
			return false;
		} finally {
			this.isLoading = false;
		}
	},
	
	/**
	 * Returns 'ok' on success, 'invalid' for bad credentials, or 'offline'
	 * when the backend is unreachable. The two failure modes are visually
	 * distinct in the UI so the user knows whether to retype or wait.
	 */
	async login(username: string, password: string): Promise<'ok' | 'invalid' | 'offline'> {
		this.isLoading = true;
		try {
			const response = await api.post<SuccessResponse<LoginData>>(ENDPOINTS.USER_LOGIN, {
				username,
				password
			});

			const { token, user } = response.data;

			this.isAuthenticated = true;
			this.user = user;

			if (browser) {
				localStorage.setItem('powerlab_token', token.access_token);
				localStorage.setItem('powerlab_user', JSON.stringify(user));

				if (dev && localStorage.getItem('powerlab_dev_autologin') === 'true') {
					localStorage.setItem('powerlab_dev_creds', JSON.stringify({ username, password }));
				}
			}

			setAuthToken(token.access_token);
			return 'ok';
		} catch (error: unknown) {
			console.error('Login failed:', error);
			// ApiError shape from lib/api/client.ts: { status, message, raw }.
			// status === 0 means "network/transport failed" — backend unreachable.
			// Any 4xx means the request reached the server and was rejected.
			const apiErr = error as { status?: number };
			if (apiErr?.status === 0) return 'offline';
			return 'invalid';
		} finally {
			this.isLoading = false;
		}
	},

	logout() {
		this.isAuthenticated = false;
		this.user = null;
		setAuthToken(null);
		if (browser) {
			localStorage.removeItem('powerlab_token');
			localStorage.removeItem('powerlab_user');
			// Clearing creds too — explicit logout means user wants out, even
			// from dev autologin. Toggling autologin back on requires logging
			// in once more.
			localStorage.removeItem('powerlab_dev_creds');
		}
	},

	async checkSession() {
		if (!browser) return;

		await this.checkStatus();

		const savedToken = localStorage.getItem('powerlab_token');

		if (savedToken) {
			setAuthToken(savedToken);
			try {
				const response = await api.get<SuccessResponse<User>>(ENDPOINTS.USER_CURRENT);
				this.user = response.data;
				this.isAuthenticated = true;
				localStorage.setItem('powerlab_user', JSON.stringify(this.user));
				return;
			} catch (error) {
				console.error('Session verification failed:', error);
				this.logout();
			}
		}

		// Fallback: dev autologin. If enabled and credentials were saved on a
		// prior login, re-login silently. Avoids the friction of typing creds
		// every time the dev rebuilds and the JWT is cleared.
		if (dev && localStorage.getItem('powerlab_dev_autologin') === 'true') {
			const credsRaw = localStorage.getItem('powerlab_dev_creds');
			if (credsRaw) {
				try {
					const { username, password } = JSON.parse(credsRaw) as { username: string; password: string };
					await this.login(username, password);
					return;
				} catch (e) {
					console.warn('Dev autologin failed:', e);
					localStorage.removeItem('powerlab_dev_creds');
				}
			}
		}

		this.isAuthenticated = false;
		this.user = null;
	}
});
