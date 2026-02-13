<!--
  Penjualan (Sales) — monthly sales summaries with POS sync, manual entry, and ledger posting.
  Rows grouped by date. POS rows read-only; manual unposted rows editable/deletable.
-->
<script lang="ts">
	import { enhance } from '$app/forms';
	import { formatRupiah } from '$lib/utils/format';
	import type { AcctSalesDailySummary } from '$lib/types/api';

	let { data, form } = $props();

	// ── Constants ────────────────────
	const channelOptions = ['Dine In', 'Take Away', 'GoFood', 'ShopeeFood', 'Catering', 'Delivery'];
	const paymentMethodOptions = ['Cash', 'QRIS', 'Transfer'];
	// POS sync uses DB payment method keys (uppercase) to match payments table values
	const syncPaymentMethods = ['CASH', 'QRIS', 'TRANSFER'];

	// ── State ────────────────────
	let submitting = $state(false);
	let showSuccess = $state(false);
	let successMessage = $state('');
	let showError = $state(false);

	// Sync POS dialog
	let showSyncDialog = $state(false);
	let syncStartDate = $state(data.startDate);
	let syncEndDate = $state(data.endDate);
	let syncCashAccounts = $state<Record<string, string>>(getDefaultPaymentAccountMap());

	// Create/Edit modal
	let showCreateModal = $state(false);
	let editingId = $state<string | null>(null);
	let formSalesDate = $state(new Date().toISOString().slice(0, 10));
	let formChannel = $state('Dine In');
	let formPaymentMethod = $state('Cash');
	let formGrossSales = $state('');
	let formDiscountAmount = $state('0');
	let formCashAccountId = $state(data.cashAccounts.length > 0 ? data.cashAccounts[0].id : '');

	// Post dialog
	let showPostDialog = $state(false);
	let postSalesDate = $state(new Date().toISOString().slice(0, 10));
	let postAccountId = $state(getDefaultRevenueAccountId());

	// ── Derived ────────────────────

	let formNetSales = $derived.by(() => {
		const gross = parseFloat(formGrossSales) || 0;
		const discount = parseFloat(formDiscountAmount) || 0;
		return gross - discount;
	});

	// Group summaries by date for visual grouping
	let groupedByDate = $derived.by(() => {
		const groups: { date: string; rows: AcctSalesDailySummary[] }[] = [];
		const map = new Map<string, AcctSalesDailySummary[]>();
		for (const s of data.summaries) {
			const dateKey = s.sales_date;
			if (!map.has(dateKey)) {
				map.set(dateKey, []);
			}
			map.get(dateKey)!.push(s);
		}
		// Sort dates descending
		const sortedDates = Array.from(map.keys()).sort((a, b) => b.localeCompare(a));
		for (const date of sortedDates) {
			groups.push({ date, rows: map.get(date)! });
		}
		return groups;
	});

	// Count unposted summaries for context
	let unpostedCount = $derived(data.summaries.filter((s) => !s.posted_at).length);

	// Cash account name lookup
	let cashAccountMap = $derived.by(() => {
		const map = new Map<string, string>();
		for (const ca of data.cashAccounts) {
			map.set(ca.id, `${ca.cash_account_code} - ${ca.cash_account_name}`);
		}
		return map;
	});

	// ── Helpers ────────────────────

	function getDefaultPaymentAccountMap(): Record<string, string> {
		const defaultId = data.cashAccounts.length > 0 ? data.cashAccounts[0].id : '';
		return {
			CASH: defaultId,
			QRIS: defaultId,
			TRANSFER: defaultId
		};
	}

	function getDefaultRevenueAccountId(): string {
		const revenueAccount = data.accounts.find((a) => a.account_type === 'Revenue');
		if (revenueAccount) return revenueAccount.id;
		return data.accounts.length > 0 ? data.accounts[0].id : '';
	}

	function formatDate(dateStr: string): string {
		if (!dateStr) return '-';
		const d = new Date(dateStr + 'T00:00:00');
		return d.toLocaleDateString('id-ID', { weekday: 'short', day: '2-digit', month: 'short', year: 'numeric' });
	}

	function getCashAccountName(id: string): string {
		return cashAccountMap.get(id) ?? '-';
	}

	function isManualUnposted(row: AcctSalesDailySummary): boolean {
		return row.source === 'manual' && !row.posted_at;
	}

	// ── Sync POS ────────────────────

	function openSyncDialog() {
		syncStartDate = data.startDate;
		syncEndDate = data.endDate;
		syncCashAccounts = getDefaultPaymentAccountMap();
		showSyncDialog = true;
	}

	function closeSyncDialog() {
		showSyncDialog = false;
	}

	function buildSyncData(): string {
		return JSON.stringify({
			start_date: syncStartDate,
			end_date: syncEndDate,
			outlet_id: data.outletId,
			payment_method_accounts: syncCashAccounts
		});
	}

	// ── Create/Edit modal ────────────────────

	function openCreateModal() {
		editingId = null;
		formSalesDate = new Date().toISOString().slice(0, 10);
		formChannel = 'Dine In';
		formPaymentMethod = 'Cash';
		formGrossSales = '';
		formDiscountAmount = '0';
		formCashAccountId = data.cashAccounts.length > 0 ? data.cashAccounts[0].id : '';
		showCreateModal = true;
	}

	function openEditModal(row: AcctSalesDailySummary) {
		editingId = row.id;
		formSalesDate = row.sales_date;
		formChannel = row.channel;
		formPaymentMethod = row.payment_method;
		formGrossSales = row.gross_sales;
		formDiscountAmount = row.discount_amount;
		formCashAccountId = row.cash_account_id;
		showCreateModal = true;
	}

	function closeCreateModal() {
		showCreateModal = false;
		editingId = null;
	}

	function buildCreateData(): string {
		const net = (parseFloat(formGrossSales) || 0) - (parseFloat(formDiscountAmount) || 0);
		return JSON.stringify({
			sales_date: formSalesDate,
			channel: formChannel,
			payment_method: formPaymentMethod,
			gross_sales: formGrossSales,
			discount_amount: formDiscountAmount || '0',
			net_sales: net.toFixed(2),
			cash_account_id: formCashAccountId,
			outlet_id: data.outletId
		});
	}

	function buildUpdateData(): string {
		const net = (parseFloat(formGrossSales) || 0) - (parseFloat(formDiscountAmount) || 0);
		return JSON.stringify({
			channel: formChannel,
			payment_method: formPaymentMethod,
			gross_sales: formGrossSales,
			discount_amount: formDiscountAmount || '0',
			net_sales: net.toFixed(2),
			cash_account_id: formCashAccountId
		});
	}

	// ── Post dialog ────────────────────

	function openPostDialog() {
		postSalesDate = new Date().toISOString().slice(0, 10);
		postAccountId = getDefaultRevenueAccountId();
		showPostDialog = true;
	}

	function closePostDialog() {
		showPostDialog = false;
	}

	function buildPostData(): string {
		return JSON.stringify({
			sales_date: postSalesDate,
			outlet_id: data.outletId,
			account_id: postAccountId
		});
	}

	// ── Keyboard handler ────────────────────
	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			if (showPostDialog) closePostDialog();
			else if (showSyncDialog) closeSyncDialog();
			else if (showCreateModal) closeCreateModal();
		}
	}
