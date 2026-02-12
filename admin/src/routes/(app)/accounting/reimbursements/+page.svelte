<!--
  Reimbursement Management — review, match, batch, and post reimbursement requests.
  Replaces Google Sheet workflow. Items come via WhatsApp auto-parse or manual entry.
-->
<script lang="ts">
	import { enhance } from '$app/forms';
	import { goto } from '$app/navigation';
	import { formatRupiah } from '$lib/utils/format';
	import type { AcctItem, AcctReimbursementRequest } from '$lib/types/api';

	let { data, form } = $props();

	// ── Line-type options ────────────────────
	const lineTypeOptions = ['ASSET', 'INVENTORY', 'EXPENSE', 'SALES', 'COGS', 'LIABILITY', 'CAPITAL', 'DRAWING'];
	const statusOptions = ['Draft', 'Ready', 'Posted'] as const;

	// ── Filter state (initial values from server, not reactive to data changes) ────────────────────
	const initialFilterStatus = data.filterStatus;
	const initialFilterRequester = data.filterRequester;
	let filterStatus = $state(initialFilterStatus);
	let filterRequester = $state(initialFilterRequester);

	// ── Selection state (for batch assign) ────────────────────
	let selectedIds = $state<Set<string>>(new Set());

	// ── Edit modal state ────────────────────
	let editingId = $state<string | null>(null);
	let editDescription = $state('');
	let editItemId = $state<string | null>(null);
	let editQty = $state('');
	let editUnitPrice = $state('');
	let editLineType = $state('');
	let editAccountId = $state('');
	let editStatus = $state<'Draft' | 'Ready'>('Draft');
	let editExpenseDate = $state('');
	let editReceiptLink = $state('');
	let editShowSuggestions = $state(false);
	let editFilteredItems = $state<AcctItem[]>([]);

	// ── Batch post dialog state ────────────────────
	let showPostDialog = $state(false);
	let postBatchId = $state('');
	let postPaymentDate = $state(new Date().toISOString().slice(0, 10));
	const initialCashAccountId = data.cashAccounts.length > 0 ? data.cashAccounts[0].id : '';
	let postCashAccountId = $state(initialCashAccountId);

	// ── Submission state ────────────────────
	let submitting = $state(false);
	let showSuccess = $state(false);
	let successMessage = $state('');
	let showError = $state(true);

	// ── Derived: computed amount for edit form ────────────────────
	let editAmount = $derived.by(() => {
		const qty = parseFloat(editQty) || 0;
		const price = parseFloat(editUnitPrice) || 0;
		return qty * price;
	});

	// ── Derived: total selected amount ────────────────────
	let selectedTotal = $derived.by(() => {
		let total = 0;
		for (const r of data.reimbursements) {
			if (selectedIds.has(r.id)) {
				total += parseFloat(r.amount) || 0;
			}
		}
		return total;
	});

	// ── Derived: unique batch IDs from current data ────────────────────
	let batchIds = $derived.by(() => {
		const ids = new Set<string>();
		for (const r of data.reimbursements) {
			if (r.batch_id) ids.add(r.batch_id);
		}
		return Array.from(ids).sort();
	});

	// ── Derived: draft items for select-all checkbox ────────────────────
	let draftItems = $derived(data.reimbursements.filter((r) => r.status === 'Draft'));

	let allDraftsSelected = $derived(
		draftItems.length > 0 && draftItems.every((r) => selectedIds.has(r.id))
	);

	// ── Item autocomplete (same pattern as purchase page) ────────────────────
	function filterItems(query: string): AcctItem[] {
		if (!query.trim() || data.items.length === 0) return [];
		const q = query.toLowerCase();
		return data.items
			.filter((item) => {
				const keywords = item.keywords.toLowerCase().split(',').map((k) => k.trim());
				const nameMatch = item.item_name.toLowerCase().includes(q);
				const keywordMatch = keywords.some((kw) => kw.includes(q));
				return nameMatch || keywordMatch;
			})
			.slice(0, 5);
	}

	function onEditDescriptionInput() {
		editItemId = null;
		editFilteredItems = filterItems(editDescription);
		editShowSuggestions = editFilteredItems.length > 0;
	}

	function selectEditItem(item: AcctItem) {
		editDescription = item.item_name;
		editItemId = item.id;
		if (item.last_price) {
			editUnitPrice = item.last_price;
		}
		editShowSuggestions = false;
		editFilteredItems = [];
	}

	function hideEditSuggestions() {
		setTimeout(() => {
			editShowSuggestions = false;
		}, 200);
	}

	// ── Helpers ────────────────────
	function getItemName(itemId: string | null): string {
		if (!itemId) return '-';
		const item = data.items.find((i) => i.id === itemId);
		return item ? item.item_name : '-';
	}

	function getAccountName(accountId: string): string {
		const acct = data.accounts.find((a) => a.id === accountId);
		return acct ? `${acct.account_code} - ${acct.account_name}` : '-';
	}

	function formatDate(dateStr: string): string {
		if (!dateStr) return '-';
		const d = new Date(dateStr);
		return d.toLocaleDateString('id-ID', { day: '2-digit', month: 'short', year: 'numeric' });
	}

	function shortBatchId(batchId: string | null): string {
		if (!batchId) return '-';
		return batchId.slice(0, 8);
	}

	// ── Filter apply ────────────────────
	function applyFilters() {
		const params = new URLSearchParams();
		if (filterStatus) params.set('status', filterStatus);
		if (filterRequester) params.set('requester', filterRequester);
		const qs = params.toString();
		goto(qs ? `?${qs}` : '?', { invalidateAll: true });
	}

	function clearFilters() {
		filterStatus = '';
		filterRequester = '';
		goto('?', { invalidateAll: true });
	}

	// ── Selection ────────────────────
	function toggleSelect(id: string) {
		const next = new Set(selectedIds);
		if (next.has(id)) {
			next.delete(id);
		} else {
			next.add(id);
		}
		selectedIds = next;
	}

	function toggleSelectAll() {
		if (allDraftsSelected) {
			selectedIds = new Set();
		} else {
			selectedIds = new Set(draftItems.map((r) => r.id));
		}
	}

	// ── Edit modal open/close ────────────────────
	function openEdit(r: AcctReimbursementRequest) {
		editingId = r.id;
		editDescription = r.description;
		editItemId = r.item_id;
		editQty = r.qty;
		editUnitPrice = r.unit_price;
		editLineType = r.line_type;
		editAccountId = r.account_id;
		editStatus = r.status === 'Posted' ? 'Ready' : r.status as 'Draft' | 'Ready';
		editExpenseDate = r.expense_date?.slice(0, 10) ?? '';
		editReceiptLink = r.receipt_link ?? '';
		editShowSuggestions = false;
		editFilteredItems = [];
	}

	function closeEdit() {
		editingId = null;
	}

	// ── Build edit payload (serialized in enhance callback) ────────────────────
	function buildEditData(): string {
		return JSON.stringify({
			expense_date: editExpenseDate,
			item_id: editItemId || null,
			description: editDescription,
			qty: editQty,
			unit_price: editUnitPrice,
			amount: (Math.round(editAmount * 100) / 100).toFixed(2),
			line_type: editLineType,
			account_id: editAccountId,
			status: editStatus,
			receipt_link: editReceiptLink || null
		});
	}

	// ── Build batch assign payload ────────────────────
	function buildBatchAssignIds(): string {
		return JSON.stringify(Array.from(selectedIds));
	}

	// ── Build batch post payload ────────────────────
	function buildBatchPostData(): string {
		return JSON.stringify({
			batch_id: postBatchId,
			payment_date: postPaymentDate,
			cash_account_id: postCashAccountId
		});
	}

	// ── Batch post dialog open ────────────────────
	function openPostDialog(batchId: string) {
		postBatchId = batchId;
		postPaymentDate = new Date().toISOString().slice(0, 10);
		postCashAccountId = data.cashAccounts.length > 0 ? data.cashAccounts[0].id : '';
		showPostDialog = true;
	}

	function closePostDialog() {
		showPostDialog = false;
	}
	// ── Keyboard handler ────────────────────
	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			if (showPostDialog) closePostDialog();
			else if (editingId) closeEdit();
		}
	}
