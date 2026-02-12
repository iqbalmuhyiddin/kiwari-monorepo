<!--
  Ringkasan Keuangan — Accounting dashboard overview.
  Shows cash balances, monthly P&L, pending reimbursements, and recent transactions.
-->
<script lang="ts">
	import { formatRupiah } from '$lib/utils/format';

	let { data } = $props();

	const MONTH_NAMES = [
		'Januari', 'Februari', 'Maret', 'April', 'Mei', 'Juni',
		'Juli', 'Agustus', 'September', 'Oktober', 'November', 'Desember'
	];

	function formatPeriod(period: string): string {
		if (!period) return '-';
		const [year, month] = period.split('-');
		const monthIndex = parseInt(month, 10) - 1;
		if (monthIndex < 0 || monthIndex > 11) return period;
		return `${MONTH_NAMES[monthIndex]} ${year}`;
	}

	function formatDate(dateStr: string): string {
		const d = new Date(dateStr);
		return d.toLocaleDateString('id-ID', { day: 'numeric', month: 'short', year: 'numeric' });
	}

	function lineTypeBadgeClass(lineType: string): string {
		const map: Record<string, string> = {
			SALES: 'badge-sales',
			COGS: 'badge-cogs',
			INVENTORY: 'badge-inventory',
			EXPENSE: 'badge-expense',
			CAPITAL: 'badge-capital',
			DRAWING: 'badge-drawing'
		};
		return map[lineType] ?? 'badge-default';
	}

	function lineTypeLabel(lineType: string): string {
		const map: Record<string, string> = {
			SALES: 'Penjualan',
			COGS: 'HPP',
			INVENTORY: 'Persediaan',
			EXPENSE: 'Beban',
			CAPITAL: 'Modal',
			DRAWING: 'Prive'
		};
		return map[lineType] ?? lineType;
	}

	let db = $derived(data.dashboard);
	let pnl = $derived(db.monthly_pnl);
	let netProfitNum = $derived(parseFloat(pnl.net_profit) || 0);
</script>

<svelte:head>
	<title>Ringkasan Keuangan - Kiwari POS</title>
</svelte:head>