</script>

<svelte:window onkeydown={handleKeydown} />

<svelte:head>
	<title>Penjualan - Kiwari POS</title>
</svelte:head>

<div class="sales-page">
	<div class="page-header">
		<h1 class="page-title">Penjualan</h1>
		<p class="page-subtitle">Ringkasan penjualan harian — POS sync & entri manual</p>
	</div>

	<!-- Success banner -->
	{#if showSuccess}
		<div class="success-banner">
			{successMessage}
			<button type="button" class="dismiss-btn" onclick={() => { showSuccess = false; }}>Tutup</button>
		</div>
	{/if}

	<!-- Error banner -->
	{#if form?.error && showError}
		<div class="error-banner">
			{form.error}
			<button type="button" class="dismiss-btn dismiss-error" onclick={() => { showError = false; }}>Tutup</button>
		</div>
	{/if}

	<!-- ═══════════════════════════════════════ -->
	<!-- Date range filter                      -->
	<!-- ═══════════════════════════════════════ -->
	<form method="GET" class="date-filter-bar">
		<div class="filter-group">
			<label for="start_date" class="filter-label">Dari</label>
			<input id="start_date" name="start_date" type="date" class="input-field" value={data.startDate} />
		</div>
		<div class="filter-group">
			<label for="end_date" class="filter-label">Sampai</label>
			<input id="end_date" name="end_date" type="date" class="input-field" value={data.endDate} />
		</div>
		<div class="filter-actions">
			<button type="submit" class="btn-primary btn-filter">Tampilkan</button>
		</div>
	</form>

	<!-- ═══════════════════════════════════════ -->
	<!-- Action buttons                         -->
	<!-- ═══════════════════════════════════════ -->
	<div class="action-bar">
		<button type="button" class="btn-primary btn-action" onclick={openSyncDialog}>
			Sinkronkan POS
		</button>
		<button type="button" class="btn-secondary btn-action" onclick={openCreateModal}>
			Tambah Manual
		</button>
		{#if unpostedCount > 0}
			<button type="button" class="btn-secondary btn-action" onclick={openPostDialog}>
				Posting ({unpostedCount} belum diposting)
			</button>
		{/if}
	</div>

	<!-- ═══════════════════════════════════════ -->
	<!-- Data table                              -->
	<!-- ═══════════════════════════════════════ -->
	{#if data.summaries.length === 0}
		<div class="empty-state">
			<p class="empty-text">Belum ada data penjualan untuk periode ini.</p>
		</div>
	{:else}
		<div class="data-table">
			<div class="table-header sales-grid">
				<span>Tanggal</span>
				<span>Channel</span>
				<span>Metode Bayar</span>
				<span class="col-right">Penjualan Kotor</span>
				<span class="col-right">Diskon</span>
				<span class="col-right">Penjualan Bersih</span>
				<span>Kas/Bank</span>
				<span>Sumber</span>
				<span>Status</span>
				<span>Aksi</span>
			</div>

			{#each groupedByDate as group (group.date)}
				<!-- Date group header -->
				<div class="date-group-header">
					<span class="date-group-label">{formatDate(group.date)}</span>
				</div>

				{#each group.rows as row (row.id)}
					<div class="table-row sales-grid">
						<!-- Date (hidden on grouped view, shown for context) -->
						<span class="cell-text cell-date">{row.sales_date}</span>

						<!-- Channel -->
						<span class="cell-name">{row.channel}</span>

						<!-- Payment method -->
						<span class="cell-text">{row.payment_method}</span>

						<!-- Gross sales -->
						<span class="cell-price col-right">{formatRupiah(row.gross_sales)}</span>

						<!-- Discount -->
						<span class="cell-text col-right">{formatRupiah(row.discount_amount)}</span>

						<!-- Net sales -->
						<span class="cell-price col-right">{formatRupiah(row.net_sales)}</span>

						<!-- Cash account -->
						<span class="cell-text cell-truncate">{getCashAccountName(row.cash_account_id)}</span>

						<!-- Source badge -->
						<span class="cell-badge">
							<span
								class="source-badge"
								class:source-pos={row.source === 'pos'}
								class:source-manual={row.source === 'manual'}
							>{row.source === 'pos' ? 'POS' : 'Manual'}</span>
						</span>

						<!-- Posted status -->
						<span class="cell-badge">
							{#if row.posted_at}
								<span class="status-badge status-posted">Posted</span>
							{:else}
								<span class="status-badge status-draft">Belum</span>
							{/if}
						</span>

						<!-- Actions -->
						<span class="col-actions">
							{#if isManualUnposted(row)}
								<button type="button" class="btn-icon" onclick={() => openEditModal(row)}>Ubah</button>
								<form
									method="POST"
									action="?/delete"
									use:enhance={({ formData }) => {
										formData.set('id', row.id);
										submitting = true;
										showSuccess = false;
										showError = true;
										return async ({ result, update }) => {
											submitting = false;
											if (result.type === 'success') {
												showSuccess = true;
												successMessage = 'Entri penjualan berhasil dihapus.';
											}
											await update();
										};
									}}
								>
									<button
										type="submit"
										class="btn-icon btn-danger"
										disabled={submitting}
										onclick={(e) => { if (!confirm('Hapus entri penjualan ini?')) e.preventDefault(); }}
									>Hapus</button>
								</form>
							{/if}
						</span>
					</div>
				{/each}
			{/each}
		</div>
	{/if}

	<!-- ═══════════════════════════════════════ -->
	<!-- Sync POS dialog                        -->
	<!-- ═══════════════════════════════════════ -->
	{#if showSyncDialog}
		<div class="modal-overlay" role="presentation" onclick={closeSyncDialog}>
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<!-- svelte-ignore a11y_click_events_have_key_events -->
			<div class="modal-content" onclick={(e) => e.stopPropagation()}>
				<div class="modal-header">
					<h3 class="modal-title">Sinkronkan POS</h3>
					<button type="button" class="modal-close" onclick={closeSyncDialog}>&times;</button>
				</div>

				<p class="modal-info">
					Aggregasi data order POS menjadi ringkasan penjualan harian. Data yang sudah ada akan di-update.
				</p>

				<form
					method="POST"
					action="?/syncPos"
					use:enhance={({ formData }) => {
						formData.set('data', buildSyncData());
						submitting = true;
						showSuccess = false;
						showError = true;
						return async ({ result, update }) => {
							submitting = false;
							if (result.type === 'success') {
								showSyncDialog = false;
								showSuccess = true;
								const count = (result.data as { syncedCount?: number })?.syncedCount ?? 0;
								successMessage = `POS berhasil disinkronkan. ${count} ringkasan diproses.`;
							}
							await update();
						};
					}}
				>
					<div class="sync-form-grid">
						<div class="form-group">
							<label for="sync-start_date" class="form-label">Dari Tanggal *</label>
							<input
								id="sync-start_date"
								type="date"
								class="input-field"
								bind:value={syncStartDate}
								required
							/>
						</div>
						<div class="form-group">
							<label for="sync-end_date" class="form-label">Sampai Tanggal *</label>
							<input
								id="sync-end_date"
								type="date"
								class="input-field"
								bind:value={syncEndDate}
								required
							/>
						</div>
					</div>

					<h4 class="mapping-title">Mapping Metode Bayar ke Kas/Bank</h4>

					<div class="mapping-grid">
						{#each syncPaymentMethods as method}
							<div class="mapping-row">
								<span class="mapping-method">{method}</span>
								<select
									class="input-field mapping-select"
									bind:value={syncCashAccounts[method]}
									required
								>
									<option value="">Pilih kas...</option>
									{#each data.cashAccounts as ca}
										<option value={ca.id}>{ca.cash_account_code} - {ca.cash_account_name}</option>
									{/each}
								</select>
							</div>
						{/each}
					</div>

					<div class="modal-actions">
						<button type="submit" class="btn-primary btn-sm" disabled={submitting}>
							{submitting ? 'Menyinkronkan...' : 'Sinkronkan'}
						</button>
						<button type="button" class="btn-secondary btn-sm" onclick={closeSyncDialog}>Batal</button>
					</div>
				</form>
			</div>
		</div>
	{/if}

	<!-- ═══════════════════════════════════════ -->
	<!-- Create / Edit modal                    -->
	<!-- ═══════════════════════════════════════ -->
	{#if showCreateModal}
		<div class="modal-overlay" role="presentation" onclick={closeCreateModal}>
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<!-- svelte-ignore a11y_click_events_have_key_events -->
			<div class="modal-content" onclick={(e) => e.stopPropagation()}>
				<div class="modal-header">
					<h3 class="modal-title">{editingId ? 'Edit Penjualan' : 'Tambah Penjualan Manual'}</h3>
					<button type="button" class="modal-close" onclick={closeCreateModal}>&times;</button>
				</div>

				<form
					method="POST"
					action={editingId ? '?/update' : '?/create'}
					use:enhance={({ formData }) => {
						const isEdit = !!editingId;
						if (editingId) {
							formData.set('id', editingId);
							formData.set('data', buildUpdateData());
						} else {
							formData.set('data', buildCreateData());
						}
						submitting = true;
						showSuccess = false;
						showError = true;
						return async ({ result, update }) => {
							submitting = false;
							if (result.type === 'success') {
								showCreateModal = false;
								editingId = null;
								showSuccess = true;
								successMessage = isEdit ? 'Entri penjualan berhasil diperbarui.' : 'Entri penjualan berhasil ditambahkan.';
							}
							await update();
						};
					}}
				>
					<div class="modal-form-grid">
						<!-- Date (create only) -->
						{#if !editingId}
							<div class="form-group">
								<label for="form-sales_date" class="form-label">Tanggal *</label>
								<input
									id="form-sales_date"
									type="date"
									class="input-field"
									bind:value={formSalesDate}
									required
								/>
							</div>
						{/if}

						<!-- Channel -->
						<div class="form-group">
							<label for="form-channel" class="form-label">Channel *</label>
							<select id="form-channel" class="input-field" bind:value={formChannel} required>
								{#each channelOptions as ch}
									<option value={ch}>{ch}</option>
								{/each}
							</select>
						</div>

						<!-- Payment method -->
						<div class="form-group">
							<label for="form-payment_method" class="form-label">Metode Bayar *</label>
							<select id="form-payment_method" class="input-field" bind:value={formPaymentMethod} required>
								{#each paymentMethodOptions as pm}
									<option value={pm}>{pm}</option>
								{/each}
							</select>
						</div>

						<!-- Gross sales -->
						<div class="form-group">
							<label for="form-gross_sales" class="form-label">Penjualan Kotor *</label>
							<input
								id="form-gross_sales"
								type="text"
								class="input-field input-right"
								inputmode="decimal"
								placeholder="0"
								bind:value={formGrossSales}
								required
							/>
						</div>

						<!-- Discount -->
						<div class="form-group">
							<label for="form-discount" class="form-label">Diskon</label>
							<input
								id="form-discount"
								type="text"
								class="input-field input-right"
								inputmode="decimal"
								placeholder="0"
								bind:value={formDiscountAmount}
							/>
						</div>

						<!-- Net sales (calculated) -->
						<div class="form-group">
							<span class="form-label">Penjualan Bersih</span>
							<span class="amount-display">{formatRupiah(formNetSales)}</span>
						</div>

						<!-- Cash account -->
						<div class="form-group">
							<label for="form-cash_account_id" class="form-label">Kas/Bank *</label>
							<select id="form-cash_account_id" class="input-field" bind:value={formCashAccountId} required>
								<option value="">Pilih kas...</option>
								{#each data.cashAccounts as ca}
									<option value={ca.id}>{ca.cash_account_code} - {ca.cash_account_name}</option>
								{/each}
							</select>
						</div>
					</div>

					<div class="modal-actions">
						<button type="submit" class="btn-primary btn-sm" disabled={submitting}>
							{submitting ? 'Menyimpan...' : 'Simpan'}
						</button>
						<button type="button" class="btn-secondary btn-sm" onclick={closeCreateModal}>Batal</button>
					</div>
				</form>
			</div>
		</div>
	{/if}

	<!-- ═══════════════════════════════════════ -->
	<!-- Post dialog                            -->
	<!-- ═══════════════════════════════════════ -->
	{#if showPostDialog}
		<div class="modal-overlay" role="presentation" onclick={closePostDialog}>
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<!-- svelte-ignore a11y_click_events_have_key_events -->
			<div class="modal-content modal-sm" onclick={(e) => e.stopPropagation()}>
				<div class="modal-header">
					<h3 class="modal-title">Posting Penjualan</h3>
					<button type="button" class="modal-close" onclick={closePostDialog}>&times;</button>
				</div>

				<p class="modal-info">
					Posting semua ringkasan penjualan yang belum diposting pada tanggal tertentu ke buku besar.
				</p>

				<form
					method="POST"
					action="?/postSales"
					use:enhance={({ formData }) => {
						formData.set('data', buildPostData());
						submitting = true;
						showSuccess = false;
						showError = true;
						return async ({ result, update }) => {
							submitting = false;
							if (result.type === 'success') {
								showPostDialog = false;
								showSuccess = true;
								const res = result.data as { postedCount?: number; transactionsCreated?: number };
								successMessage = `${res?.postedCount ?? 0} ringkasan di-posting, ${res?.transactionsCreated ?? 0} transaksi dibuat.`;
							}
							await update();
						};
					}}
				>
					<div class="post-form-grid">
						<div class="form-group">
							<label for="post-sales_date" class="form-label">Tanggal Penjualan *</label>
							<input
								id="post-sales_date"
								type="date"
								class="input-field"
								bind:value={postSalesDate}
								required
							/>
						</div>
						<div class="form-group">
							<label for="post-account_id" class="form-label">Akun Pendapatan *</label>
							<select id="post-account_id" class="input-field" bind:value={postAccountId} required>
								<option value="">Pilih akun...</option>
								{#each data.accounts.filter(a => a.account_type === 'Revenue') as acct}
									<option value={acct.id}>{acct.account_code} - {acct.account_name}</option>
								{/each}
							</select>
						</div>
					</div>

					<div class="modal-actions">
						<button type="submit" class="btn-primary btn-sm" disabled={submitting}>
							{submitting ? 'Memproses...' : 'Posting'}
						</button>
						<button type="button" class="btn-secondary btn-sm" onclick={closePostDialog}>Batal</button>
					</div>
				</form>
			</div>
		</div>
	{/if}
</div>

<style>
	.sales-page {
		max-width: 1400px;
		display: flex;
		flex-direction: column;
		gap: 20px;
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

	/* ── Banners ────────── */

	.success-banner {
		background-color: #ecfdf5;
		color: #065f46;
		font-size: 13px;
		font-weight: 500;
		padding: 10px 14px;
		border-radius: var(--radius-chip);
		display: flex;
		align-items: center;
		justify-content: space-between;
	}

	.dismiss-btn {
		background: none;
		border: none;
		color: #065f46;
		font-size: 12px;
		font-weight: 600;
		cursor: pointer;
		padding: 2px 8px;
		border-radius: 4px;
	}

	.dismiss-btn:hover {
		background-color: rgba(6, 95, 70, 0.1);
	}

	.error-banner {
		background-color: var(--color-error-bg);
		color: var(--color-error);
		font-size: 13px;
		font-weight: 500;
		padding: 10px 14px;
		border-radius: var(--radius-chip);
		display: flex;
		align-items: center;
		justify-content: space-between;
	}

	.dismiss-error {
		color: var(--color-error);
	}

	.dismiss-error:hover {
		background-color: rgba(220, 38, 38, 0.1);
	}

	/* ── Date filter bar ────────── */

	.date-filter-bar {
		display: flex;
		align-items: flex-end;
		gap: 12px;
		flex-wrap: wrap;
	}

	.filter-group {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.filter-label {
		font-size: 12px;
		font-weight: 500;
		color: var(--color-text-secondary);
	}

	.filter-actions {
		display: flex;
		gap: 8px;
	}

	.btn-filter {
		padding: 10px 16px;
		font-size: 13px;
		border: none;
		cursor: pointer;
	}

	/* ── Action bar ────────── */

	.action-bar {
		display: flex;
		gap: 8px;
		flex-wrap: wrap;
	}

	.btn-action {
		padding: 10px 16px;
		font-size: 13px;
		border: none;
		cursor: pointer;
	}

	/* ── Data table ────────── */

	.data-table {
		border: 1px solid var(--color-border);
		border-radius: var(--radius-card);
		overflow-x: auto;
	}

	.table-header {
		display: grid;
		gap: 8px;
		padding: 10px 12px;
		background-color: var(--color-surface);
		border-bottom: 1px solid var(--color-border);
		font-size: 11px;
		font-weight: 600;
		color: var(--color-text-secondary);
		text-transform: uppercase;
		letter-spacing: 0.02em;
		min-width: 1000px;
	}

	.table-row {
		display: grid;
		gap: 8px;
		padding: 10px 12px;
		border-bottom: 1px solid var(--color-border);
		align-items: center;
		min-width: 1000px;
	}

	.table-row:last-child {
		border-bottom: none;
	}

	.sales-grid {
		grid-template-columns: 80px 90px 80px 110px 80px 110px 1fr 70px 70px 90px;
	}

	/* ── Date group header ────────── */

	.date-group-header {
		padding: 8px 12px;
		background-color: var(--color-surface);
		border-bottom: 1px solid var(--color-border);
		min-width: 1000px;
	}

	.date-group-label {
		font-size: 12px;
		font-weight: 700;
		color: var(--color-text-primary);
	}

	/* ── Cell styles ────────── */

	.col-right {
		text-align: right;
	}

	.cell-text {
		font-size: 12px;
		color: var(--color-text-secondary);
	}

	.cell-date {
		font-size: 11px;
		font-family: monospace;
	}

	.cell-name {
		font-size: 12px;
		font-weight: 600;
		color: var(--color-text-primary);
	}

	.cell-price {
		font-size: 12px;
		font-weight: 600;
		color: var(--color-text-primary);
	}

	.cell-truncate {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.cell-badge {
		font-size: 12px;
	}

	/* ── Source badges ────────── */

	.source-badge {
		font-size: 11px;
		font-weight: 600;
		padding: 2px 8px;
		border-radius: var(--radius-chip);
		text-transform: uppercase;
		letter-spacing: 0.02em;
		white-space: nowrap;
	}

	.source-pos {
		background-color: #dcfce7;
		color: #166534;
		border: 1px solid #bbf7d0;
	}

	.source-manual {
		background-color: #dbeafe;
		color: #1e40af;
		border: 1px solid #bfdbfe;
	}

	/* ── Status badges ────────── */

	.status-badge {
		font-size: 11px;
		font-weight: 600;
		padding: 2px 8px;
		border-radius: var(--radius-chip);
		text-transform: uppercase;
		letter-spacing: 0.02em;
		white-space: nowrap;
	}

	.status-draft {
		background-color: #fef9c3;
		color: #854d0e;
		border: 1px solid #fde68a;
	}

	.status-posted {
		background-color: #dcfce7;
		color: #166534;
		border: 1px solid #bbf7d0;
	}

	/* ── Action buttons ────────── */

	.col-actions {
		display: flex;
		align-items: center;
		gap: 2px;
		flex-wrap: wrap;
	}

	.btn-icon {
		background: none;
		border: none;
		font-size: 11px;
		font-weight: 600;
		color: var(--color-text-secondary);
		cursor: pointer;
		padding: 4px 6px;
		border-radius: 4px;
		white-space: nowrap;
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

	/* ── Empty state ────────── */

	.empty-state {
		text-align: center;
		padding: 48px 24px;
	}

	.empty-text {
		font-size: 13px;
		color: var(--color-text-secondary);
		margin: 0;
	}

	/* ── Modal ────────── */

	.modal-overlay {
		position: fixed;
		inset: 0;
		background-color: rgba(0, 0, 0, 0.4);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 100;
		padding: 24px;
	}

	.modal-content {
		background-color: var(--color-bg);
		border-radius: var(--radius-sheet);
		width: 100%;
		max-width: 640px;
		max-height: 90vh;
		overflow-y: auto;
		padding: 24px;
		box-shadow: 0 8px 32px rgba(0, 0, 0, 0.15);
	}

	.modal-sm {
		max-width: 440px;
	}

	.modal-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		margin-bottom: 16px;
	}

	.modal-title {
		font-size: var(--text-heading);
		font-weight: 700;
		color: var(--color-text-primary);
		margin: 0;
	}

	.modal-close {
		background: none;
		border: none;
		font-size: 24px;
		line-height: 1;
		color: var(--color-text-secondary);
		cursor: pointer;
		padding: 4px 8px;
		border-radius: 4px;
	}

	.modal-close:hover {
		background-color: var(--color-surface);
		color: var(--color-text-primary);
	}

	.modal-info {
		font-size: 13px;
		color: var(--color-text-secondary);
		margin: 0 0 16px;
		line-height: 1.5;
	}

	.modal-form-grid {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 12px;
	}

	.modal-actions {
		display: flex;
		gap: 8px;
		margin-top: 16px;
	}

	.btn-sm {
		padding: 8px 16px;
		font-size: 13px;
		border: none;
		cursor: pointer;
	}

	/* ── Form groups ────────── */

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

	.input-right {
		text-align: right;
	}

	.amount-display {
		font-size: 13px;
		font-weight: 600;
		color: var(--color-text-primary);
		padding: 10px 0;
	}

	/* ── Sync form ────────── */

	.sync-form-grid {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 12px;
		margin-bottom: 16px;
	}

	.mapping-title {
		font-size: 13px;
		font-weight: 600;
		color: var(--color-text-primary);
		margin: 0 0 12px;
	}

	.mapping-grid {
		display: flex;
		flex-direction: column;
		gap: 10px;
	}

	.mapping-row {
		display: flex;
		align-items: center;
		gap: 12px;
	}

	.mapping-method {
		font-size: 13px;
		font-weight: 600;
		color: var(--color-text-primary);
		min-width: 70px;
	}

	.mapping-select {
		flex: 1;
	}

	/* ── Post dialog ────────── */

	.post-form-grid {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}

	/* ── Mobile responsive ────────── */

	@media (max-width: 768px) {
		.date-filter-bar {
			flex-direction: column;
			align-items: stretch;
		}

		.action-bar {
			flex-direction: column;
		}

		.table-header {
			display: none;
		}

		.date-group-header {
			min-width: 0;
		}

		.table-row {
			display: flex;
			flex-wrap: wrap;
			gap: 6px;
			min-width: 0;
		}

		.sales-grid {
			grid-template-columns: 1fr;
		}

		.col-right {
			text-align: left;
		}

		.col-actions {
			width: 100%;
			padding-top: 4px;
			border-top: 1px solid var(--color-border);
		}

		.modal-form-grid {
			grid-template-columns: 1fr;
		}

		.sync-form-grid {
			grid-template-columns: 1fr;
		}

		.mapping-row {
			flex-direction: column;
			align-items: stretch;
		}

		.modal-content {
			padding: 16px;
		}
	}
</style>
