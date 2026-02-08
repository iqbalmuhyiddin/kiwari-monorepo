<!--
  Menu management page — category tabs at top, product grid below.
  Categories can be managed inline. Products navigate to detail page on click.
-->
<script lang="ts">
	import { enhance } from '$app/forms';
	import { formatRupiah } from '$lib/utils/format';
	import type { Category, Product } from '$lib/types/api';

	let { data, form } = $props();

	let selectedCategoryId = $state<string | null>(null);
	let searchQuery = $state('');
	let showCategoryManager = $state(false);
	let editingCategoryId = $state<string | null>(null);
	let showAddCategory = $state(false);

	// Filter products by selected category and search
	let filteredProducts = $derived.by(() => {
		let products: Product[] = data.products;

		if (selectedCategoryId) {
			products = products.filter((p) => p.category_id === selectedCategoryId);
		}

		if (searchQuery.trim()) {
			const q = searchQuery.trim().toLowerCase();
			products = products.filter(
				(p) =>
					p.name.toLowerCase().includes(q) ||
					p.description?.toLowerCase().includes(q)
			);
		}

		return products;
	});

	function getCategoryName(categoryId: string): string {
		const cat = data.categories.find((c: Category) => c.id === categoryId);
		return cat?.name ?? '-';
	}

	function getStationLabel(station: string): string {
		const labels: Record<string, string> = {
			GRILL: 'Grill',
			BEVERAGE: 'Beverage',
			RICE: 'Rice',
			DESSERT: 'Dessert'
		};
		return labels[station] ?? '';
	}
</script>

<svelte:head>
	<title>Menu - Kiwari POS</title>
</svelte:head>

