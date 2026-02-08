/**
 * SvelteKit server hooks — runs on every request.
 *
 * Parses the JWT access_token from httpOnly cookies and populates
 * event.locals.user with the decoded payload. If the token is expired,
 * attempts a refresh using the refresh_token cookie.
 *
 * The Go API is the authority for token validation — we only decode
 * the JWT payload here (base64), never verify the signature.
 */

import type { Handle } from '@sveltejs/kit';
import { apiRequest } from '$lib/server/api';
import { setAuthCookies } from '$lib/server/cookies';
import type { LoginResponse, SessionUser, UserRole } from '$lib/types/api';

interface JwtPayload {
	user_id: string; // JWT uses "user_id", not "sub"
	role: string;
	outlet_id: string;
	exp: number;
}

/**
 * Decode a JWT payload without verification.
 * Returns null if parsing fails for any reason.
 */
function decodeJwtPayload(token: string): JwtPayload | null {
	try {
		const parts = token.split('.');
		if (parts.length !== 3) return null;

		// Base64url decode the payload (middle segment)
		const payload = parts[1]
			.replace(/-/g, '+')
			.replace(/_/g, '/');

		const decoded = atob(payload);
		return JSON.parse(decoded) as JwtPayload;
	} catch {
		return null;
	}
}

/**
 * Check if a JWT is expired (with 30s buffer to refresh proactively).
 */
function isExpired(payload: JwtPayload): boolean {
	const nowSeconds = Math.floor(Date.now() / 1000);
	return payload.exp < nowSeconds + 30;
}

export const handle: Handle = async ({ event, resolve }) => {
	const accessToken = event.cookies.get('access_token');
	const refreshToken = event.cookies.get('refresh_token');
	const isSecure = event.url.protocol === 'https:';

	event.locals.user = null;

	if (accessToken) {
		const payload = decodeJwtPayload(accessToken);

		if (payload && !isExpired(payload)) {
			// Token is valid and not expired — get supplementary user info from cookie
			let fullName = 'User'; // CRITICAL FIX: fallback to 'User' if missing
			let email = '';
			const userInfoStr = event.cookies.get('user_info');
			if (userInfoStr) {
				try {
					const info = JSON.parse(userInfoStr);
					fullName = info.full_name ?? 'User';
					email = info.email ?? '';
				} catch {
					// Ignore parse errors
				}
			}

			event.locals.user = {
				id: payload.user_id,
				outlet_id: payload.outlet_id,
				full_name: fullName,
				email: email,
				role: payload.role as UserRole
			};
		} else if (refreshToken) {
			// Token expired or unparseable — try refresh
			const result = await apiRequest<LoginResponse>('/auth/refresh', {
				method: 'POST',
				body: { refresh_token: refreshToken }
			});

			if (result.ok) {
				const { access_token, refresh_token, user } = result.data;

				// Set new cookies using shared utility
				setAuthCookies(
					event.cookies,
					{ access_token, refresh_token },
					{ id: user.id, full_name: user.full_name, email: user.email },
					isSecure
				);

				event.locals.user = {
					id: user.id,
					outlet_id: user.outlet_id,
					full_name: user.full_name,
					email: user.email,
					role: user.role as UserRole
				};
			} else {
				// Refresh failed — clear cookies
				event.cookies.delete('access_token', { path: '/' });
				event.cookies.delete('refresh_token', { path: '/' });
				event.cookies.delete('user_info', { path: '/' });
			}
		}
	}

	return resolve(event);
};
