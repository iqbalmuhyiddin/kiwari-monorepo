/**
 * Login page server — form action that authenticates against the Go API.
 *
 * On success: sets httpOnly cookies for access_token and refresh_token,
 * then redirects to the dashboard.
 *
 * On failure: returns the error message for the login form to display.
 */

import { fail, redirect } from '@sveltejs/kit';
import { apiRequest } from '$lib/server/api';
import { setAuthCookies } from '$lib/server/cookies';
import type { LoginResponse } from '$lib/types/api';
import type { Actions, PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ locals }) => {
	// If already authenticated, redirect to dashboard
	if (locals.user) {
		redirect(302, '/');
	}
};

export const actions: Actions = {
	default: async ({ request, cookies, url }) => {
		const formData = await request.formData();
		const email = formData.get('email')?.toString().trim() ?? '';
		const password = formData.get('password')?.toString() ?? '';

		// Basic client-side validation
		if (!email) {
			return fail(400, { email, error: 'Email is required' });
		}
		if (!password) {
			return fail(400, { email, error: 'Password is required' });
		}

		// Call the Go API
		const result = await apiRequest<LoginResponse>('/auth/login', {
			method: 'POST',
			body: { email, password }
		});

		if (!result.ok) {
			// Pass the API error message back to the form
			const message = result.status === 0
				? 'Cannot reach the server. Please try again.'
				: result.message;
			return fail(result.status || 400, { email, error: message });
		}

		const { access_token, refresh_token, user } = result.data;
		const isSecure = url.protocol === 'https:';

		// Set all auth cookies using shared utility
		setAuthCookies(
			cookies,
			{ access_token, refresh_token },
			{ id: user.id, full_name: user.full_name, email: user.email },
			isSecure
		);

		// CRITICAL FIX: Prevent open redirect vulnerability
		const redirectTo = url.searchParams.get('redirect') ?? '/';
		const safeRedirect = redirectTo.startsWith('/') && !redirectTo.startsWith('//') ? redirectTo : '/';
		redirect(303, safeRedirect);
	}
};
