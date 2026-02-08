/**
 * Reports page server load — fetches sales, product, and payment reports.
 *
 * Date range defaults to the last 30 days.
 * Outlet comparison data is only fetched for OWNER role.
 */

import { apiRequest } from '$lib/server/api';
import type {
	DailySales,
	ProductSales,
	PaymentSummary,
	OutletComparison
} from '$lib/types/api';
import type { PageServerLoad } from './$types';

function getTodayJakarta(): string {
	return new Date().toLocaleDateString('sv-SE', { timeZone: 'Asia/Jakarta' });
}

function getThirtyDaysAgoJakarta(): string {
	const d = new Date();
	d.setDate(d.getDate() - 30);
	return d.toLocaleDateString('sv-SE', { timeZone: 'Asia/Jakarta' });
}

export const load: PageServerLoad = async ({ locals, cookies, url }) => {
	const user = locals.user!;
	const accessToken = cookies.get('access_token')!;
	const oid = user.outlet_id;

	// Date range from URL params with defaults (last 30 days)
	const startDate = url.searchParams.get('start_date') ?? getThirtyDaysAgoJakarta();
	const endDate = url.searchParams.get('end_date') ?? getTodayJakarta();

	// Parallel API calls for the three outlet-scoped reports
	const [dailySalesResult, productSalesResult, paymentResult] = await Promise.all([
		apiRequest<DailySales[]>(
			`/outlets/${oid}/reports/daily-sales?start_date=${startDate}&end_date=${endDate}`,
			{ accessToken }
		),
		apiRequest<ProductSales[]>(
			`/outlets/${oid}/reports/product-sales?start_date=${startDate}&end_date=${endDate}&limit=50`,
			{ accessToken }
		),
		apiRequest<PaymentSummary[]>(
			`/outlets/${oid}/reports/payment-summary?start_date=${startDate}&end_date=${endDate}`,
			{ accessToken }
		)
	]);

	// Outlet comparison only for OWNER — uses root endpoint (no outlet_id prefix)
	let outletComparison: OutletComparison[] = [];
	if (user.role === 'OWNER') {
		const result = await apiRequest<OutletComparison[]>(
			`/reports/outlet-comparison?start_date=${startDate}&end_date=${endDate}`,
			{ accessToken }
		);
		if (result.ok) outletComparison = result.data;
	}

	return {
		dailySales: dailySalesResult.ok ? dailySalesResult.data : [],
		productSales: productSalesResult.ok ? productSalesResult.data : [],
		paymentSummary: paymentResult.ok ? paymentResult.data : [],
		outletComparison,
		startDate,
		endDate,
		userRole: user.role
	};
};
