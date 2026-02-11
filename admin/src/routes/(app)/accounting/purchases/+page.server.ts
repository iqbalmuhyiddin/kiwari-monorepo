/**
 * Purchase entry page server — loads items, cash accounts, accounts.
 * Form action serializes purchase data as JSON and POSTs to Go API.
 */

import { fail, redirect } from '@sveltejs/kit';
import { apiRequest } from '$lib/server/api';
import type { AcctItem, AcctCashAccount, AcctAccount } from '$lib/types/api';
import type { PageServerLoad, Actions } from './$types';

export const load: PageServerLoad = async ({ locals, cookies }) => {
	const user = locals.user!;

	// Role guard — only OWNER can access
	if (user.role !== 'OWNER') {
		redirect(302, '/');
	}

	const accessToken = cookies.get('access_token')!;

	const [itemsResult, cashAccountsResult, accountsResult] = await Promise.all([
		apiRequest<AcctItem[]>('/accounting/master/items', { accessToken }),
		apiRequest<AcctCashAccount[]>('/accounting/master/cash-accounts', { accessToken }),
		apiRequest<AcctAccount[]>('/accounting/master/accounts', { accessToken })
	]);

	return {
		items: itemsResult.ok ? itemsResult.data : [],
		cashAccounts: cashAccountsResult.ok ? cashAccountsResult.data : [],
		accounts: accountsResult.ok ? accountsResult.data : []
	};
};

export const actions: Actions = {
	create: async ({ request, cookies, locals }) => {
		const user = locals.user!;
		if (user.role !== 'OWNER') {
			return fail(403, { error: 'Akses ditolak' });
		}

		const accessToken = cookies.get('access_token')!;
		const formData = await request.formData();
		const purchaseDataStr = formData.get('purchase_data')?.toString() ?? '';

		let purchaseData;
		try {
			purchaseData = JSON.parse(purchaseDataStr);
		} catch {
			return fail(400, { error: 'Data pembelian tidak valid' });
		}

		const result = await apiRequest('/accounting/purchases', {
			method: 'POST',
			body: purchaseData,
			accessToken
		});

		if (!result.ok) {
			return fail(result.status || 400, { error: result.message });
		}

		return { success: true };
	}
};
