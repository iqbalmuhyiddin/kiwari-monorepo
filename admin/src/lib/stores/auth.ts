/**
 * Auth store — manages user session state (token + user info).
 *
 * Uses Svelte 5 runes ($state) for reactivity.
 * Token is persisted to localStorage on the client side.
 */

import { browser } from '$app/environment';

export interface User {
	id: string;
	email: string;
	name: string;
	role: string;
	outlet_id: string;
}

interface AuthState {
	token: string | null;
	user: User | null;
}

function createAuthStore() {
	let state = $state<AuthState>({
		token: null,
		user: null
	});

	// Hydrate from localStorage on init
	if (browser) {
		const savedToken = localStorage.getItem('auth_token');
		const savedUser = localStorage.getItem('auth_user');
		if (savedToken && savedUser) {
			try {
				state.token = savedToken;
				state.user = JSON.parse(savedUser);
			} catch {
				// Corrupted stored data — clear it
				localStorage.removeItem('auth_token');
				localStorage.removeItem('auth_user');
			}
		}
	}

	return {
		get token() {
			return state.token;
		},
		get user() {
			return state.user;
		},
		get isAuthenticated() {
			return state.token !== null;
		},

		login(token: string, user: User) {
			state.token = token;
			state.user = user;
			if (browser) {
				localStorage.setItem('auth_token', token);
				localStorage.setItem('auth_user', JSON.stringify(user));
			}
		},

		logout() {
			state.token = null;
			state.user = null;
			if (browser) {
				localStorage.removeItem('auth_token');
				localStorage.removeItem('auth_user');
			}
		}
	};
}

export const auth = createAuthStore();
