<!--
  Accounting Master Data — 3-tab CRUD page for Akun, Item, Kas.
  Each tab shows a data table with add/edit/delete functionality.
-->
<script lang="ts">
	import { enhance } from '$app/forms';
	import { formatRupiah } from '$lib/utils/format';

	let { data, form } = $props();

	// ── State ────────────────────
	type Tab = 'akun' | 'item' | 'kas';
	let activeTab = $state<Tab>('akun');
	let showAddForm = $state(false);
	let editingId = $state<string | null>(null);

	// ── Options ────────────────────
	const accountTypeOptions = ['Asset', 'Liability', 'Equity', 'Revenue', 'Expense'];
	const lineTypeOptions = ['ASSET', 'INVENTORY', 'EXPENSE', 'SALES', 'COGS', 'LIABILITY', 'CAPITAL', 'DRAWING'];
	const itemCategoryOptions = [
		{ value: 'Raw Material', label: 'Raw Material' },
		{ value: 'Packaging', label: 'Packaging' },
		{ value: 'Consumable', label: 'Consumable' }
	];
	const ownershipOptions = [
		{ value: 'Business', label: 'Bisnis' },
		{ value: 'Personal', label: 'Personal' }
	];

	// ── Helpers ────────────────────
	function switchTab(tab: Tab) {
		activeTab = tab;
		showAddForm = false;
		editingId = null;
	}
</script>

<svelte:head>
	<title>Master Data Akuntansi - Kiwari POS</title>
</svelte:head>

