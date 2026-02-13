<!--
  Laporan Keuangan — P&L and Cash Flow reports with pivot tables.
  Date range filtering, monthly column layout, CSV export per tab.
-->
<script lang="ts">
	import { formatRupiah } from '$lib/utils/format';
	import type { PnlPeriod, CashFlowPeriod, CashFlowAccount } from '$lib/types/api';

	let { data } = $props();

	type Tab = 'pnl' | 'cashflow';

	let activeTab = $state<Tab>('pnl');

	// Date range state — syncs from server data
	let startDate = $state(data.startDate);
	let endDate = $state(data.endDate);

	$effect(() => {
		startDate = data.startDate;
		endDate = data.endDate;
	});

	// ── Derived data ──────────────────────

	let pnlPeriods = $derived<PnlPeriod[]>(data.pnl.periods ?? []);
	let cashFlowPeriods = $derived<CashFlowPeriod[]>(data.cashFlow.periods ?? []);

	let pnlMonths = $derived(pnlPeriods.map((p) => p.period));
	let cashFlowMonths = $derived(cashFlowPeriods.map((p) => p.period));

	// Collect all unique expense account rows across all P&L periods
	let allExpenseAccounts = $derived.by(() => {
		const seen = new Map<string, string>();
		for (const period of pnlPeriods) {
			for (const exp of period.expenses) {
				if (!seen.has(exp.account_code)) {
					seen.set(exp.account_code, exp.account_name);
				}
			}
		}
		return Array.from(seen.entries()).map(([code, name]) => ({ account_code: code, account_name: name }));
	});

	// Collect all unique cash accounts across all cash flow periods
	let allCashAccounts = $derived.by(() => {
		const seen = new Map<string, string>();
		for (const period of cashFlowPeriods) {
			for (const acct of period.accounts) {
				if (!seen.has(acct.cash_account_code)) {
					seen.set(acct.cash_account_code, acct.cash_account_name);
				}
			}
		}
		return Array.from(seen.entries()).map(([code, name]) => ({ cash_account_code: code, cash_account_name: name }));
	});

	// ── Helpers ──────────────────────

	function formatMonth(period: string): string {
		const [year, month] = period.split('-');
		const d = new Date(parseInt(year), parseInt(month) - 1, 1);
		return d.toLocaleDateString('id-ID', { month: 'short', year: 'numeric' });
	}

	function getExpenseAmount(period: PnlPeriod, accountCode: string): string {
		const row = period.expenses.find((e) => e.account_code === accountCode);
		return row ? row.amount : '0';
	}

	function getCashAccountData(period: CashFlowPeriod, accountCode: string): CashFlowAccount | null {
		return period.accounts.find((a) => a.cash_account_code === accountCode) ?? null;
	}

	function formatPct(value: string): string {
		const num = parseFloat(value);
		if (isNaN(num)) return '0.0%';
		return num.toFixed(1) + '%';
	}

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

	function exportPnl() {
		const headers = ['', ...pnlMonths.map(formatMonth)];
		const rows: string[][] = [];

		rows.push(['Pendapatan Bersih', ...pnlPeriods.map((p) => p.net_sales)]);
		rows.push(['HPP', ...pnlPeriods.map((p) => p.cogs)]);
		rows.push(['Laba Kotor', ...pnlPeriods.map((p) => p.gross_profit)]);

		for (const acct of allExpenseAccounts) {
			rows.push([
				`${acct.account_code} - ${acct.account_name}`,
				...pnlPeriods.map((p) => getExpenseAmount(p, acct.account_code))
			]);
		}

		rows.push(['Total Beban', ...pnlPeriods.map((p) => p.total_expenses)]);
		rows.push(['Laba Bersih', ...pnlPeriods.map((p) => p.net_profit)]);
		rows.push(['Margin Kotor %', ...pnlPeriods.map((p) => formatPct(p.gross_margin_pct))]);
		rows.push(['Margin Bersih %', ...pnlPeriods.map((p) => formatPct(p.net_margin_pct))]);

		const suffix = startDate && endDate ? `_${startDate}_${endDate}` : '';
		downloadCsv(`laba_rugi${suffix}.csv`, headers, rows);
	}

	function exportCashFlow() {
		const headers = ['', ...cashFlowMonths.map(formatMonth)];
		const rows: string[][] = [];

		for (const acct of allCashAccounts) {
			rows.push([
				`${acct.cash_account_name} - Kas Masuk`,
				...cashFlowPeriods.map((p) => {
					const d = getCashAccountData(p, acct.cash_account_code);
					return d ? d.cash_in : '0';
				})
			]);
			rows.push([
				`${acct.cash_account_name} - Kas Keluar`,
				...cashFlowPeriods.map((p) => {
					const d = getCashAccountData(p, acct.cash_account_code);
					return d ? d.cash_out : '0';
				})
			]);
			rows.push([
				`${acct.cash_account_name} - Neto`,
				...cashFlowPeriods.map((p) => {
					const d = getCashAccountData(p, acct.cash_account_code);
					return d ? d.net : '0';
				})
			]);
		}

		rows.push(['Total Kas Masuk', ...cashFlowPeriods.map((p) => p.total_cash_in)]);
		rows.push(['Total Kas Keluar', ...cashFlowPeriods.map((p) => p.total_cash_out)]);
		rows.push(['Total Neto', ...cashFlowPeriods.map((p) => p.total_net)]);

		const suffix = startDate && endDate ? `_${startDate}_${endDate}` : '';
		downloadCsv(`arus_kas${suffix}.csv`, headers, rows);
	}
