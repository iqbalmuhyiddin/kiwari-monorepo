/**
 * Auth store — client-side reactive user state.
 *
 * Tokens are stored as httpOnly cookies (set by the server).
 * This store only holds the user object for client-side components
 * (sidebar, role checks, etc.). It is populated from page data
 * returned by the (app) layout server load.
 *
 * No localStorage. No token access from JavaScript.
 */

import type { SessionUser } from '$lib/types/api';

// Re-export SessionUser as User for convenience
export type { SessionUser as User } from '$lib/types/api';

function createAuthStore() {
	let user = $state<SessionUser | null>(null);

	return {
		get user() {
			return user;
		},
		get isAuthenticated() {
			return user !== null;
		},

		/** Called from the (app) layout to sync server-provided user data */
		setUser(newUser: SessionUser | null) {
			user = newUser;
		}
	};
}

export const auth = createAuthStore();
