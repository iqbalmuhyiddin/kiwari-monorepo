/**
 * Server-side API helper for calling the Go backend.
 *
 * Reads the access_token from cookies and forwards it as a Bearer header.
 * This keeps tokens out of client-side JavaScript entirely.
 *
 * The Go API has NO version prefix — routes are /auth/login, /outlets/{oid}/products, etc.
 */

import { env } from '$env/dynamic/private';

const API_BASE_URL = env.API_URL ?? 'http://localhost:8081';

export interface ApiResponse<T> {
	ok: true;
	data: T;
}

export interface ApiErrorResponse {
	ok: false;
	status: number;
	message: string;
}

export type ApiResult<T> = ApiResponse<T> | ApiErrorResponse;

/**
 * Make an authenticated request to the Go API.
 * Pass the access_token from cookies for authenticated endpoints.
 */
export async function apiRequest<T>(
	path: string,
	options: {
		method?: string;
		body?: unknown;
		accessToken?: string;
	} = {}
): Promise<ApiResult<T>> {
	const { method = 'GET', body, accessToken } = options;
	const url = `${API_BASE_URL}${path}`;

	const headers: Record<string, string> = {};

	if (body !== undefined) {
		headers['Content-Type'] = 'application/json';
	}

	if (accessToken) {
		headers['Authorization'] = `Bearer ${accessToken}`;
	}

	try {
		const response = await fetch(url, {
			method,
			headers,
			body: body !== undefined ? JSON.stringify(body) : undefined
		});

		if (!response.ok) {
			let message = response.statusText;
			try {
				const errorBody = await response.json();
				message = errorBody.error ?? errorBody.message ?? message;
			} catch {
				// Response body wasn't JSON — use statusText
			}
			return { ok: false, status: response.status, message };
		}

		if (response.status === 204) {
			return { ok: true, data: undefined as unknown as T };
		}

		const data = (await response.json()) as T;
		return { ok: true, data };
	} catch (err) {
		// Network error (Go API unreachable, etc.)
		const message = err instanceof Error ? err.message : 'Network error';
		return { ok: false, status: 0, message };
	}
}
