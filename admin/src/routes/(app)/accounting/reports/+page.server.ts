import { redirect } from '@sveltejs/kit';
import { apiRequest } from '$lib/server/api';
import type { PnlResponse, CashFlowResponse } from '$lib/types/api';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ locals, cookies, url }) => {
	const user = locals.user!;
	if (user.role !== 'OWNER') {
		redirect(302, '/');
	}

	const accessToken = cookies.get('access_token')!;
	const startDate = url.searchParams.get('start_date') || '';
	const endDate = url.searchParams.get('end_date') || '';

	// Build query string
	const params = new URLSearchParams();
	if (startDate) params.set('start_date', startDate);
	if (endDate) params.set('end_date', endDate);
	const qs = params.toString() ? `?${params.toString()}` : '';

	// Load both P&L and Cash Flow in parallel
	const [pnlResult, cashFlowResult] = await Promise.all([
		apiRequest<PnlResponse>(`/accounting/reports/pnl${qs}`, { accessToken }),
		apiRequest<CashFlowResponse>(`/accounting/reports/cashflow${qs}`, { accessToken })
	]);

	return {
		pnl: pnlResult.ok ? pnlResult.data : { periods: [] },
		cashFlow: cashFlowResult.ok ? cashFlowResult.data : { periods: [] },
		startDate,
		endDate
	};
};
