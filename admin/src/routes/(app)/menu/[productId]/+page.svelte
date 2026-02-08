<!--
  Product detail/edit page — form for basic info, variant groups, modifier groups, combo items.
  For new products (productId=new), shows create form only.
  For existing products, shows full editor with sub-entity management.
-->
<script lang="ts">
	import { enhance } from '$app/forms';
	import ProductForm from '$lib/components/ProductForm.svelte';
	import VariantGroupEditor from '$lib/components/VariantGroupEditor.svelte';
	import ModifierGroupEditor from '$lib/components/ModifierGroupEditor.svelte';
	import { formatRupiah } from '$lib/utils/format';
	import type { Product } from '$lib/types/api';

	let { data, form } = $props();

	let showAddCombo = $state(false);
	let confirmDelete = $state(false);

	// For combo product dropdown — filter out the current product
	let availableProducts = $derived(
		data.allProducts.filter((p: Product) => p.id !== data.product?.id && !p.is_combo)
	);

	function getProductName(productId: string): string {
		const p = data.allProducts.find((prod: Product) => prod.id === productId);
		return p?.name ?? productId;
	}
</script>

<svelte:head>
	<title>{data.isNew ? 'Produk Baru' : data.product?.name ?? 'Produk'} - Kiwari POS</title>
</svelte:head>

