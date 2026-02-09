/**
 * Product detail page server — loads product with variant groups, modifier groups,
 * combo items. Handles all CRUD actions for the product and its sub-entities.
 *
 * For a new product (productId === "new"), returns empty data.
 */

import { fail, redirect } from '@sveltejs/kit';
import { apiRequest } from '$lib/server/api';
import type {
	Category,
	Product,
	VariantGroup,
	Variant,
	ModifierGroup,
	Modifier,
	ComboItem
} from '$lib/types/api';
import type { Actions, PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ params, locals, cookies }) => {
	const user = locals.user!;
	const accessToken = cookies.get('access_token')!;
	const oid = user.outlet_id;
	const { productId } = params;

	// Always load categories for the dropdown
	const categoriesResult = await apiRequest<Category[]>(`/outlets/${oid}/categories`, {
		accessToken
	});
	const categories = categoriesResult.ok ? categoriesResult.data : [];

	// New product — return empty data
	if (productId === 'new') {
		return {
			isNew: true,
			product: null,
			categories,
			variantGroups: [],
			modifierGroups: [],
			comboItems: [],
			allProducts: []
		};
	}

	// Existing product — load everything in parallel
	const [productResult, variantGroupsResult, modifierGroupsResult] = await Promise.all([
		apiRequest<Product>(`/outlets/${oid}/products/${productId}`, { accessToken }),
		apiRequest<VariantGroup[]>(`/outlets/${oid}/products/${productId}/variant-groups`, {
			accessToken
		}),
		apiRequest<ModifierGroup[]>(`/outlets/${oid}/products/${productId}/modifier-groups`, {
			accessToken
		})
	]);

	if (!productResult.ok) {
		if (productResult.status === 401) {
			cookies.delete('access_token', { path: '/' });
			cookies.delete('refresh_token', { path: '/' });
			cookies.delete('user_info', { path: '/' });
			redirect(302, '/login');
		}
		redirect(302, '/menu');
	}

	const product = productResult.data;
	const variantGroups = variantGroupsResult.ok ? variantGroupsResult.data : [];
	const modifierGroups = modifierGroupsResult.ok ? modifierGroupsResult.data : [];

	// Load variants for each variant group in parallel
	const variantGroupsWithVariants = await Promise.all(
		variantGroups.map(async (vg) => {
			const variantsResult = await apiRequest<Variant[]>(
				`/outlets/${oid}/products/${productId}/variant-groups/${vg.id}/variants`,
				{ accessToken }
			);
			return { ...vg, variants: variantsResult.ok ? variantsResult.data : [] };
		})
	);

	// Load modifiers for each modifier group in parallel
	const modifierGroupsWithModifiers = await Promise.all(
		modifierGroups.map(async (mg) => {
			const modifiersResult = await apiRequest<Modifier[]>(
				`/outlets/${oid}/products/${productId}/modifier-groups/${mg.id}/modifiers`,
				{ accessToken }
			);
			return { ...mg, modifiers: modifiersResult.ok ? modifiersResult.data : [] };
		})
	);

	// Load combo items if product is a combo
	let comboItems: ComboItem[] = [];
	let allProducts: Product[] = [];
	if (product.is_combo) {
		const [comboResult, productsResult] = await Promise.all([
			apiRequest<ComboItem[]>(`/outlets/${oid}/products/${productId}/combo-items`, {
				accessToken
			}),
			apiRequest<Product[]>(`/outlets/${oid}/products`, { accessToken })
		]);
		comboItems = comboResult.ok ? comboResult.data : [];
		allProducts = productsResult.ok ? productsResult.data : [];
	}

	return {
		isNew: false,
		product,
		categories,
		variantGroups: variantGroupsWithVariants,
		modifierGroups: modifierGroupsWithModifiers,
		comboItems,
		allProducts
	};
};