</script>

<svelte:window onkeydown={handleKeydown} />

<svelte:head>
	<title>Reimbursement - Kiwari POS</title>
</svelte:head>

<div class="reimburse-page">
	<div class="page-header">
		<h1 class="page-title">Reimbursement</h1>
		<p class="page-subtitle">Review & posting permintaan reimbursement</p>
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
	<!-- Filter bar                             -->
	<!-- ═══════════════════════════════════════ -->
	<div class="filter-bar">
		<div class="filter-group">
			<label for="filter-status" class="filter-label">Status</label>
			<select id="filter-status" class="input-field filter-select" bind:value={filterStatus}>
				<option value="">Semua</option>
				{#each statusOptions as opt}
					<option value={opt}>{opt}</option>
				{/each}
			</select>
		</div>
		<div class="filter-group">
			<label for="filter-requester" class="filter-label">Requester</label>
			<input
				id="filter-requester"
				type="text"
				class="input-field filter-input"
				placeholder="Nama requester..."
				bind:value={filterRequester}
				onkeydown={(e) => { if (e.key === 'Enter') applyFilters(); }}
			/>
		</div>
		<div class="filter-actions">
			<button type="button" class="btn-primary btn-filter" onclick={applyFilters}>Terapkan</button>
			{#if data.filterStatus || data.filterRequester}
				<button type="button" class="btn-secondary btn-filter" onclick={clearFilters}>Reset</button>
			{/if}
		</div>
	</div>

	<!-- ═══════════════════════════════════════ -->
	<!-- Batch controls                         -->
	<!-- ═══════════════════════════════════════ -->
	{#if selectedIds.size > 0}
		<div class="batch-bar">
			<span class="batch-info">
				{selectedIds.size} item dipilih — Total: {formatRupiah(selectedTotal)}
			</span>
			<form
				method="POST"
				action="?/batchAssign"
				use:enhance={({ formData }) => {
					formData.set('ids', buildBatchAssignIds());
					submitting = true;
					showSuccess = false;
					showError = true;
					return async ({ result, update }) => {
						submitting = false;
						if (result.type === 'success') {
							selectedIds = new Set();
							showSuccess = true;
							successMessage = `Batch berhasil dibuat. ${(result.data as { assigned?: number })?.assigned ?? ''} item di-assign.`;
						}
						await update();
					};
				}}
			>
				<button type="submit" class="btn-primary btn-batch" disabled={submitting}>
					{submitting ? 'Membuat batch...' : 'Buat Batch'}
				</button>
			</form>
		</div>
	{/if}

	<!-- ═══════════════════════════════════════ -->
	<!-- Data table                              -->
	<!-- ═══════════════════════════════════════ -->
	{#if data.reimbursements.length === 0}
		<div class="empty-state">
			<p class="empty-text">Belum ada data reimbursement.</p>
		</div>
	{:else}
		<div class="data-table">
			<div class="table-header reimburse-grid">
				<span class="col-check">
					<input
						type="checkbox"
						checked={allDraftsSelected}
						onchange={toggleSelectAll}
						title="Pilih semua Draft"
					/>
				</span>
				<span>Tanggal</span>
				<span>Requester</span>
				<span>Deskripsi</span>
				<span class="col-right">Qty</span>
				<span class="col-right">Harga</span>
				<span class="col-right">Jumlah</span>
				<span>Item Match</span>
				<span>Status</span>
				<span>Batch</span>
				<span>Aksi</span>
			</div>

			{#each data.reimbursements as r (r.id)}
				<div class="table-row reimburse-grid" class:row-selected={selectedIds.has(r.id)}>
					<!-- Checkbox (Draft only) -->
					<span class="col-check">
						{#if r.status === 'Draft'}
							<input
								type="checkbox"
								checked={selectedIds.has(r.id)}
								onchange={() => toggleSelect(r.id)}
							/>
						{/if}
					</span>

					<!-- Date -->
					<span class="cell-text">{formatDate(r.expense_date)}</span>

					<!-- Requester -->
					<span class="cell-name">{r.requester}</span>

					<!-- Description -->
					<span class="cell-text cell-desc">{r.description}</span>

					<!-- Qty -->
					<span class="cell-text col-right">{r.qty}</span>

					<!-- Unit price -->
					<span class="cell-price col-right">{formatRupiah(r.unit_price)}</span>

					<!-- Amount -->
					<span class="cell-price col-right">{formatRupiah(r.amount)}</span>

					<!-- Item match -->
					<span class="cell-text">{getItemName(r.item_id)}</span>

					<!-- Status badge -->
					<span class="cell-badge">
						<span
							class="status-badge"
							class:status-draft={r.status === 'Draft'}
							class:status-ready={r.status === 'Ready'}
							class:status-posted={r.status === 'Posted'}
						>{r.status}</span>
					</span>

					<!-- Batch ID -->
					<span class="cell-code">{shortBatchId(r.batch_id)}</span>

					<!-- Actions -->
					<span class="col-actions">
						{#if r.status !== 'Posted'}
							<button
								type="button"
								class="btn-icon"
								onclick={() => openEdit(r)}
							>Ubah</button>
						{/if}
						{#if r.status === 'Draft'}
							<form
								method="POST"
								action="?/delete"
								use:enhance={({ formData }) => {
									formData.set('id', r.id);
									submitting = true;
									showSuccess = false;
									showError = true;
									return async ({ result, update }) => {
										submitting = false;
										if (result.type === 'success') {
											showSuccess = true;
											successMessage = 'Item berhasil dihapus.';
										}
										await update();
									};
								}}
							>
								<button
									type="submit"
									class="btn-icon btn-danger"
									disabled={submitting}
									onclick={(e) => { if (!confirm('Hapus item "' + r.description + '"?')) e.preventDefault(); }}
								>Hapus</button>
							</form>
						{/if}
						{#if r.status === 'Ready' && r.batch_id}
							<button
								type="button"
								class="btn-icon btn-post"
								onclick={() => openPostDialog(r.batch_id!)}
							>Post</button>
						{/if}
					</span>
				</div>
			{/each}
		</div>

		<!-- Batch summary: show unique batches with Ready status and post button -->
		{#if batchIds.length > 0}
			<div class="batch-summary-card">
				<h3 class="batch-summary-title">Batch Tersedia</h3>
				<div class="batch-list">
					{#each batchIds as bid}
						{@const batchItems = data.reimbursements.filter((r) => r.batch_id === bid)}
						{@const batchTotal = batchItems.reduce((s, r) => s + (parseFloat(r.amount) || 0), 0)}
						{@const isReady = batchItems.every((r) => r.status === 'Ready')}
						{@const isPosted = batchItems.some((r) => r.status === 'Posted')}
						<div class="batch-row">
							<span class="batch-id" title={bid}>{shortBatchId(bid)}</span>
							<span class="batch-count">{batchItems.length} item</span>
							<span class="batch-total">{formatRupiah(batchTotal)}</span>
							<span class="batch-status">
								{#if isPosted}
									<span class="status-badge status-posted">Posted</span>
								{:else if isReady}
									<span class="status-badge status-ready">Ready</span>
								{:else}
									<span class="status-badge status-draft">Belum Ready</span>
								{/if}
							</span>
							<span class="batch-action">
								{#if isReady && !isPosted}
									<button
										type="button"
										class="btn-primary btn-sm"
										onclick={() => openPostDialog(bid)}
									>Post Batch</button>
								{/if}
							</span>
						</div>
					{/each}
				</div>
			</div>
		{/if}
	{/if}

	<!-- ═══════════════════════════════════════ -->
	<!-- Edit modal                              -->
	<!-- ═══════════════════════════════════════ -->
	{#if editingId}
		<div class="modal-overlay" role="presentation" onclick={closeEdit}>
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<!-- svelte-ignore a11y_click_events_have_key_events -->
			<div class="modal-content" onclick={(e) => e.stopPropagation()}>
				<div class="modal-header">
					<h3 class="modal-title">Edit Reimbursement</h3>
					<button type="button" class="modal-close" onclick={closeEdit}>&times;</button>
				</div>

				<form
					method="POST"
					action="?/update"
					use:enhance={({ formData }) => {
						formData.set('id', editingId!);
						formData.set('data', buildEditData());
						submitting = true;
						showSuccess = false;
						return async ({ result, update }) => {
							submitting = false;
							if (result.type === 'success') {
								editingId = null;
								showSuccess = true;
								successMessage = 'Item berhasil diperbarui.';
							}
							await update();
						};
					}}
				>
					<div class="modal-form-grid">
						<!-- Expense date -->
						<div class="form-group">
							<label for="edit-expense_date" class="form-label">Tanggal Pengeluaran *</label>
							<input
								id="edit-expense_date"
								type="date"
								class="input-field"
								bind:value={editExpenseDate}
								required
							/>
						</div>

						<!-- Status -->
						<div class="form-group">
							<label for="edit-status" class="form-label">Status *</label>
							<select id="edit-status" class="input-field" bind:value={editStatus} required>
								<option value="Draft">Draft</option>
								<option value="Ready">Ready</option>
							</select>
						</div>

						<!-- Description with autocomplete -->
						<div class="form-group form-group-wide">
							<label for="edit-description" class="form-label">Deskripsi *</label>
							<div class="autocomplete-wrapper">
								<input
									id="edit-description"
									type="text"
									class="input-field"
									placeholder="Ketik nama item..."
									bind:value={editDescription}
									oninput={onEditDescriptionInput}
									onfocusout={hideEditSuggestions}
									autocomplete="off"
									required
								/>
								{#if editShowSuggestions && editFilteredItems.length > 0}
									<div class="suggestions-dropdown">
										{#each editFilteredItems as item}
											<button
												type="button"
												class="suggestion-item"
												onmousedown={() => selectEditItem(item)}
											>
												<span class="suggestion-name">{item.item_name}</span>
												{#if item.last_price}
													<span class="suggestion-price">{formatRupiah(item.last_price)}/{item.unit}</span>
												{/if}
											</button>
										{/each}
									</div>
								{/if}
							</div>
							{#if editItemId}
								<span class="matched-item">Matched: {getItemName(editItemId)}</span>
							{/if}
						</div>

						<!-- Qty -->
						<div class="form-group">
							<label for="edit-qty" class="form-label">Qty *</label>
							<input
								id="edit-qty"
								type="number"
								class="input-field input-right"
								step="any"
								min="0"
								bind:value={editQty}
								required
							/>
						</div>

						<!-- Unit price -->
						<div class="form-group">
							<label for="edit-unit_price" class="form-label">Harga Satuan *</label>
							<input
								id="edit-unit_price"
								type="text"
								class="input-field input-right"
								inputmode="decimal"
								placeholder="0"
								bind:value={editUnitPrice}
								required
							/>
						</div>

						<!-- Amount (calculated) -->
						<div class="form-group">
							<span class="form-label">Jumlah</span>
							<span class="amount-display">{formatRupiah(editAmount)}</span>
						</div>

						<!-- Line type -->
						<div class="form-group">
							<label for="edit-line_type" class="form-label">Tipe Baris *</label>
							<select id="edit-line_type" class="input-field" bind:value={editLineType} required>
								<option value="">Pilih tipe baris...</option>
								{#each lineTypeOptions as opt}
									<option value={opt}>{opt}</option>
								{/each}
							</select>
						</div>

						<!-- Account -->
						<div class="form-group">
							<label for="edit-account_id" class="form-label">Akun Pembukuan *</label>
							<select id="edit-account_id" class="input-field" bind:value={editAccountId} required>
								<option value="">Pilih akun...</option>
								{#each data.accounts as acct}
									<option value={acct.id}>{acct.account_code} - {acct.account_name}</option>
								{/each}
							</select>
						</div>

						<!-- Receipt link -->
						<div class="form-group form-group-wide">
							<label for="edit-receipt_link" class="form-label">Link Bukti (opsional)</label>
							<input
								id="edit-receipt_link"
								type="url"
								class="input-field"
								placeholder="https://..."
								bind:value={editReceiptLink}
							/>
						</div>
					</div>

					<div class="modal-actions">
						<button type="submit" class="btn-primary btn-sm" disabled={submitting}>
							{submitting ? 'Menyimpan...' : 'Simpan'}
						</button>
						<button type="button" class="btn-secondary btn-sm" onclick={closeEdit}>Batal</button>
					</div>
				</form>
			</div>
		</div>
	{/if}

	<!-- ═══════════════════════════════════════ -->
	<!-- Batch post dialog                       -->
	<!-- ═══════════════════════════════════════ -->
	{#if showPostDialog}
		<div class="modal-overlay" role="presentation" onclick={closePostDialog}>
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<!-- svelte-ignore a11y_click_events_have_key_events -->
			<div class="modal-content modal-sm" onclick={(e) => e.stopPropagation()}>
				<div class="modal-header">
					<h3 class="modal-title">Post Batch</h3>
					<button type="button" class="modal-close" onclick={closePostDialog}>&times;</button>
				</div>

				<p class="post-info">
					Batch: <strong>{shortBatchId(postBatchId)}</strong>
				</p>

				<form
					method="POST"
					action="?/batchPost"
					use:enhance={({ formData }) => {
						formData.set('data', buildBatchPostData());
						submitting = true;
						showSuccess = false;
						return async ({ result, update }) => {
							submitting = false;
							if (result.type === 'success') {
								showPostDialog = false;
								showSuccess = true;
								successMessage = `Batch berhasil di-post. ${(result.data as { posted?: number })?.posted ?? ''} transaksi dibuat.`;
							}
							await update();
						};
					}}
				>
					<div class="post-form-grid">
						<div class="form-group">
							<label for="post-payment_date" class="form-label">Tanggal Pembayaran *</label>
							<input
								id="post-payment_date"
								type="date"
								class="input-field"
								bind:value={postPaymentDate}
								required
							/>
						</div>
						<div class="form-group">
							<label for="post-cash_account_id" class="form-label">Kas/Bank *</label>
							<select id="post-cash_account_id" class="input-field" bind:value={postCashAccountId} required>
								<option value="">Pilih kas...</option>
								{#each data.cashAccounts as ca}
									<option value={ca.id}>{ca.cash_account_code} - {ca.cash_account_name}</option>
								{/each}
							</select>
						</div>
					</div>

					<div class="modal-actions">
						<button type="submit" class="btn-primary btn-sm" disabled={submitting}>
							{submitting ? 'Memproses...' : 'Konfirmasi Post'}
						</button>
						<button type="button" class="btn-secondary btn-sm" onclick={closePostDialog}>Batal</button>
					</div>
				</form>
			</div>
		</div>
	{/if}
</div>

<style>
	.reimburse-page {
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

	/* ── Filter bar ────────── */

	.filter-bar {
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

	.filter-select {
		min-width: 140px;
	}

	.filter-input {
		min-width: 180px;
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

	/* ── Batch bar ────────── */

	.batch-bar {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 12px 16px;
		background-color: #eff6ff;
		border: 1px solid #bfdbfe;
		border-radius: var(--radius-chip);
	}

	.batch-info {
		font-size: 13px;
		font-weight: 600;
		color: #1e40af;
	}

	.btn-batch {
		padding: 8px 16px;
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

	.table-row.row-selected {
		background-color: #eff6ff;
	}

	.reimburse-grid {
		grid-template-columns: 36px 80px 90px 1.5fr 50px 90px 90px 1fr 70px 70px 100px;
	}

	/* ── Cell styles ────────── */

	.col-check {
		display: flex;
		align-items: center;
		justify-content: center;
	}

	.col-check input[type='checkbox'] {
		width: 16px;
		height: 16px;
		accent-color: var(--color-primary);
		cursor: pointer;
	}

	.col-right {
		text-align: right;
	}

	.cell-text {
		font-size: 12px;
		color: var(--color-text-secondary);
	}

	.cell-desc {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
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

	.cell-code {
		font-size: 11px;
		font-family: monospace;
		color: var(--color-text-secondary);
	}

	.cell-badge {
		font-size: 12px;
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

	.status-ready {
		background-color: #dbeafe;
		color: #1e40af;
		border: 1px solid #bfdbfe;
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

	.btn-post {
		color: var(--color-primary);
	}

	.btn-post:hover {
		background-color: #ecfdf5;
		color: var(--color-primary);
	}

	/* ── Batch summary card ────────── */

	.batch-summary-card {
		background-color: var(--color-bg);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-card);
		padding: 16px;
	}

	.batch-summary-title {
		font-size: var(--text-title);
		font-weight: 600;
		color: var(--color-text-primary);
		margin: 0 0 12px;
	}

	.batch-list {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	.batch-row {
		display: flex;
		align-items: center;
		gap: 16px;
		padding: 8px 12px;
		background-color: var(--color-surface);
		border-radius: var(--radius-chip);
	}

	.batch-id {
		font-size: 12px;
		font-family: monospace;
		font-weight: 600;
		color: var(--color-text-primary);
		min-width: 80px;
	}

	.batch-count {
		font-size: 12px;
		color: var(--color-text-secondary);
		min-width: 60px;
	}

	.batch-total {
		font-size: 13px;
		font-weight: 700;
		color: var(--color-text-primary);
		min-width: 100px;
	}

	.batch-status {
		min-width: 80px;
	}

	.batch-action {
		margin-left: auto;
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

	.modal-form-grid {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 12px;
	}

	.form-group {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.form-group-wide {
		grid-column: 1 / -1;
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

	.matched-item {
		font-size: 11px;
		color: var(--color-primary);
		font-weight: 500;
		margin-top: 2px;
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

	/* ── Autocomplete ────────── */

	.autocomplete-wrapper {
		position: relative;
	}

	.suggestions-dropdown {
		position: absolute;
		top: 100%;
		left: 0;
		right: 0;
		z-index: 50;
		background-color: var(--color-bg);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-chip);
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
		margin-top: 4px;
		overflow: hidden;
	}

	.suggestion-item {
		display: flex;
		align-items: center;
		justify-content: space-between;
		width: 100%;
		padding: 10px 12px;
		border: none;
		background: none;
		font-size: 13px;
		color: var(--color-text-primary);
		cursor: pointer;
		text-align: left;
	}

	.suggestion-item:hover {
		background-color: var(--color-surface);
	}

	.suggestion-item + .suggestion-item {
		border-top: 1px solid var(--color-border);
	}

	.suggestion-name {
		font-weight: 500;
	}

	.suggestion-price {
		font-size: 12px;
		color: var(--color-text-secondary);
	}

	/* ── Post dialog ────────── */

	.post-info {
		font-size: 13px;
		color: var(--color-text-secondary);
		margin: 0 0 16px;
	}

	.post-form-grid {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}

	/* ── Mobile responsive ────────── */

	@media (max-width: 768px) {
		.filter-bar {
			flex-direction: column;
			align-items: stretch;
		}

		.filter-select,
		.filter-input {
			min-width: 0;
			width: 100%;
		}

		.filter-actions {
			justify-content: flex-start;
		}

		.table-header {
			display: none;
		}

		.table-row {
			display: flex;
			flex-wrap: wrap;
			gap: 6px;
			min-width: 0;
		}

		.reimburse-grid {
			grid-template-columns: 1fr;
		}

		.col-check {
			justify-content: flex-start;
		}

		.col-right {
			text-align: left;
		}

		.col-actions {
			width: 100%;
			padding-top: 4px;
			border-top: 1px solid var(--color-border);
		}

		.batch-bar {
			flex-direction: column;
			gap: 12px;
			align-items: stretch;
		}

		.batch-row {
			flex-wrap: wrap;
		}

		.batch-action {
			margin-left: 0;
			width: 100%;
		}

		.modal-form-grid {
			grid-template-columns: 1fr;
		}

		.modal-content {
			padding: 16px;
		}
	}
</style>
