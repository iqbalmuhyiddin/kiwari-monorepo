/**
 * Customers page server — loads customer list with search/pagination, handles CRUD.
 *
 * Supports query params: search (phone/name), page (1-based).
 * Form actions handle create, update, and soft-delete.
 */

import { fail } from '@sveltejs/kit';
import { apiRequest } from '$lib/server/api';
import type { Customer } from '$lib/types/api';
import type { Actions, PageServerLoad } from './$types';

const PAGE_SIZE = 20;

export const load: PageServerLoad = async ({ locals, cookies, url }) => {
	const user = locals.user!;
	const accessToken = cookies.get('access_token')!;
	const oid = user.outlet_id;

	const search = url.searchParams.get('search') ?? '';
	const page = parseInt(url.searchParams.get('page') ?? '1', 10);
	const offset = (page - 1) * PAGE_SIZE;

	const result = await apiRequest<Customer[]>(
		`/outlets/${oid}/customers?limit=${PAGE_SIZE}&offset=${offset}&search=${encodeURIComponent(search)}`,
		{ accessToken }
	);

	return {
		customers: result.ok ? result.data : [],
		search,
		page,
		hasMore: result.ok ? result.data.length >= PAGE_SIZE : false
	};
};

export const actions: Actions = {
	create: async ({ request, locals, cookies }) => {
		const user = locals.user!;
		const accessToken = cookies.get('access_token')!;
		const oid = user.outlet_id;

		const formData = await request.formData();
		const name = formData.get('name')?.toString()?.trim() ?? '';
		const phone = formData.get('phone')?.toString()?.trim() ?? '';
		const email = formData.get('email')?.toString()?.trim() || null;
		const notes = formData.get('notes')?.toString()?.trim() || null;

		if (!name || !phone) {
			return fail(400, { createError: 'Nama dan nomor HP wajib diisi' });
		}

		const result = await apiRequest<Customer>(`/outlets/${oid}/customers`, {
			method: 'POST',
			body: { name, phone, email, notes },
			accessToken
		});

		if (!result.ok) {
			if (result.status === 409) return fail(409, { createError: 'Nomor HP sudah terdaftar' });
			return fail(result.status || 400, { createError: result.message });
		}

		return { createSuccess: true };
	},

	update: async ({ request, locals, cookies }) => {
		const user = locals.user!;
		const accessToken = cookies.get('access_token')!;
		const oid = user.outlet_id;

		const formData = await request.formData();
		const id = formData.get('id')?.toString() ?? '';
		const name = formData.get('name')?.toString()?.trim() ?? '';
		const phone = formData.get('phone')?.toString()?.trim() ?? '';
		const email = formData.get('email')?.toString()?.trim() || null;
		const notes = formData.get('notes')?.toString()?.trim() || null;

		if (!id || !name || !phone) {
			return fail(400, { updateError: 'Nama dan nomor HP wajib diisi' });
		}

		const result = await apiRequest<Customer>(`/outlets/${oid}/customers/${id}`, {
			method: 'PUT',
			body: { name, phone, email, notes },
			accessToken
		});

		if (!result.ok) {
			if (result.status === 409) return fail(409, { updateError: 'Nomor HP sudah terdaftar' });
			return fail(result.status || 400, { updateError: result.message });
		}

		return { updateSuccess: true };
	},

	delete: async ({ request, locals, cookies }) => {
		const user = locals.user!;
		const accessToken = cookies.get('access_token')!;
		const oid = user.outlet_id;

		const formData = await request.formData();
		const id = formData.get('id')?.toString() ?? '';

		if (!id) {
			return fail(400, { deleteError: 'ID pelanggan tidak valid' });
		}

		const result = await apiRequest<void>(`/outlets/${oid}/customers/${id}`, {
			method: 'DELETE',
			accessToken
		});

		if (!result.ok) {
			return fail(result.status || 400, { deleteError: result.message });
		}

		return { deleteSuccess: true };
	}
};
