<!--
  Variant group editor — manages variant groups and their variants for a product.
  Each group can be expanded to show/add/edit/delete variants.
  Uses SvelteKit form actions for all mutations.
-->
<script lang="ts">
	import { enhance } from '$app/forms';
	import { formatRupiah } from '$lib/utils/format';
	import type { VariantGroup } from '$lib/types/api';

	let { variantGroups, form }: { variantGroups: VariantGroup[]; form: Record<string, unknown> | null } = $props();

	let showAddGroup = $state(false);
	let editingGroupId = $state<string | null>(null);
	let expandedGroupId = $state<string | null>(null);
	let addingVariantToGroupId = $state<string | null>(null);
	let editingVariantId = $state<string | null>(null);

	function toggleExpand(groupId: string) {
		expandedGroupId = expandedGroupId === groupId ? null : groupId;
	}
</script>

<div class="section">
	<div class="section-header">
		<h3 class="section-title">Grup Varian</h3>
		<button type="button" class="btn-add" onclick={() => { showAddGroup = true; editingGroupId = null; }}>
			+ Tambah Grup
		</button>
	</div>

	{#if form?.variantGroupError}
		<div class="error-banner">{form.variantGroupError}</div>
	{/if}

	<!-- Add new group form -->
	{#if showAddGroup}
		<form method="POST" action="?/createVariantGroup" use:enhance={() => {
			return async ({ result, update }) => {
				if (result.type === 'success') {
					showAddGroup = false;
				}
				await update();
			};
		}}>
			<div class="inline-form">
				<div class="form-row">
					<div class="form-group flex-1">
						<label class="form-label">Nama Grup
							<input name="name" type="text" class="input-field" placeholder="Contoh: Ukuran, Level Pedas" required />
						</label>
					</div>
					<div class="form-group">
						<label class="form-label">Urutan
							<input name="sort_order" type="number" class="input-field input-sm" value="0" />
						</label>
					</div>
					<div class="form-group">
						<label class="form-label checkbox-label">
							<input name="is_required" type="checkbox" value="true" />
							Wajib
						</label>
					</div>
				</div>
				<div class="form-actions">
					<button type="submit" class="btn-primary btn-sm">Simpan</button>
					<button type="button" class="btn-secondary btn-sm" onclick={() => { showAddGroup = false; }}>Batal</button>
				</div>
			</div>
		</form>
	{/if}

	<!-- List of variant groups -->
	{#if variantGroups.length === 0 && !showAddGroup}
		<p class="empty-text">Belum ada grup varian. Klik "Tambah Grup" untuk membuat.</p>
	{/if}

	{#each variantGroups as group (group.id)}
		<div class="group-card">
			<!-- Group header -->
			<div class="group-header">
				<button type="button" class="group-toggle" onclick={() => toggleExpand(group.id)}>
					<span class="toggle-icon">{expandedGroupId === group.id ? '−' : '+'}</span>
					<span class="group-name">{group.name}</span>
					{#if group.is_required}
						<span class="badge badge-required">Wajib</span>
					{/if}
					<span class="variant-count">{group.variants?.length ?? 0} varian</span>
				</button>
				<div class="group-actions">
					<button type="button" class="btn-icon" onclick={() => { editingGroupId = editingGroupId === group.id ? null : group.id; }}>Edit</button>
					<form method="POST" action="?/deleteVariantGroup" use:enhance={() => {
						return async ({ update }) => { await update(); };
					}}>
						<input type="hidden" name="id" value={group.id} />
						<button type="submit" class="btn-icon btn-danger"
							onclick={(e) => { if (!confirm('Hapus grup varian "' + group.name + '"?')) e.preventDefault(); }}>
							Hapus
						</button>
					</form>
				</div>
			</div>

			<!-- Edit group form (inline) -->
			{#if editingGroupId === group.id}
				<form method="POST" action="?/updateVariantGroup" use:enhance={() => {
					return async ({ result, update }) => {
						if (result.type === 'success') {
							editingGroupId = null;
						}
						await update();
					};
				}}>
					<input type="hidden" name="id" value={group.id} />
					<div class="inline-form">
						<div class="form-row">
							<div class="form-group flex-1">
								<label class="form-label">Nama Grup
									<input name="name" type="text" class="input-field" value={group.name} required />
								</label>
							</div>
							<div class="form-group">
								<label class="form-label">Urutan
									<input name="sort_order" type="number" class="input-field input-sm" value={group.sort_order} />
								</label>
							</div>
							<div class="form-group">
								<label class="form-label checkbox-label">
									<input name="is_required" type="checkbox" value="true" checked={group.is_required} />
									Wajib
								</label>
							</div>
						</div>
						<div class="form-actions">
							<button type="submit" class="btn-primary btn-sm">Simpan</button>
							<button type="button" class="btn-secondary btn-sm" onclick={() => { editingGroupId = null; }}>Batal</button>
						</div>
					</div>
				</form>
			{/if}

			<!-- Expanded: variants list -->
			{#if expandedGroupId === group.id}
				<div class="variants-list">
					{#if form?.variantError}
						<div class="error-banner">{form.variantError}</div>
					{/if}

					{#if (group.variants?.length ?? 0) === 0}
						<p class="empty-text">Belum ada varian di grup ini.</p>
					{/if}

					{#each group.variants ?? [] as variant (variant.id)}
						<div class="variant-item">
							{#if editingVariantId === variant.id}
								<!-- Edit variant inline -->
								<form method="POST" action="?/updateVariant" use:enhance={() => {
									return async ({ result, update }) => {
										if (result.type === 'success') {
											editingVariantId = null;
										}
										await update();
									};
								}}>
									<input type="hidden" name="variant_group_id" value={group.id} />
									<input type="hidden" name="id" value={variant.id} />
									<div class="form-row">
										<div class="form-group flex-1">
											<input name="name" type="text" class="input-field" value={variant.name} required />
										</div>
										<div class="form-group">
											<input name="price_adjustment" type="text" class="input-field input-sm" value={variant.price_adjustment} placeholder="0.00" />
										</div>
										<div class="form-group">
											<input name="sort_order" type="number" class="input-field input-xs" value={variant.sort_order} />
										</div>
										<button type="submit" class="btn-primary btn-sm">OK</button>
										<button type="button" class="btn-secondary btn-sm" onclick={() => { editingVariantId = null; }}>X</button>
									</div>
								</form>
							{:else}
								<div class="variant-info">
									<span class="variant-name">{variant.name}</span>
									<span class="variant-price">
										{parseFloat(variant.price_adjustment) !== 0 ? (parseFloat(variant.price_adjustment) > 0 ? '+' : '') + formatRupiah(variant.price_adjustment) : '-'}
									</span>
								</div>
								<div class="variant-actions">
									<button type="button" class="btn-icon" onclick={() => { editingVariantId = variant.id; }}>Edit</button>
									<form method="POST" action="?/deleteVariant" use:enhance>
										<input type="hidden" name="variant_group_id" value={group.id} />
										<input type="hidden" name="id" value={variant.id} />
										<button type="submit" class="btn-icon btn-danger"
											onclick={(e) => { if (!confirm('Hapus varian "' + variant.name + '"?')) e.preventDefault(); }}>
											Hapus
										</button>
									</form>
								</div>
							{/if}
						</div>
					{/each}

					<!-- Add variant form -->
					{#if addingVariantToGroupId === group.id}
						<form method="POST" action="?/createVariant" use:enhance={() => {
							return async ({ result, update }) => {
								if (result.type === 'success') {
									addingVariantToGroupId = null;
								}
								await update();
							};
						}}>
							<input type="hidden" name="variant_group_id" value={group.id} />
							<div class="form-row variant-add-form">
								<div class="form-group flex-1">
									<input name="name" type="text" class="input-field" placeholder="Nama varian" required />
								</div>
								<div class="form-group">
									<input name="price_adjustment" type="text" class="input-field input-sm" placeholder="0.00" value="0.00" />
								</div>
								<div class="form-group">
									<input name="sort_order" type="number" class="input-field input-xs" value="0" />
								</div>
								<button type="submit" class="btn-primary btn-sm">Tambah</button>
								<button type="button" class="btn-secondary btn-sm" onclick={() => { addingVariantToGroupId = null; }}>X</button>
							</div>
						</form>
					{:else}
						<button type="button" class="btn-add-variant" onclick={() => { addingVariantToGroupId = group.id; }}>
							+ Tambah Varian
						</button>
					{/if}
				</div>
			{/if}
		</div>
	{/each}
</div>

<style>
	.section {
		margin-bottom: 24px;
	}

	.section-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		margin-bottom: 12px;
	}

	.section-title {
		font-size: var(--text-title);
		font-weight: 600;
		color: var(--color-text-primary);
		margin: 0;
	}

	.btn-add {
		background: none;
		border: 1px dashed var(--color-primary);
		color: var(--color-primary);
		font-size: 13px;
		font-weight: 600;
		padding: 6px 12px;
		border-radius: var(--radius-btn);
		cursor: pointer;
	}

	.btn-add:hover {
		background-color: var(--color-surface);
	}

	.error-banner {
		background-color: var(--color-error-bg);
		color: var(--color-error);
		font-size: 13px;
		font-weight: 500;
		padding: 8px 12px;
		border-radius: var(--radius-chip);
		margin-bottom: 8px;
	}

	.empty-text {
		font-size: 13px;
		color: var(--color-text-secondary);
		margin: 8px 0;
	}

	.group-card {
		border: 1px solid var(--color-border);
		border-radius: var(--radius-card);
		margin-bottom: 8px;
		overflow: hidden;
	}

	.group-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 12px 16px;
		background-color: var(--color-surface);
	}

	.group-toggle {
		display: flex;
		align-items: center;
		gap: 8px;
		background: none;
		border: none;
		cursor: pointer;
		font-size: var(--text-body);
		color: var(--color-text-primary);
		padding: 0;
	}

	.toggle-icon {
		width: 20px;
		height: 20px;
		display: flex;
		align-items: center;
		justify-content: center;
		font-weight: 700;
		color: var(--color-text-secondary);
	}

	.group-name {
		font-weight: 600;
	}

	.badge {
		font-size: 11px;
		padding: 2px 6px;
		border-radius: 4px;
		font-weight: 500;
	}

	.badge-required {
		background-color: var(--color-accent);
		color: var(--color-text-primary);
	}

	.variant-count {
		font-size: 12px;
		color: var(--color-text-secondary);
	}

	.group-actions {
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
		background-color: var(--color-bg);
		color: var(--color-text-primary);
	}

	.btn-danger {
		color: var(--color-error);
	}

	.btn-danger:hover {
		background-color: var(--color-error-bg);
		color: var(--color-error);
	}

	.inline-form {
		padding: 12px 16px;
		border-top: 1px solid var(--color-border);
		background-color: var(--color-bg);
	}

	.form-row {
		display: flex;
		align-items: flex-end;
		gap: 8px;
		flex-wrap: wrap;
	}

	.form-group {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.flex-1 {
		flex: 1;
		min-width: 160px;
	}

	.form-label {
		display: block;
		font-size: 12px;
		font-weight: 600;
		color: var(--color-text-primary);
	}

	.form-label :global(input),
	.form-label :global(select) {
		display: block;
		margin-top: 4px;
	}

	.checkbox-label {
		display: flex;
		align-items: center;
		gap: 6px;
		padding-top: 20px;
		cursor: pointer;
	}

	.input-sm {
		width: 100px;
	}

	.input-xs {
		width: 64px;
	}

	.form-actions {
		display: flex;
		gap: 8px;
		margin-top: 8px;
	}

	.btn-sm {
		padding: 6px 14px;
		font-size: 13px;
		border: none;
		cursor: pointer;
	}

	.variants-list {
		padding: 12px 16px;
		border-top: 1px solid var(--color-border);
	}

	.variant-item {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 8px 0;
		border-bottom: 1px solid var(--color-border);
	}

	.variant-item:last-of-type {
		border-bottom: none;
	}

	.variant-info {
		display: flex;
		align-items: center;
		gap: 12px;
	}

	.variant-name {
		font-size: 13px;
		font-weight: 500;
		color: var(--color-text-primary);
	}

	.variant-price {
		font-size: 12px;
		color: var(--color-text-secondary);
	}

	.variant-actions {
		display: flex;
		align-items: center;
		gap: 4px;
	}

	.variant-add-form {
		padding-top: 8px;
	}

	.btn-add-variant {
		background: none;
		border: none;
		color: var(--color-primary);
		font-size: 13px;
		font-weight: 600;
		cursor: pointer;
		padding: 8px 0;
	}

	.btn-add-variant:hover {
		text-decoration: underline;
	}
</style>
