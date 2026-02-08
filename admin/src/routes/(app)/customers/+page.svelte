<!--
  Customer list page — search, pagination, add/edit/delete customers.
  Uses server-side data loading with URL-based search and SvelteKit form actions.
-->
<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { enhance } from '$app/forms';
	import { formatDateTime } from '$lib/utils/labels';

	let { data, form } = $props();

	let search = $state(data.search);
	let showAddForm = $state(false);
	let editingId = $state<string | null>(null);

	// Sync search state when server data changes (e.g. back/forward navigation)
	$effect(() => {
		search = data.search;
	});

	// Close add form on successful create
	$effect(() => {
		if (form?.createSuccess) {
			showAddForm = false;
		}
	});

	// Close edit form on successful update
	$effect(() => {
		if (form?.updateSuccess) {
			editingId = null;
		}
	});

	function applySearch() {
		const params = new URLSearchParams();
		if (search.trim()) params.set('search', search.trim());
		goto(`/customers?${params.toString()}`);
	}

	function clearSearch() {
		search = '';
		goto('/customers');
	}

	function goToPage(newPage: number) {
		const params = new URLSearchParams(page.url.searchParams);
		params.set('page', String(newPage));
		goto(`/customers?${params.toString()}`);
	}
</script>

<svelte:head>
	<title>Pelanggan - Kiwari POS</title>
</svelte:head>

