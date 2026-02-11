/**
 * Accounting master data page server — loads accounts, items, cash accounts.
 * Form actions handle CRUD for all three entity types.
 */

import { fail, redirect } from '@sveltejs/kit';
import { apiRequest } from '$lib/server/api';
import type { AcctAccount, AcctItem, AcctCashAccount } from '$lib/types/api';
import type { Actions, PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ locals, cookies }) => {
	const user = locals.user!;

	// Role guard — only OWNER can access accounting master data
	if (user.role !== 'OWNER') {
		redirect(302, '/');
	}

	const accessToken = cookies.get('access_token')!;

	const [accountsResult, itemsResult, cashAccountsResult] = await Promise.all([
		apiRequest<AcctAccount[]>('/accounting/master/accounts', { accessToken }),
		apiRequest<AcctItem[]>('/accounting/master/items', { accessToken }),
		apiRequest<AcctCashAccount[]>('/accounting/master/cash-accounts', { accessToken })
	]);

	return {
		accounts: accountsResult.ok ? accountsResult.data : [],
		items: itemsResult.ok ? itemsResult.data : [],
		cashAccounts: cashAccountsResult.ok ? cashAccountsResult.data : []
	};
};

export const actions: Actions = {
	// ── Account actions ────────────────────

	createAccount: async ({ request, cookies }) => {
		const accessToken = cookies.get('access_token')!;
		const formData = await request.formData();

		const account_code = formData.get('account_code')?.toString()?.trim() ?? '';
		const account_name = formData.get('account_name')?.toString()?.trim() ?? '';
		const account_type = formData.get('account_type')?.toString() ?? '';
		const line_type = formData.get('line_type')?.toString() ?? '';

		if (!account_code || !account_name || !account_type || !line_type) {
			return fail(400, { createAccountError: 'Semua field wajib diisi' });
		}

		const result = await apiRequest<AcctAccount>('/accounting/master/accounts', {
			method: 'POST',
			body: { account_code, account_name, account_type, line_type },
			accessToken
		});

		if (!result.ok) {
			if (result.status === 409) return fail(409, { createAccountError: 'Kode akun sudah digunakan' });
			return fail(result.status || 400, { createAccountError: result.message });
		}

		return { createAccountSuccess: true };
	},

	updateAccount: async ({ request, cookies }) => {
		const accessToken = cookies.get('access_token')!;
		const formData = await request.formData();

		const id = formData.get('id')?.toString() ?? '';
		const account_name = formData.get('account_name')?.toString()?.trim() ?? '';
		const account_type = formData.get('account_type')?.toString() ?? '';
		const line_type = formData.get('line_type')?.toString() ?? '';

		if (!id || !account_name || !account_type || !line_type) {
			return fail(400, { updateAccountError: 'Semua field wajib diisi' });
		}

		const result = await apiRequest<AcctAccount>(`/accounting/master/accounts/${id}`, {
			method: 'PUT',
			body: { account_name, account_type, line_type },
			accessToken
		});

		if (!result.ok) {
			return fail(result.status || 400, { updateAccountError: result.message });
		}

		return { updateAccountSuccess: true };
	},

	deleteAccount: async ({ request, cookies }) => {
		const accessToken = cookies.get('access_token')!;
		const formData = await request.formData();
		const id = formData.get('id')?.toString() ?? '';

		if (!id) {
			return fail(400, { deleteAccountError: 'ID akun tidak valid' });
		}

		const result = await apiRequest<void>(`/accounting/master/accounts/${id}`, {
			method: 'DELETE',
			accessToken
		});

		if (!result.ok) {
			return fail(result.status || 400, { deleteAccountError: result.message });
		}

		return { deleteAccountSuccess: true };
	},

	// ── Item actions ────────────────────

	createItem: async ({ request, cookies }) => {
		const accessToken = cookies.get('access_token')!;
		const formData = await request.formData();

		const item_code = formData.get('item_code')?.toString()?.trim() ?? '';
		const item_name = formData.get('item_name')?.toString()?.trim() ?? '';
		const item_category = formData.get('item_category')?.toString() ?? '';
		const unit = formData.get('unit')?.toString()?.trim() ?? '';
		const is_inventory = formData.get('is_inventory') === 'on';
		const average_price = formData.get('average_price')?.toString()?.trim() || null;
		const last_price = formData.get('last_price')?.toString()?.trim() || null;
		const for_hpp = formData.get('for_hpp')?.toString()?.trim() || null;
		const keywords = formData.get('keywords')?.toString()?.trim() ?? '';

		if (!item_code || !item_name || !item_category || !unit || !keywords) {
			return fail(400, { createItemError: 'Semua field wajib diisi' });
		}

		const body: Record<string, unknown> = {
			item_code,
			item_name,
			item_category,
			unit,
			is_inventory,
			average_price: average_price || null,
			last_price: last_price || null,
			for_hpp: for_hpp || null,
			keywords
		};

		const result = await apiRequest<AcctItem>('/accounting/master/items', {
			method: 'POST',
			body,
			accessToken
		});

		if (!result.ok) {
			if (result.status === 409) return fail(409, { createItemError: 'Kode item sudah digunakan' });
			return fail(result.status || 400, { createItemError: result.message });
		}

		return { createItemSuccess: true };
	},

	updateItem: async ({ request, cookies }) => {
		const accessToken = cookies.get('access_token')!;
		const formData = await request.formData();

		const id = formData.get('id')?.toString() ?? '';
		const item_name = formData.get('item_name')?.toString()?.trim() ?? '';
		const item_category = formData.get('item_category')?.toString() ?? '';
		const unit = formData.get('unit')?.toString()?.trim() ?? '';
		const is_inventory = formData.get('is_inventory') === 'on';
		const average_price = formData.get('average_price')?.toString()?.trim() || null;
		const last_price = formData.get('last_price')?.toString()?.trim() || null;
		const for_hpp = formData.get('for_hpp')?.toString()?.trim() || null;
		const keywords = formData.get('keywords')?.toString()?.trim() ?? '';

		if (!id || !item_name || !item_category || !unit || !keywords) {
			return fail(400, { updateItemError: 'Semua field wajib diisi' });
		}

		const body: Record<string, unknown> = {
			item_name,
			item_category,
			unit,
			is_inventory,
			average_price: average_price || null,
			last_price: last_price || null,
			for_hpp: for_hpp || null,
			keywords
		};

		const result = await apiRequest<AcctItem>(`/accounting/master/items/${id}`, {
			method: 'PUT',
			body,
			accessToken
		});

		if (!result.ok) {
			return fail(result.status || 400, { updateItemError: result.message });
		}

		return { updateItemSuccess: true };
	},

	deleteItem: async ({ request, cookies }) => {
		const accessToken = cookies.get('access_token')!;
		const formData = await request.formData();
		const id = formData.get('id')?.toString() ?? '';

		if (!id) {
			return fail(400, { deleteItemError: 'ID item tidak valid' });
		}

		const result = await apiRequest<void>(`/accounting/master/items/${id}`, {
			method: 'DELETE',
			accessToken
		});

		if (!result.ok) {
			return fail(result.status || 400, { deleteItemError: result.message });
		}

		return { deleteItemSuccess: true };
	},

	// ── Cash Account actions ────────────────────

	createCashAccount: async ({ request, cookies }) => {
		const accessToken = cookies.get('access_token')!;
		const formData = await request.formData();

		const cash_account_code = formData.get('cash_account_code')?.toString()?.trim() ?? '';
		const cash_account_name = formData.get('cash_account_name')?.toString()?.trim() ?? '';
		const bank_name = formData.get('bank_name')?.toString()?.trim() || null;
		const ownership = formData.get('ownership')?.toString() ?? '';

		if (!cash_account_code || !cash_account_name || !ownership) {
			return fail(400, { createCashAccountError: 'Semua field wajib diisi' });
		}

		const body: Record<string, unknown> = {
			cash_account_code,
			cash_account_name,
			bank_name: bank_name || null,
			ownership
		};

		const result = await apiRequest<AcctCashAccount>('/accounting/master/cash-accounts', {
			method: 'POST',
			body,
			accessToken
		});

		if (!result.ok) {
			if (result.status === 409) return fail(409, { createCashAccountError: 'Kode kas sudah digunakan' });
			return fail(result.status || 400, { createCashAccountError: result.message });
		}

		return { createCashAccountSuccess: true };
	},

	updateCashAccount: async ({ request, cookies }) => {
		const accessToken = cookies.get('access_token')!;
		const formData = await request.formData();

		const id = formData.get('id')?.toString() ?? '';
		const cash_account_name = formData.get('cash_account_name')?.toString()?.trim() ?? '';
		const bank_name = formData.get('bank_name')?.toString()?.trim() || null;
		const ownership = formData.get('ownership')?.toString() ?? '';

		if (!id || !cash_account_name || !ownership) {
			return fail(400, { updateCashAccountError: 'Semua field wajib diisi' });
		}

		const body: Record<string, unknown> = {
			cash_account_name,
			bank_name: bank_name || null,
			ownership
		};

		const result = await apiRequest<AcctCashAccount>(`/accounting/master/cash-accounts/${id}`, {
			method: 'PUT',
			body,
			accessToken
		});

		if (!result.ok) {
			return fail(result.status || 400, { updateCashAccountError: result.message });
		}

		return { updateCashAccountSuccess: true };
	},

	deleteCashAccount: async ({ request, cookies }) => {
		const accessToken = cookies.get('access_token')!;
		const formData = await request.formData();
		const id = formData.get('id')?.toString() ?? '';

		if (!id) {
			return fail(400, { deleteCashAccountError: 'ID kas tidak valid' });
		}

		const result = await apiRequest<void>(`/accounting/master/cash-accounts/${id}`, {
			method: 'DELETE',
			accessToken
		});

		if (!result.ok) {
			return fail(result.status || 400, { deleteCashAccountError: result.message });
		}

		return { deleteCashAccountSuccess: true };
	}
};