<div class="menu-page">
	<div class="page-header">
		<div class="header-left">
			<h1 class="page-title">Menu</h1>
			<p class="page-subtitle">{data.products.length} produk, {data.categories.length} kategori</p>
		</div>
		<div class="header-actions">
			<button
				type="button"
				class="btn-secondary btn-manage-cat"
				onclick={() => { showCategoryManager = !showCategoryManager; }}
			>
				{showCategoryManager ? 'Tutup Kategori' : 'Kelola Kategori'}
			</button>
			<a href="/menu/new" class="btn-primary btn-new">+ Tambah Produk</a>
		</div>
	</div>

	<!-- Category Manager (toggle panel) -->
	{#if showCategoryManager}
		<div class="category-manager">
			<div class="cat-manager-header">
				<h3 class="cat-manager-title">Kelola Kategori</h3>
				<button type="button" class="btn-add-cat" onclick={() => { showAddCategory = true; editingCategoryId = null; }}>
					+ Tambah
				</button>
			</div>

			{#if form?.categoryError}
				<div class="error-banner">{form.categoryError}</div>
			{/if}

			<!-- Add category form -->
			{#if showAddCategory}
				<form method="POST" action="?/createCategory" use:enhance={() => {
					return async ({ result, update }) => {
						if (result.type === 'success') {
							showAddCategory = false;
						}
						await update();
					};
				}}>
					<div class="cat-form">
						<div class="cat-form-row">
							<input name="name" type="text" class="input-field" placeholder="Nama kategori" required />
							<input name="description" type="text" class="input-field" placeholder="Deskripsi (opsional)" />
							<input name="sort_order" type="number" class="input-field input-sm" value="0" />
						</div>
						<div class="cat-form-actions">
							<button type="submit" class="btn-primary btn-sm">Simpan</button>
							<button type="button" class="btn-secondary btn-sm" onclick={() => { showAddCategory = false; }}>Batal</button>
						</div>
					</div>
				</form>
			{/if}

			<!-- Category list -->
			{#each data.categories as cat (cat.id)}
				<div class="cat-row">
					{#if editingCategoryId === cat.id}
						<form method="POST" action="?/updateCategory" use:enhance={() => {
							return async ({ result, update }) => {
								if (result.type === 'success') {
									editingCategoryId = null;
								}
								await update();
							};
						}}>
							<input type="hidden" name="id" value={cat.id} />
							<div class="cat-form-row">
								<input name="name" type="text" class="input-field" value={cat.name} required />
								<input name="description" type="text" class="input-field" value={cat.description ?? ''} />
								<input name="sort_order" type="number" class="input-field input-sm" value={cat.sort_order} />
							</div>
							<div class="cat-form-actions">
								<button type="submit" class="btn-primary btn-sm">Simpan</button>
								<button type="button" class="btn-secondary btn-sm" onclick={() => { editingCategoryId = null; }}>Batal</button>
							</div>
						</form>
					{:else}
						<div class="cat-info">
							<span class="cat-name">{cat.name}</span>
							{#if cat.description}
								<span class="cat-desc">{cat.description}</span>
							{/if}
							<span class="cat-order">#{cat.sort_order}</span>
						</div>
						<div class="cat-actions">
							<button type="button" class="btn-icon" onclick={() => { editingCategoryId = cat.id; }}>Edit</button>
							<form method="POST" action="?/deleteCategory" use:enhance>
								<input type="hidden" name="id" value={cat.id} />
								<button type="submit" class="btn-icon btn-danger"
									onclick={(e) => { if (!confirm('Hapus kategori "' + cat.name + '"?')) e.preventDefault(); }}>
									Hapus
								</button>
							</form>
						</div>
					{/if}
				</div>
			{/each}

			{#if data.categories.length === 0}
				<p class="empty-text">Belum ada kategori.</p>
			{/if}
		</div>
	{/if}

	<!-- Category tabs -->
	<div class="category-tabs">
		<button
			type="button"
			class="cat-chip"
			class:active={selectedCategoryId === null}
			onclick={() => { selectedCategoryId = null; }}
		>
			Semua
		</button>
		{#each data.categories as cat (cat.id)}
			<button
				type="button"
				class="cat-chip"
				class:active={selectedCategoryId === cat.id}
				onclick={() => { selectedCategoryId = selectedCategoryId === cat.id ? null : cat.id; }}
			>
				{cat.name}
			</button>
		{/each}
	</div>

	<!-- Search bar -->
	<div class="search-bar">
		<input
			type="text"
			class="input-field search-input"
			placeholder="Cari produk..."
			bind:value={searchQuery}
		/>
	</div>

	<!-- Products grid -->
	{#if filteredProducts.length === 0}
		<div class="empty-state">
			<p class="empty-text">
				{searchQuery.trim() ? 'Tidak ada produk yang cocok dengan pencarian.' : 'Belum ada produk di kategori ini.'}
			</p>
		</div>
	{:else}
		<div class="product-grid">
			{#each filteredProducts as product (product.id)}
				<a href="/menu/{product.id}" class="product-card">
					<div class="product-info">
						<span class="product-name">{product.name}</span>
						<span class="product-price">{formatRupiah(product.base_price)}</span>
						<span class="product-category">{getCategoryName(product.category_id)}</span>
					</div>
					<div class="product-meta">
						{#if product.station}
							<span class="station-badge">{getStationLabel(product.station)}</span>
						{/if}
						{#if product.is_combo}
							<span class="combo-badge">Combo</span>
						{/if}
						<span class="status-dot" class:active={product.is_active} class:inactive={!product.is_active}></span>
					</div>
				</a>
			{/each}
		</div>
	{/if}
</div>

<style>
	.menu-page {
		max-width: 1200px;
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
	}

	.header-actions {
		display: flex;
		gap: 8px;
	}

	.btn-manage-cat {
		padding: 8px 16px;
		font-size: 13px;
		cursor: pointer;
	}

	.btn-new {
		padding: 8px 16px;
		font-size: 13px;
		text-decoration: none;
		display: inline-flex;
		align-items: center;
		border: none;
	}

	/* Category Manager Panel */
	.category-manager {
		background-color: var(--color-bg);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-card);
		padding: 16px;
		margin-bottom: 20px;
	}

	.cat-manager-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		margin-bottom: 12px;
	}

	.cat-manager-title {
		font-size: var(--text-title);
		font-weight: 600;
		color: var(--color-text-primary);
		margin: 0;
	}

	.btn-add-cat {
		background: none;
		border: 1px dashed var(--color-primary);
		color: var(--color-primary);
		font-size: 13px;
		font-weight: 600;
		padding: 4px 12px;
		border-radius: var(--radius-btn);
		cursor: pointer;
	}

	.btn-add-cat:hover {
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

	.cat-form {
		padding: 8px 0;
		border-bottom: 1px solid var(--color-border);
	}

	.cat-form-row {
		display: flex;
		gap: 8px;
		align-items: center;
		flex-wrap: wrap;
	}

	.cat-form-row .input-field {
		flex: 1;
		min-width: 120px;
	}

	.input-sm {
		max-width: 80px;
		flex: 0 0 80px !important;
		min-width: 80px !important;
	}

	.cat-form-actions {
		display: flex;
		gap: 8px;
		margin-top: 8px;
	}

	.btn-sm {
		padding: 6px 14px;
		font-size: 13px;
		border: none;
		cursor: pointer;
	}

	.cat-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 8px 0;
		border-bottom: 1px solid var(--color-border);
	}

	.cat-row:last-of-type {
		border-bottom: none;
	}

	.cat-info {
		display: flex;
		align-items: center;
		gap: 12px;
	}

	.cat-name {
		font-size: 13px;
		font-weight: 600;
		color: var(--color-text-primary);
	}

	.cat-desc {
		font-size: 12px;
		color: var(--color-text-secondary);
	}

	.cat-order {
		font-size: 11px;
		color: var(--color-text-secondary);
		background-color: var(--color-surface);
		padding: 2px 6px;
		border-radius: 4px;
	}

	.cat-actions {
		display: flex;
		align-items: center;
		gap: 4px;
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

	.btn-icon:hover {
		background-color: var(--color-surface);
		color: var(--color-text-primary);
	}

	.btn-danger {
		color: var(--color-error);
	}

	.btn-danger:hover {
		background-color: var(--color-error-bg);
		color: var(--color-error);
	}

	.empty-text {
		font-size: 13px;
		color: var(--color-text-secondary);
		margin: 8px 0;
	}

	/* Category Tabs */
	.category-tabs {
		display: flex;
		gap: 8px;
		flex-wrap: wrap;
		margin-bottom: 16px;
	}

	.cat-chip {
		padding: 6px 16px;
		border-radius: var(--radius-chip);
		font-size: 13px;
		font-weight: 500;
		border: 1px solid var(--color-border);
		background-color: var(--color-bg);
		color: var(--color-text-secondary);
		cursor: pointer;
		transition: all 0.15s ease;
	}

	.cat-chip:hover {
		background-color: var(--color-surface);
		color: var(--color-text-primary);
	}

	.cat-chip.active {
		background-color: var(--color-accent);
		border-color: var(--color-accent);
		color: var(--color-text-primary);
		font-weight: 600;
	}

	/* Search */
	.search-bar {
		margin-bottom: 16px;
	}

	.search-input {
		width: 100%;
		max-width: 400px;
		box-sizing: border-box;
	}

	/* Empty State */
	.empty-state {
		text-align: center;
		padding: 48px 24px;
	}

	/* Product Grid */
	.product-grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
		gap: 12px;
	}

	.product-card {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
		padding: 16px;
		background-color: var(--color-bg);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-card);
		text-decoration: none;
		color: inherit;
		transition: border-color 0.15s ease;
	}

	.product-card:hover {
		border-color: var(--color-primary);
	}

	.product-info {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.product-name {
		font-size: 14px;
		font-weight: 600;
		color: var(--color-text-primary);
		line-height: 1.3;
	}

	.product-price {
		font-size: var(--text-price);
		font-weight: 700;
		color: var(--color-primary);
	}

	.product-category {
		font-size: 12px;
		color: var(--color-text-secondary);
	}

	.product-meta {
		display: flex;
		flex-direction: column;
		align-items: flex-end;
		gap: 6px;
	}

	.station-badge {
		font-size: 11px;
		font-weight: 500;
		color: var(--color-text-secondary);
		background-color: var(--color-surface);
		padding: 2px 8px;
		border-radius: 4px;
	}

	.combo-badge {
		font-size: 11px;
		font-weight: 600;
		color: var(--color-primary);
		background-color: color-mix(in srgb, var(--color-primary) 10%, white);
		padding: 2px 8px;
		border-radius: 4px;
	}

	.status-dot {
		width: 8px;
		height: 8px;
		border-radius: 50%;
	}

	.status-dot.active {
		background-color: var(--color-primary);
	}

	.status-dot.inactive {
		background-color: var(--color-text-secondary);
	}
</style>
