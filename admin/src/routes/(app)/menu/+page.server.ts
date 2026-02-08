/**
 * Menu page server — loads categories and products, handles category CRUD.
 *
 * Categories and products are loaded in parallel on page load.
 * Form actions handle category create/edit/delete and product delete (soft).
 */

import { fail } from '@sveltejs/kit';
import { apiRequest } from '$lib/server/api';
import type { Category, Product } from '$lib/types/api';
import type { Actions, PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ locals, cookies }) => {
	const user = locals.user!;
	const accessToken = cookies.get('access_token')!;
	const oid = user.outlet_id;

	const [categoriesResult, productsResult] = await Promise.all([
		apiRequest<Category[]>(`/outlets/${oid}/categories`, { accessToken }),
		apiRequest<Product[]>(`/outlets/${oid}/products`, { accessToken })
	]);

	return {
		categories: categoriesResult.ok ? categoriesResult.data : [],
		products: productsResult.ok ? productsResult.data : []
	};
};

export const actions: Actions = {
	createCategory: async ({ request, locals, cookies }) => {
		const user = locals.user!;
		const accessToken = cookies.get('access_token')!;
		const oid = user.outlet_id;

		const formData = await request.formData();
		const name = formData.get('name')?.toString().trim() ?? '';
		const description = formData.get('description')?.toString().trim() ?? '';
		const sort_order = parseInt(formData.get('sort_order')?.toString() ?? '0', 10);

		if (!name) {
			return fail(400, { categoryError: 'Nama kategori wajib diisi' });
		}

		const result = await apiRequest<Category>(`/outlets/${oid}/categories`, {
			method: 'POST',
			body: { name, description, sort_order },
			accessToken
		});

		if (!result.ok) {
			return fail(result.status || 400, { categoryError: result.message });
		}

		return { categorySuccess: true };
	},

	updateCategory: async ({ request, locals, cookies }) => {
		const user = locals.user!;
		const accessToken = cookies.get('access_token')!;
		const oid = user.outlet_id;

		const formData = await request.formData();
		const id = formData.get('id')?.toString() ?? '';
		const name = formData.get('name')?.toString().trim() ?? '';
		const description = formData.get('description')?.toString().trim() ?? '';
		const sort_order = parseInt(formData.get('sort_order')?.toString() ?? '0', 10);

		if (!name) {
			return fail(400, { categoryError: 'Nama kategori wajib diisi' });
		}

		const result = await apiRequest<Category>(`/outlets/${oid}/categories/${id}`, {
			method: 'PUT',
			body: { name, description, sort_order },
			accessToken
		});

		if (!result.ok) {
			return fail(result.status || 400, { categoryError: result.message });
		}

		return { categorySuccess: true };
	},

	deleteCategory: async ({ request, locals, cookies }) => {
		const user = locals.user!;
		const accessToken = cookies.get('access_token')!;
		const oid = user.outlet_id;

		const formData = await request.formData();
		const id = formData.get('id')?.toString() ?? '';

		const result = await apiRequest<void>(`/outlets/${oid}/categories/${id}`, {
			method: 'DELETE',
			accessToken
		});

		if (!result.ok) {
			return fail(result.status || 400, { categoryError: result.message });
		}

		return { categorySuccess: true };
	}
};