<div class="master-page">
	<div class="page-header">
		<h1 class="page-title">Master Data</h1>
		<p class="page-subtitle">Kelola akun, item, dan kas untuk pembukuan</p>
	</div>

	<!-- Tab switcher -->
	<div class="tab-bar">
		<button
			type="button"
			class="tab-chip"
			class:active={activeTab === 'akun'}
			onclick={() => switchTab('akun')}
		>Akun</button>
		<button
			type="button"
			class="tab-chip"
			class:active={activeTab === 'item'}
			onclick={() => switchTab('item')}
		>Item</button>
		<button
			type="button"
			class="tab-chip"
			class:active={activeTab === 'kas'}
			onclick={() => switchTab('kas')}
		>Kas</button>
	</div>

	<!-- ══════════════════════════════════════════ -->
	<!-- Tab: Akun (Accounts)                      -->
	<!-- ══════════════════════════════════════════ -->
	{#if activeTab === 'akun'}
		<section class="tab-section">
			<div class="section-header">
				<div class="header-left">
					<h2 class="section-title">Daftar Akun</h2>
					<p class="section-subtitle">{data.accounts.length} akun</p>
				</div>
				<div class="header-actions">
					<button
						type="button"
						class="btn-primary btn-add"
						onclick={() => { showAddForm = !showAddForm; editingId = null; }}
					>
						{showAddForm ? 'Tutup' : '+ Tambah'}
					</button>
				</div>
			</div>

			<!-- Error banners -->
			{#if form?.createAccountError}
				<div class="error-banner">{form.createAccountError}</div>
			{/if}
			{#if form?.updateAccountError}
				<div class="error-banner">{form.updateAccountError}</div>
			{/if}
			{#if form?.deleteAccountError}
				<div class="error-banner">{form.deleteAccountError}</div>
			{/if}

			<!-- Add form -->
			{#if showAddForm}
				<div class="add-form-card">
					<h3 class="form-title">Tambah Akun</h3>
					<form method="POST" action="?/createAccount" use:enhance={() => {
						return async ({ result, update }) => {
							if (result.type === 'success') {
								showAddForm = false;
							}
							await update();
						};
					}}>
						<div class="form-grid">
							<div class="form-group">
								<label for="add-account_code" class="form-label">Kode Akun *</label>
								<input id="add-account_code" name="account_code" type="text" class="input-field" placeholder="Contoh: 1-1001" required />
							</div>
							<div class="form-group">
								<label for="add-account_name" class="form-label">Nama Akun *</label>
								<input id="add-account_name" name="account_name" type="text" class="input-field" placeholder="Contoh: Kas Besar" required />
							</div>
							<div class="form-group">
								<label for="add-account_type" class="form-label">Tipe Akun *</label>
								<select id="add-account_type" name="account_type" class="input-field" required>
									<option value="">Pilih tipe...</option>
									{#each accountTypeOptions as opt}
										<option value={opt}>{opt}</option>
									{/each}
								</select>
							</div>
							<div class="form-group">
								<label for="add-line_type" class="form-label">Tipe Baris *</label>
								<select id="add-line_type" name="line_type" class="input-field" required>
									<option value="">Pilih tipe baris...</option>
									{#each lineTypeOptions as opt}
										<option value={opt}>{opt}</option>
									{/each}
								</select>
							</div>
						</div>
						<div class="form-actions">
							<button type="submit" class="btn-primary btn-sm">Simpan</button>
							<button type="button" class="btn-secondary btn-sm" onclick={() => { showAddForm = false; }}>Batal</button>
						</div>
					</form>
				</div>
			{/if}

			<!-- Account table -->
			{#if data.accounts.length === 0}
				<div class="empty-state">
					<p class="empty-text">Belum ada data akun.</p>
				</div>
			{:else}
				<div class="data-table">
					<div class="table-header account-grid">
						<span>Kode Akun</span>
						<span>Nama Akun</span>
						<span>Tipe Akun</span>
						<span>Tipe Baris</span>
						<span>Aksi</span>
					</div>
					{#each data.accounts as acct (acct.id)}
						{#if editingId === acct.id}
							<div class="table-row edit-row">
								<form method="POST" action="?/updateAccount" use:enhance={() => {
									return async ({ result, update }) => {
										if (result.type === 'success') {
											editingId = null;
										}
										await update();
									};
								}}>
									<input type="hidden" name="id" value={acct.id} />
									<div class="edit-grid">
										<div class="edit-field">
											<label for="edit-account_name" class="edit-label">Nama Akun *</label>
											<input id="edit-account_name" name="account_name" type="text" class="input-field" value={acct.account_name} required />
										</div>
										<div class="edit-field">
											<label for="edit-account_type" class="edit-label">Tipe Akun *</label>
											<select id="edit-account_type" name="account_type" class="input-field" required>
												{#each accountTypeOptions as opt}
													<option value={opt} selected={opt === acct.account_type}>{opt}</option>
												{/each}
											</select>
										</div>
										<div class="edit-field">
											<label for="edit-line_type" class="edit-label">Tipe Baris *</label>
											<select id="edit-line_type" name="line_type" class="input-field" required>
												{#each lineTypeOptions as opt}
													<option value={opt} selected={opt === acct.line_type}>{opt}</option>
												{/each}
											</select>
										</div>
									</div>
									<div class="edit-actions">
										<button type="submit" class="btn-primary btn-sm">Simpan</button>
										<button type="button" class="btn-secondary btn-sm" onclick={() => { editingId = null; }}>Batal</button>
									</div>
								</form>
							</div>
						{:else}
							<div class="table-row account-grid">
								<span class="cell-code">{acct.account_code}</span>
								<span class="cell-name">{acct.account_name}</span>
								<span class="cell-text">{acct.account_type}</span>
								<span class="cell-badge"><span class="badge">{acct.line_type}</span></span>
								<span class="col-actions">
									<button type="button" class="btn-icon" onclick={() => { editingId = acct.id; showAddForm = false; }}>Ubah</button>
									<form method="POST" action="?/deleteAccount" use:enhance>
										<input type="hidden" name="id" value={acct.id} />
										<button
											type="submit"
											class="btn-icon btn-danger"
											onclick={(e) => { if (!confirm('Hapus akun "' + acct.account_name + '"?')) e.preventDefault(); }}
										>Hapus</button>
									</form>
								</span>
							</div>
						{/if}
					{/each}
				</div>
			{/if}
		</section>
	{/if}

	<!-- ══════════════════════════════════════════ -->
	<!-- Tab: Item (Inventory Items)               -->
	<!-- ══════════════════════════════════════════ -->
	{#if activeTab === 'item'}
		<section class="tab-section">
			<div class="section-header">
				<div class="header-left">
					<h2 class="section-title">Daftar Item</h2>
					<p class="section-subtitle">{data.items.length} item</p>
				</div>
				<div class="header-actions">
					<button
						type="button"
						class="btn-primary btn-add"
						onclick={() => { showAddForm = !showAddForm; editingId = null; }}
					>
						{showAddForm ? 'Tutup' : '+ Tambah'}
					</button>
				</div>
			</div>

			<!-- Error banners -->
			{#if form?.createItemError}
				<div class="error-banner">{form.createItemError}</div>
			{/if}
			{#if form?.updateItemError}
				<div class="error-banner">{form.updateItemError}</div>
			{/if}
			{#if form?.deleteItemError}
				<div class="error-banner">{form.deleteItemError}</div>
			{/if}

			<!-- Add form -->
			{#if showAddForm}
				<div class="add-form-card">
					<h3 class="form-title">Tambah Item</h3>
					<form method="POST" action="?/createItem" use:enhance={() => {
						return async ({ result, update }) => {
							if (result.type === 'success') {
								showAddForm = false;
							}
							await update();
						};
					}}>
						<div class="form-grid">
							<div class="form-group">
								<label for="add-item_code" class="form-label">Kode Item *</label>
								<input id="add-item_code" name="item_code" type="text" class="input-field" placeholder="Contoh: RM-001" required />
							</div>
							<div class="form-group">
								<label for="add-item_name" class="form-label">Nama Item *</label>
								<input id="add-item_name" name="item_name" type="text" class="input-field" placeholder="Contoh: Beras 5kg" required />
							</div>
							<div class="form-group">
								<label for="add-item_category" class="form-label">Kategori *</label>
								<select id="add-item_category" name="item_category" class="input-field" required>
									<option value="">Pilih kategori...</option>
									{#each itemCategoryOptions as opt}
										<option value={opt.value}>{opt.label}</option>
									{/each}
								</select>
							</div>
							<div class="form-group">
								<label for="add-item_unit" class="form-label">Satuan *</label>
								<input id="add-item_unit" name="unit" type="text" class="input-field" placeholder="Contoh: kg, pcs, ltr" required />
							</div>
							<div class="form-group">
								<label for="add-item_keywords" class="form-label">Keywords *</label>
								<input id="add-item_keywords" name="keywords" type="text" class="input-field" placeholder="Kata kunci pencarian" required />
							</div>
							<div class="form-group form-group-checkbox">
								<label class="checkbox-label">
									<input name="is_inventory" type="checkbox" checked />
									<span>Inventori</span>
								</label>
							</div>
							<div class="form-group">
								<label for="add-item_avg_price" class="form-label">Harga Rata-rata</label>
								<input id="add-item_avg_price" name="average_price" type="text" class="input-field" placeholder="0" inputmode="decimal" />
							</div>
							<div class="form-group">
								<label for="add-item_last_price" class="form-label">Harga Terakhir</label>
								<input id="add-item_last_price" name="last_price" type="text" class="input-field" placeholder="0" inputmode="decimal" />
							</div>
							<div class="form-group">
								<label for="add-item_for_hpp" class="form-label">Untuk HPP</label>
								<input id="add-item_for_hpp" name="for_hpp" type="text" class="input-field" placeholder="0" inputmode="decimal" />
							</div>
						</div>
						<div class="form-actions">
							<button type="submit" class="btn-primary btn-sm">Simpan</button>
							<button type="button" class="btn-secondary btn-sm" onclick={() => { showAddForm = false; }}>Batal</button>
						</div>
					</form>
				</div>
			{/if}

			<!-- Item table -->
			{#if data.items.length === 0}
				<div class="empty-state">
					<p class="empty-text">Belum ada data item.</p>
				</div>
			{:else}
				<div class="data-table">
					<div class="table-header item-grid">
						<span>Kode</span>
						<span>Nama</span>
						<span>Kategori</span>
						<span>Satuan</span>
						<span>Inventori</span>
						<span>Harga Terakhir</span>
						<span>Aksi</span>
					</div>
					{#each data.items as item (item.id)}
						{#if editingId === item.id}
							<div class="table-row edit-row">
								<form method="POST" action="?/updateItem" use:enhance={() => {
									return async ({ result, update }) => {
										if (result.type === 'success') {
											editingId = null;
										}
										await update();
									};
								}}>
									<input type="hidden" name="id" value={item.id} />
									<div class="edit-grid">
										<div class="edit-field">
											<label for="edit-item_name" class="edit-label">Nama Item *</label>
											<input id="edit-item_name" name="item_name" type="text" class="input-field" value={item.item_name} required />
										</div>
										<div class="edit-field">
											<label for="edit-item_category" class="edit-label">Kategori *</label>
											<select id="edit-item_category" name="item_category" class="input-field" required>
												{#each itemCategoryOptions as opt}
													<option value={opt.value} selected={opt.value === item.item_category}>{opt.label}</option>
												{/each}
											</select>
										</div>
										<div class="edit-field">
											<label for="edit-item_unit" class="edit-label">Satuan *</label>
											<input id="edit-item_unit" name="unit" type="text" class="input-field" value={item.unit} required />
										</div>
										<div class="edit-field">
											<label for="edit-item_keywords" class="edit-label">Keywords *</label>
											<input id="edit-item_keywords" name="keywords" type="text" class="input-field" value={item.keywords} required />
										</div>
										<div class="edit-field edit-field-checkbox">
											<label class="checkbox-label">
												<input name="is_inventory" type="checkbox" checked={item.is_inventory} />
												<span>Inventori</span>
											</label>
										</div>
										<div class="edit-field">
											<label for="edit-item_avg_price" class="edit-label">Harga Rata-rata</label>
											<input id="edit-item_avg_price" name="average_price" type="text" class="input-field" value={item.average_price ?? ''} inputmode="decimal" />
										</div>
										<div class="edit-field">
											<label for="edit-item_last_price" class="edit-label">Harga Terakhir</label>
											<input id="edit-item_last_price" name="last_price" type="text" class="input-field" value={item.last_price ?? ''} inputmode="decimal" />
										</div>
										<div class="edit-field">
											<label for="edit-item_for_hpp" class="edit-label">Untuk HPP</label>
											<input id="edit-item_for_hpp" name="for_hpp" type="text" class="input-field" value={item.for_hpp ?? ''} inputmode="decimal" />
										</div>
									</div>
									<div class="edit-actions">
										<button type="submit" class="btn-primary btn-sm">Simpan</button>
										<button type="button" class="btn-secondary btn-sm" onclick={() => { editingId = null; }}>Batal</button>
									</div>
								</form>
							</div>
						{:else}
							<div class="table-row item-grid">
								<span class="cell-code">{item.item_code}</span>
								<span class="cell-name">{item.item_name}</span>
								<span class="cell-text">{item.item_category}</span>
								<span class="cell-text">{item.unit}</span>
								<span class="cell-text">{item.is_inventory ? 'Ya' : 'Tidak'}</span>
								<span class="cell-price">{item.last_price ? formatRupiah(item.last_price) : '-'}</span>
								<span class="col-actions">
									<button type="button" class="btn-icon" onclick={() => { editingId = item.id; showAddForm = false; }}>Ubah</button>
									<form method="POST" action="?/deleteItem" use:enhance>
										<input type="hidden" name="id" value={item.id} />
										<button
											type="submit"
											class="btn-icon btn-danger"
											onclick={(e) => { if (!confirm('Hapus item "' + item.item_name + '"?')) e.preventDefault(); }}
										>Hapus</button>
									</form>
								</span>
							</div>
						{/if}
					{/each}
				</div>
			{/if}
		</section>
	{/if}

	<!-- ══════════════════════════════════════════ -->
	<!-- Tab: Kas (Cash Accounts)                  -->
	<!-- ══════════════════════════════════════════ -->
	{#if activeTab === 'kas'}
		<section class="tab-section">
			<div class="section-header">
				<div class="header-left">
					<h2 class="section-title">Daftar Kas</h2>
					<p class="section-subtitle">{data.cashAccounts.length} kas</p>
				</div>
				<div class="header-actions">
					<button
						type="button"
						class="btn-primary btn-add"
						onclick={() => { showAddForm = !showAddForm; editingId = null; }}
					>
						{showAddForm ? 'Tutup' : '+ Tambah'}
					</button>
				</div>
			</div>

			<!-- Error banners -->
			{#if form?.createCashAccountError}
				<div class="error-banner">{form.createCashAccountError}</div>
			{/if}
			{#if form?.updateCashAccountError}
				<div class="error-banner">{form.updateCashAccountError}</div>
			{/if}
			{#if form?.deleteCashAccountError}
				<div class="error-banner">{form.deleteCashAccountError}</div>
			{/if}

			<!-- Add form -->
			{#if showAddForm}
				<div class="add-form-card">
					<h3 class="form-title">Tambah Kas</h3>
					<form method="POST" action="?/createCashAccount" use:enhance={() => {
						return async ({ result, update }) => {
							if (result.type === 'success') {
								showAddForm = false;
							}
							await update();
						};
					}}>
						<div class="form-grid">
							<div class="form-group">
								<label for="add-cash_account_code" class="form-label">Kode Kas *</label>
								<input id="add-cash_account_code" name="cash_account_code" type="text" class="input-field" placeholder="Contoh: KAS-001" required />
							</div>
							<div class="form-group">
								<label for="add-cash_account_name" class="form-label">Nama Kas *</label>
								<input id="add-cash_account_name" name="cash_account_name" type="text" class="input-field" placeholder="Contoh: Kas Utama" required />
							</div>
							<div class="form-group">
								<label for="add-bank_name" class="form-label">Bank</label>
								<input id="add-bank_name" name="bank_name" type="text" class="input-field" placeholder="Contoh: BCA, BRI (opsional)" />
							</div>
							<div class="form-group">
								<label for="add-ownership" class="form-label">Kepemilikan *</label>
								<select id="add-ownership" name="ownership" class="input-field" required>
									<option value="">Pilih kepemilikan...</option>
									{#each ownershipOptions as opt}
										<option value={opt.value}>{opt.label}</option>
									{/each}
								</select>
							</div>
						</div>
						<div class="form-actions">
							<button type="submit" class="btn-primary btn-sm">Simpan</button>
							<button type="button" class="btn-secondary btn-sm" onclick={() => { showAddForm = false; }}>Batal</button>
						</div>
					</form>
				</div>
			{/if}

			<!-- Cash account table -->
			{#if data.cashAccounts.length === 0}
				<div class="empty-state">
					<p class="empty-text">Belum ada data kas.</p>
				</div>
			{:else}
				<div class="data-table">
					<div class="table-header cash-grid">
						<span>Kode</span>
						<span>Nama</span>
						<span>Bank</span>
						<span>Kepemilikan</span>
						<span>Aksi</span>
					</div>
					{#each data.cashAccounts as ca (ca.id)}
						{#if editingId === ca.id}
							<div class="table-row edit-row">
								<form method="POST" action="?/updateCashAccount" use:enhance={() => {
									return async ({ result, update }) => {
										if (result.type === 'success') {
											editingId = null;
										}
										await update();
									};
								}}>
									<input type="hidden" name="id" value={ca.id} />
									<div class="edit-grid">
										<div class="edit-field">
											<label for="edit-cash_account_name" class="edit-label">Nama Kas *</label>
											<input id="edit-cash_account_name" name="cash_account_name" type="text" class="input-field" value={ca.cash_account_name} required />
										</div>
										<div class="edit-field">
											<label for="edit-bank_name" class="edit-label">Bank</label>
											<input id="edit-bank_name" name="bank_name" type="text" class="input-field" value={ca.bank_name ?? ''} />
										</div>
										<div class="edit-field">
											<label for="edit-ownership" class="edit-label">Kepemilikan *</label>
											<select id="edit-ownership" name="ownership" class="input-field" required>
												{#each ownershipOptions as opt}
													<option value={opt.value} selected={opt.value === ca.ownership}>{opt.label}</option>
												{/each}
											</select>
										</div>
									</div>
									<div class="edit-actions">
										<button type="submit" class="btn-primary btn-sm">Simpan</button>
										<button type="button" class="btn-secondary btn-sm" onclick={() => { editingId = null; }}>Batal</button>
									</div>
								</form>
							</div>
						{:else}
							<div class="table-row cash-grid">
								<span class="cell-code">{ca.cash_account_code}</span>
								<span class="cell-name">{ca.cash_account_name}</span>
								<span class="cell-text">{ca.bank_name ?? '-'}</span>
								<span class="cell-text">{ca.ownership === 'Business' ? 'Bisnis' : 'Personal'}</span>
								<span class="col-actions">
									<button type="button" class="btn-icon" onclick={() => { editingId = ca.id; showAddForm = false; }}>Ubah</button>
									<form method="POST" action="?/deleteCashAccount" use:enhance>
										<input type="hidden" name="id" value={ca.id} />
										<button
											type="submit"
											class="btn-icon btn-danger"
											onclick={(e) => { if (!confirm('Hapus kas "' + ca.cash_account_name + '"?')) e.preventDefault(); }}
										>Hapus</button>
									</form>
								</span>
							</div>
						{/if}
					{/each}
				</div>
			{/if}
		</section>
	{/if}
</div>

<style>
	.master-page {
		max-width: 1200px;
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

	/* ── Tab bar ────────── */

	.tab-bar {
		display: flex;
		gap: 8px;
	}

	.tab-chip {
		padding: 8px 20px;
		font-size: 13px;
		font-weight: 600;
		border: 1px solid var(--color-border);
		border-radius: var(--radius-chip);
		background-color: var(--color-bg);
		color: var(--color-text-secondary);
		cursor: pointer;
		transition: all 0.15s ease;
	}

	.tab-chip:hover {
		background-color: var(--color-surface);
		color: var(--color-text-primary);
	}

	.tab-chip.active {
		background-color: var(--color-primary);
		color: white;
		border-color: var(--color-primary);
	}

	/* ── Section ────────── */

	.tab-section {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}

	.section-header {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
	}

	.header-left {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.section-title {
		font-size: 20px;
		font-weight: 700;
		color: var(--color-text-primary);
		margin: 0;
	}

	.section-subtitle {
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

	/* ── Error banner ────────── */

	.error-banner {
		background-color: var(--color-error-bg);
		color: var(--color-error);
		font-size: 13px;
		font-weight: 500;
		padding: 8px 12px;
		border-radius: var(--radius-chip);
	}

	/* ── Add form ────────── */

	.add-form-card {
		background-color: var(--color-bg);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-card);
		padding: 16px;
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

	.form-group-checkbox {
		display: flex;
		align-items: flex-end;
	}

	.checkbox-label {
		display: flex;
		align-items: center;
		gap: 8px;
		font-size: 13px;
		font-weight: 500;
		color: var(--color-text-primary);
		cursor: pointer;
		padding: 10px 0;
	}

	.checkbox-label input[type='checkbox'] {
		width: 16px;
		height: 16px;
		accent-color: var(--color-primary);
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

	/* ── Data table ────────── */

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
		padding: 12px 16px;
		border-bottom: 1px solid var(--color-border);
		align-items: center;
	}

	.table-row:last-child {
		border-bottom: none;
	}

	.table-row.edit-row {
		display: block;
		padding: 16px;
	}

	/* Grid layouts per tab */

	.account-grid {
		grid-template-columns: 1fr 2fr 1fr 1fr 120px;
	}

	.item-grid {
		grid-template-columns: 1fr 2fr 1fr 0.7fr 0.7fr 1fr 120px;
	}

	.cash-grid {
		grid-template-columns: 1fr 2fr 1.5fr 1fr 120px;
	}

	/* ── Cell styles ────────── */

	.cell-code {
		font-size: 13px;
		font-weight: 600;
		color: var(--color-text-primary);
		font-family: monospace;
	}

	.cell-name {
		font-size: 13px;
		font-weight: 600;
		color: var(--color-text-primary);
	}

	.cell-text {
		font-size: 13px;
		color: var(--color-text-secondary);
	}

	.cell-price {
		font-size: 13px;
		font-weight: 600;
		color: var(--color-text-primary);
	}

	.cell-badge {
		font-size: 13px;
	}

	.badge {
		font-size: 11px;
		font-weight: 600;
		padding: 2px 8px;
		border-radius: var(--radius-chip);
		background-color: var(--color-surface);
		color: var(--color-text-secondary);
		border: 1px solid var(--color-border);
		text-transform: uppercase;
		letter-spacing: 0.02em;
	}

	/* ── Inline edit ────────── */

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

	.edit-field-checkbox {
		display: flex;
		align-items: flex-end;
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

	/* ── Action buttons ────────── */

	.col-actions {
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

	/* ── Mobile responsive ────────── */

	@media (max-width: 768px) {
		.table-header {
			display: none;
		}

		.table-row:not(.edit-row) {
			grid-template-columns: 1fr 1fr;
			grid-template-rows: auto;
			gap: 6px;
		}

		.col-actions {
			grid-column: 1 / -1;
		}

		.form-grid,
		.edit-grid {
			grid-template-columns: 1fr;
		}

		.section-header {
			flex-direction: column;
			gap: 12px;
		}

		.tab-bar {
			flex-wrap: wrap;
		}
	}
</style>
