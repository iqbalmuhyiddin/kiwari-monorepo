/**
 * Order detail endpoint — proxies GET request for a single order to Go API.
 *
 * Called by the orders page to load full order detail (items, payments)
 * when a user clicks an order row. Keeps JWT server-side.
 *
 * Also resolves product names by fetching the outlet's product list,
 * since the Go API only returns product_id on order items (not names).
 */

import { json } from '@sveltejs/kit';
import { apiRequest } from '$lib/server/api';
import type { Order, Product } from '$lib/types/api';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = async ({ locals, cookies, params }) => {
	const user = locals.user;
	if (!user) {
		return json({ error: 'Unauthorized' }, { status: 401 });
	}

	const accessToken = cookies.get('access_token');
	if (!accessToken) {
		return json({ error: 'Unauthorized' }, { status: 401 });
	}

	// Fetch order detail and products list in parallel
	const [orderResult, productsResult] = await Promise.all([
		apiRequest<Order>(
			`/outlets/${user.outlet_id}/orders/${params.id}`,
			{ accessToken }
		),
		apiRequest<Product[]>(
			`/outlets/${user.outlet_id}/products`,
			{ accessToken }
		)
	]);

	if (!orderResult.ok) {
		return json({ error: orderResult.message }, { status: orderResult.status || 502 });
	}

	const order = orderResult.data;

	// Build product name lookup map and enrich order items
	if (order.items && productsResult.ok) {
		const productMap = new Map<string, string>();
		for (const product of productsResult.data) {
			productMap.set(product.id, product.name);
		}

		for (const item of order.items) {
			item.product_name = productMap.get(item.product_id) ?? undefined;
		}
	}

	// Fetch customer details if customer_id exists
	if (order.customer_id) {
		const customerResult = await apiRequest<{ id: string; full_name: string; phone: string }>(
			`/outlets/${user.outlet_id}/customers/${order.customer_id}`,
			{ accessToken }
		);

		if (customerResult.ok) {
			order.customer_name = customerResult.data.full_name;
			order.customer_phone = customerResult.data.phone;
		}
	}

	return json(order, {
		headers: { 'Cache-Control': 'no-store' }
	});
};
