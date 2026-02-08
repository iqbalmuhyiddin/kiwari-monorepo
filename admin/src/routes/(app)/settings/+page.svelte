<!--
  Settings page — user management and app info.
  Only accessible by OWNER and MANAGER roles.
-->
<script lang="ts">
	import { enhance } from '$app/forms';

	let { data, form } = $props();

	let showAddForm = $state(false);
	let editingId = $state<string | null>(null);

	const roleOptions = [
		{ value: 'OWNER', label: 'Pemilik' },
		{ value: 'MANAGER', label: 'Manajer' },
		{ value: 'CASHIER', label: 'Kasir' },
		{ value: 'KITCHEN', label: 'Dapur' }
	];

	function getRoleLabel(role: string): string {
		return roleOptions.find((r) => r.value === role)?.label ?? role;
	}

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
</script>

<svelte:head>
	<title>Pengaturan - Kiwari POS</title>
</svelte:head>

<div class="settings-page">
	<!-- App Info Section -->
	<section class="info-section">
		<h2 class="section-title">Informasi Aplikasi</h2>
		<div class="info-card">
			<div class="info-row">
				<span class="info-label">Outlet</span>
				<span class="info-value">Kiwari POS</span>
			</div>
			<div class="info-row">
				<span class="info-label">Versi</span>
				<span class="info-value">1.0.0</span>
			</div>
			<div class="info-row">
				<span class="info-label">Pengguna aktif</span>
				<span class="info-value">{data.currentUser.full_name} ({getRoleLabel(data.currentUser.role)})</span>
			</div>
			<p class="info-note">Pengaturan lainnya akan segera tersedia.</p>
		</div>
	</section>

	<!-- User Management Section -->
	<section class="users-section">
		<div class="section-header">
			<div class="header-left">
				<h2 class="section-title">Manajemen Pengguna</h2>
				<p class="section-subtitle">{data.users.length} pengguna</p>
			</div>
			<div class="header-actions">
				<button
					type="button"
					class="btn-primary btn-add"
					onclick={() => { showAddForm = !showAddForm; editingId = null; }}
				>
					{showAddForm ? 'Tutup' : '+ Tambah Pengguna'}
				</button>
			</div>
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

		<!-- Add user form -->
		{#if showAddForm}
			<div class="add-form-card">
				<h3 class="form-title">Tambah Pengguna</h3>
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
							<label for="add-full_name" class="form-label">Nama Lengkap *</label>
							<input id="add-full_name" name="full_name" type="text" class="input-field" placeholder="Nama lengkap" required />
						</div>
						<div class="form-group">
							<label for="add-email" class="form-label">Email *</label>
							<input id="add-email" name="email" type="email" class="input-field" placeholder="email@contoh.com" required />
						</div>
						<div class="form-group">
							<label for="add-password" class="form-label">Kata Sandi *</label>
							<input id="add-password" name="password" type="password" class="input-field" placeholder="Kata sandi" required />
						</div>
						<div class="form-group">
							<label for="add-role" class="form-label">Peran *</label>
							<select id="add-role" name="role" class="input-field" required>
								<option value="">Pilih peran...</option>
								{#each roleOptions as opt}
									<option value={opt.value}>{opt.label}</option>
								{/each}
							</select>
						</div>
						<div class="form-group">
							<label for="add-pin" class="form-label">PIN</label>
							<input id="add-pin" name="pin" type="text" class="input-field" placeholder="4-6 digit (opsional)" maxlength="6" inputmode="numeric" />
						</div>
					</div>
					<div class="form-actions">
						<button type="submit" class="btn-primary btn-sm">Simpan</button>
						<button type="button" class="btn-secondary btn-sm" onclick={() => { showAddForm = false; }}>Batal</button>
					</div>
				</form>
			</div>
		{/if}

		<!-- User list -->
		{#if data.users.length === 0}
			<div class="empty-state">
				<p class="empty-text">Belum ada data pengguna.</p>
			</div>
		{:else}
			<div class="user-table">
				<div class="table-header">
					<span class="col-name">Nama</span>
					<span class="col-email">Email</span>
					<span class="col-role">Peran</span>
					<span class="col-pin">PIN</span>
					<span class="col-status">Status</span>
					<span class="col-actions">Aksi</span>
				</div>
				{#each data.users as u (u.id)}
					{#if editingId === u.id}
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
								<input type="hidden" name="id" value={u.id} />
								<div class="edit-grid">
									<div class="edit-field">
										<label for="edit-full_name" class="edit-label">Nama Lengkap *</label>
										<input id="edit-full_name" name="full_name" type="text" class="input-field" value={u.full_name} required />
									</div>
									<div class="edit-field">
										<label for="edit-email" class="edit-label">Email *</label>
										<input id="edit-email" name="email" type="email" class="input-field" value={u.email} required />
									</div>
									<div class="edit-field">
										<label for="edit-role" class="edit-label">Peran *</label>
										<select id="edit-role" name="role" class="input-field" required>
											{#each roleOptions as opt}
												<option value={opt.value} selected={opt.value === u.role}>{opt.label}</option>
											{/each}
										</select>
									</div>
									<div class="edit-field">
										<label for="edit-pin" class="edit-label">PIN</label>
										<input id="edit-pin" name="pin" type="text" class="input-field" value={u.pin ?? ''} maxlength="6" inputmode="numeric" placeholder="4-6 digit (opsional)" />
									</div>
								</div>
								<p class="edit-hint">Kata sandi tidak dapat diubah melalui halaman ini.</p>
								<div class="edit-actions">
									<button type="submit" class="btn-primary btn-sm">Simpan</button>
									<button type="button" class="btn-secondary btn-sm" onclick={() => { editingId = null; }}>Batal</button>
								</div>
							</form>
						</div>
					{:else}
						<!-- Normal row -->
						<div class="table-row">
							<span class="col-name">
								<span class="user-name">{u.full_name}</span>
								{#if u.id === data.currentUser.id}
									<span class="badge-self">Anda</span>
								{/if}
							</span>
							<span class="col-email">{u.email}</span>
							<span class="col-role">
								<span class="role-badge role-{u.role.toLowerCase()}">{getRoleLabel(u.role)}</span>
							</span>
							<span class="col-pin">{u.pin ?? '-'}</span>
							<span class="col-status">
								<span class="status-dot" class:active={u.is_active}></span>
								{u.is_active ? 'Aktif' : 'Nonaktif'}
							</span>
							<span class="col-actions">
								<button type="button" class="btn-icon" onclick={() => { editingId = u.id; showAddForm = false; }}>Ubah</button>
								{#if u.id !== data.currentUser.id}
									<form method="POST" action="?/delete" use:enhance>
										<input type="hidden" name="id" value={u.id} />
										<button
											type="submit"
											class="btn-icon btn-danger"
											onclick={(e) => { if (!confirm('Hapus pengguna "' + u.full_name + '"? Pengguna akan dinonaktifkan.')) e.preventDefault(); }}
										>
											Hapus
										</button>
									</form>
								{/if}
							</span>
						</div>
					{/if}
				{/each}
			</div>
		{/if}
	</section>
</div>

<style>
	.settings-page {
		max-width: 1200px;
		display: flex;
		flex-direction: column;
		gap: 32px;
	}

	/* ── Info Section ────────── */

	.info-section {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}

	.section-title {
		font-size: 20px;
		font-weight: 700;
		color: var(--color-text-primary);
		margin: 0;
	}

	.info-card {
		background-color: var(--color-bg);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-card);
		padding: 16px;
		display: flex;
		flex-direction: column;
		gap: 10px;
	}

	.info-row {
		display: flex;
		align-items: center;
		gap: 16px;
	}

	.info-label {
		font-size: 13px;
		font-weight: 500;
		color: var(--color-text-secondary);
		min-width: 120px;
	}

	.info-value {
		font-size: 13px;
		font-weight: 600;
		color: var(--color-text-primary);
	}

	.info-note {
		font-size: 12px;
		color: var(--color-text-secondary);
		margin: 4px 0 0;
		font-style: italic;
	}

	/* ── Users Section ────────── */

	.users-section {
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

	/* ── User table ────────── */

	.user-table {
		border: 1px solid var(--color-border);
		border-radius: var(--radius-card);
		overflow: hidden;
	}

	.table-header {
		display: grid;
		grid-template-columns: 2fr 2fr 1fr 0.8fr 0.8fr 120px;
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
		grid-template-columns: 2fr 2fr 1fr 0.8fr 0.8fr 120px;
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

	.col-name {
		display: flex;
		align-items: center;
		gap: 6px;
	}

	.user-name {
		font-size: 13px;
		font-weight: 600;
		color: var(--color-text-primary);
	}

	.badge-self {
		font-size: 10px;
		font-weight: 600;
		background-color: var(--color-surface);
		color: var(--color-text-secondary);
		padding: 2px 6px;
		border-radius: 4px;
		border: 1px solid var(--color-border);
	}

	.col-email {
		font-size: 13px;
		color: var(--color-text-secondary);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.col-role {
		font-size: 13px;
	}

	.role-badge {
		font-size: 11px;
		font-weight: 600;
		padding: 2px 8px;
		border-radius: var(--radius-chip);
		text-transform: uppercase;
		letter-spacing: 0.02em;
	}

	.role-owner {
		background-color: #fef3c7;
		color: #92400e;
	}

	.role-manager {
		background-color: #dbeafe;
		color: #1e40af;
	}

	.role-cashier {
		background-color: #d1fae5;
		color: #065f46;
	}

	.role-kitchen {
		background-color: #fce7f3;
		color: #9d174d;
	}

	.col-pin {
		font-size: 13px;
		color: var(--color-text-primary);
		font-family: monospace;
	}

	.col-status {
		font-size: 12px;
		color: var(--color-text-secondary);
		display: flex;
		align-items: center;
		gap: 6px;
	}

	.status-dot {
		width: 8px;
		height: 8px;
		border-radius: 50%;
		background-color: var(--color-text-secondary);
		flex-shrink: 0;
	}

	.status-dot.active {
		background-color: var(--color-primary);
	}

	.col-actions {
		display: flex;
		align-items: center;
		gap: 4px;
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

	.edit-label {
		font-size: 12px;
		font-weight: 500;
		color: var(--color-text-secondary);
	}

	.edit-hint {
		font-size: 11px;
		color: var(--color-text-secondary);
		font-style: italic;
		margin: 8px 0 0;
	}

	.edit-actions {
		display: flex;
		gap: 8px;
		margin-top: 10px;
	}

	/* ── Action buttons ────────── */

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

		.table-row {
			grid-template-columns: 1fr 1fr;
			grid-template-rows: auto auto auto;
			gap: 6px;
		}

		.col-email {
			grid-column: 1 / -1;
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
	}
</style>
