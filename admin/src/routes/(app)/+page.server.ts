/**
 * Dashboard server load — fetches today's KPI data from the Go API.
 *
 * Loads daily sales, hourly sales, payment summary, and active orders
 * in parallel. Uses Asia/Jakarta timezone to determine "today".
 */

import { apiRequest } from '$lib/server/api';
import type {
	DailySales,
	HourlySales,
	PaymentSummary,
	OrderListResponse
} from '$lib/types/api';
import type { PageServerLoad } from './$types';

function getTodayJakarta(): string {
	return new Date().toLocaleDateString('sv-SE', { timeZone: 'Asia/Jakarta' });
}

export const load: PageServerLoad = async ({ locals, cookies }) => {
	// Guaranteed non-null by (app)/+layout.server.ts auth guard
	const user = locals.user!;
	const accessToken = cookies.get('access_token')!;
	const today = getTodayJakarta();
	const oid = user.outlet_id;

	const [
		dailySalesResult,
		hourlySalesResult,
		paymentSummaryResult,
		newOrdersResult,
		preparingOrdersResult
	] = await Promise.all([
		apiRequest<DailySales[]>(
			`/outlets/${oid}/reports/daily-sales?start_date=${today}&end_date=${today}`,
			{ accessToken }
		),
		apiRequest<HourlySales[]>(
			`/outlets/${oid}/reports/hourly-sales?start_date=${today}&end_date=${today}`,
			{ accessToken }
		),
		apiRequest<PaymentSummary[]>(
			`/outlets/${oid}/reports/payment-summary?start_date=${today}&end_date=${today}`,
			{ accessToken }
		),
		apiRequest<OrderListResponse>(`/outlets/${oid}/orders?status=NEW&limit=20`, {
			accessToken
		}),
		apiRequest<OrderListResponse>(`/outlets/${oid}/orders?status=PREPARING&limit=20`, {
			accessToken
		})
	]);

	const activeOrders = [
		...(newOrdersResult.ok ? newOrdersResult.data.orders : []),
		...(preparingOrdersResult.ok ? preparingOrdersResult.data.orders : [])
	];

	return {
		today,
		dailySales: dailySalesResult.ok ? dailySalesResult.data : [],
		hourlySales: hourlySalesResult.ok ? hourlySalesResult.data : [],
		paymentSummary: paymentSummaryResult.ok ? paymentSummaryResult.data : [],
		activeOrders
	};
};