<div class="product-page">
	<!-- Header -->
	<div class="page-header">
		<div class="header-left">
			<a href="/menu" class="back-link">&larr; Kembali ke Menu</a>
			<h1 class="page-title">{data.isNew ? 'Buat Produk Baru' : 'Edit Produk'}</h1>
			{#if !data.isNew && data.product}
				<p class="page-subtitle">
					{data.product.name}
					{#if !data.product.is_active}
						<span class="inactive-badge">Nonaktif</span>
					{/if}
				</p>
			{/if}
		</div>
		{#if !data.isNew && data.product}
			<div class="header-actions">
				{#if confirmDelete}
					<span class="delete-confirm-text">Yakin hapus?</span>
					<form method="POST" action="?/deleteProduct" use:enhance>
						<button type="submit" class="btn-delete-confirm">Ya, Hapus</button>
					</form>
					<button type="button" class="btn-secondary btn-sm" onclick={() => { confirmDelete = false; }}>Batal</button>
				{:else}
					<button type="button" class="btn-delete" onclick={() => { confirmDelete = true; }}>Hapus Produk</button>
				{/if}
			</div>
		{/if}
	</div>

	<!-- Basic Info Form -->
	<div class="card">
		<h2 class="card-title">Informasi Dasar</h2>
		<ProductForm
			product={data.product}
			categories={data.categories}
			isNew={data.isNew}
			{form}
		/>
	</div>

	<!-- Variant Groups (only for existing products) -->
	{#if !data.isNew}
		<div class="card">
			<VariantGroupEditor variantGroups={data.variantGroups} {form} />
		</div>
	{/if}

	<!-- Modifier Groups (only for existing products) -->
	{#if !data.isNew}
		<div class="card">
			<ModifierGroupEditor modifierGroups={data.modifierGroups} {form} />
		</div>
	{/if}

	<!-- Combo Items (only for existing combo products) -->
	{#if !data.isNew && data.product?.is_combo}
		<div class="card">
			<div class="section-header">
				<h3 class="section-title">Item Combo</h3>
				<button type="button" class="btn-add" onclick={() => { showAddCombo = true; }}>
					+ Tambah Item
				</button>
			</div>

			{#if form?.comboError}
				<div class="error-banner">{form.comboError}</div>
			{/if}

			<!-- Add combo item form -->
			{#if showAddCombo}
				<form method="POST" action="?/addComboItem" use:enhance={() => {
					return async ({ result, update }) => {
						if (result.type === 'success') {
							showAddCombo = false;
						}
						await update();
					};
				}}>
					<div class="combo-form">
						<div class="combo-form-row">
							<div class="form-group flex-1">
								<label class="form-label">Produk
									<select name="product_id" class="input-field" required>
										<option value="">-- Pilih Produk --</option>
										{#each availableProducts as p (p.id)}
											<option value={p.id}>{p.name} ({formatRupiah(p.base_price)})</option>
										{/each}
									</select>
								</label>
							</div>
							<div class="form-group">
								<label class="form-label">Qty
									<input name="quantity" type="number" class="input-field input-sm" value="1" min="1" required />
								</label>
							</div>
							<div class="form-group">
								<label class="form-label">Urutan
									<input name="sort_order" type="number" class="input-field input-xs" value="0" />
								</label>
							</div>
						</div>
						<div class="combo-form-actions">
							<button type="submit" class="btn-primary btn-sm">Tambah</button>
							<button type="button" class="btn-secondary btn-sm" onclick={() => { showAddCombo = false; }}>Batal</button>
						</div>
					</div>
				</form>
			{/if}

			<!-- Combo items list -->
			{#if data.comboItems.length === 0 && !showAddCombo}
				<p class="empty-text">Belum ada item combo. Klik "Tambah Item" untuk menambah.</p>
			{/if}

			{#each data.comboItems as item (item.id)}
				<div class="combo-item">
					<div class="combo-info">
						<span class="combo-product-name">{getProductName(item.product_id)}</span>
						<span class="combo-qty">x{item.quantity}</span>
					</div>
					<form method="POST" action="?/removeComboItem" use:enhance>
						<input type="hidden" name="id" value={item.id} />
						<button type="submit" class="btn-icon btn-danger"
							onclick={(e) => { if (!confirm('Hapus item combo ini?')) e.preventDefault(); }}>
							Hapus
						</button>
					</form>
				</div>
			{/each}
		</div>
	{/if}
</div>

<style>
	.product-page {
		max-width: 900px;
	}

	.page-header {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		margin-bottom: 20px;
	}

	.header-left {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.back-link {
		font-size: 13px;
		color: var(--color-text-secondary);
		text-decoration: none;
		margin-bottom: 4px;
	}

	.back-link:hover {
		color: var(--color-primary);
	}

	.page-title {
		font-size: 20px;
		font-weight: 700;
		color: var(--color-text-primary);
		margin: 0;
	}

	.page-subtitle {
		font-size: 13px;
		color: var(--color-text-secondary);
		margin: 0;
		display: flex;
		align-items: center;
		gap: 8px;
	}

	.inactive-badge {
		font-size: 11px;
		font-weight: 500;
		color: var(--color-error);
		background-color: var(--color-error-bg);
		padding: 2px 8px;
		border-radius: 4px;
	}

	.header-actions {
		display: flex;
		align-items: center;
		gap: 8px;
	}

	.btn-delete {
		background: none;
		border: 1px solid var(--color-error);
		color: var(--color-error);
		padding: 8px 16px;
		font-size: 13px;
		font-weight: 600;
		border-radius: var(--radius-btn);
		cursor: pointer;
	}

	.btn-delete:hover {
		background-color: var(--color-error-bg);
	}

	.btn-delete-confirm {
		background-color: var(--color-error);
		color: white;
		border: none;
		padding: 8px 16px;
		font-size: 13px;
		font-weight: 600;
		border-radius: var(--radius-btn);
		cursor: pointer;
	}

	.delete-confirm-text {
		font-size: 13px;
		font-weight: 600;
		color: var(--color-error);
	}

	.btn-sm {
		padding: 6px 14px;
		font-size: 13px;
		border: none;
		cursor: pointer;
	}

	.card {
		background-color: var(--color-bg);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-card);
		padding: 20px;
		margin-bottom: 16px;
	}

	.card-title {
		font-size: var(--text-title);
		font-weight: 600;
		color: var(--color-text-primary);
		margin: 0 0 16px;
	}

	/* Combo section */
	.section-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		margin-bottom: 12px;
	}

	.section-title {
		font-size: var(--text-title);
		font-weight: 600;
		color: var(--color-text-primary);
		margin: 0;
	}

	.btn-add {
		background: none;
		border: 1px dashed var(--color-primary);
		color: var(--color-primary);
		font-size: 13px;
		font-weight: 600;
		padding: 6px 12px;
		border-radius: var(--radius-btn);
		cursor: pointer;
	}

	.btn-add:hover {
		background-color: var(--color-surface);
	}

	.error-banner {
		background-color: var(--color-error-bg);
		color: var(--color-error);
		font-size: 13px;
		font-weight: 500;
		padding: 8px 12px;
		border-radius: var(--radius-chip);
		margin-bottom: 8px;
	}

	.combo-form {
		padding: 12px 0;
		border-bottom: 1px solid var(--color-border);
	}

	.combo-form-row {
		display: flex;
		gap: 8px;
		align-items: flex-end;
		flex-wrap: wrap;
	}

	.form-group {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.flex-1 {
		flex: 1;
		min-width: 200px;
	}

	.form-label {
		display: block;
		font-size: 12px;
		font-weight: 600;
		color: var(--color-text-primary);
	}

	.form-label :global(input),
	.form-label :global(select) {
		display: block;
		margin-top: 4px;
	}

	.input-sm {
		width: 80px;
	}

	.input-xs {
		width: 64px;
	}

	.combo-form-actions {
		display: flex;
		gap: 8px;
		margin-top: 8px;
	}

	.empty-text {
		font-size: 13px;
		color: var(--color-text-secondary);
		margin: 8px 0;
	}

	.combo-item {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 10px 0;
		border-bottom: 1px solid var(--color-border);
	}

	.combo-item:last-of-type {
		border-bottom: none;
	}

	.combo-info {
		display: flex;
		align-items: center;
		gap: 12px;
	}

	.combo-product-name {
		font-size: 13px;
		font-weight: 500;
		color: var(--color-text-primary);
	}

	.combo-qty {
		font-size: 12px;
		font-weight: 600;
		color: var(--color-text-secondary);
		background-color: var(--color-surface);
		padding: 2px 8px;
		border-radius: 4px;
	}

	.btn-icon {
		background: none;
		border: none;
		font-size: 12px;
		font-weight: 600;
		color: var(--color-text-secondary);
		cursor: pointer;
		padding: 4px 8px;
		border-radius: 4px;
	}

	.btn-danger {
		color: var(--color-error);
	}

	.btn-danger:hover {
		background-color: var(--color-error-bg);
	}

	select.input-field {
		appearance: auto;
	}
</style>