export const actions: Actions = {
	// ── Product CRUD ──────────────────────────────

	saveProduct: async ({ request, params, locals, cookies }) => {
		const user = locals.user!;
		const accessToken = cookies.get('access_token')!;
		const oid = user.outlet_id;
		const { productId } = params;

		const formData = await request.formData();
		const name = formData.get('name')?.toString().trim() ?? '';
		const category_id = formData.get('category_id')?.toString() ?? '';
		const base_price = formData.get('base_price')?.toString() ?? '0';
		const description = formData.get('description')?.toString().trim() ?? '';
		const image_url = formData.get('image_url')?.toString().trim() ?? '';
		const station = formData.get('station')?.toString() ?? '';
		const preparation_time_str = formData.get('preparation_time')?.toString() ?? '';
		const preparation_time = preparation_time_str ? parseInt(preparation_time_str, 10) : undefined;
		const is_combo = formData.get('is_combo') === 'true';
		const is_active = formData.get('is_active') === 'true';

		if (!name) {
			return fail(400, { error: 'Nama produk wajib diisi' });
		}
		if (!category_id) {
			return fail(400, { error: 'Kategori wajib dipilih' });
		}

		const body = {
			name,
			category_id,
			base_price,
			description: description || undefined,
			image_url: image_url || undefined,
			station: station || undefined,
			preparation_time,
			is_combo,
			is_active
		};

		if (productId === 'new') {
			const result = await apiRequest<Product>(`/outlets/${oid}/products`, {
				method: 'POST',
				body,
				accessToken
			});

			if (!result.ok) {
				return fail(result.status || 400, { error: result.message });
			}

			redirect(303, `/menu/${result.data.id}`);
		} else {
			const result = await apiRequest<Product>(`/outlets/${oid}/products/${productId}`, {
				method: 'PUT',
				body,
				accessToken
			});

			if (!result.ok) {
				return fail(result.status || 400, { error: result.message });
			}

			return { success: true };
		}
	},

	deleteProduct: async ({ params, locals, cookies }) => {
		const user = locals.user!;
		const accessToken = cookies.get('access_token')!;
		const oid = user.outlet_id;
		const { productId } = params;

		const result = await apiRequest<void>(`/outlets/${oid}/products/${productId}`, {
			method: 'DELETE',
			accessToken
		});

		if (!result.ok) {
			return fail(result.status || 400, { error: result.message });
		}

		redirect(303, '/menu');
	},

	// ── Variant Group CRUD ────────────────────────

	createVariantGroup: async ({ request, params, locals, cookies }) => {
		const user = locals.user!;
		const accessToken = cookies.get('access_token')!;
		const oid = user.outlet_id;
		const { productId } = params;

		const formData = await request.formData();
		const name = formData.get('name')?.toString().trim() ?? '';
		const is_required = formData.get('is_required') === 'true';
		const sort_order = parseInt(formData.get('sort_order')?.toString() ?? '0', 10);

		if (!name) {
			return fail(400, { variantGroupError: 'Nama grup varian wajib diisi' });
		}

		const result = await apiRequest<VariantGroup>(
			`/outlets/${oid}/products/${productId}/variant-groups`,
			{
				method: 'POST',
				body: { name, is_required, sort_order },
				accessToken
			}
		);

		if (!result.ok) {
			return fail(result.status || 400, { variantGroupError: result.message });
		}

		return { variantGroupSuccess: true };
	},

	updateVariantGroup: async ({ request, params, locals, cookies }) => {
		const user = locals.user!;
		const accessToken = cookies.get('access_token')!;
		const oid = user.outlet_id;
		const { productId } = params;

		const formData = await request.formData();
		const vgId = formData.get('id')?.toString() ?? '';
		const name = formData.get('name')?.toString().trim() ?? '';
		const is_required = formData.get('is_required') === 'true';
		const sort_order = parseInt(formData.get('sort_order')?.toString() ?? '0', 10);

		if (!name) {
			return fail(400, { variantGroupError: 'Nama grup varian wajib diisi' });
		}

		const result = await apiRequest<VariantGroup>(
			`/outlets/${oid}/products/${productId}/variant-groups/${vgId}`,
			{
				method: 'PUT',
				body: { name, is_required, sort_order },
				accessToken
			}
		);

		if (!result.ok) {
			return fail(result.status || 400, { variantGroupError: result.message });
		}

		return { variantGroupSuccess: true };
	},

	deleteVariantGroup: async ({ request, params, locals, cookies }) => {
		const user = locals.user!;
		const accessToken = cookies.get('access_token')!;
		const oid = user.outlet_id;
		const { productId } = params;

		const formData = await request.formData();
		const vgId = formData.get('id')?.toString() ?? '';

		const result = await apiRequest<void>(
			`/outlets/${oid}/products/${productId}/variant-groups/${vgId}`,
			{
				method: 'DELETE',
				accessToken
			}
		);

		if (!result.ok) {
			return fail(result.status || 400, { variantGroupError: result.message });
		}

		return { variantGroupSuccess: true };
	},

	// ── Variant CRUD ──────────────────────────────

	createVariant: async ({ request, params, locals, cookies }) => {
		const user = locals.user!;
		const accessToken = cookies.get('access_token')!;
		const oid = user.outlet_id;
		const { productId } = params;

		const formData = await request.formData();
		const vgId = formData.get('variant_group_id')?.toString() ?? '';
		const name = formData.get('name')?.toString().trim() ?? '';
		const price_adjustment = formData.get('price_adjustment')?.toString() ?? '0.00';
		const sort_order = parseInt(formData.get('sort_order')?.toString() ?? '0', 10);

		if (!name) {
			return fail(400, { variantError: 'Nama varian wajib diisi' });
		}

		const result = await apiRequest<Variant>(
			`/outlets/${oid}/products/${productId}/variant-groups/${vgId}/variants`,
			{
				method: 'POST',
				body: { name, price_adjustment, sort_order },
				accessToken
			}
		);

		if (!result.ok) {
			return fail(result.status || 400, { variantError: result.message });
		}

		return { variantSuccess: true };
	},

	updateVariant: async ({ request, params, locals, cookies }) => {
		const user = locals.user!;
		const accessToken = cookies.get('access_token')!;
		const oid = user.outlet_id;
		const { productId } = params;

		const formData = await request.formData();
		const vgId = formData.get('variant_group_id')?.toString() ?? '';
		const vid = formData.get('id')?.toString() ?? '';
		const name = formData.get('name')?.toString().trim() ?? '';
		const price_adjustment = formData.get('price_adjustment')?.toString() ?? '0.00';
		const sort_order = parseInt(formData.get('sort_order')?.toString() ?? '0', 10);

		if (!name) {
			return fail(400, { variantError: 'Nama varian wajib diisi' });
		}

		const result = await apiRequest<Variant>(
			`/outlets/${oid}/products/${productId}/variant-groups/${vgId}/variants/${vid}`,
			{
				method: 'PUT',
				body: { name, price_adjustment, sort_order },
				accessToken
			}
		);

		if (!result.ok) {
			return fail(result.status || 400, { variantError: result.message });
		}

		return { variantSuccess: true };
	},

	deleteVariant: async ({ request, params, locals, cookies }) => {
		const user = locals.user!;
		const accessToken = cookies.get('access_token')!;
		const oid = user.outlet_id;
		const { productId } = params;

		const formData = await request.formData();
		const vgId = formData.get('variant_group_id')?.toString() ?? '';
		const vid = formData.get('id')?.toString() ?? '';

		const result = await apiRequest<void>(
			`/outlets/${oid}/products/${productId}/variant-groups/${vgId}/variants/${vid}`,
			{
				method: 'DELETE',
				accessToken
			}
		);

		if (!result.ok) {
			return fail(result.status || 400, { variantError: result.message });
		}

		return { variantSuccess: true };
	},

	// ── Modifier Group CRUD ───────────────────────

	createModifierGroup: async ({ request, params, locals, cookies }) => {
		const user = locals.user!;
		const accessToken = cookies.get('access_token')!;
		const oid = user.outlet_id;
		const { productId } = params;

		const formData = await request.formData();
		const name = formData.get('name')?.toString().trim() ?? '';
		const min_select = parseInt(formData.get('min_select')?.toString() ?? '0', 10);
		const max_select = parseInt(formData.get('max_select')?.toString() ?? '0', 10);
		const sort_order = parseInt(formData.get('sort_order')?.toString() ?? '0', 10);

		if (!name) {
			return fail(400, { modifierGroupError: 'Nama grup modifier wajib diisi' });
		}

		const result = await apiRequest<ModifierGroup>(
			`/outlets/${oid}/products/${productId}/modifier-groups`,
			{
				method: 'POST',
				body: { name, min_select, max_select: max_select || undefined, sort_order },
				accessToken
			}
		);

		if (!result.ok) {
			return fail(result.status || 400, { modifierGroupError: result.message });
		}

		return { modifierGroupSuccess: true };
	},

	updateModifierGroup: async ({ request, params, locals, cookies }) => {
		const user = locals.user!;
		const accessToken = cookies.get('access_token')!;
		const oid = user.outlet_id;
		const { productId } = params;

		const formData = await request.formData();
		const mgId = formData.get('id')?.toString() ?? '';
		const name = formData.get('name')?.toString().trim() ?? '';
		const min_select = parseInt(formData.get('min_select')?.toString() ?? '0', 10);
		const max_select = parseInt(formData.get('max_select')?.toString() ?? '0', 10);
		const sort_order = parseInt(formData.get('sort_order')?.toString() ?? '0', 10);

		if (!name) {
			return fail(400, { modifierGroupError: 'Nama grup modifier wajib diisi' });
		}

		const result = await apiRequest<ModifierGroup>(
			`/outlets/${oid}/products/${productId}/modifier-groups/${mgId}`,
			{
				method: 'PUT',
				body: { name, min_select, max_select: max_select || undefined, sort_order },
				accessToken
			}
		);

		if (!result.ok) {
			return fail(result.status || 400, { modifierGroupError: result.message });
		}

		return { modifierGroupSuccess: true };
	},

	deleteModifierGroup: async ({ request, params, locals, cookies }) => {
		const user = locals.user!;
		const accessToken = cookies.get('access_token')!;
		const oid = user.outlet_id;
		const { productId } = params;

		const formData = await request.formData();
		const mgId = formData.get('id')?.toString() ?? '';

		const result = await apiRequest<void>(
			`/outlets/${oid}/products/${productId}/modifier-groups/${mgId}`,
			{
				method: 'DELETE',
				accessToken
			}
		);

		if (!result.ok) {
			return fail(result.status || 400, { modifierGroupError: result.message });
		}

		return { modifierGroupSuccess: true };
	},

	// ── Modifier CRUD ─────────────────────────────

	createModifier: async ({ request, params, locals, cookies }) => {
		const user = locals.user!;
		const accessToken = cookies.get('access_token')!;
		const oid = user.outlet_id;
		const { productId } = params;

		const formData = await request.formData();
		const mgId = formData.get('modifier_group_id')?.toString() ?? '';
		const name = formData.get('name')?.toString().trim() ?? '';
		const price = formData.get('price')?.toString() ?? '0.00';
		const sort_order = parseInt(formData.get('sort_order')?.toString() ?? '0', 10);

		if (!name) {
			return fail(400, { modifierError: 'Nama modifier wajib diisi' });
		}

		const result = await apiRequest<Modifier>(
			`/outlets/${oid}/products/${productId}/modifier-groups/${mgId}/modifiers`,
			{
				method: 'POST',
				body: { name, price, sort_order },
				accessToken
			}
		);

		if (!result.ok) {
			return fail(result.status || 400, { modifierError: result.message });
		}

		return { modifierSuccess: true };
	},

	updateModifier: async ({ request, params, locals, cookies }) => {
		const user = locals.user!;
		const accessToken = cookies.get('access_token')!;
		const oid = user.outlet_id;
		const { productId } = params;

		const formData = await request.formData();
		const mgId = formData.get('modifier_group_id')?.toString() ?? '';
		const mid = formData.get('id')?.toString() ?? '';
		const name = formData.get('name')?.toString().trim() ?? '';
		const price = formData.get('price')?.toString() ?? '0.00';
		const sort_order = parseInt(formData.get('sort_order')?.toString() ?? '0', 10);

		if (!name) {
			return fail(400, { modifierError: 'Nama modifier wajib diisi' });
		}

		const result = await apiRequest<Modifier>(
			`/outlets/${oid}/products/${productId}/modifier-groups/${mgId}/modifiers/${mid}`,
			{
				method: 'PUT',
				body: { name, price, sort_order },
				accessToken
			}
		);

		if (!result.ok) {
			return fail(result.status || 400, { modifierError: result.message });
		}

		return { modifierSuccess: true };
	},

	deleteModifier: async ({ request, params, locals, cookies }) => {
		const user = locals.user!;
		const accessToken = cookies.get('access_token')!;
		const oid = user.outlet_id;
		const { productId } = params;

		const formData = await request.formData();
		const mgId = formData.get('modifier_group_id')?.toString() ?? '';
		const mid = formData.get('id')?.toString() ?? '';

		const result = await apiRequest<void>(
			`/outlets/${oid}/products/${productId}/modifier-groups/${mgId}/modifiers/${mid}`,
			{
				method: 'DELETE',
				accessToken
			}
		);

		if (!result.ok) {
			return fail(result.status || 400, { modifierError: result.message });
		}

		return { modifierSuccess: true };
	},

	// ── Combo Item CRUD ───────────────────────────

	addComboItem: async ({ request, params, locals, cookies }) => {
		const user = locals.user!;
		const accessToken = cookies.get('access_token')!;
		const oid = user.outlet_id;
		const { productId } = params;

		const formData = await request.formData();
		const product_id = formData.get('product_id')?.toString() ?? '';
		const quantity = parseInt(formData.get('quantity')?.toString() ?? '1', 10);
		const sort_order = parseInt(formData.get('sort_order')?.toString() ?? '0', 10);

		if (!product_id) {
			return fail(400, { comboError: 'Pilih produk untuk ditambahkan' });
		}

		const result = await apiRequest<ComboItem>(
			`/outlets/${oid}/products/${productId}/combo-items`,
			{
				method: 'POST',
				body: { product_id, quantity, sort_order },
				accessToken
			}
		);

		if (!result.ok) {
			return fail(result.status || 400, { comboError: result.message });
		}

		return { comboSuccess: true };
	},

	removeComboItem: async ({ request, params, locals, cookies }) => {
		const user = locals.user!;
		const accessToken = cookies.get('access_token')!;
		const oid = user.outlet_id;
		const { productId } = params;

		const formData = await request.formData();
		const comboItemId = formData.get('id')?.toString() ?? '';

		const result = await apiRequest<void>(
			`/outlets/${oid}/products/${productId}/combo-items/${comboItemId}`,
			{
				method: 'DELETE',
				accessToken
			}
		);

		if (!result.ok) {
			return fail(result.status || 400, { comboError: result.message });
		}

		return { comboSuccess: true };
	}
};
