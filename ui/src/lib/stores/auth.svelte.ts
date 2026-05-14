import { browser, dev } from '$app/environment';
import { api, onAuthError, setAuthToken } from '$lib/api/client';
import { ENDPOINTS, type SuccessResponse } from '$lib/api/endpoints';
import { toast } from '$lib/stores/toast.svelte';

/**
 * RegisterResult is the discriminated outcome of `auth.register()`.
 * Each non-ok code maps 1:1 to a specific i18n key in SetupWizard
 * (issue #306 — the original generic "backend error" was confusing
 * users on legitimate validation failures).
 */
export type RegisterResult =
	| 'ok'             // register + auto-login both succeeded
	| 'passTooShort'   // backend PWD_IS_TOO_SIMPLE (10005)
	| 'keyExpired'     // backend KEY_NOT_EXIST (10010) — setup token gone, restart wizard
	| 'userExists'     // backend USER_EXIST (10007)
	| 'failed';        // anything else (network, 5xx, unknown 4xx)

// Backend response codes from backend/common/utils/common_err/e.go
const BACKEND_CODE_PWD_TOO_SIMPLE = 10005;
const BACKEND_CODE_KEY_NOT_EXIST = 10010;
const BACKEND_CODE_USER_EXIST = 10007;

/**
 * mapBackendCodeToRegisterResult inspects an error thrown by `api.post`
 * and extracts the backend `Success` code if present, mapping it to
 * a RegisterResult. The API client typically wraps the response body
 * in the error's `.data` or `.response` property — we probe both to
 * be defensive.
 */
function mapBackendCodeToRegisterResult(error: unknown): RegisterResult {
	if (error && typeof error === 'object') {
		// Try common error shapes: { data: { Success } }, { response: { data: { Success } } }
		const e = error as { data?: { Success?: number }; response?: { data?: { Success?: number } } };
		const code = e.data?.Success ?? e.response?.data?.Success;
		if (code === BACKEND_CODE_PWD_TOO_SIMPLE) return 'passTooShort';
		if (code === BACKEND_CODE_KEY_NOT_EXIST) return 'keyExpired';
		if (code === BACKEND_CODE_USER_EXIST) return 'userExists';
	}
	return 'failed';
}

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

// Rehydrate the JWT into the http client SYNCHRONOUSLY at module init.
// Without this, page-level onMount() handlers that fire requests (e.g.
// the launchpad's fetchInstalledApps) race against the layout's async
// checkSession() — the unauthenticated request hits the gateway, gets
// 401, the apps store stays {}, and the user sees "No apps installed"
// after a refresh even though the session is valid. checkSession()
// still runs to verify the token is live and refresh user info.
if (browser) {
	const _savedToken = _safeGet('powerlab_token');
	if (_savedToken) setAuthToken(_savedToken);
}

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

	/**
	 * Register a new user. Returns a discriminated result code so the
	 * SetupWizard can show a specific error message instead of the
	 * generic "Failed to initialize" — issue #306.
	 *
	 * Backend codes mapped (see backend/common/utils/common_err/e.go):
	 *   10005 PWD_IS_TOO_SIMPLE  → 'passTooShort'
	 *   10010 KEY_NOT_EXIST      → 'keyExpired' (setup token gone — restart wizard)
	 *   10007 USER_EXIST         → 'userExists'
	 *   anything else            → 'failed' (generic — backend logs)
	 *
	 * 'ok' means register + auto-login both succeeded.
	 */
	async register(username: string, password: string): Promise<RegisterResult> {
		// Race-fix: SetupWizard may submit before checkStatus() has resolved.
		// In that case registrationKey is empty — fetch it on-demand instead
		// of failing silently.
		if (!this.registrationKey) {
			await this.checkStatus();
		}
		if (!this.registrationKey) {
			console.error('No registration key available — system may already be initialized');
			return 'keyExpired';
		}

		this.isLoading = true;
		try {
			await api.post(ENDPOINTS.USER_REGISTER, {
				username,
				password,
				key: this.registrationKey
			});

			// After registration, immediately log in. Either success or
			// fall through to a generic 'failed' (the wizard treats
			// login failure right after register as "weird state").
			return (await this.login(username, password)) === 'ok' ? 'ok' : 'failed';
		} catch (error) {
			console.error('Registration failed:', error);
			return mapBackendCodeToRegisterResult(error);
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

// Wire the centralised 401 handler. Fires once per 401 from any
// api call, regardless of which store/component made the request.
// Logs the user out, surfaces a single clear toast, and lets
// +layout.svelte's auth-gate naturally show LoginScreen because
// auth.isAuthenticated flips to false.
//
// Without this, an expired JWT in localStorage produced opaque
// error messages from individual callers ("Could not reach the
// release manifest: Unauthorized") with no hint about the real
// cause. Hit live during v0.6.11 testing — Sprint 16 C3.
//
// Guard with a flag so the toast only fires once even if multiple
// in-flight requests all 401 at the same time (typical when a
// session expires and the dashboard's parallel polls all fail
// together).
let authErrorToastInflight = false;
onAuthError(() => {
	if (!auth.isAuthenticated) return; // already logged out — no-op
	if (authErrorToastInflight) return;
	authErrorToastInflight = true;
	try {
		auth.logout();
		toast.warning('Session expired — please sign in again');
	} finally {
		// Reset on next tick so a fresh session can fire again.
		setTimeout(() => {
			authErrorToastInflight = false;
		}, 0);
	}
});
