import { redirect } from '@sveltejs/kit';
import { apiRequest } from '$lib/server/api';
import type { DashboardResponse } from '$lib/types/api';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ locals, cookies }) => {
	const user = locals.user!;
	if (user.role !== 'OWNER') {
		redirect(302, '/');
	}

	const accessToken = cookies.get('access_token')!;
	const result = await apiRequest<DashboardResponse>('/accounting/dashboard', { accessToken });

	return {
		dashboard: result.ok
			? result.data
			: {
					cash_balances: [],
					monthly_pnl: {
						period: '',
						net_sales: '0',
						cogs: '0',
						gross_profit: '0',
						total_expenses: '0',
						net_profit: '0'
					},
					pending_reimbursements: { count: 0, total_amount: '0' },
					recent_transactions: []
				}
	};
};
