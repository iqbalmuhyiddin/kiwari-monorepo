/**
 * Customer detail page server — loads customer, stats, and order history.
 *
 * Three parallel API calls: customer info, customer stats, customer orders.
 * Form actions handle update and delete (soft).
 */

import { fail, redirect } from '@sveltejs/kit';
import { apiRequest } from '$lib/server/api';
import type { Customer, CustomerStats, Order } from '$lib/types/api';
import type { Actions, PageServerLoad } from './$types';

const ORDERS_PAGE_SIZE = 10;

export const load: PageServerLoad = async ({ locals, cookies, params, url }) => {
	const user = locals.user!;
	const accessToken = cookies.get('access_token')!;
	const oid = user.outlet_id;
	const customerId = params.id;

	const ordersPage = parseInt(url.searchParams.get('orders_page') ?? '1', 10);
	const ordersOffset = (ordersPage - 1) * ORDERS_PAGE_SIZE;

	const [customerResult, statsResult, ordersResult] = await Promise.all([
		apiRequest<Customer>(`/outlets/${oid}/customers/${customerId}`, { accessToken }),
		apiRequest<CustomerStats>(`/outlets/${oid}/customers/${customerId}/stats`, { accessToken }),
		apiRequest<Order[]>(
			`/outlets/${oid}/customers/${customerId}/orders?limit=${ORDERS_PAGE_SIZE}&offset=${ordersOffset}`,
			{ accessToken }
		)
	]);

	if (!customerResult.ok) {
		throw redirect(302, '/customers');
	}

	return {
		customer: customerResult.data,
		stats: statsResult.ok ? statsResult.data : null,
		orders: ordersResult.ok ? ordersResult.data : [],
		ordersPage,
		ordersHasMore: ordersResult.ok ? ordersResult.data.length >= ORDERS_PAGE_SIZE : false
	};
};

export const actions: Actions = {
	update: async ({ request, locals, cookies, params }) => {
		const user = locals.user!;
		const accessToken = cookies.get('access_token')!;
		const oid = user.outlet_id;
		const customerId = params.id;

		const formData = await request.formData();
		const name = formData.get('name')?.toString()?.trim() ?? '';
		const phone = formData.get('phone')?.toString()?.trim() ?? '';
		const email = formData.get('email')?.toString()?.trim() || null;
		const notes = formData.get('notes')?.toString()?.trim() || null;

		if (!name || !phone) {
			return fail(400, { updateError: 'Nama dan nomor HP wajib diisi' });
		}

		const result = await apiRequest<Customer>(`/outlets/${oid}/customers/${customerId}`, {
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

	delete: async ({ locals, cookies, params }) => {
		const user = locals.user!;
		const accessToken = cookies.get('access_token')!;
		const oid = user.outlet_id;
		const customerId = params.id;

		const result = await apiRequest<void>(`/outlets/${oid}/customers/${customerId}`, {
			method: 'DELETE',
			accessToken
		});

		if (!result.ok) {
			return fail(result.status || 400, { deleteError: result.message });
		}

		throw redirect(302, '/customers');
	}
};
