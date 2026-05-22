/**
 * PowerLab HTTP Client
 *
 * Thin wrapper around native `fetch` for consuming the CasaOS Go REST API.
 * Zero business logic — just builds requests, sends them, and returns typed responses.
 *
 * Features:
 * - JWT token injection via Authorization header
 * - Automatic JSON parsing
 * - Typed error handling
 * - Request/response interceptors
 * - Retry logic for transient failures
 */

export interface ApiError {
	status: number;
	message: string;
	raw?: unknown;
}

export interface ApiResponse<T> {
	data: T;
	message?: string;
}

type RequestInterceptor = (config: RequestInit) => RequestInit;
type ResponseInterceptor = (response: Response) => Response;

const requestInterceptors: RequestInterceptor[] = [];
const responseInterceptors: ResponseInterceptor[] = [];

let authToken: string | null = null;

/**
 * Set the JWT auth token for all subsequent requests.
 */
export function setAuthToken(token: string | null): void {
	authToken = token;
}

/**
 * Returns the current JWT auth token (or null if not signed in). Used by
 * the websocket clients (terminal, app log streaming) which can't use
 * the Authorization header — they have to push the token through the
 * URL query string because the WebSocket constructor doesn't accept
 * custom headers in browsers.
 */
export function getAuthToken(): string | null {
	return authToken;
}

/**
 * Register a request interceptor.
 */
export function addRequestInterceptor(interceptor: RequestInterceptor): void {
	requestInterceptors.push(interceptor);
}

/**
 * Register a response interceptor.
 */
export function addResponseInterceptor(interceptor: ResponseInterceptor): void {
	responseInterceptors.push(interceptor);
}

// ─── 401 Unauthorized signal (Sprint 16 C3) ──────────────────────────────────
// Centralised hook fired on every 401 response, so the auth store can
// log out + toast "Session expired" in one place instead of every
// caller catching ApiError and special-casing status===401. The
// trigger that motivated this: a user with an expired JWT in
// localStorage hitting the updater check and seeing
// "Could not reach the release manifest: Unauthorized" — the
// caller's catch path showed the wrapped error message and the
// user had no idea they just needed to re-login.

export interface AuthErrorInfo {
	status: 401;
	url: string;
	method: string;
	rawBody: unknown;
}

type AuthErrorHandler = (info: AuthErrorInfo) => void;

const authErrorHandlers: AuthErrorHandler[] = [];

/**
 * Register a callback that fires every time the api receives an HTTP
 * 401 from the backend. Returns an unsubscribe function. Callbacks
 * fire IN ADDITION TO the ApiError throw — the request still rejects
 * normally; this hook is for cross-cutting concerns (logout, toast,
 * redirect) that should happen regardless of which caller hit the 401.
 */
export function onAuthError(handler: AuthErrorHandler): () => void {
	authErrorHandlers.push(handler);
	return () => {
		const i = authErrorHandlers.indexOf(handler);
		if (i >= 0) authErrorHandlers.splice(i, 1);
	};
}

function emitAuthError(info: AuthErrorInfo): void {
	// Defensive: a handler throwing must not block the others, and
	// must not change the rejected ApiError seen by the caller.
	for (const h of authErrorHandlers) {
		try {
			h(info);
		} catch (e) {
			console.error('onAuthError handler threw:', e);
		}
	}
}

/**
 * Core fetch wrapper. All API calls go through here.
 */
