<!--
  Customer detail page — contact info, stats cards, favorite items, order history.
  Data loaded server-side from three API endpoints in parallel.
-->
<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { enhance } from '$app/forms';
	import { formatRupiah } from '$lib/utils/format';
	import { getStatusLabel, getOrderTypeLabel, formatDateTime, formatShortDateTime } from '$lib/utils/labels';
	import StatsCard from '$lib/components/StatsCard.svelte';

	let { data, form } = $props();

	let editing = $state(false);

	// Close edit on successful update
	$effect(() => {
		if (form?.updateSuccess) {
			editing = false;
		}
	});

	function goToOrdersPage(newPage: number) {
		const params = new URLSearchParams(page.url.searchParams);
		params.set('orders_page', String(newPage));
		goto(`/customers/${data.customer.id}?${params.toString()}`);
	}
</script>

<svelte:head>
	<title>{data.customer.name} - Pelanggan - Kiwari POS</title>
</svelte:head>

<div class="detail-page">
	<!-- Back link -->
	<a href="/customers" class="back-link">← Kembali ke Daftar Pelanggan</a>

	<!-- Error banners -->
	{#if form?.updateError}
		<div class="error-banner">{form.updateError}</div>
	{/if}
	{#if form?.deleteError}
		<div class="error-banner">{form.deleteError}</div>
	{/if}

	<!-- Contact info card -->
	<div class="info-card">
		<div class="info-header">
			<h1 class="info-name">{data.customer.name}</h1>
			<div class="info-actions">
				{#if !editing}
					<button type="button" class="btn-secondary btn-sm" onclick={() => { editing = true; }}>Edit</button>
					<form method="POST" action="?/delete" use:enhance>
						<button
							type="submit"
							class="btn-danger-outline btn-sm"
							onclick={(e) => { if (!confirm('Hapus pelanggan "' + data.customer.name + '"? Data akan dinonaktifkan.')) e.preventDefault(); }}
						>
							Hapus
						</button>
					</form>
				{/if}
			</div>
		</div>

		{#if editing}
			<form method="POST" action="?/update" use:enhance={() => {
				return async ({ result, update }) => {
					if (result.type === 'success') {
						editing = false;
					}
					await update();
				};
			}}>
				<div class="edit-grid">
					<div class="edit-field">
						<label class="edit-label">Nama *</label>
						<input name="name" type="text" class="input-field" value={data.customer.name} required />
					</div>
					<div class="edit-field">
						<label class="edit-label">No. HP *</label>
						<input name="phone" type="tel" class="input-field" value={data.customer.phone} required />
					</div>
					<div class="edit-field">
						<label class="edit-label">Email</label>
						<input name="email" type="email" class="input-field" value={data.customer.email ?? ''} />
					</div>
					<div class="edit-field">
						<label class="edit-label">Catatan</label>
						<input name="notes" type="text" class="input-field" value={data.customer.notes ?? ''} />
					</div>
				</div>
				<div class="edit-actions">
					<button type="submit" class="btn-primary btn-sm">Simpan</button>
					<button type="button" class="btn-secondary btn-sm" onclick={() => { editing = false; }}>Batal</button>
				</div>
			</form>
		{:else}
			<div class="info-grid">
				<div class="info-item">
					<span class="info-label">No. HP</span>
					<span class="info-value">{data.customer.phone}</span>
				</div>
				<div class="info-item">
					<span class="info-label">Email</span>
					<span class="info-value">{data.customer.email ?? '-'}</span>
				</div>
				{#if data.customer.notes}
					<div class="info-item full-width">
						<span class="info-label">Catatan</span>
						<span class="info-value">{data.customer.notes}</span>
					</div>
				{/if}
				<div class="info-item">
					<span class="info-label">Terdaftar</span>
					<span class="info-value">{formatDateTime(data.customer.created_at)}</span>
				</div>
			</div>
		{/if}
	</div>

	<!-- Stats cards -->
	{#if data.stats}
		<div class="stats-grid">
			<StatsCard value={formatRupiah(data.stats.total_spend)} label="Total Belanja" />
			<StatsCard value={String(data.stats.total_orders)} label="Kunjungan" />
			<StatsCard
				value={data.stats.total_orders > 0 ? formatRupiah(data.stats.avg_ticket) : 'Rp 0'}
				label="Rata-rata Belanja"
			/>
		</div>
	{/if}

	<!-- Favorite items -->
	{#if data.stats && data.stats.top_items.length > 0}
		<div class="section">
			<h2 class="section-title">Produk Favorit</h2>
			<div class="favorites-list">
				{#each data.stats.top_items as item, i}
					<div class="fav-item">
						<span class="fav-rank">#{i + 1}</span>
						<span class="fav-name">{item.product_name}</span>
						<span class="fav-qty">{item.total_qty}x</span>
					</div>
				{/each}
			</div>
		</div>
	{/if}

	<!-- Order history -->
	<div class="section">
		<h2 class="section-title">Riwayat Pesanan</h2>

		{#if data.orders.length === 0}
			<div class="empty-state">
				<p class="empty-text">Belum ada riwayat pesanan.</p>
			</div>
		{:else}
			<div class="order-table">
				<div class="order-header">
					<span class="ocol-number">No. Pesanan</span>
					<span class="ocol-type">Tipe</span>
					<span class="ocol-status">Status</span>
					<span class="ocol-total">Total</span>
					<span class="ocol-date">Waktu</span>
				</div>
				{#each data.orders as order (order.id)}
					<div class="order-row">
						<span class="ocol-number">
							<span class="order-num">{order.order_number}</span>
						</span>
						<span class="ocol-type">
							<span class="type-chip">{getOrderTypeLabel(order.order_type)}</span>
						</span>
						<span class="ocol-status">
							<span class="status-badge status-{order.status.toLowerCase()}">{getStatusLabel(order.status)}</span>
						</span>
						<span class="ocol-total">{formatRupiah(order.total_amount)}</span>
						<span class="ocol-date">{formatShortDateTime(order.created_at)}</span>
					</div>
				{/each}
			</div>

			<!-- Order pagination -->
			<div class="pagination">
				<button
					type="button"
					class="btn-secondary btn-page"
					disabled={data.ordersPage <= 1}
					onclick={() => goToOrdersPage(data.ordersPage - 1)}
				>
					Sebelumnya
				</button>
				<span class="page-info">Halaman {data.ordersPage}</span>
				<button
					type="button"
					class="btn-secondary btn-page"
					disabled={!data.ordersHasMore}
					onclick={() => goToOrdersPage(data.ordersPage + 1)}
				>
					Berikutnya
				</button>
			</div>
		{/if}
	</div>
</div>

<style>
	.detail-page {
		max-width: 900px;
	}

	.back-link {
		display: inline-block;
		font-size: 13px;
		font-weight: 500;
		color: var(--color-text-secondary);
		text-decoration: none;
		margin-bottom: 16px;
		transition: color 0.15s ease;
	}

	.back-link:hover {
		color: var(--color-primary);
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

	/* Info card */
	.info-card {
		background-color: var(--color-bg);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-card);
		padding: 20px;
		margin-bottom: 20px;
	}

	.info-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		margin-bottom: 16px;
	}

	.info-name {
		font-size: 20px;
		font-weight: 700;
		color: var(--color-text-primary);
		margin: 0;
	}

	.info-actions {
		display: flex;
		gap: 8px;
	}

	.btn-sm {
		padding: 6px 14px;
		font-size: 13px;
		border: none;
		cursor: pointer;
	}

	.btn-danger-outline {
		background: none;
		border: 1px solid var(--color-error);
		color: var(--color-error);
		border-radius: var(--radius-btn);
		font-weight: 600;
		transition: all 0.15s ease;
	}

	.btn-danger-outline:hover {
		background-color: var(--color-error-bg);
	}

	.info-grid {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 16px;
	}

	.info-item {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}

	.info-item.full-width {
		grid-column: 1 / -1;
	}

	.info-label {
		font-size: 12px;
		font-weight: 500;
		color: var(--color-text-secondary);
	}

	.info-value {
		font-size: 14px;
		color: var(--color-text-primary);
	}

	/* Edit form */
	.edit-grid {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 12px;
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
		margin-top: 12px;
	}

	/* Stats grid */
	.stats-grid {
		display: grid;
		grid-template-columns: repeat(3, 1fr);
		gap: 16px;
		margin-bottom: 24px;
	}

	/* Sections */
	.section {
		margin-bottom: 24px;
	}

	.section-title {
		font-size: var(--text-title);
		font-weight: 600;
		color: var(--color-text-primary);
		margin: 0 0 12px;
	}

	/* Favorites */
	.favorites-list {
		background-color: var(--color-bg);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-card);
		overflow: hidden;
	}

	.fav-item {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 10px 16px;
		border-bottom: 1px solid var(--color-border);
	}

	.fav-item:last-child {
		border-bottom: none;
	}

	.fav-rank {
		font-size: 12px;
		font-weight: 700;
		color: var(--color-primary);
		min-width: 24px;
	}

	.fav-name {
		font-size: 13px;
		font-weight: 500;
		color: var(--color-text-primary);
		flex: 1;
	}

	.fav-qty {
		font-size: 13px;
		font-weight: 600;
		color: var(--color-text-secondary);
	}

	/* Order table */
	.order-table {
		border: 1px solid var(--color-border);
		border-radius: var(--radius-card);
		overflow: hidden;
	}

	.order-header {
		display: grid;
		grid-template-columns: 2fr 1.5fr 1fr 1fr 1.2fr;
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

	.order-row {
		display: grid;
		grid-template-columns: 2fr 1.5fr 1fr 1fr 1.2fr;
		gap: 12px;
		padding: 12px 16px;
		border-bottom: 1px solid var(--color-border);
	}

	.order-row:last-child {
		border-bottom: none;
	}

	.order-num {
		font-size: 13px;
		font-weight: 600;
		color: var(--color-text-primary);
	}

	.ocol-type {
		display: flex;
		align-items: center;
	}

	.type-chip {
		font-size: 12px;
		font-weight: 500;
		color: var(--color-text-secondary);
		background-color: var(--color-surface);
		padding: 2px 8px;
		border-radius: var(--radius-chip);
	}

	.ocol-status {
		display: flex;
		align-items: center;
	}

	.ocol-total {
		font-size: 13px;
		font-weight: 600;
		color: var(--color-text-primary);
		display: flex;
		align-items: center;
	}

	.ocol-date {
		font-size: 12px;
		color: var(--color-text-secondary);
		display: flex;
		align-items: center;
	}

	/* Status badges */
	.status-badge {
		font-size: 11px;
		font-weight: 600;
		padding: 2px 8px;
		border-radius: var(--radius-chip);
		display: inline-block;
	}

	.status-new {
		background-color: #dbeafe;
		color: #1e40af;
	}

	.status-preparing {
		background-color: #fef3c7;
		color: #92400e;
	}

	.status-ready {
		background-color: #dcfce7;
		color: #166534;
	}

	.status-completed {
		background-color: var(--color-surface);
		color: var(--color-text-secondary);
	}

	.status-cancelled {
		background-color: var(--color-error-bg);
		color: var(--color-error);
	}

	/* Empty state */
	.empty-state {
		text-align: center;
		padding: 32px 24px;
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
		margin-top: 16px;
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
		.info-grid,
		.edit-grid {
			grid-template-columns: 1fr;
		}

		.stats-grid {
			grid-template-columns: 1fr;
		}

		.order-header {
			display: none;
		}

		.order-row {
			grid-template-columns: 1fr 1fr;
			grid-template-rows: auto auto;
			gap: 6px;
		}

		.ocol-date {
			grid-column: 1 / -1;
		}

		.info-header {
			flex-direction: column;
			align-items: flex-start;
			gap: 12px;
		}
	}
</style>
