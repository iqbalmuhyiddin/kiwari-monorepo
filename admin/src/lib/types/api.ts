/**
 * Shared API types used across server and client code.
 *
 * Purpose: Eliminate duplication of LoginResponse and User types
 * across hooks.server.ts, login/+page.server.ts, auth store, and components.
 */

export type UserRole = 'OWNER' | 'ADMIN' | 'CASHIER' | 'KITCHEN';

export interface SessionUser {
	id: string;
	outlet_id: string;
	full_name: string;
	email: string;
	role: UserRole;
}

export interface LoginResponse {
	access_token: string;
	refresh_token: string;
	user: {
		id: string;
		outlet_id: string;
		full_name: string;
		email: string;
		role: string;
	};
}
