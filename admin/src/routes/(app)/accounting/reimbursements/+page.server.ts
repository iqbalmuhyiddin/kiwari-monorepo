/**
 * Reimbursement management page server — loads reimbursements, items, accounts, cash accounts.
 * Form actions for update, delete, batch assign, and batch post.
 */

import { fail, redirect } from '@sveltejs/kit';
import { apiRequest } from '$lib/server/api';
import type {
	AcctReimbursementRequest,
	AcctItem,
	AcctAccount,
	AcctCashAccount,
	BatchAssignResponse,
	BatchPostResponse
} from '$lib/types/api';
import type { PageServerLoad, Actions } from './$types';

export const load: PageServerLoad = async ({ locals, cookies, url }) => {
	const user = locals.user!;
	if (user.role !== 'OWNER') {
		redirect(302, '/');
	}

	const accessToken = cookies.get('access_token')!;

	// Build query params from URL
	const status = url.searchParams.get('status') || '';
	const requester = url.searchParams.get('requester') || '';
	let queryParams = '?limit=200&offset=0';
	if (status) queryParams += `&status=${encodeURIComponent(status)}`;
	if (requester) queryParams += `&requester=${encodeURIComponent(requester)}`;

	const [reimbursementsResult, itemsResult, accountsResult, cashAccountsResult] =
		await Promise.all([
			apiRequest<AcctReimbursementRequest[]>(
				`/accounting/reimbursements/${queryParams}`,
				{ accessToken }
			),
			apiRequest<AcctItem[]>('/accounting/master/items', { accessToken }),
			apiRequest<AcctAccount[]>('/accounting/master/accounts', { accessToken }),
			apiRequest<AcctCashAccount[]>('/accounting/master/cash-accounts', { accessToken })
		]);

	return {
		reimbursements: reimbursementsResult.ok ? reimbursementsResult.data : [],
		items: itemsResult.ok ? itemsResult.data : [],
		accounts: accountsResult.ok ? accountsResult.data : [],
		cashAccounts: cashAccountsResult.ok ? cashAccountsResult.data : [],
		filterStatus: status,
		filterRequester: requester
	};
};

export const actions: Actions = {
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

		const result = await apiRequest(`/accounting/reimbursements/${id}`, {
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

		const result = await apiRequest(`/accounting/reimbursements/${id}`, {
			method: 'DELETE',
			accessToken
		});

		if (!result.ok) {
			return fail(result.status || 400, { error: result.message });
		}
		return { success: true };
	},

	batchAssign: async ({ request, cookies, locals }) => {
		const user = locals.user!;
		if (user.role !== 'OWNER') return fail(403, { error: 'Akses ditolak' });

		const accessToken = cookies.get('access_token')!;
		const formData = await request.formData();
		const idsStr = formData.get('ids')?.toString() ?? '';

		let ids: string[];
		try {
			ids = JSON.parse(idsStr);
		} catch {
			return fail(400, { error: 'IDs tidak valid' });
		}

		const result = await apiRequest<BatchAssignResponse>('/accounting/reimbursements/batch', {
			method: 'POST',
			body: { ids },
			accessToken
		});

		if (!result.ok) {
			return fail(result.status || 400, { error: result.message });
		}
		return { success: true, batchId: result.data.batch_id, assigned: result.data.assigned };
	},

	batchPost: async ({ request, cookies, locals }) => {
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

		const result = await apiRequest<BatchPostResponse>(
			'/accounting/reimbursements/batch/post',
			{
				method: 'POST',
				body: data,
				accessToken
			}
		);

		if (!result.ok) {
			return fail(result.status || 400, { error: result.message });
		}
		return { success: true, posted: result.data.posted };
	}
};
