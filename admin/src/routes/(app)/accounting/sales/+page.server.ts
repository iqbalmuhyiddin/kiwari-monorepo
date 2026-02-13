/**
 * Penjualan (Sales) page server — loads sales summaries, cash accounts, accounts.
 * Actions: create manual entry, update, delete, sync POS, post to ledger.
 */

import { fail, redirect } from '@sveltejs/kit';
import { apiRequest } from '$lib/server/api';
import type {
	AcctSalesDailySummary,
	AcctCashAccount,
	AcctAccount,
	POSSyncResponse
} from '$lib/types/api';
import type { PageServerLoad, Actions } from './$types';

export const load: PageServerLoad = async ({ locals, cookies, url }) => {
	const user = locals.user!;
	if (user.role !== 'OWNER') {
		redirect(302, '/');
	}

	const accessToken = cookies.get('access_token')!;

	// Default to current month
	const now = new Date();
	const startDate =
		url.searchParams.get('start_date') ||
		new Date(now.getFullYear(), now.getMonth(), 1).toISOString().slice(0, 10);
	const endDate =
		url.searchParams.get('end_date') ||
		new Date(now.getFullYear(), now.getMonth() + 1, 0).toISOString().slice(0, 10);

	const [summariesResult, cashAccountsResult, accountsResult] = await Promise.all([
		apiRequest<AcctSalesDailySummary[]>(
			`/accounting/sales?start_date=${startDate}&end_date=${endDate}&limit=500`,
			{ accessToken }
		),
		apiRequest<AcctCashAccount[]>('/accounting/master/cash-accounts', { accessToken }),
		apiRequest<AcctAccount[]>('/accounting/master/accounts', { accessToken })
	]);

	return {
		summaries: summariesResult.ok ? summariesResult.data : [],
		cashAccounts: cashAccountsResult.ok ? cashAccountsResult.data : [],
		accounts: accountsResult.ok ? accountsResult.data : [],
		startDate,
		endDate,
		outletId: user.outlet_id
	};
};

export const actions: Actions = {
	create: async ({ request, cookies, locals }) => {
		const user = locals.user!;
		if (user.role !== 'OWNER') return fail(403, { error: 'Akses ditolak' });

		const accessToken = cookies.get('access_token')!;
		const formData = await request.formData();
		const dataStr = formData.get('data')?.toString() ?? '';

		let data;
		try {
			data = JSON.parse(dataStr);
		} catch {
			return fail(400, { error: 'Data tidak valid' });
		}

		const result = await apiRequest('/accounting/sales', {
			method: 'POST',
			body: data,
			accessToken
		});

		if (!result.ok) {
			return fail(result.status || 400, { error: result.message });
		}

		return { success: true };
	},

	update: async ({ request, cookies, locals }) => {
		const user = locals.user!;
		if (user.role !== 'OWNER') return fail(403, { error: 'Akses ditolak' });

		const accessToken = cookies.get('access_token')!;
		const formData = await request.formData();
		const id = formData.get('id')?.toString() ?? '';
		const dataStr = formData.get('data')?.toString() ?? '';

		let data;
		try {
			data = JSON.parse(dataStr);
		} catch {
			return fail(400, { error: 'Data tidak valid' });
		}

		const result = await apiRequest(`/accounting/sales/${id}`, {
			method: 'PUT',
			body: data,
			accessToken
		});

		if (!result.ok) {
			return fail(result.status || 400, { error: result.message });
		}

		return { success: true };
	},

	delete: async ({ request, cookies, locals }) => {
		const user = locals.user!;
		if (user.role !== 'OWNER') return fail(403, { error: 'Akses ditolak' });

		const accessToken = cookies.get('access_token')!;
		const formData = await request.formData();
		const id = formData.get('id')?.toString() ?? '';

		const result = await apiRequest(`/accounting/sales/${id}`, {
			method: 'DELETE',
			accessToken
		});

		if (!result.ok) {
			return fail(result.status || 400, { error: result.message });
		}

		return { success: true };
	},

	syncPos: async ({ request, cookies, locals }) => {
		const user = locals.user!;
		if (user.role !== 'OWNER') return fail(403, { error: 'Akses ditolak' });

		const accessToken = cookies.get('access_token')!;
		const formData = await request.formData();
		const dataStr = formData.get('data')?.toString() ?? '';

		let data;
		try {
			data = JSON.parse(dataStr);
		} catch {
			return fail(400, { error: 'Data tidak valid' });
		}

		const result = await apiRequest<POSSyncResponse>('/accounting/sales/sync-pos', {
			method: 'POST',
			body: data,
			accessToken
		});

		if (!result.ok) {
			return fail(result.status || 400, { error: result.message });
		}

		return { success: true, syncedCount: result.data.synced_count };
	},

	postSales: async ({ request, cookies, locals }) => {
		const user = locals.user!;
		if (user.role !== 'OWNER') return fail(403, { error: 'Akses ditolak' });

		const accessToken = cookies.get('access_token')!;
		const formData = await request.formData();
		const dataStr = formData.get('data')?.toString() ?? '';

		let data;
		try {
			data = JSON.parse(dataStr);
		} catch {
			return fail(400, { error: 'Data tidak valid' });
		}

		const result = await apiRequest<{ posted_count: number; transactions_created: number }>(
			'/accounting/sales/post',
			{
				method: 'POST',
				body: data,
				accessToken
			}
		);

		if (!result.ok) {
			return fail(result.status || 400, { error: result.message });
		}

		return {
			success: true,
			postedCount: result.data.posted_count,
			transactionsCreated: result.data.transactions_created
		};
	}
};
