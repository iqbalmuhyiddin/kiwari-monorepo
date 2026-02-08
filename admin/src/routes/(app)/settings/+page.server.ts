/**
 * Settings page server — loads user list, handles user CRUD.
 *
 * Only OWNER and MANAGER roles can access this page.
 * Form actions handle create, update, and soft-delete of users.
 */

import { fail, redirect } from '@sveltejs/kit';
import { apiRequest } from '$lib/server/api';
import type { AdminUser } from '$lib/types/api';
import type { Actions, PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ locals, cookies }) => {
	const user = locals.user!;

	// Role guard — only OWNER and MANAGER can access settings
	if (user.role !== 'OWNER' && user.role !== 'MANAGER') {
		redirect(302, '/');
	}

	const accessToken = cookies.get('access_token')!;
	const oid = user.outlet_id;

	const result = await apiRequest<AdminUser[]>(`/outlets/${oid}/users`, { accessToken });

	return {
		users: result.ok ? result.data : [],
		currentUser: user
	};
};

export const actions: Actions = {
	create: async ({ request, locals, cookies }) => {
		const user = locals.user!;
		if (user.role !== 'OWNER' && user.role !== 'MANAGER') {
			return fail(403, { createError: 'Akses ditolak' });
		}

		const accessToken = cookies.get('access_token')!;
		const oid = user.outlet_id;

		const formData = await request.formData();
		const full_name = formData.get('full_name')?.toString()?.trim() ?? '';
		const email = formData.get('email')?.toString()?.trim() ?? '';
		const password = formData.get('password')?.toString() ?? '';
		const role = formData.get('role')?.toString() ?? '';
		const pin = formData.get('pin')?.toString()?.trim() || undefined;

		if (!full_name || !email || !password || !role) {
			return fail(400, { createError: 'Nama, email, kata sandi, dan peran wajib diisi' });
		}

		if (!email.includes('@')) {
			return fail(400, { createError: 'Format email tidak valid' });
		}

		if (pin && (pin.length < 4 || pin.length > 6 || !/^\d+$/.test(pin))) {
			return fail(400, { createError: 'PIN harus 4-6 digit angka' });
		}

		const body: Record<string, string> = { full_name, email, password, role };
		if (pin) body.pin = pin;

		const result = await apiRequest<AdminUser>(`/outlets/${oid}/users`, {
			method: 'POST',
			body,
			accessToken
		});

		if (!result.ok) {
			if (result.status === 409) return fail(409, { createError: 'Email sudah terdaftar' });
			return fail(result.status || 400, { createError: result.message });
		}

		return { createSuccess: true };
	},

	update: async ({ request, locals, cookies }) => {
		const user = locals.user!;
		if (user.role !== 'OWNER' && user.role !== 'MANAGER') {
			return fail(403, { updateError: 'Akses ditolak' });
		}

		const accessToken = cookies.get('access_token')!;
		const oid = user.outlet_id;

		const formData = await request.formData();
		const id = formData.get('id')?.toString() ?? '';
		const full_name = formData.get('full_name')?.toString()?.trim() ?? '';
		const email = formData.get('email')?.toString()?.trim() ?? '';
		const role = formData.get('role')?.toString() ?? '';
		const pin = formData.get('pin')?.toString()?.trim() || undefined;

		if (!id || !full_name || !email || !role) {
			return fail(400, { updateError: 'Nama, email, dan peran wajib diisi' });
		}

		if (!email.includes('@')) {
			return fail(400, { updateError: 'Format email tidak valid' });
		}

		if (pin && (pin.length < 4 || pin.length > 6 || !/^\d+$/.test(pin))) {
			return fail(400, { updateError: 'PIN harus 4-6 digit angka' });
		}

		const body: Record<string, string> = { full_name, email, role };
		if (pin) body.pin = pin;

		const result = await apiRequest<AdminUser>(`/outlets/${oid}/users/${id}`, {
			method: 'PUT',
			body,
			accessToken
		});

		if (!result.ok) {
			if (result.status === 409) return fail(409, { updateError: 'Email sudah terdaftar' });
			return fail(result.status || 400, { updateError: result.message });
		}

		return { updateSuccess: true };
	},

	delete: async ({ request, locals, cookies }) => {
		const user = locals.user!;
		if (user.role !== 'OWNER' && user.role !== 'MANAGER') {
			return fail(403, { deleteError: 'Akses ditolak' });
		}

		const accessToken = cookies.get('access_token')!;
		const oid = user.outlet_id;

		const formData = await request.formData();
		const id = formData.get('id')?.toString() ?? '';

		if (!id) {
			return fail(400, { deleteError: 'ID pengguna tidak valid' });
		}

		// Self-protection: users cannot delete themselves
		if (id === user.id) {
			return fail(400, { deleteError: 'Tidak dapat menghapus akun sendiri' });
		}

		const result = await apiRequest<void>(`/outlets/${oid}/users/${id}`, {
			method: 'DELETE',
			accessToken
		});

		if (!result.ok) {
			return fail(result.status || 400, { deleteError: result.message });
		}

		return { deleteSuccess: true };
	}
};
