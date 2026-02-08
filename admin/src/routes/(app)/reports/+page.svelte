<!--
  Reports page — date-filtered reports with 4 tabs:
  Penjualan (daily sales), Produk (product ranking),
  Pembayaran (payment methods), Per Outlet (owner only).
  Pure CSS charts, client-side CSV export, URL-based tab state.
-->
<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { formatRupiah } from '$lib/utils/format';
	import type { DailySales, ProductSales, PaymentSummary, OutletComparison } from '$lib/types/api';

	let { data } = $props();

	type Tab = 'penjualan' | 'produk' | 'pembayaran' | 'outlet';

	// Tab from URL param (default "penjualan")
	let activeTab = $derived<Tab>((page.url.searchParams.get('tab') as Tab) ?? 'penjualan');

	// Date range state — syncs from server data
	let startDate = $state(data.startDate);
	let endDate = $state(data.endDate);

	$effect(() => {
		startDate = data.startDate;
		endDate = data.endDate;
	});

	// ── Navigation helpers ──────────────────────

	function switchTab(tab: Tab) {
		const params = new URLSearchParams();
		params.set('tab', tab);
		params.set('start_date', startDate);
		params.set('end_date', endDate);
		goto(`/reports?${params.toString()}`);
	}

	function applyDateRange() {
		const params = new URLSearchParams();
		params.set('tab', activeTab);
		params.set('start_date', startDate);
		params.set('end_date', endDate);
		goto(`/reports?${params.toString()}`);
	}

	// ── Compact format for chart labels ──────────────────────

	function formatCompact(amount: number): string {
		if (amount >= 1_000_000) return `${(amount / 1_000_000).toFixed(1)}jt`;
		if (amount >= 1_000) return `${(amount / 1_000).toFixed(0)}rb`;
		return amount.toString();
	}

	// ── Daily sales chart data ──────────────────────

	let maxDailyRevenue = $derived(
		Math.max(...data.dailySales.map((d: DailySales) => parseFloat(d.net_revenue)), 1)
	);

	// ── Sales totals ──────────────────────

	let salesTotals = $derived.by(() => {
		let totalOrders = 0;
		let totalRevenue = 0;
		let totalDiscount = 0;
		let totalNet = 0;
		for (const d of data.dailySales) {
			totalOrders += d.order_count;
			totalRevenue += parseFloat(d.total_revenue);
			totalDiscount += parseFloat(d.total_discount);
			totalNet += parseFloat(d.net_revenue);
		}
		return { totalOrders, totalRevenue, totalDiscount, totalNet };
	});

	// ── Product sales chart data ──────────────────────

	let maxProductRevenue = $derived(
		Math.max(...data.productSales.map((p: ProductSales) => parseFloat(p.total_revenue)), 1)
	);

	// ── Payment chart data ──────────────────────

	let totalPaymentAmount = $derived(
		data.paymentSummary.reduce((sum: number, p: PaymentSummary) => sum + parseFloat(p.total_amount), 0)
	);

	let paymentColors: Record<string, string> = {
		CASH: '#0c7721',
		QRIS: '#2563eb',
		TRANSFER: '#d97706'
	};

	function getPaymentColor(method: string): string {
		return paymentColors[method] ?? '#6b7280';
	}

	function getPaymentLabel(method: string): string {
		const labels: Record<string, string> = {
			CASH: 'Tunai',
			QRIS: 'QRIS',
			TRANSFER: 'Transfer'
		};
		return labels[method] ?? method;
	}

	// ── Outlet comparison chart ──────────────────────

	let maxOutletRevenue = $derived(
		Math.max(...data.outletComparison.map((o: OutletComparison) => parseFloat(o.total_revenue)), 1)
	);

	// ── CSV export ──────────────────────

	function escapeCsvField(field: string): string {
		if (field.includes(',') || field.includes('"') || field.includes('\n')) {
			return `"${field.replace(/"/g, '""')}"`;
		}
		return field;
	}

	function downloadCsv(filename: string, headers: string[], rows: string[][]) {
		const csvContent = [
			headers.map(escapeCsvField).join(','),
			...rows.map((r) => r.map(escapeCsvField).join(','))
		].join('\n');
		const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
		const url = URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		a.download = filename;
		a.click();
		URL.revokeObjectURL(url);
	}

	function exportDailySales() {
		const headers = ['Tanggal', 'Jumlah Pesanan', 'Pendapatan Kotor', 'Diskon', 'Pendapatan Bersih'];
		const rows = data.dailySales.map((d: DailySales) => [
			d.date,
			String(d.order_count),
			d.total_revenue,
			d.total_discount,
			d.net_revenue
		]);
		downloadCsv(`penjualan_${startDate}_${endDate}.csv`, headers, rows);
	}

	function exportProductSales() {
		const headers = ['Nama Produk', 'Terjual', 'Pendapatan'];
		const rows = data.productSales.map((p: ProductSales) => [
			p.product_name,
			String(p.quantity_sold),
			p.total_revenue
		]);
		downloadCsv(`produk_${startDate}_${endDate}.csv`, headers, rows);
	}

	function exportPaymentSummary() {
		const headers = ['Metode Pembayaran', 'Jumlah Transaksi', 'Total'];
		const rows = data.paymentSummary.map((p: PaymentSummary) => [
			p.payment_method,
			String(p.transaction_count),
			p.total_amount
		]);
		downloadCsv(`pembayaran_${startDate}_${endDate}.csv`, headers, rows);
	}

	function exportOutletComparison() {
		const headers = ['Outlet', 'Total Pesanan', 'Pendapatan'];
		const rows = data.outletComparison.map((o: OutletComparison) => [
			o.outlet_name,
			String(o.order_count),
			o.total_revenue
		]);
		downloadCsv(`outlet_${startDate}_${endDate}.csv`, headers, rows);
	}

	// ── Date formatting ──────────────────────

	function formatShortDate(dateStr: string): string {
		const d = new Date(dateStr + 'T00:00:00');
		return d.toLocaleDateString('id-ID', {
			day: 'numeric',
			month: 'short',
			timeZone: 'Asia/Jakarta'
		});
	}