<div class="customers-page">
	<div class="page-header">
		<div class="header-left">
			<h1 class="page-title">Pelanggan</h1>
			<p class="page-subtitle">{data.customers.length}{data.hasMore ? '+' : ''} pelanggan ditampilkan</p>
		</div>
		<div class="header-actions">
			<button
				type="button"
				class="btn-primary btn-add"
				onclick={() => { showAddForm = !showAddForm; editingId = null; }}
			>
				{showAddForm ? 'Tutup' : '+ Tambah Pelanggan'}
			</button>
		</div>
	</div>

	<!-- Search -->
	<div class="search-row">
		<input
			type="text"
			class="input-field search-input"
			placeholder="Cari nama atau nomor HP..."
			bind:value={search}
			onkeydown={(e) => { if (e.key === 'Enter') applySearch(); }}
		/>
		<button type="button" class="btn-secondary btn-search" onclick={applySearch}>Cari</button>
		{#if data.search}
			<button type="button" class="btn-clear" onclick={clearSearch}>Reset</button>
		{/if}
	</div>

	<!-- Error banners -->
	{#if form?.createError}
		<div class="error-banner">{form.createError}</div>
	{/if}
	{#if form?.updateError}
		<div class="error-banner">{form.updateError}</div>
	{/if}
	{#if form?.deleteError}
		<div class="error-banner">{form.deleteError}</div>
	{/if}

	<!-- Add customer form -->
	{#if showAddForm}
		<div class="add-form-card">
			<h3 class="form-title">Tambah Pelanggan</h3>
			<form method="POST" action="?/create" use:enhance={() => {
				return async ({ result, update }) => {
					if (result.type === 'success') {
						showAddForm = false;
					}
					await update();
				};
			}}>
				<div class="form-grid">
					<div class="form-group">
						<label for="add-name" class="form-label">Nama *</label>
						<input id="add-name" name="name" type="text" class="input-field" placeholder="Nama pelanggan" required />
					</div>
					<div class="form-group">
						<label for="add-phone" class="form-label">No. HP *</label>
						<input id="add-phone" name="phone" type="tel" class="input-field" placeholder="08xxxxxxxxxx" required />
					</div>
					<div class="form-group">
						<label for="add-email" class="form-label">Email</label>
						<input id="add-email" name="email" type="email" class="input-field" placeholder="email@contoh.com" />
					</div>
					<div class="form-group">
						<label for="add-notes" class="form-label">Catatan</label>
						<input id="add-notes" name="notes" type="text" class="input-field" placeholder="Catatan tambahan" />
					</div>
				</div>
				<div class="form-actions">
					<button type="submit" class="btn-primary btn-sm">Simpan</button>
					<button type="button" class="btn-secondary btn-sm" onclick={() => { showAddForm = false; }}>Batal</button>
				</div>
			</form>
		</div>
	{/if}

	<!-- Customer list -->
	{#if data.customers.length === 0}
		<div class="empty-state">
			<p class="empty-text">
				{data.search ? 'Tidak ada pelanggan yang cocok dengan pencarian.' : 'Belum ada data pelanggan.'}
			</p>
		</div>
	{:else}
		<div class="customer-table">
			<div class="table-header">
				<span class="col-name">Nama</span>
				<span class="col-phone">No. HP</span>
				<span class="col-email">Email</span>
				<span class="col-date">Terdaftar</span>
				<span class="col-actions">Aksi</span>
			</div>
			{#each data.customers as customer (customer.id)}
				{#if editingId === customer.id}
					<!-- Inline edit form -->
					<div class="table-row edit-row">
						<form method="POST" action="?/update" use:enhance={() => {
							return async ({ result, update }) => {
								if (result.type === 'success') {
									editingId = null;
								}
								await update();
							};
						}}>
							<input type="hidden" name="id" value={customer.id} />
							<div class="edit-grid">
								<div class="edit-field">
									<label class="edit-label">Nama *</label>
									<input name="name" type="text" class="input-field" value={customer.name} required />
								</div>
								<div class="edit-field">
									<label class="edit-label">No. HP *</label>
									<input name="phone" type="tel" class="input-field" value={customer.phone} required />
								</div>
								<div class="edit-field">
									<label class="edit-label">Email</label>
									<input name="email" type="email" class="input-field" value={customer.email ?? ''} />
								</div>
								<div class="edit-field">
									<label class="edit-label">Catatan</label>
									<input name="notes" type="text" class="input-field" value={customer.notes ?? ''} />
								</div>
							</div>
							<div class="edit-actions">
								<button type="submit" class="btn-primary btn-sm">Simpan</button>
								<button type="button" class="btn-secondary btn-sm" onclick={() => { editingId = null; }}>Batal</button>
							</div>
						</form>
					</div>
				{:else}
					<!-- Normal row -->
					<a href="/customers/{customer.id}" class="table-row">
						<span class="col-name">
							<span class="customer-name">{customer.name}</span>
							{#if customer.notes}
								<span class="customer-notes">{customer.notes}</span>
							{/if}
						</span>
						<span class="col-phone">{customer.phone}</span>
						<span class="col-email">{customer.email ?? '-'}</span>
						<span class="col-date">{formatDateTime(customer.created_at)}</span>
						<span class="col-actions" onclick={(e) => e.preventDefault()}>
							<button type="button" class="btn-icon" onclick={(e) => { e.preventDefault(); e.stopPropagation(); editingId = customer.id; }}>Edit</button>
							<form method="POST" action="?/delete" use:enhance onclick={(e) => e.stopPropagation()}>
								<input type="hidden" name="id" value={customer.id} />
								<button
									type="submit"
									class="btn-icon btn-danger"
									onclick={(e) => { if (!confirm('Hapus pelanggan "' + customer.name + '"?')) e.preventDefault(); }}
								>
									Hapus
								</button>
							</form>
						</span>
					</a>
				{/if}
			{/each}
		</div>
	{/if}

	<!-- Pagination -->
	{#if data.customers.length > 0}
		<div class="pagination">
			<button
				type="button"
				class="btn-secondary btn-page"
				disabled={data.page <= 1}
				onclick={() => goToPage(data.page - 1)}
			>
				Sebelumnya
			</button>
			<span class="page-info">Halaman {data.page}</span>
			<button
				type="button"
				class="btn-secondary btn-page"
				disabled={!data.hasMore}
				onclick={() => goToPage(data.page + 1)}
			>
				Berikutnya
			</button>
		</div>
	{/if}
</div>

<style>
	.customers-page {
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

	.btn-add {
		padding: 8px 16px;
		font-size: 13px;
		border: none;
		cursor: pointer;
	}

	/* Search */
	.search-row {
		display: flex;
		gap: 8px;
		align-items: center;
		margin-bottom: 16px;
	}

	.search-input {
		flex: 1;
		max-width: 400px;
		box-sizing: border-box;
	}

	.btn-search {
		padding: 10px 16px;
		font-size: 13px;
		cursor: pointer;
	}

	.btn-clear {
		background: none;
		border: 1px solid var(--color-border);
		color: var(--color-text-secondary);
		font-size: 13px;
		font-weight: 500;
		padding: 10px 14px;
		border-radius: var(--radius-btn);
		cursor: pointer;
		transition: all 0.15s ease;
	}

	.btn-clear:hover {
		background-color: var(--color-surface);
		color: var(--color-text-primary);
	}

	/* Error banner */
	.error-banner {
		background-color: var(--color-error-bg);
		color: var(--color-error);
		font-size: 13px;
		font-weight: 500;
		padding: 8px 12px;
		border-radius: var(--radius-chip);
		margin-bottom: 12px;
	}

	/* Add form */
	.add-form-card {
		background-color: var(--color-bg);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-card);
		padding: 16px;
		margin-bottom: 16px;
	}

	.form-title {
		font-size: var(--text-title);
		font-weight: 600;
		color: var(--color-text-primary);
		margin: 0 0 12px;
	}

	.form-grid {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 12px;
	}

	.form-group {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.form-label {
		font-size: 12px;
		font-weight: 500;
		color: var(--color-text-secondary);
	}

	.form-actions {
		display: flex;
		gap: 8px;
		margin-top: 12px;
	}

	.btn-sm {
		padding: 6px 14px;
		font-size: 13px;
		border: none;
		cursor: pointer;
	}

	/* Table */
	.customer-table {
		border: 1px solid var(--color-border);
		border-radius: var(--radius-card);
		overflow: hidden;
	}

	.table-header {
		display: grid;
		grid-template-columns: 2fr 1.2fr 1.5fr 1.2fr 120px;
		gap: 12px;
		padding: 10px 16px;
		background-color: var(--color-surface);
		border-bottom: 1px solid var(--color-border);
		font-size: 12px;
		font-weight: 600;
		color: var(--color-text-secondary);
		text-transform: uppercase;
		letter-spacing: 0.02em;
	}

	.table-row {
		display: grid;
		grid-template-columns: 2fr 1.2fr 1.5fr 1.2fr 120px;
		gap: 12px;
		padding: 12px 16px;
		border-bottom: 1px solid var(--color-border);
		background: none;
		width: 100%;
		text-align: left;
		cursor: pointer;
		transition: background-color 0.15s ease;
		text-decoration: none;
		color: inherit;
		font-family: inherit;
	}

	.table-row:last-child {
		border-bottom: none;
	}

	.table-row:hover {
		background-color: var(--color-surface);
	}

	.table-row.edit-row {
		display: block;
		padding: 16px;
		cursor: default;
	}

	.col-name {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}

	.customer-name {
		font-size: 13px;
		font-weight: 600;
		color: var(--color-text-primary);
	}

	.customer-notes {
		font-size: 11px;
		color: var(--color-text-secondary);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.col-phone {
		font-size: 13px;
		color: var(--color-text-primary);
		display: flex;
		align-items: center;
	}

	.col-email {
		font-size: 13px;
		color: var(--color-text-secondary);
		display: flex;
		align-items: center;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.col-date {
		font-size: 12px;
		color: var(--color-text-secondary);
		display: flex;
		align-items: center;
	}

	.col-actions {
		display: flex;
		align-items: center;
		gap: 4px;
	}

	/* Inline edit */
	.edit-grid {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 10px;
	}

	.edit-field {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.edit-label {
		font-size: 12px;
		font-weight: 500;
		color: var(--color-text-secondary);
	}

	.edit-actions {
		display: flex;
		gap: 8px;
		margin-top: 10px;
	}

	/* Action buttons */
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

	/* Empty state */
	.empty-state {
		text-align: center;
		padding: 48px 24px;
	}

	.empty-text {
		font-size: 13px;
		color: var(--color-text-secondary);
		margin: 0;
	}

	/* Pagination */
	.pagination {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 16px;
		margin-top: 20px;
		padding: 12px 0;
	}

	.btn-page {
		padding: 8px 16px;
		font-size: 13px;
		cursor: pointer;
	}

	.btn-page:disabled {
		opacity: 0.4;
		cursor: not-allowed;
	}

	.page-info {
		font-size: 13px;
		color: var(--color-text-secondary);
		font-weight: 500;
	}

	@media (max-width: 768px) {
		.table-header {
			display: none;
		}

		.table-row {
			grid-template-columns: 1fr 1fr;
			grid-template-rows: auto auto;
			gap: 6px;
		}

		.col-email,
		.col-date {
			grid-column: 1 / -1;
		}

		.form-grid,
		.edit-grid {
			grid-template-columns: 1fr;
		}

		.search-row {
			flex-wrap: wrap;
		}

		.search-input {
			max-width: none;
		}
	}
</style>
