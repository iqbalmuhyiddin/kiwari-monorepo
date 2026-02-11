<!--
  Purchase Entry — multi-line form for recording daily purchases.
  Item autocomplete filters master items by keywords.
-->
<script lang="ts">
	import { enhance } from '$app/forms';
	import { formatRupiah } from '$lib/utils/format';
	import type { AcctItem } from '$lib/types/api';

	let { data, form } = $props();

	// ── Types ────────────────────
	interface LineItem {
		_key: number;
		description: string;
		item_id: string | null;
		quantity: string;
		unit_price: string;
		showSuggestions: boolean;
		filteredItems: AcctItem[];
	}

	// ── State ────────────────────
	let lineKeyCounter = 0;
	let transactionDate = $state(new Date().toISOString().slice(0, 10));
	let accountId = $state(getDefaultAccountId());
	let cashAccountId = $state(data.cashAccounts.length > 0 ? data.cashAccounts[0].id : '');
	let lines = $state<LineItem[]>([createEmptyLine()]);
	let showSuccess = $state(false);
	let submitting = $state(false);

	// ── Helpers ────────────────────
	function getDefaultAccountId(): string {
		const inventoryAccount = data.accounts.find((a) => a.line_type === 'INVENTORY');
		if (inventoryAccount) return inventoryAccount.id;
		return data.accounts.length > 0 ? data.accounts[0].id : '';
	}

	function createEmptyLine(): LineItem {
		return {
			_key: lineKeyCounter++,
			description: '',
			item_id: null,
			quantity: '1',
			unit_price: '',
			showSuggestions: false,
			filteredItems: []
		};
	}

	function calcLineAmount(line: LineItem): number {
		const qty = parseFloat(line.quantity) || 0;
		const price = parseFloat(line.unit_price) || 0;
		return qty * price;
	}

	let totalAmount = $derived(
		lines.reduce((sum, line) => sum + calcLineAmount(line), 0)
	);

	// ── Item autocomplete ────────────────────
	function filterItems(query: string): AcctItem[] {
		if (!query.trim() || data.items.length === 0) return [];
		const q = query.toLowerCase();
		return data.items
			.filter((item) => {
				// Match against keywords (comma-separated) and item_name
				const keywords = item.keywords.toLowerCase().split(',').map((k) => k.trim());
				const nameMatch = item.item_name.toLowerCase().includes(q);
				const keywordMatch = keywords.some((kw) => kw.includes(q));
				return nameMatch || keywordMatch;
			})
			.slice(0, 5);
	}

	function onDescriptionInput(index: number) {
		const line = lines[index];
		// If user is typing, clear any previously selected item_id
		line.item_id = null;
		line.filteredItems = filterItems(line.description);
		line.showSuggestions = line.filteredItems.length > 0;
	}

	function selectItem(index: number, item: AcctItem) {
		const line = lines[index];
		line.description = item.item_name;
		line.item_id = item.id;
		if (item.last_price) {
			line.unit_price = item.last_price;
		}
		line.showSuggestions = false;
		line.filteredItems = [];
	}

	function hideSuggestions(index: number) {
		// Delay to allow click on suggestion to register
		setTimeout(() => {
			if (lines[index]) {
				lines[index].showSuggestions = false;
			}
		}, 200);
	}

	// ── Line management ────────────────────
	function addLine() {
		lines.push(createEmptyLine());
	}

	function removeLine(index: number) {
		if (lines.length <= 1) return;
		lines.splice(index, 1);
	}

	// ── Form serialization ────────────────────
	function buildPurchaseData(): string {
		return JSON.stringify({
			transaction_date: transactionDate,
			account_id: accountId,
			cash_account_id: cashAccountId,
			outlet_id: null,
			items: lines.map((line) => ({
				item_id: line.item_id || null,
				description: line.description,
				quantity: line.quantity,
				unit_price: line.unit_price
			}))
		});
	}

	// ── Form reset ────────────────────
	function resetForm() {
		transactionDate = new Date().toISOString().slice(0, 10);
		accountId = getDefaultAccountId();
		cashAccountId = data.cashAccounts.length > 0 ? data.cashAccounts[0].id : '';
		lines = [createEmptyLine()];
	}
</script>

<svelte:head>
	<title>Pembelian - Kiwari POS</title>
</svelte:head>

