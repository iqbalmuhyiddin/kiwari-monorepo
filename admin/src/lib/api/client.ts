/**
 * API client for the Kiwari POS Go backend.
 *
 * Routes have NO version prefix — e.g. /auth/login, /outlets/{oid}/products.
 *
 * For authenticated requests in the admin panel, API calls should go through
 * SvelteKit server-side load functions and form actions (which use
 * $lib/server/api.ts). This client-side module can be used for any
 * unauthenticated calls or future client-side needs, but tokens are NOT
 * accessible from JavaScript (httpOnly cookies).
 */

const API_BASE_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:8081';

export interface ApiError {
	status: number;
	message: string;
}

export class ApiClientError extends Error {
	status: number;

	constructor(status: number, message: string) {
		super(message);
		this.name = 'ApiClientError';
		this.status = status;
	}
}

/**
 * Typed fetch wrapper. Automatically handles JSON serialization
 * and error responses.
 */
async function request<T>(
	path: string,
	options: RequestInit = {}
): Promise<T> {
	const url = `${API_BASE_URL}${path}`;

	const headers: Record<string, string> = {
		...((options.headers as Record<string, string>) ?? {})
	};

	// Set Content-Type only when body exists
	if (options.body) {
		headers['Content-Type'] = 'application/json';
	}

	const response = await fetch(url, {
		...options,
		headers
	});

	if (!response.ok) {
		let message = response.statusText;
		try {
			const body = await response.json();
			message = body.error ?? body.message ?? message;
		} catch {
			// Response body wasn't JSON — use statusText
		}
		throw new ApiClientError(response.status, message);
	}

	// 204 No Content — return undefined
	if (response.status === 204) {
		return undefined as unknown as T;
	}

	return response.json() as Promise<T>;
}

export const api = {
	get<T>(path: string): Promise<T> {
		return request<T>(path, { method: 'GET' });
	},

	post<T>(path: string, body?: unknown): Promise<T> {
		return request<T>(path, {
			method: 'POST',
			body: body != null ? JSON.stringify(body) : undefined
		});
	},

	put<T>(path: string, body?: unknown): Promise<T> {
		return request<T>(path, {
			method: 'PUT',
			body: body != null ? JSON.stringify(body) : undefined
		});
	},

	patch<T>(path: string, body?: unknown): Promise<T> {
		return request<T>(path, {
			method: 'PATCH',
			body: body != null ? JSON.stringify(body) : undefined
		});
	},

	delete(path: string): Promise<void> {
		return request<void>(path, { method: 'DELETE' });
	}
};
