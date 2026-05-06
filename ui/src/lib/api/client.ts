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

/**
 * Core fetch wrapper. All API calls go through here.
 */
async function request<T>(
	url: string,
	options: RequestInit = {},
	retries = 2
): Promise<T> {
	let config: RequestInit = {
		...options,
		headers: {
			'Content-Type': 'application/json',
			...(authToken ? { Authorization: authToken } : {}),
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
