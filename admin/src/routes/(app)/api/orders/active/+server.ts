/**
 * Polling endpoint for active orders.
 *
 * The client-side LiveOrders component calls this every 10 seconds.
 * This keeps the JWT server-side (httpOnly cookie) and proxies the
 * request to the Go API.
 */

import { json } from '@sveltejs/kit';
import { apiRequest } from '$lib/server/api';
import type { OrderListResponse } from '$lib/types/api';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = async ({ locals, cookies }) => {
	const user = locals.user;
	if (!user) {
		return json({ error: 'Unauthorized' }, { status: 401 });
	}

	const accessToken = cookies.get('access_token');
	if (!accessToken) {
		return json({ error: 'Unauthorized' }, { status: 401 });
	}

	const [newOrdersResult, preparingOrdersResult] = await Promise.all([
		apiRequest<OrderListResponse>(`/outlets/${user.outlet_id}/orders?status=NEW&limit=20`, {
			accessToken
		}),
		apiRequest<OrderListResponse>(`/outlets/${user.outlet_id}/orders?status=PREPARING&limit=20`, {
			accessToken
		})
	]);

	if (!newOrdersResult.ok) {
		return json({ error: newOrdersResult.message }, { status: newOrdersResult.status || 502 });
	}

	if (!preparingOrdersResult.ok) {
		return json(
			{ error: preparingOrdersResult.message },
			{ status: preparingOrdersResult.status || 502 }
		);
	}

	const activeOrders = [...newOrdersResult.data.orders, ...preparingOrdersResult.data.orders];

	return json(activeOrders, {
		headers: { 'Cache-Control': 'no-store' }
	});
};
