/**
 * Protected layout guard — redirects unauthenticated users to /login.
 *
 * Passes the user object from locals (set by hooks.server.ts) to page data,
 * making it available to the (app) layout and all child pages.
 */

import { redirect } from '@sveltejs/kit';
import type { LayoutServerLoad } from './$types';

export const load: LayoutServerLoad = async ({ locals, url }) => {
	if (!locals.user) {
		const redirectParam = url.pathname === '/' ? '' : `?redirect=${encodeURIComponent(url.pathname)}`;
		redirect(302, `/login${redirectParam}`);
	}

	return {
		user: locals.user
	};
};
