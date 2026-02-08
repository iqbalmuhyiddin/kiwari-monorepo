/**
 * Shared cookie management utilities.
 *
 * Purpose: Extract cookie configuration to a single source of truth,
 * eliminating duplication across hooks.server.ts, login/+page.server.ts,
 * and logout/+page.server.ts.
 */

import type { Cookies } from '@sveltejs/kit';

const COOKIE_DEFAULTS = {
	path: '/',
	httpOnly: true,
	sameSite: 'lax' as const
};

export function setAuthCookies(
	cookies: Cookies,
	tokens: { access_token: string; refresh_token: string },
	userInfo: { id: string; full_name: string; email: string },
	secure: boolean
) {
	cookies.set('access_token', tokens.access_token, {
		...COOKIE_DEFAULTS,
		secure,
		maxAge: 60 * 60 * 24 // 1 day
	});
	cookies.set('refresh_token', tokens.refresh_token, {
		...COOKIE_DEFAULTS,
		secure,
		maxAge: 60 * 60 * 24 * 7 // 7 days
	});
	cookies.set('user_info', JSON.stringify(userInfo), {
		...COOKIE_DEFAULTS,
		secure,
		maxAge: 60 * 60 * 24 * 7 // 7 days
	});
}

export function clearAuthCookies(cookies: Cookies) {
	cookies.delete('access_token', { path: '/' });
	cookies.delete('refresh_token', { path: '/' });
	cookies.delete('user_info', { path: '/' });
}
