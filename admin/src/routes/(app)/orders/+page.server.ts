/**
 * Orders page server — loads order list with filters, handles status changes.
 *
 * Supports query params for filtering: status, type, start_date, end_date, search.
 * Form actions handle order status updates and cancellation.
 */

import { fail } from '@sveltejs/kit';
import { apiRequest } from '$lib/server/api';
import type { Order, FullOrderListResponse } from '$lib/types/api';
import type { Actions, PageServerLoad } from './$types';

const PAGE_SIZE = 20;

export const load: PageServerLoad = async ({ locals, cookies, url }) => {
	const user = locals.user!;
	const accessToken = cookies.get('access_token')!;
	const oid = user.outlet_id;

	// Read filter params from URL
	const status = url.searchParams.get('status') ?? '';
	const type = url.searchParams.get('type') ?? '';
	const startDate = url.searchParams.get('start_date') ?? '';
	const endDate = url.searchParams.get('end_date') ?? '';
	const search = url.searchParams.get('search') ?? '';
	const page = parseInt(url.searchParams.get('page') ?? '1', 10);
	const offset = (page - 1) * PAGE_SIZE;

	// Build query string
	const params = new URLSearchParams();
	params.set('limit', String(PAGE_SIZE));
	params.set('offset', String(offset));
	if (status) params.set('status', status);
	if (type) params.set('type', type);
	if (startDate) params.set('start_date', startDate);
	if (endDate) params.set('end_date', endDate);

	const ordersResult = await apiRequest<FullOrderListResponse>(
		`/outlets/${oid}/orders?${params.toString()}`,
		{ accessToken }
	);

	let orders: Order[] = ordersResult.ok ? ordersResult.data.orders : [];

	// Client-side search filter by order number (API may not support text search)
	if (search) {
		const q = search.toLowerCase();
		orders = orders.filter((o) => o.order_number.toLowerCase().includes(q));
	}

	return {
		orders,
		filters: { status, type, startDate, endDate, search },
		page,
		pageSize: PAGE_SIZE,
		hasMore: ordersResult.ok ? ordersResult.data.orders.length >= PAGE_SIZE : false
	};
};

export const actions: Actions = {
	updateStatus: async ({ request, locals, cookies }) => {
		const user = locals.user!;
		const accessToken = cookies.get('access_token')!;
		const oid = user.outlet_id;

		const formData = await request.formData();
		const orderId = formData.get('order_id')?.toString() ?? '';
		const newStatus = formData.get('status')?.toString() ?? '';

		if (!orderId || !newStatus) {
			return fail(400, { statusError: 'ID pesanan dan status wajib diisi' });
		}

		const result = await apiRequest<Order>(`/outlets/${oid}/orders/${orderId}/status`, {
			method: 'PATCH',
			body: { status: newStatus },
			accessToken
		});

		if (!result.ok) {
			return fail(result.status || 400, { statusError: result.message });
		}

		return { statusSuccess: true };
	},

	cancelOrder: async ({ request, locals, cookies }) => {
		const user = locals.user!;
		const accessToken = cookies.get('access_token')!;
		const oid = user.outlet_id;

		const formData = await request.formData();
		const orderId = formData.get('order_id')?.toString() ?? '';

		if (!orderId) {
			return fail(400, { statusError: 'ID pesanan wajib diisi' });
		}

		const result = await apiRequest<void>(`/outlets/${oid}/orders/${orderId}`, {
			method: 'DELETE',
			accessToken
		});

		if (!result.ok) {
			return fail(result.status || 400, { statusError: result.message });
		}

		return { statusSuccess: true };
	}
};