<div class="purchase-page">
	<div class="page-header">
		<h1 class="page-title">Pembelian</h1>
		<p class="page-subtitle">Catat pembelian harian</p>
	</div>

	<!-- Success banner -->
	{#if showSuccess}
		<div class="success-banner">
			Pembelian berhasil disimpan.
			<button type="button" class="dismiss-btn" onclick={() => { showSuccess = false; }}>Tutup</button>
		</div>
	{/if}

	<!-- Error banner -->
	{#if form?.error}
		<div class="error-banner">{form.error}</div>
	{/if}

	<form
		method="POST"
		action="?/create"
		use:enhance={({ formData }) => {
			formData.set('purchase_data', buildPurchaseData());
			submitting = true;
			showSuccess = false;
			return async ({ result, update }) => {
				submitting = false;
				if (result.type === 'success') {
					showSuccess = true;
					resetForm();
				}
				await update();
			};
		}}
	>

		<!-- Header fields -->
		<div class="form-card">
			<div class="form-grid">
				<div class="form-group">
					<label for="transaction_date" class="form-label">Tanggal *</label>
					<input
						id="transaction_date"
						type="date"
						class="input-field"
						bind:value={transactionDate}
						required
					/>
				</div>
				<div class="form-group">
					<label for="account_id" class="form-label">Akun Pembukuan *</label>
					<select id="account_id" class="input-field" bind:value={accountId} required>
						<option value="">Pilih akun...</option>
						{#each data.accounts as acct}
							<option value={acct.id}>{acct.account_code} - {acct.account_name}</option>
						{/each}
					</select>
				</div>
				<div class="form-group">
					<label for="cash_account_id" class="form-label">Kas/Bank *</label>
					<select id="cash_account_id" class="input-field" bind:value={cashAccountId} required>
						<option value="">Pilih kas...</option>
						{#each data.cashAccounts as ca}
							<option value={ca.id}>{ca.cash_account_code} - {ca.cash_account_name}</option>
						{/each}
					</select>
				</div>
			</div>
		</div>

		<!-- Item lines -->
		<div class="lines-card">
			<div class="lines-header">
				<h2 class="lines-title">Item Pembelian</h2>
			</div>

			<!-- Column headers (desktop) -->
			<div class="lines-col-header">
				<span class="col-desc">Deskripsi</span>
				<span class="col-qty">Qty</span>
				<span class="col-price">Harga Satuan</span>
				<span class="col-amount">Jumlah</span>
				<span class="col-action"></span>
			</div>

			{#each lines as line, index (line._key)}
				<div class="line-row">
					<!-- Description with autocomplete -->
					<div class="line-field line-desc">
						<label for="desc_{index}" class="line-label">Deskripsi</label>
						<div class="autocomplete-wrapper">
							<input
								id="desc_{index}"
								type="text"
								class="input-field"
								placeholder="Ketik nama item..."
								bind:value={line.description}
								oninput={() => onDescriptionInput(index)}
								onfocusout={() => hideSuggestions(index)}
								autocomplete="off"
								required
							/>
							{#if line.showSuggestions && line.filteredItems.length > 0}
								<div class="suggestions-dropdown">
									{#each line.filteredItems as item}
										<button
											type="button"
											class="suggestion-item"
											onmousedown={() => selectItem(index, item)}
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
					</div>

					<!-- Quantity -->
					<div class="line-field line-qty">
						<label for="qty_{index}" class="line-label">Qty</label>
						<input
							id="qty_{index}"
							type="number"
							class="input-field input-right"
							step="any"
							min="0"
							bind:value={line.quantity}
						/>
					</div>

					<!-- Unit price -->
					<div class="line-field line-price">
						<label for="price_{index}" class="line-label">Harga Satuan</label>
						<input
							id="price_{index}"
							type="text"
							class="input-field input-right"
							inputmode="decimal"
							placeholder="0"
							bind:value={line.unit_price}
							required
						/>
					</div>

					<!-- Amount (calculated) -->
					<div class="line-field line-amount">
						<span class="line-label">Jumlah</span>
						<span class="amount-display">{formatRupiah(calcLineAmount(line))}</span>
					</div>

					<!-- Remove button -->
					<div class="line-field line-action">
						{#if lines.length > 1}
							<button
								type="button"
								class="btn-remove"
								onclick={() => removeLine(index)}
								title="Hapus baris"
							>&times;</button>
						{/if}
					</div>
				</div>
			{/each}

			<div class="lines-footer">
				<button type="button" class="btn-secondary btn-add-line" onclick={addLine}>
					+ Tambah Baris
				</button>
			</div>
		</div>

		<!-- Total + Submit -->
		<div class="submit-card">
			<div class="total-row">
				<span class="total-label">Total</span>
				<span class="total-amount">{formatRupiah(totalAmount)}</span>
			</div>
			<button type="submit" class="btn-primary btn-submit" disabled={submitting}>
				{submitting ? 'Menyimpan...' : 'Simpan'}
			</button>
		</div>
	</form>
</div>

<style>
	.purchase-page {
		max-width: 900px;
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
	}

	/* ── Form card ────────── */

	.form-card {
		background-color: var(--color-bg);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-card);
		padding: 16px;
	}

	.form-grid {
		display: grid;
		grid-template-columns: 1fr 1fr 1fr;
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

	/* ── Lines card ────────── */

	.lines-card {
		background-color: var(--color-bg);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-card);
		overflow: hidden;
	}

	.lines-header {
		padding: 16px 16px 12px;
	}

	.lines-title {
		font-size: var(--text-title);
		font-weight: 600;
		color: var(--color-text-primary);
		margin: 0;
	}

	.lines-col-header {
		display: grid;
		grid-template-columns: 3fr 1fr 1.5fr 1.5fr 40px;
		gap: 12px;
		padding: 8px 16px;
		background-color: var(--color-surface);
		border-top: 1px solid var(--color-border);
		border-bottom: 1px solid var(--color-border);
		font-size: 12px;
		font-weight: 600;
		color: var(--color-text-secondary);
		text-transform: uppercase;
		letter-spacing: 0.02em;
	}

	.col-qty,
	.col-price,
	.col-amount {
		text-align: right;
	}

	/* ── Line row ────────── */

	.line-row {
		display: grid;
		grid-template-columns: 3fr 1fr 1.5fr 1.5fr 40px;
		gap: 12px;
		padding: 12px 16px;
		border-bottom: 1px solid var(--color-border);
		align-items: start;
	}

	.line-row:last-of-type {
		border-bottom: none;
	}

	.line-field {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.line-label {
		display: none;
		font-size: 12px;
		font-weight: 500;
		color: var(--color-text-secondary);
	}

	.input-right {
		text-align: right;
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

	/* ── Amount display ────────── */

	.line-amount {
		display: flex;
		align-items: center;
		justify-content: flex-end;
		min-height: 42px;
	}

	.amount-display {
		font-size: 13px;
		font-weight: 600;
		color: var(--color-text-primary);
		text-align: right;
	}

	/* ── Line action ────────── */

	.line-action {
		display: flex;
		align-items: center;
		justify-content: center;
		min-height: 42px;
	}

	.btn-remove {
		width: 28px;
		height: 28px;
		display: flex;
		align-items: center;
		justify-content: center;
		background: none;
		border: 1px solid var(--color-border);
		border-radius: 4px;
		font-size: 18px;
		line-height: 1;
		color: var(--color-text-secondary);
		cursor: pointer;
	}

	.btn-remove:hover {
		background-color: var(--color-error-bg);
		border-color: var(--color-error);
		color: var(--color-error);
	}

	/* ── Lines footer ────────── */

	.lines-footer {
		padding: 12px 16px;
		border-top: 1px solid var(--color-border);
	}

	.btn-add-line {
		padding: 8px 16px;
		font-size: 13px;
		border: none;
		cursor: pointer;
	}

	/* ── Submit card ────────── */

	.submit-card {
		background-color: var(--color-bg);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-card);
		padding: 16px;
		display: flex;
		align-items: center;
		justify-content: space-between;
	}

	.total-row {
		display: flex;
		align-items: center;
		gap: 12px;
	}

	.total-label {
		font-size: 14px;
		font-weight: 600;
		color: var(--color-text-secondary);
	}

	.total-amount {
		font-size: 18px;
		font-weight: 700;
		color: var(--color-text-primary);
	}

	.btn-submit {
		padding: 10px 24px;
		font-size: 14px;
		border: none;
		cursor: pointer;
	}

	/* ── Mobile responsive ────────── */

	@media (max-width: 768px) {
		.form-grid {
			grid-template-columns: 1fr;
		}

		.lines-col-header {
			display: none;
		}

		.line-label {
			display: block;
		}

		.line-row {
			grid-template-columns: 1fr 1fr;
			gap: 10px;
		}

		.line-desc {
			grid-column: 1 / -1;
		}

		.line-amount {
			justify-content: flex-start;
			align-items: flex-start;
			min-height: auto;
		}

		.line-action {
			justify-content: flex-end;
			align-items: flex-start;
			min-height: auto;
		}

		.submit-card {
			flex-direction: column;
			gap: 16px;
			align-items: stretch;
		}

		.total-row {
			justify-content: space-between;
		}

		.btn-submit {
			text-align: center;
		}
	}
</style>