<div class="dashboard-page">
	<div class="page-header">
		<h1 class="page-title">Ringkasan Keuangan</h1>
		<p class="page-subtitle">Ikhtisar keuangan bulan ini</p>
	</div>

	<!-- Cash Balance Cards -->
	<section class="section">
		<h2 class="section-title">Saldo Kas</h2>
		{#if db.cash_balances.length === 0}
			<div class="empty-state">Belum ada akun kas.</div>
		{:else}
			<div class="cash-grid">
				{#each db.cash_balances as acct}
					<div class="cash-card">
						<span class="cash-name">{acct.cash_account_name}</span>
						<span class="cash-code">{acct.cash_account_code}</span>
						<span class="cash-balance">{formatRupiah(acct.balance)}</span>
					</div>
				{/each}
			</div>
		{/if}
	</section>

	<!-- Monthly P&L + Pending Reimbursements -->
	<div class="summary-grid">
		<!-- Monthly P&L -->
		<div class="summary-card">
			<h2 class="card-title">Laba Rugi — {formatPeriod(pnl.period)}</h2>
			<div class="pnl-rows">
				<div class="pnl-row">
					<span class="pnl-label">Penjualan Bersih</span>
					<span class="pnl-value">{formatRupiah(pnl.net_sales)}</span>
				</div>
				<div class="pnl-row">
					<span class="pnl-label">HPP</span>
					<span class="pnl-value pnl-negative">({formatRupiah(pnl.cogs)})</span>
				</div>
				<div class="pnl-row pnl-subtotal">
					<span class="pnl-label">Laba Kotor</span>
					<span class="pnl-value">{formatRupiah(pnl.gross_profit)}</span>
				</div>
				<div class="pnl-row">
					<span class="pnl-label">Beban</span>
					<span class="pnl-value pnl-negative">({formatRupiah(pnl.total_expenses)})</span>
				</div>
				<div class="pnl-row pnl-total">
					<span class="pnl-label">Laba Bersih</span>
					<span class="pnl-value" class:pnl-positive={netProfitNum >= 0} class:pnl-loss={netProfitNum < 0}>
						{formatRupiah(pnl.net_profit)}
					</span>
				</div>
			</div>
		</div>

		<!-- Pending Reimbursements -->
		<div class="summary-card reimbursement-card">
			<h2 class="card-title">Reimbursement Pending</h2>
			<div class="reimburse-body">
				<div class="reimburse-stats">
					<span class="reimburse-count">{db.pending_reimbursements.count}</span>
					<span class="reimburse-count-label">pengajuan</span>
				</div>
				<div class="reimburse-total">
					<span class="reimburse-total-label">Total</span>
					<span class="reimburse-total-amount">{formatRupiah(db.pending_reimbursements.total_amount)}</span>
				</div>
			</div>
			<a href="/accounting/reimbursements" class="btn-secondary btn-reimburse">
				Lihat Reimbursement
			</a>
		</div>
	</div>

	<!-- Recent Transactions -->
	<section class="section">
		<h2 class="section-title">10 Transaksi Terakhir</h2>
		{#if db.recent_transactions.length === 0}
			<div class="empty-state">Belum ada transaksi.</div>
		{:else}
			<!-- Desktop table -->
			<div class="table-wrapper">
				<table class="txn-table">
					<thead>
						<tr>
							<th>Kode</th>
							<th>Tanggal</th>
							<th>Deskripsi</th>
							<th class="col-right">Jumlah</th>
							<th>Tipe</th>
						</tr>
					</thead>
					<tbody>
						{#each db.recent_transactions as txn}
							<tr>
								<td class="txn-code">{txn.transaction_code}</td>
								<td class="txn-date">{formatDate(txn.transaction_date)}</td>
								<td class="txn-desc">{txn.description}</td>
								<td class="col-right txn-amount">{formatRupiah(txn.amount)}</td>
								<td>
									<span class="badge {lineTypeBadgeClass(txn.line_type)}">
										{lineTypeLabel(txn.line_type)}
									</span>
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>

			<!-- Mobile cards -->
			<div class="txn-cards-mobile">
				{#each db.recent_transactions as txn}
					<div class="txn-card-mobile">
						<div class="txn-card-top">
							<span class="txn-code">{txn.transaction_code}</span>
							<span class="badge {lineTypeBadgeClass(txn.line_type)}">
								{lineTypeLabel(txn.line_type)}
							</span>
						</div>
						<div class="txn-card-desc">{txn.description}</div>
						<div class="txn-card-bottom">
							<span class="txn-date">{formatDate(txn.transaction_date)}</span>
							<span class="txn-amount">{formatRupiah(txn.amount)}</span>
						</div>
					</div>
				{/each}
			</div>
		{/if}
	</section>
</div>

<style>
	.dashboard-page {
		max-width: 1000px;
		display: flex;
		flex-direction: column;
		gap: 24px;
	}

	/* ── Page header ────────── */

	.page-header {
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

	/* ── Section ────────── */

	.section {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}

	.section-title {
		font-size: 15px;
		font-weight: 600;
		color: var(--color-text-primary);
		margin: 0;
	}

	/* ── Empty state ────────── */

	.empty-state {
		background-color: var(--color-surface);
		color: var(--color-text-secondary);
		font-size: 13px;
		text-align: center;
		padding: 24px;
		border-radius: var(--radius-card);
		border: 1px solid var(--color-border);
	}

	/* ── Cash balance cards ────────── */

	.cash-grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
		gap: 12px;
	}

	.cash-card {
		background-color: var(--color-bg);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-card);
		padding: 16px;
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.cash-name {
		font-size: 13px;
		font-weight: 600;
		color: var(--color-text-primary);
	}

	.cash-code {
		font-size: 11px;
		color: var(--color-text-secondary);
	}

	.cash-balance {
		font-size: 18px;
		font-weight: 700;
		color: var(--color-primary);
		margin-top: 4px;
	}

	/* ── Summary grid (P&L + Reimbursements) ────────── */

	.summary-grid {
		display: grid;
		grid-template-columns: 1fr 300px;
		gap: 16px;
	}

	.summary-card {
		background-color: var(--color-bg);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-card);
		padding: 16px;
	}

	.card-title {
		font-size: 14px;
		font-weight: 600;
		color: var(--color-text-primary);
		margin: 0 0 16px;
	}

	/* ── P&L rows ────────── */

	.pnl-rows {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	.pnl-row {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: 4px 0;
	}

	.pnl-label {
		font-size: 13px;
		color: var(--color-text-secondary);
	}

	.pnl-value {
		font-size: 13px;
		font-weight: 600;
		color: var(--color-text-primary);
	}

	.pnl-negative {
		color: var(--color-text-secondary);
	}

	.pnl-subtotal {
		border-top: 1px solid var(--color-border);
		padding-top: 8px;
	}

	.pnl-total {
		border-top: 2px solid var(--color-text-primary);
		padding-top: 8px;
	}

	.pnl-total .pnl-label {
		font-weight: 700;
		color: var(--color-text-primary);
	}

	.pnl-total .pnl-value {
		font-size: 15px;
		font-weight: 700;
	}

	.pnl-positive {
		color: var(--color-primary);
	}

	.pnl-loss {
		color: var(--color-error);
	}

	/* ── Reimbursement card ────────── */

	.reimbursement-card {
		display: flex;
		flex-direction: column;
	}

	.reimburse-body {
		flex: 1;
		display: flex;
		flex-direction: column;
		gap: 16px;
	}

	.reimburse-stats {
		display: flex;
		align-items: baseline;
		gap: 6px;
	}

	.reimburse-count {
		font-size: 32px;
		font-weight: 700;
		color: var(--color-text-primary);
	}

	.reimburse-count-label {
		font-size: 13px;
		color: var(--color-text-secondary);
	}

	.reimburse-total {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}

	.reimburse-total-label {
		font-size: 12px;
		color: var(--color-text-secondary);
	}

	.reimburse-total-amount {
		font-size: 16px;
		font-weight: 700;
		color: var(--color-text-primary);
	}

	.btn-reimburse {
		margin-top: 16px;
		padding: 8px 16px;
		font-size: 13px;
		text-align: center;
		text-decoration: none;
		display: inline-block;
		cursor: pointer;
	}

	/* ── Transactions table (desktop) ────────── */

	.table-wrapper {
		background-color: var(--color-bg);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-card);
		overflow: hidden;
	}

	.txn-table {
		width: 100%;
		border-collapse: collapse;
		font-size: 13px;
	}

	.txn-table thead {
		background-color: var(--color-surface);
	}

	.txn-table th {
		padding: 10px 14px;
		text-align: left;
		font-size: 12px;
		font-weight: 600;
		color: var(--color-text-secondary);
		text-transform: uppercase;
		letter-spacing: 0.02em;
		border-bottom: 1px solid var(--color-border);
	}

	.txn-table td {
		padding: 10px 14px;
		color: var(--color-text-primary);
		border-bottom: 1px solid var(--color-border);
	}

	.txn-table tbody tr:last-child td {
		border-bottom: none;
	}

	.col-right {
		text-align: right;
	}

	.txn-code {
		font-weight: 500;
		font-size: 12px;
		color: var(--color-text-secondary);
		font-family: monospace;
	}

	.txn-date {
		font-size: 12px;
		color: var(--color-text-secondary);
	}

	.txn-desc {
		max-width: 300px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.txn-amount {
		font-weight: 600;
	}

	/* ── Mobile transaction cards ────────── */

	.txn-cards-mobile {
		display: none;
		flex-direction: column;
		gap: 8px;
	}

	.txn-card-mobile {
		background-color: var(--color-bg);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-card);
		padding: 12px 14px;
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.txn-card-top {
		display: flex;
		justify-content: space-between;
		align-items: center;
	}

	.txn-card-desc {
		font-size: 13px;
		color: var(--color-text-primary);
	}

	.txn-card-bottom {
		display: flex;
		justify-content: space-between;
		align-items: center;
	}

	/* ── Badges ────────── */

	.badge {
		display: inline-block;
		font-size: 11px;
		font-weight: 600;
		padding: 2px 8px;
		border-radius: var(--radius-chip);
		white-space: nowrap;
	}

	.badge-sales {
		background-color: #ecfdf5;
		color: #0c7721;
	}

	.badge-cogs {
		background-color: #fffbeb;
		color: #d97706;
	}

	.badge-inventory {
		background-color: #eff6ff;
		color: #2563eb;
	}

	.badge-expense {
		background-color: #fef2f2;
		color: #dc2626;
	}

	.badge-capital {
		background-color: #f5f3ff;
		color: #7c3aed;
	}

	.badge-drawing {
		background-color: #f3f4f6;
		color: #6b7280;
	}

	.badge-default {
		background-color: #f3f4f6;
		color: #6b7280;
	}

	/* ── Responsive ────────── */

	@media (max-width: 768px) {
		.summary-grid {
			grid-template-columns: 1fr;
		}

		.table-wrapper {
			display: none;
		}

		.txn-cards-mobile {
			display: flex;
		}

		.cash-grid {
			grid-template-columns: 1fr 1fr;
		}
	}

	@media (max-width: 480px) {
		.cash-grid {
			grid-template-columns: 1fr;
		}
	}
</style>
