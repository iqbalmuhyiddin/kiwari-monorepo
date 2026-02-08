<!--
  Orders page — order list with filters, catering tab, and detail slide-in panel.
  Uses server-side data loading with URL-based filters and SvelteKit form actions.
-->
<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { formatRupiah } from '$lib/utils/format';
	import {
		getStatusLabel,
		getOrderTypeLabel,
		getCateringStatusLabel,
		formatDate
	} from '$lib/utils/labels';
	import OrderDetail from '$lib/components/OrderDetail.svelte';
	import type { Order } from '$lib/types/api';

	let { data, form } = $props();

	// Tab state
	type Tab = 'all' | 'catering';
	let activeTab = $state<Tab>('all');

	// Filter state — syncs with URL params when server data changes
	let filterStatus = $state('');
	let filterType = $state('');
	let filterStartDate = $state('');
	let filterEndDate = $state('');
	let filterSearch = $state('');

	$effect(() => {
		filterStatus = data.filters.status;
		filterType = data.filters.type;
		filterStartDate = data.filters.startDate;
		filterEndDate = data.filters.endDate;
		filterSearch = data.filters.search;
	});

	// Detail panel
	let selectedOrder = $state<Order | null>(null);
	let loadingDetail = $state(false);
	let detailError = $state<string | null>(null);

	// Orders to display — server already filters by type=CATERING when catering tab is active
	let displayOrders = $derived(data.orders);

	function applyFilters() {
		const params = new URLSearchParams();
		if (activeTab === 'catering') {
			params.set('type', 'CATERING');
		} else {
			if (filterStatus) params.set('status', filterStatus);
			if (filterType) params.set('type', filterType);
		}
		if (filterStartDate) params.set('start_date', filterStartDate);
		if (filterEndDate) params.set('end_date', filterEndDate);
		if (filterSearch) params.set('search', filterSearch);
		goto(`/orders?${params.toString()}`);
	}

	function clearFilters() {
		filterStatus = '';
		filterType = '';
		filterStartDate = '';
		filterEndDate = '';
		filterSearch = '';
		goto('/orders');
	}

	function switchTab(tab: Tab) {
		activeTab = tab;
		// When switching to catering, reset type filter since it's implicit
		if (tab === 'catering') {
			filterType = '';
			filterStatus = '';
			const params = new URLSearchParams();
			params.set('type', 'CATERING');
			if (filterStartDate) params.set('start_date', filterStartDate);
			if (filterEndDate) params.set('end_date', filterEndDate);
			goto(`/orders?${params.toString()}`);
		} else {
			goto('/orders');
		}
	}

	function goToPage(newPage: number) {
		const params = new URLSearchParams(page.url.searchParams);
		params.set('page', String(newPage));
		goto(`/orders?${params.toString()}`);
	}

	async function openDetail(order: Order) {
		loadingDetail = true;
		detailError = null;

		try {
			const res = await fetch(`/api/orders/${order.id}`);
			if (res.ok) {
				const detail: Order = await res.json();
				selectedOrder = detail;
			} else {
				// Fallback to basic order info from list (no items/payments)
				selectedOrder = order;
				detailError = 'Gagal memuat detail lengkap pesanan.';
			}
		} catch {
			selectedOrder = order;
			detailError = 'Gagal memuat detail pesanan.';
		} finally {
			loadingDetail = false;
		}
	}

	function closeDetail() {
		selectedOrder = null;
		detailError = null;
	}

	// Local short-format datetime for table view
	function formatDateTime(iso: string): string {
		const d = new Date(iso);
		return d.toLocaleString('id-ID', {
			day: 'numeric',
			month: 'short',
			hour: '2-digit',
			minute: '2-digit',
			timeZone: 'Asia/Jakarta'
		});
	}

	function isFutureDate(dateStr: string): boolean {
		const d = new Date(dateStr);
		const now = new Date();
		return d > now;
	}

	function getRemainingBalance(order: Order): number {
		const total = parseFloat(order.total_amount);
		const paid = (order.payments ?? []).reduce((sum, p) => sum + parseFloat(p.amount), 0);
		return total - paid;
	}

	// Update selected order when form action succeeds (status change)
	$effect(() => {
		if (form?.statusSuccess) {
			selectedOrder = null;
		}
	});
