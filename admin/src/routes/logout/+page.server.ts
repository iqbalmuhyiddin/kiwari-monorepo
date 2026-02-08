/**
 * Logout — clears auth cookies and redirects to login.
 *
 * This is a GET route (clicking "Logout" link in sidebar navigates here).
 * It clears cookies in the load function and redirects immediately.
 */

import { redirect } from '@sveltejs/kit';
import { clearAuthCookies } from '$lib/server/cookies';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ cookies }) => {
	clearAuthCookies(cookies);
	redirect(302, '/login');
};