</script>

<svelte:head>
	<title>Laporan Keuangan - Kiwari POS</title>
</svelte:head>

<div class="reports-page">
	<div class="page-header">
		<h1 class="page-title">Laporan Keuangan</h1>
		<p class="page-subtitle">Laba rugi dan arus kas</p>
	</div>

	<!-- Date range filter -->
	<form method="GET" class="date-range">
		<label class="date-label">
			<span class="label-text">Tanggal Mulai</span>
			<input type="date" name="start_date" class="input-field date-input" bind:value={startDate} />
		</label>
		<label class="date-label">
			<span class="label-text">Tanggal Akhir</span>
			<input type="date" name="end_date" class="input-field date-input" bind:value={endDate} />
		</label>
		<button type="submit" class="btn-primary btn-apply">
			Terapkan
		</button>
	</form>

	<!-- Tab bar -->
	<div class="tab-bar">
		<button
			type="button"
			class="tab-item"
			class:active={activeTab === 'pnl'}
			onclick={() => { activeTab = 'pnl'; }}
		>
			Laba Rugi
		</button>
		<button
			type="button"
			class="tab-item"
			class:active={activeTab === 'cashflow'}
			onclick={() => { activeTab = 'cashflow'; }}
		>
			Arus Kas
		</button>
	</div>

	<!-- ═══════════════════════════════════════════════ -->
	<!-- TAB: Laba Rugi (P&L)                           -->
	<!-- ═══════════════════════════════════════════════ -->
	{#if activeTab === 'pnl'}
		<div class="tab-content">
			<div class="tab-toolbar">
				<h2 class="section-title">Laba Rugi</h2>
				<button type="button" class="btn-secondary btn-export" onclick={exportPnl} disabled={pnlPeriods.length === 0}>
					Unduh CSV
				</button>
			</div>

			{#if pnlPeriods.length === 0}
				<div class="empty-state">
					<p class="empty-text">Tidak ada data laba rugi untuk periode ini.</p>
					<p class="empty-hint">Pilih rentang tanggal dan klik Terapkan.</p>
				</div>
			{:else}
				<div class="table-scroll">
					<table class="pivot-table">
						<thead>
							<tr>
								<th class="col-label">Akun</th>
								{#each pnlMonths as month}
									<th class="col-value">{formatMonth(month)}</th>
								{/each}
							</tr>
						</thead>
						<tbody>
							<!-- Net Sales -->
							<tr class="row-primary">
								<td class="col-label">Pendapatan Bersih</td>
								{#each pnlPeriods as period}
									<td class="col-value">{formatRupiah(period.net_sales)}</td>
								{/each}
							</tr>

							<!-- COGS -->
							<tr>
								<td class="col-label">HPP</td>
								{#each pnlPeriods as period}
									<td class="col-value col-negative">{formatRupiah(period.cogs)}</td>
								{/each}
							</tr>

							<!-- Gross Profit -->
							<tr class="row-subtotal">
								<td class="col-label">Laba Kotor</td>
								{#each pnlPeriods as period}
									<td class="col-value">{formatRupiah(period.gross_profit)}</td>
								{/each}
							</tr>

							<!-- Separator -->
							<tr class="row-separator"><td colspan={1 + pnlMonths.length}></td></tr>

							<!-- Expense accounts -->
							{#each allExpenseAccounts as acct}
								<tr>
									<td class="col-label col-indent">{acct.account_code} - {acct.account_name}</td>
									{#each pnlPeriods as period}
										<td class="col-value col-negative">{formatRupiah(getExpenseAmount(period, acct.account_code))}</td>
									{/each}
								</tr>
							{/each}

							<!-- Total Expenses -->
							<tr class="row-subtotal">
								<td class="col-label">Total Beban</td>
								{#each pnlPeriods as period}
									<td class="col-value col-negative">{formatRupiah(period.total_expenses)}</td>
								{/each}
							</tr>

							<!-- Separator -->
							<tr class="row-separator"><td colspan={1 + pnlMonths.length}></td></tr>

							<!-- Net Profit -->
							<tr class="row-total">
								<td class="col-label">Laba Bersih</td>
								{#each pnlPeriods as period}
									<td class="col-value">{formatRupiah(period.net_profit)}</td>
								{/each}
							</tr>

							<!-- Margins -->
							<tr class="row-margin">
								<td class="col-label">Margin Kotor</td>
								{#each pnlPeriods as period}
									<td class="col-value">{formatPct(period.gross_margin_pct)}</td>
								{/each}
							</tr>
							<tr class="row-margin">
								<td class="col-label">Margin Bersih</td>
								{#each pnlPeriods as period}
									<td class="col-value">{formatPct(period.net_margin_pct)}</td>
								{/each}
							</tr>
						</tbody>
					</table>
				</div>
			{/if}
		</div>
	{/if}

	<!-- ═══════════════════════════════════════════════ -->
	<!-- TAB: Arus Kas (Cash Flow)                      -->
	<!-- ═══════════════════════════════════════════════ -->
	{#if activeTab === 'cashflow'}
		<div class="tab-content">
			<div class="tab-toolbar">
				<h2 class="section-title">Arus Kas</h2>
				<button type="button" class="btn-secondary btn-export" onclick={exportCashFlow} disabled={cashFlowPeriods.length === 0}>
					Unduh CSV
				</button>
			</div>

			{#if cashFlowPeriods.length === 0}
				<div class="empty-state">
					<p class="empty-text">Tidak ada data arus kas untuk periode ini.</p>
					<p class="empty-hint">Pilih rentang tanggal dan klik Terapkan.</p>
				</div>
			{:else}
				<div class="table-scroll">
					<table class="pivot-table">
						<thead>
							<tr>
								<th class="col-label">Akun</th>
								{#each cashFlowMonths as month}
									<th class="col-value">{formatMonth(month)}</th>
								{/each}
							</tr>
						</thead>
						<tbody>
							{#each allCashAccounts as acct}
								<!-- Account header -->
								<tr class="row-account-header">
									<td class="col-label" colspan={1 + cashFlowMonths.length}>
										{acct.cash_account_code} - {acct.cash_account_name}
									</td>
								</tr>

								<!-- Cash In -->
								<tr>
									<td class="col-label col-indent">Kas Masuk</td>
									{#each cashFlowPeriods as period}
										{@const d = getCashAccountData(period, acct.cash_account_code)}
										<td class="col-value col-positive">{formatRupiah(d ? d.cash_in : '0')}</td>
									{/each}
								</tr>

								<!-- Cash Out -->
								<tr>
									<td class="col-label col-indent">Kas Keluar</td>
									{#each cashFlowPeriods as period}
										{@const d = getCashAccountData(period, acct.cash_account_code)}
										<td class="col-value col-negative">{formatRupiah(d ? d.cash_out : '0')}</td>
									{/each}
								</tr>

								<!-- Net -->
								<tr class="row-subtotal">
									<td class="col-label col-indent">Neto</td>
									{#each cashFlowPeriods as period}
										{@const d = getCashAccountData(period, acct.cash_account_code)}
										<td class="col-value">{formatRupiah(d ? d.net : '0')}</td>
									{/each}
								</tr>
							{/each}

							<!-- Separator -->
							<tr class="row-separator"><td colspan={1 + cashFlowMonths.length}></td></tr>

							<!-- Totals -->
							<tr class="row-total">
								<td class="col-label">Total Kas Masuk</td>
								{#each cashFlowPeriods as period}
									<td class="col-value col-positive">{formatRupiah(period.total_cash_in)}</td>
								{/each}
							</tr>
							<tr class="row-total">
								<td class="col-label">Total Kas Keluar</td>
								{#each cashFlowPeriods as period}
									<td class="col-value col-negative">{formatRupiah(period.total_cash_out)}</td>
								{/each}
							</tr>
							<tr class="row-total row-grand-total">
								<td class="col-label">Total Neto</td>
								{#each cashFlowPeriods as period}
									<td class="col-value">{formatRupiah(period.total_net)}</td>
								{/each}
							</tr>
						</tbody>
					</table>
				</div>
			{/if}
		</div>
	{/if}
</div>

<style>
	.reports-page {
		max-width: 1200px;
		display: flex;
		flex-direction: column;
		gap: 0;
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

	.btn-export:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	/* ── Table scroll wrapper ────────────────── */

	.table-scroll {
		overflow-x: auto;
		border: 1px solid var(--color-border);
		border-radius: var(--radius-card);
	}

	/* ── Pivot table ────────────────── */

	.pivot-table {
		width: 100%;
		border-collapse: collapse;
		font-size: 13px;
		min-width: 500px;
	}

	.pivot-table thead tr {
		background-color: var(--color-surface);
		border-bottom: 1px solid var(--color-border);
	}

	.pivot-table th {
		padding: 10px 16px;
		font-size: 12px;
		font-weight: 600;
		color: var(--color-text-secondary);
		text-transform: uppercase;
		letter-spacing: 0.02em;
		white-space: nowrap;
	}

	.pivot-table td {
		padding: 8px 16px;
		border-bottom: 1px solid var(--color-border);
		color: var(--color-text-primary);
	}

	.pivot-table tbody tr:last-child td {
		border-bottom: none;
	}

	.col-label {
		text-align: left;
		font-weight: 500;
		white-space: nowrap;
		min-width: 180px;
	}

	.col-value {
		text-align: right;
		white-space: nowrap;
		font-variant-numeric: tabular-nums;
	}

	.col-indent {
		padding-left: 32px !important;
		font-weight: 400;
	}

	.col-negative {
		color: var(--color-text-secondary);
	}

	.col-positive {
		color: var(--color-primary);
	}

	/* ── Row styles ────────────────── */

	.row-primary td {
		font-weight: 600;
	}

	.row-subtotal td {
		font-weight: 600;
		border-top: 1px solid var(--color-border);
	}

	.row-total td {
		font-weight: 700;
		background-color: var(--color-surface);
	}

	.row-grand-total td {
		font-size: 14px;
	}

	.row-separator td {
		padding: 4px 0;
		border-bottom: none;
	}

	.row-margin td {
		font-style: italic;
		color: var(--color-text-secondary);
		font-weight: 500;
	}

	.row-account-header td {
		font-weight: 600;
		font-size: 13px;
		padding-top: 14px;
		padding-bottom: 6px;
		border-bottom: none;
		color: var(--color-text-primary);
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

	.empty-hint {
		font-size: 12px;
		color: var(--color-text-secondary);
		margin: 8px 0 0;
		opacity: 0.7;
	}

	/* ── Responsive ────────────────── */

	@media (max-width: 768px) {
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

		.tab-item {
			white-space: nowrap;
		}

		.col-label {
			min-width: 140px;
		}

		.pivot-table th,
		.pivot-table td {
			padding: 8px 12px;
		}
	}
</style>