</script>

<svelte:head>
	<title>Pesanan - Kiwari POS</title>
</svelte:head>

<div class="orders-page">
	<div class="page-header">
		<div class="header-left">
			<h1 class="page-title">Pesanan</h1>
			<p class="page-subtitle">{displayOrders.length}{data.hasMore ? '+' : ''} pesanan ditampilkan</p>
		</div>
	</div>

	<!-- Tabs -->
	<div class="tab-bar">
		<button
			type="button"
			class="tab-item"
			class:active={activeTab === 'all'}
			onclick={() => switchTab('all')}
		>
			Semua Pesanan
		</button>
		<button
			type="button"
			class="tab-item"
			class:active={activeTab === 'catering'}
			onclick={() => switchTab('catering')}
		>
			Katering
		</button>
	</div>

	<!-- Filters -->
	<div class="filter-row">
		{#if activeTab === 'all'}
			<select class="input-field filter-select" bind:value={filterStatus} onchange={applyFilters}>
				<option value="">Semua Status</option>
				<option value="NEW">Baru</option>
				<option value="PREPARING">Diproses</option>
				<option value="READY">Siap</option>
				<option value="COMPLETED">Selesai</option>
				<option value="CANCELLED">Dibatalkan</option>
			</select>
			<select class="input-field filter-select" bind:value={filterType} onchange={applyFilters}>
				<option value="">Semua Tipe</option>
				<option value="DINE_IN">Makan di Tempat</option>
				<option value="TAKEAWAY">Bawa Pulang</option>
				<option value="DELIVERY">Pengiriman</option>
				<option value="CATERING">Katering</option>
			</select>
		{/if}
		<input
			type="date"
			class="input-field filter-date"
			bind:value={filterStartDate}
			onchange={applyFilters}
			placeholder="Dari tanggal"
		/>
		<input
			type="date"
			class="input-field filter-date"
			bind:value={filterEndDate}
			onchange={applyFilters}
			placeholder="Sampai tanggal"
		/>
		<input
			type="text"
			class="input-field filter-search"
			bind:value={filterSearch}
			placeholder="Cari nomor pesanan..."
			onkeydown={(e) => { if (e.key === 'Enter') applyFilters(); }}
		/>
		{#if filterStatus || filterType || filterStartDate || filterEndDate || filterSearch}
			<button type="button" class="btn-clear" onclick={clearFilters}>Reset</button>
		{/if}
	</div>

	<!-- Status error from form action -->
	{#if form?.statusError}
		<div class="error-banner">{form.statusError}</div>
	{/if}

	<!-- ALL ORDERS TAB -->
	{#if activeTab === 'all'}
		{#if displayOrders.length === 0}
			<div class="empty-state">
				<p class="empty-text">Tidak ada pesanan ditemukan.</p>
			</div>
		{:else}
			<div class="order-table">
				<div class="table-header">
					<span class="col-number">No. Pesanan</span>
					<span class="col-type">Tipe</span>
					<span class="col-status">Status</span>
					<span class="col-total">Total</span>
					<span class="col-date">Waktu</span>
				</div>
				{#each displayOrders as order (order.id)}
					<button type="button" class="table-row" onclick={() => openDetail(order)}>
						<span class="col-number">
							<span class="order-num">{order.order_number}</span>
							{#if order.table_number}
								<span class="table-num">Meja {order.table_number}</span>
							{/if}
						</span>
						<span class="col-type">
							<span class="type-chip">{getOrderTypeLabel(order.order_type)}</span>
						</span>
						<span class="col-status">
							<span class="status-badge status-{order.status.toLowerCase()}">{getStatusLabel(order.status)}</span>
						</span>
						<span class="col-total">{formatRupiah(order.total_amount)}</span>
						<span class="col-date">{formatDateTime(order.created_at)}</span>
					</button>
				{/each}
			</div>
		{/if}
	{/if}

	<!-- CATERING TAB -->
	{#if activeTab === 'catering'}
		{#if displayOrders.length === 0}
			<div class="empty-state">
				<p class="empty-text">Tidak ada pesanan katering ditemukan.</p>
			</div>
		{:else}
			<div class="catering-list">
				{#each displayOrders as order (order.id)}
					<button type="button" class="catering-card" class:upcoming={order.catering_date && isFutureDate(order.catering_date)} onclick={() => openDetail(order)}>
						<div class="catering-top">
							<span class="order-num">{order.order_number}</span>
							{#if order.catering_status}
								<span class="catering-badge catering-{order.catering_status.toLowerCase()}">{getCateringStatusLabel(order.catering_status)}</span>
							{/if}
						</div>
						{#if order.catering_date}
							<div class="catering-date">
								<span class="date-label">Tanggal Katering</span>
								<span class="date-value" class:future={isFutureDate(order.catering_date)}>{formatDate(order.catering_date)}</span>
							</div>
						{/if}
						<div class="catering-details">
							<div class="detail-col">
								<span class="detail-label">Total</span>
								<span class="detail-value">{formatRupiah(order.total_amount)}</span>
							</div>
							{#if order.catering_dp_amount}
								<div class="detail-col">
									<span class="detail-label">DP</span>
									<span class="detail-value">{formatRupiah(order.catering_dp_amount)}</span>
								</div>
							{/if}
							<div class="detail-col">
								<span class="detail-label">Sisa</span>
								<span class="detail-value remaining">{formatRupiah(getRemainingBalance(order))}</span>
							</div>
						</div>
						{#if order.delivery_address}
							<div class="catering-address">
								<span class="address-label">Alamat:</span>
								<span class="address-value">{order.delivery_address}</span>
							</div>
						{/if}
						{#if order.customer_id}
							<!-- TODO: resolve customer name when API supports it -->
							<div class="catering-customer">
								<span class="customer-label">Pelanggan ID:</span>
								<span class="customer-value">{order.customer_id}</span>
							</div>
						{/if}
						<div class="catering-footer">
							<span class="status-badge status-{order.status.toLowerCase()}">{getStatusLabel(order.status)}</span>
							<span class="catering-time">{formatDateTime(order.created_at)}</span>
						</div>
					</button>
				{/each}
			</div>
		{/if}
	{/if}

	<!-- Pagination -->
	{#if displayOrders.length > 0}
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

<!-- Loading indicator -->
{#if loadingDetail}
	<div class="loading-overlay">
		<div class="loading-spinner"></div>
	</div>
{/if}

<!-- Detail panel -->
{#if selectedOrder}
	<OrderDetail
		order={selectedOrder}
		onClose={closeDetail}
		statusError={form?.statusError ?? detailError}
	/>
{/if}

<style>
	.orders-page {
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

	/* Tabs */
	.tab-bar {
		display: flex;
		gap: 0;
		border-bottom: 2px solid var(--color-border);
		margin-bottom: 16px;
	}

	.tab-item {
		padding: 10px 20px;
		font-size: 14px;
		font-weight: 500;
		color: var(--color-text-secondary);
		background: none;
		border: none;
		border-bottom: 2px solid transparent;
		margin-bottom: -2px;
		cursor: pointer;
		transition: all 0.15s ease;
	}

	.tab-item:hover {
		color: var(--color-text-primary);
	}

	.tab-item.active {
		color: var(--color-primary);
		font-weight: 600;
		border-bottom-color: var(--color-primary);
	}

	/* Filters */
	.filter-row {
		display: flex;
		gap: 8px;
		flex-wrap: wrap;
		margin-bottom: 16px;
		align-items: center;
	}

	.filter-select {
		min-width: 150px;
		max-width: 180px;
		cursor: pointer;
	}

	.filter-date {
		max-width: 160px;
	}

	.filter-search {
		min-width: 180px;
		max-width: 240px;
	}

	.btn-clear {
		background: none;
		border: 1px solid var(--color-border);
		color: var(--color-text-secondary);
		font-size: 13px;
		font-weight: 500;
		padding: 8px 14px;
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

	/* Order Table */
	.order-table {
		border: 1px solid var(--color-border);
		border-radius: var(--radius-card);
		overflow: hidden;
	}

	.table-header {
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

	.table-row {
		display: grid;
		grid-template-columns: 2fr 1.5fr 1fr 1fr 1.2fr;
		gap: 12px;
		padding: 12px 16px;
		border: none;
		border-bottom: 1px solid var(--color-border);
		background: none;
		width: 100%;
		text-align: left;
		cursor: pointer;
		transition: background-color 0.15s ease;
		font-family: inherit;
	}

	.table-row:last-child {
		border-bottom: none;
	}

	.table-row:hover {
		background-color: var(--color-surface);
	}

	.col-number {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}

	.order-num {
		font-size: 13px;
		font-weight: 600;
		color: var(--color-text-primary);
	}

	.table-num {
		font-size: 11px;
		color: var(--color-text-secondary);
	}

	.col-type {
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

	.col-status {
		display: flex;
		align-items: center;
	}

	.col-total {
		font-size: 13px;
		font-weight: 600;
		color: var(--color-text-primary);
		display: flex;
		align-items: center;
	}

	.col-date {
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

	/* Catering list */
	.catering-list {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
		gap: 12px;
	}

	.catering-card {
		display: flex;
		flex-direction: column;
		gap: 10px;
		padding: 16px;
		background-color: var(--color-bg);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-card);
		cursor: pointer;
		transition: border-color 0.15s ease;
		text-align: left;
		font-family: inherit;
		width: 100%;
	}

	.catering-card:hover {
		border-color: var(--color-primary);
	}

	.catering-card.upcoming {
		border-left: 3px solid var(--color-primary);
	}

	.catering-top {
		display: flex;
		align-items: center;
		justify-content: space-between;
	}

	.catering-badge {
		font-size: 11px;
		font-weight: 600;
		padding: 2px 8px;
		border-radius: var(--radius-chip);
	}

	.catering-booked {
		background-color: #dbeafe;
		color: #1e40af;
	}

	.catering-dp_paid {
		background-color: #fef3c7;
		color: #92400e;
	}

	.catering-settled {
		background-color: #dcfce7;
		color: #166534;
	}

	.catering-date {
		display: flex;
		justify-content: space-between;
		align-items: center;
	}

	.date-label {
		font-size: 12px;
		color: var(--color-text-secondary);
	}

	.date-value {
		font-size: 13px;
		font-weight: 600;
		color: var(--color-text-primary);
	}

	.date-value.future {
		color: var(--color-primary);
	}

	.catering-details {
		display: flex;
		gap: 16px;
	}

	.detail-col {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}

	.detail-label {
		font-size: 11px;
		color: var(--color-text-secondary);
	}

	.detail-value {
		font-size: 13px;
		font-weight: 600;
		color: var(--color-text-primary);
	}

	.detail-value.remaining {
		color: var(--color-error);
	}

	.catering-address,
	.catering-customer {
		font-size: 12px;
		color: var(--color-text-secondary);
	}

	.address-label,
	.customer-label {
		font-weight: 500;
	}

	.catering-footer {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding-top: 8px;
		border-top: 1px solid var(--color-border);
	}

	.catering-time {
		font-size: 12px;
		color: var(--color-text-secondary);
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

	/* Loading overlay */
	.loading-overlay {
		position: fixed;
		inset: 0;
		background-color: rgba(0, 0, 0, 0.15);
		z-index: 99;
		display: flex;
		align-items: center;
		justify-content: center;
	}

	.loading-spinner {
		width: 32px;
		height: 32px;
		border: 3px solid var(--color-border);
		border-top-color: var(--color-primary);
		border-radius: 50%;
		animation: spin 0.6s linear infinite;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
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

		.col-date {
			grid-column: 1 / -1;
		}

		.filter-row {
			flex-direction: column;
			align-items: stretch;
		}

		.filter-select,
		.filter-date,
		.filter-search {
			max-width: none;
			min-width: 0;
		}
	}
</style>