async function request<T>(
	url: string,
	options: RequestInit = {},
	retries = 2
): Promise<T> {
	// FormData bodies (file uploads) MUST be sent with the browser's
	// auto-generated `multipart/form-data; boundary=...` Content-Type.
	// If we hardcode application/json here as the default, fetch sends
	// the multipart bytes with the wrong Content-Type and the server
	// rejects with "request Content-Type isn't multipart/form-data".
	// Skip the default for FormData and let the browser set it. JSON
	// callers still get application/json by default.
	const isFormData = typeof FormData !== 'undefined' && options.body instanceof FormData;
	const baseHeaders: Record<string, string> = {};
	if (!isFormData) {
		baseHeaders['Content-Type'] = 'application/json';
	}
	if (authToken) {
		baseHeaders['Authorization'] = authToken;
	}
	let config: RequestInit = {
		// Send the HttpOnly access_token cookie on same-origin requests
		// (#35) so auth no longer depends solely on the URL/header token.
		credentials: 'include',
		...options,
		headers: {
			...baseHeaders,
			...(options.headers || {})
		}
	};

	// Apply request interceptors
	for (const interceptor of requestInterceptors) {
		config = interceptor(config);
	}

	let response: Response;

	try {
		response = await fetch(url, config);
	} catch (err) {
		if (retries > 0) {
			await new Promise((r) => setTimeout(r, 1000));
			return request<T>(url, options, retries - 1);
		}
		throw {
			status: 0,
			message: 'Network error: unable to reach the server',
			raw: err
		} satisfies ApiError;
	}

	// Apply response interceptors
	for (const interceptor of responseInterceptors) {
		response = interceptor(response);
	}

	// Read the response text exactly once to prevent 'body stream already read' errors
	const responseText = await response.text();

	if (!response.ok) {
		let errorBody: unknown = responseText;
		try {
			if (responseText) {
				errorBody = JSON.parse(responseText);
			}
		} catch {
			// fallback to raw text if it's not JSON
		}

		// Fan out the centralised 401 signal BEFORE throwing so the
		// auth store can react (logout + toast) on the same event
		// that the caller will see as a rejected ApiError. See
		// onAuthError() above — Sprint 16 C3.
		if (response.status === 401) {
			emitAuthError({
				status: 401,
				url,
				method: (config.method as string) ?? 'GET',
				rawBody: errorBody
			});
		}

		throw {
			status: response.status,
			message:
				(errorBody as { message?: string })?.message ||
				`HTTP ${response.status}: ${response.statusText}`,
			raw: errorBody
		} satisfies ApiError;
	}

	// Handle empty responses (204, etc.)
	if (response.status === 204 || responseText.length === 0) {
		return undefined as T;
	}

	const contentType = response.headers.get('content-type') || '';
	if (contentType.includes('application/json')) {
		try {
			return JSON.parse(responseText) as T;
		} catch (e) {
			throw {
				status: 500,
				message: 'Invalid JSON response from server',
				raw: responseText
			} satisfies ApiError;
		}
	}

	return responseText as unknown as T;
}

// ─── Convenience Methods ──────────────────────────────────────────────

export const api = {
	get<T>(url: string, options?: RequestInit): Promise<T> {
		return request<T>(url, { ...options, method: 'GET' });
	},

	post<T>(url: string, body?: unknown, options?: RequestInit): Promise<T> {
		return request<T>(url, {
			...options,
			method: 'POST',
			body: body ? JSON.stringify(body) : undefined
		});
	},

	put<T>(url: string, body?: unknown, options?: RequestInit): Promise<T> {
		return request<T>(url, {
			...options,
			method: 'PUT',
			body: body ? JSON.stringify(body) : undefined
		});
	},

	patch<T>(url: string, body?: unknown, options?: RequestInit): Promise<T> {
		return request<T>(url, {
			...options,
			method: 'PATCH',
			body: body ? JSON.stringify(body) : undefined
		});
	},

	delete<T>(url: string, options?: RequestInit): Promise<T> {
		return request<T>(url, { ...options, method: 'DELETE' });
	},

	/**
	 * POST with YAML content type (for compose app installation).
	 */
	postYaml<T>(url: string, yamlBody: string, options?: RequestInit): Promise<T> {
		return request<T>(url, {
			...options,
			method: 'POST',
			headers: { 'Content-Type': 'application/yaml', ...(options?.headers || {}) },
			body: yamlBody
		});
	},

	/**
	 * PUT with YAML content type (for `applyComposeAppSettings` —
	 * the edit-and-redeploy path that needs the backend's skip-self
	 * port-conflict logic).
	 */
	putYaml<T>(url: string, yamlBody: string, options?: RequestInit): Promise<T> {
		return request<T>(url, {
			...options,
			method: 'PUT',
			headers: { 'Content-Type': 'application/yaml', ...(options?.headers || {}) },
			body: yamlBody
		});
	},

	/**
	 * Upload file with multipart/form-data (for chunked file uploads).
	 */
	upload<T>(url: string, formData: FormData, options?: RequestInit): Promise<T> {
		const headers = { ...(options?.headers || {}) } as Record<string, string>;
		// Let browser set Content-Type with boundary for multipart
		delete headers['Content-Type'];

		return request<T>(url, {
			...options,
			method: 'POST',
			headers,
			body: formData
		});
	}
} as const;