</script>

<svelte:head>
	<title>Laporan - Kiwari POS</title>
</svelte:head>

<div class="reports-page">
	<div class="page-header">
		<h1 class="page-title">Laporan</h1>
		<p class="page-subtitle">
			{new Date(startDate + 'T00:00:00').toLocaleDateString('id-ID', { day: 'numeric', month: 'long', year: 'numeric' })}
			&mdash;
			{new Date(endDate + 'T00:00:00').toLocaleDateString('id-ID', { day: 'numeric', month: 'long', year: 'numeric' })}
		</p>
	</div>

	<!-- Date range picker -->
	<div class="date-range">
		<label class="date-label">
			<span class="label-text">Tanggal Mulai</span>
			<input type="date" class="input-field date-input" bind:value={startDate} />
		</label>
		<label class="date-label">
			<span class="label-text">Tanggal Akhir</span>
			<input type="date" class="input-field date-input" bind:value={endDate} />
		</label>
		<button type="button" class="btn-primary btn-apply" onclick={applyDateRange}>
			Terapkan
		</button>
	</div>

	<!-- Tab bar -->
	<div class="tab-bar">
		<button
			type="button"
			class="tab-item"
			class:active={activeTab === 'penjualan'}
			onclick={() => switchTab('penjualan')}
		>
			Penjualan
		</button>
		<button
			type="button"
			class="tab-item"
			class:active={activeTab === 'produk'}
			onclick={() => switchTab('produk')}
		>
			Produk
		</button>
		<button
			type="button"
			class="tab-item"
			class:active={activeTab === 'pembayaran'}
			onclick={() => switchTab('pembayaran')}
		>
			Pembayaran
		</button>
		{#if data.userRole === 'OWNER'}
			<button
				type="button"
				class="tab-item"
				class:active={activeTab === 'outlet'}
				onclick={() => switchTab('outlet')}
			>
				Per Outlet
			</button>
		{/if}
	</div>

	<!-- ═══════════════════════════════════════════════ -->
	<!-- TAB: Penjualan (Daily Sales)                    -->
	<!-- ═══════════════════════════════════════════════ -->
	{#if activeTab === 'penjualan'}
		<div class="tab-content">
			<div class="tab-toolbar">
				<h2 class="section-title">Penjualan Harian</h2>
				<button type="button" class="btn-secondary btn-export" onclick={exportDailySales}>
					Unduh CSV
				</button>
			</div>

			<!-- Summary cards -->
			<div class="summary-row">
				<div class="summary-card">
					<span class="summary-value">{salesTotals.totalOrders}</span>
					<span class="summary-label">Total Pesanan</span>
				</div>
				<div class="summary-card">
					<span class="summary-value">{formatRupiah(salesTotals.totalRevenue)}</span>
					<span class="summary-label">Pendapatan Kotor</span>
				</div>
				<div class="summary-card">
					<span class="summary-value">{formatRupiah(salesTotals.totalDiscount)}</span>
					<span class="summary-label">Total Diskon</span>
				</div>
				<div class="summary-card">
					<span class="summary-value">{formatRupiah(salesTotals.totalNet)}</span>
					<span class="summary-label">Pendapatan Bersih</span>
				</div>
			</div>

			<!-- Bar chart -->
			{#if data.dailySales.length > 0}
				<div class="chart-container">
					<div class="chart-header">
						<h3 class="chart-title">Pendapatan Bersih per Hari</h3>
					</div>
					<div class="chart">
						<div class="bars">
							{#each data.dailySales as day}
								<div
									class="bar-group"
									title="{day.date} — {day.order_count} pesanan, {formatRupiah(day.net_revenue)}"
								>
									<div class="bar-value">
										{#if parseFloat(day.net_revenue) > 0}
											{formatCompact(parseFloat(day.net_revenue))}
										{/if}
									</div>
									<div class="bar-track">
										<div
											class="bar-fill"
											style="height: {(parseFloat(day.net_revenue) / maxDailyRevenue) * 100}%"
										></div>
									</div>
									<div class="bar-label">{formatShortDate(day.date)}</div>
								</div>
							{/each}
						</div>
					</div>
				</div>
			{/if}

			<!-- Data table -->
			{#if data.dailySales.length === 0}
				<div class="empty-state">
					<p class="empty-text">Tidak ada data penjualan untuk periode ini.</p>
				</div>
			{:else}
				<div class="data-table">
					<div class="table-header sales-cols">
						<span>Tanggal</span>
						<span class="col-right">Pesanan</span>
						<span class="col-right">Pendapatan</span>
						<span class="col-right">Diskon</span>
						<span class="col-right">Bersih</span>
					</div>
					{#each data.dailySales as day (day.date)}
						<div class="table-row sales-cols">
							<span class="col-date-cell">{formatShortDate(day.date)}</span>
							<span class="col-right">{day.order_count}</span>
							<span class="col-right">{formatRupiah(day.total_revenue)}</span>
							<span class="col-right col-discount">{formatRupiah(day.total_discount)}</span>
							<span class="col-right col-net">{formatRupiah(day.net_revenue)}</span>
						</div>
					{/each}
				</div>
			{/if}
		</div>
	{/if}

	<!-- ═══════════════════════════════════════════════ -->
	<!-- TAB: Produk (Product Sales Ranking)             -->
	<!-- ═══════════════════════════════════════════════ -->
	{#if activeTab === 'produk'}
		<div class="tab-content">
			<div class="tab-toolbar">
				<h2 class="section-title">Penjualan Produk</h2>
				<button type="button" class="btn-secondary btn-export" onclick={exportProductSales}>
					Unduh CSV
				</button>
			</div>

			{#if data.productSales.length === 0}
				<div class="empty-state">
					<p class="empty-text">Tidak ada data produk untuk periode ini.</p>
				</div>
			{:else}
				<!-- Horizontal bar chart (top 10) -->
				<div class="chart-container">
					<div class="chart-header">
						<h3 class="chart-title">Pendapatan per Produk (Top {Math.min(data.productSales.length, 10)})</h3>
					</div>
					<div class="horizontal-bars">
						{#each data.productSales.slice(0, 10) as product, i}
							<div class="hbar-row" title="{product.product_name}: {product.quantity_sold} terjual, {formatRupiah(product.total_revenue)}">
								<div class="hbar-rank">{i + 1}</div>
								<div class="hbar-name">{product.product_name}</div>
								<div class="hbar-track">
									<div
										class="hbar-fill"
										style="width: {(parseFloat(product.total_revenue) / maxProductRevenue) * 100}%"
									></div>
								</div>
								<div class="hbar-value">{formatCompact(parseFloat(product.total_revenue))}</div>
							</div>
						{/each}
					</div>
				</div>

				<!-- Full table -->
				<div class="data-table">
					<div class="table-header product-cols">
						<span>#</span>
						<span>Nama Produk</span>
						<span class="col-right">Terjual</span>
						<span class="col-right">Pendapatan</span>
					</div>
					{#each data.productSales as product, i (product.product_id)}
						<div class="table-row product-cols">
							<span class="col-rank">{i + 1}</span>
							<span class="col-product-name">{product.product_name}</span>
							<span class="col-right">{product.quantity_sold}</span>
							<span class="col-right col-net">{formatRupiah(product.total_revenue)}</span>
						</div>
					{/each}
				</div>
			{/if}
		</div>
	{/if}

	<!-- ═══════════════════════════════════════════════ -->
	<!-- TAB: Pembayaran (Payment Method Summary)        -->
	<!-- ═══════════════════════════════════════════════ -->
	{#if activeTab === 'pembayaran'}
		<div class="tab-content">
			<div class="tab-toolbar">
				<h2 class="section-title">Ringkasan Pembayaran</h2>
				<button type="button" class="btn-secondary btn-export" onclick={exportPaymentSummary}>
					Unduh CSV
				</button>
			</div>

			{#if data.paymentSummary.length === 0}
				<div class="empty-state">
					<p class="empty-text">Tidak ada data pembayaran untuk periode ini.</p>
				</div>
			{:else}
				<!-- Payment method cards with proportional bars -->
				<div class="payment-cards">
					{#each data.paymentSummary as payment}
						<div class="payment-card">
							<div class="payment-header">
								<span class="payment-method" style="color: {getPaymentColor(payment.payment_method)}">
									{getPaymentLabel(payment.payment_method)}
								</span>
								<span class="payment-pct">
									{totalPaymentAmount > 0 ? ((parseFloat(payment.total_amount) / totalPaymentAmount) * 100).toFixed(1) : 0}%
								</span>
							</div>
							<div class="payment-amount">{formatRupiah(payment.total_amount)}</div>
							<div class="payment-count">{payment.transaction_count} transaksi</div>
							<div class="payment-bar-track">
								<div
									class="payment-bar-fill"
									style="width: {totalPaymentAmount > 0 ? (parseFloat(payment.total_amount) / totalPaymentAmount) * 100 : 0}%; background-color: {getPaymentColor(payment.payment_method)}"
								></div>
							</div>
						</div>
					{/each}
				</div>

				<!-- Stacked bar (total composition) -->
				<div class="chart-container">
					<div class="chart-header">
						<h3 class="chart-title">Komposisi Pembayaran</h3>
					</div>
					<div class="stacked-bar-container">
						<div class="stacked-bar">
							{#each data.paymentSummary as payment}
								{@const pct = totalPaymentAmount > 0 ? (parseFloat(payment.total_amount) / totalPaymentAmount) * 100 : 0}
								{#if pct > 0}
									<div
										class="stacked-segment"
										style="width: {pct}%; background-color: {getPaymentColor(payment.payment_method)}"
										title="{getPaymentLabel(payment.payment_method)}: {formatRupiah(payment.total_amount)} ({pct.toFixed(1)}%)"
									></div>
								{/if}
							{/each}
						</div>
						<div class="stacked-legend">
							{#each data.paymentSummary as payment}
								<div class="legend-item">
									<span class="legend-dot" style="background-color: {getPaymentColor(payment.payment_method)}"></span>
									<span class="legend-label">{getPaymentLabel(payment.payment_method)}</span>
								</div>
							{/each}
						</div>
					</div>
				</div>

				<!-- Data table -->
				<div class="data-table">
					<div class="table-header payment-cols">
						<span>Metode Pembayaran</span>
						<span class="col-right">Transaksi</span>
						<span class="col-right">Jumlah</span>
					</div>
					{#each data.paymentSummary as payment (payment.payment_method)}
						<div class="table-row payment-cols">
							<span class="col-payment-method">
								<span class="method-dot" style="background-color: {getPaymentColor(payment.payment_method)}"></span>
								{getPaymentLabel(payment.payment_method)}
							</span>
							<span class="col-right">{payment.transaction_count}</span>
							<span class="col-right col-net">{formatRupiah(payment.total_amount)}</span>
						</div>
					{/each}
				</div>
			{/if}
		</div>
	{/if}

	<!-- ═══════════════════════════════════════════════ -->
	<!-- TAB: Per Outlet (Owner Only)                    -->
	<!-- ═══════════════════════════════════════════════ -->
	{#if activeTab === 'outlet' && data.userRole === 'OWNER'}
		<div class="tab-content">
			<div class="tab-toolbar">
				<h2 class="section-title">Perbandingan Outlet</h2>
				<button type="button" class="btn-secondary btn-export" onclick={exportOutletComparison}>
					Unduh CSV
				</button>
			</div>

			{#if data.outletComparison.length === 0}
				<div class="empty-state">
					<p class="empty-text">Tidak ada data outlet untuk periode ini.</p>
				</div>
			{:else}
				<!-- Horizontal bar chart -->
				<div class="chart-container">
					<div class="chart-header">
						<h3 class="chart-title">Pendapatan per Outlet</h3>
					</div>
					<div class="horizontal-bars">
						{#each data.outletComparison as outlet}
							<div class="hbar-row" title="{outlet.outlet_name}: {outlet.order_count} pesanan, {formatRupiah(outlet.total_revenue)}">
								<div class="hbar-name hbar-name-wide">{outlet.outlet_name}</div>
								<div class="hbar-track">
									<div
										class="hbar-fill"
										style="width: {(parseFloat(outlet.total_revenue) / maxOutletRevenue) * 100}%"
									></div>
								</div>
								<div class="hbar-value">{formatCompact(parseFloat(outlet.total_revenue))}</div>
							</div>
						{/each}
					</div>
				</div>

				<!-- Data table -->
				<div class="data-table">
					<div class="table-header outlet-cols">
						<span>Outlet</span>
						<span class="col-right">Total Pesanan</span>
						<span class="col-right">Pendapatan</span>
					</div>
					{#each data.outletComparison as outlet (outlet.outlet_id)}
						<div class="table-row outlet-cols">
							<span class="col-outlet-name">{outlet.outlet_name}</span>
							<span class="col-right">{outlet.order_count}</span>
							<span class="col-right col-net">{formatRupiah(outlet.total_revenue)}</span>
						</div>
					{/each}
				</div>
			{/if}
		</div>
	{/if}
</div>

<style>
	.reports-page {
		max-width: 1200px;
	}

	/* ── Page header ────────────────── */

	.page-header {
		margin-bottom: 20px;
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
		margin: 4px 0 0;
	}

	/* ── Date range picker ────────────────── */

	.date-range {
		display: flex;
		gap: 12px;
		align-items: flex-end;
		margin-bottom: 20px;
		flex-wrap: wrap;
	}

	.date-label {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.label-text {
		font-size: 12px;
		font-weight: 500;
		color: var(--color-text-secondary);
	}

	.date-input {
		max-width: 170px;
	}

	.btn-apply {
		padding: 10px 20px;
		font-size: 13px;
		border: none;
		cursor: pointer;
	}

	/* ── Tab bar ────────────────── */

	.tab-bar {
		display: flex;
		gap: 0;
		border-bottom: 2px solid var(--color-border);
		margin-bottom: 20px;
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

	/* ── Tab content / toolbar ────────────────── */

	.tab-content {
		display: flex;
		flex-direction: column;
		gap: 16px;
	}

	.tab-toolbar {
		display: flex;
		align-items: center;
		justify-content: space-between;
	}

	.section-title {
		font-size: 16px;
		font-weight: 600;
		color: var(--color-text-primary);
		margin: 0;
	}

	.btn-export {
		padding: 8px 16px;
		font-size: 13px;
		cursor: pointer;
	}

	/* ── Summary cards ────────────────── */

	.summary-row {
		display: grid;
		grid-template-columns: repeat(4, 1fr);
		gap: 12px;
	}

	.summary-card {
		background-color: var(--color-bg);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-card);
		padding: 16px;
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.summary-value {
		font-size: 18px;
		font-weight: 700;
		color: var(--color-text-primary);
	}

	.summary-label {
		font-size: 12px;
		color: var(--color-text-secondary);
		font-weight: 500;
	}

	/* ── Vertical bar chart (daily sales) ────────────────── */

	.chart-container {
		background-color: var(--color-bg);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-card);
		padding: 20px;
	}

	.chart-header {
		margin-bottom: 16px;
	}

	.chart-title {
		font-size: 15px;
		font-weight: 600;
		color: var(--color-text-primary);
		margin: 0;
	}

	.chart {
		overflow-x: auto;
	}

	.bars {
		display: flex;
		align-items: flex-end;
		gap: 4px;
		height: 200px;
		min-width: 0;
	}

	.bar-group {
		flex: 1;
		display: flex;
		flex-direction: column;
		align-items: center;
		min-width: 28px;
		height: 100%;
	}

	.bar-value {
		font-size: 10px;
		color: var(--color-text-secondary);
		white-space: nowrap;
		height: 16px;
		display: flex;
		align-items: flex-end;
	}

	.bar-track {
		flex: 1;
		width: 100%;
		max-width: 32px;
		display: flex;
		align-items: flex-end;
		padding: 0 2px;
	}

	.bar-fill {
		width: 100%;
		background-color: var(--color-primary);
		border-radius: 3px 3px 0 0;
		min-height: 0;
		transition: height 0.3s ease;
	}

	.bar-label {
		font-size: 11px;
		color: var(--color-text-secondary);
		margin-top: 6px;
		height: 16px;
		white-space: nowrap;
	}

	/* ── Horizontal bar chart (products, outlets) ────────────────── */

	.horizontal-bars {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	.hbar-row {
		display: flex;
		align-items: center;
		gap: 8px;
	}

	.hbar-rank {
		width: 24px;
		font-size: 12px;
		font-weight: 600;
		color: var(--color-text-secondary);
		text-align: center;
		flex-shrink: 0;
	}

	.hbar-name {
		width: 120px;
		font-size: 13px;
		color: var(--color-text-primary);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		flex-shrink: 0;
	}

	.hbar-name-wide {
		width: 160px;
	}

	.hbar-track {
		flex: 1;
		height: 24px;
		background-color: var(--color-surface);
		border-radius: 4px;
		overflow: hidden;
	}

	.hbar-fill {
		height: 100%;
		background-color: var(--color-primary);
		border-radius: 4px;
		transition: width 0.3s ease;
	}

	.hbar-value {
		width: 64px;
		font-size: 12px;
		font-weight: 600;
		color: var(--color-text-primary);
		text-align: right;
		flex-shrink: 0;
	}

	/* ── Payment method cards ────────────────── */

	.payment-cards {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
		gap: 12px;
	}

	.payment-card {
		background-color: var(--color-bg);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-card);
		padding: 16px;
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.payment-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
	}

	.payment-method {
		font-size: 14px;
		font-weight: 700;
	}

	.payment-pct {
		font-size: 13px;
		font-weight: 600;
		color: var(--color-text-secondary);
	}

	.payment-amount {
		font-size: 20px;
		font-weight: 700;
		color: var(--color-text-primary);
	}

	.payment-count {
		font-size: 12px;
		color: var(--color-text-secondary);
	}

	.payment-bar-track {
		height: 6px;
		background-color: var(--color-surface);
		border-radius: 3px;
		overflow: hidden;
		margin-top: 4px;
	}

	.payment-bar-fill {
		height: 100%;
		border-radius: 3px;
		transition: width 0.3s ease;
	}

	/* ── Stacked bar (payment composition) ────────────────── */

	.stacked-bar-container {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}

	.stacked-bar {
		display: flex;
		height: 32px;
		border-radius: 6px;
		overflow: hidden;
	}

	.stacked-segment {
		height: 100%;
		transition: width 0.3s ease;
	}

	.stacked-legend {
		display: flex;
		gap: 16px;
		flex-wrap: wrap;
	}

	.legend-item {
		display: flex;
		align-items: center;
		gap: 6px;
	}

	.legend-dot {
		width: 10px;
		height: 10px;
		border-radius: 50%;
		flex-shrink: 0;
	}

	.legend-label {
		font-size: 12px;
		color: var(--color-text-secondary);
		font-weight: 500;
	}

	/* ── Data tables ────────────────── */

	.data-table {
		border: 1px solid var(--color-border);
		border-radius: var(--radius-card);
		overflow: hidden;
	}

	.table-header {
		display: grid;
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
		gap: 12px;
		padding: 10px 16px;
		border-bottom: 1px solid var(--color-border);
		font-size: 13px;
		color: var(--color-text-primary);
		align-items: center;
	}

	.table-row:last-child {
		border-bottom: none;
	}

	/* Grid columns per tab */
	.sales-cols {
		grid-template-columns: 1.5fr 1fr 1fr 1fr 1fr;
	}

	.product-cols {
		grid-template-columns: 40px 2fr 1fr 1fr;
	}

	.payment-cols {
		grid-template-columns: 2fr 1fr 1fr;
	}

	.outlet-cols {
		grid-template-columns: 2fr 1fr 1fr;
	}

	.col-right {
		text-align: right;
	}

	.col-date-cell {
		font-weight: 500;
	}

	.col-discount {
		color: var(--color-text-secondary);
	}

	.col-net {
		font-weight: 600;
	}

	.col-rank {
		font-size: 12px;
		font-weight: 600;
		color: var(--color-text-secondary);
	}

	.col-product-name {
		font-weight: 500;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.col-outlet-name {
		font-weight: 500;
	}

	.col-payment-method {
		display: flex;
		align-items: center;
		gap: 8px;
		font-weight: 500;
	}

	.method-dot {
		width: 8px;
		height: 8px;
		border-radius: 50%;
		flex-shrink: 0;
	}

	/* ── Empty state ────────────────── */

	.empty-state {
		text-align: center;
		padding: 48px 24px;
	}

	.empty-text {
		font-size: 13px;
		color: var(--color-text-secondary);
		margin: 0;
	}

	/* ── Responsive ────────────────── */

	@media (max-width: 768px) {
		.summary-row {
			grid-template-columns: repeat(2, 1fr);
		}

		.sales-cols {
			grid-template-columns: 1fr 1fr;
		}

		.table-header.sales-cols {
			display: none;
		}

		.table-row.sales-cols {
			grid-template-columns: 1fr 1fr;
			gap: 4px;
		}

		.date-range {
			flex-direction: column;
			align-items: stretch;
		}

		.date-input {
			max-width: none;
		}

		.tab-bar {
			overflow-x: auto;
		}

		.hbar-name {
			width: 80px;
			font-size: 12px;
		}

		.hbar-name-wide {
			width: 100px;
		}

		.payment-cards {
			grid-template-columns: 1fr;
		}
	}

	@media (max-width: 480px) {
		.summary-row {
			grid-template-columns: 1fr;
		}
	}
</style>
