<!--
  Modifier group editor — manages modifier groups and their modifiers for a product.
  Each group has min/max selection rules and can be expanded to show modifiers.
  Uses SvelteKit form actions for all mutations.
-->
<script lang="ts">
	import { enhance } from '$app/forms';
	import { formatRupiah } from '$lib/utils/format';
	import type { ModifierGroup } from '$lib/types/api';

	let { modifierGroups, form }: { modifierGroups: ModifierGroup[]; form: Record<string, unknown> | null } = $props();

	let showAddGroup = $state(false);
	let editingGroupId = $state<string | null>(null);
	let expandedGroupId = $state<string | null>(null);
	let addingModifierToGroupId = $state<string | null>(null);
	let editingModifierId = $state<string | null>(null);

	function toggleExpand(groupId: string) {
		expandedGroupId = expandedGroupId === groupId ? null : groupId;
	}
</script>

<div class="section">
	<div class="section-header">
		<h3 class="section-title">Grup Modifier</h3>
		<button type="button" class="btn-add" onclick={() => { showAddGroup = true; editingGroupId = null; }}>
			+ Tambah Grup
		</button>
	</div>

	{#if form?.modifierGroupError}
		<div class="error-banner">{form.modifierGroupError}</div>
	{/if}

	<!-- Add new group form -->
	{#if showAddGroup}
		<form method="POST" action="?/createModifierGroup" use:enhance={() => {
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
							<input name="name" type="text" class="input-field" placeholder="Contoh: Topping, Saus" required />
						</label>
					</div>
					<div class="form-group">
						<label class="form-label">Min Pilih
							<input name="min_select" type="number" class="input-field input-sm" value="0" min="0" />
						</label>
					</div>
					<div class="form-group">
						<label class="form-label">Max Pilih
							<input name="max_select" type="number" class="input-field input-sm" value="0" min="0" />
						</label>
					</div>
					<div class="form-group">
						<label class="form-label">Urutan
							<input name="sort_order" type="number" class="input-field input-xs" value="0" />
						</label>
					</div>
				</div>
				<p class="hint-text">Max 0 = tidak terbatas (dikirim sebagai null)</p>
				<div class="form-actions">
					<button type="submit" class="btn-primary btn-sm">Simpan</button>
					<button type="button" class="btn-secondary btn-sm" onclick={() => { showAddGroup = false; }}>Batal</button>
				</div>
			</div>
		</form>
	{/if}

	<!-- List of modifier groups -->
	{#if modifierGroups.length === 0 && !showAddGroup}
		<p class="empty-text">Belum ada grup modifier. Klik "Tambah Grup" untuk membuat.</p>
	{/if}

	{#each modifierGroups as group (group.id)}
		<div class="group-card">
			<!-- Group header -->
			<div class="group-header">
				<button type="button" class="group-toggle" onclick={() => toggleExpand(group.id)}>
					<span class="toggle-icon">{expandedGroupId === group.id ? '−' : '+'}</span>
					<span class="group-name">{group.name}</span>
					<span class="group-rules">
						min {group.min_select} / max {group.max_select === null ? '~' : group.max_select}
					</span>
					<span class="modifier-count">{group.modifiers?.length ?? 0} modifier</span>
				</button>
				<div class="group-actions">
					<button type="button" class="btn-icon" onclick={() => { editingGroupId = editingGroupId === group.id ? null : group.id; }}>Edit</button>
					<form method="POST" action="?/deleteModifierGroup" use:enhance={() => {
						return async ({ update }) => { await update(); };
					}}>
						<input type="hidden" name="id" value={group.id} />
						<button type="submit" class="btn-icon btn-danger"
							onclick={(e) => { if (!confirm('Hapus grup modifier "' + group.name + '"?')) e.preventDefault(); }}>
							Hapus
						</button>
					</form>
				</div>
			</div>

			<!-- Edit group form (inline) -->
			{#if editingGroupId === group.id}
				<form method="POST" action="?/updateModifierGroup" use:enhance={() => {
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
								<label class="form-label">Min Pilih
									<input name="min_select" type="number" class="input-field input-sm" value={group.min_select} min="0" />
								</label>
							</div>
							<div class="form-group">
								<label class="form-label">Max Pilih
									<input name="max_select" type="number" class="input-field input-sm" value={group.max_select ?? 0} min="0" />
								</label>
							</div>
							<div class="form-group">
								<label class="form-label">Urutan
									<input name="sort_order" type="number" class="input-field input-xs" value={group.sort_order} />
								</label>
							</div>
						</div>
						<p class="hint-text">Max 0 = tidak terbatas (dikirim sebagai null)</p>
						<div class="form-actions">
							<button type="submit" class="btn-primary btn-sm">Simpan</button>
							<button type="button" class="btn-secondary btn-sm" onclick={() => { editingGroupId = null; }}>Batal</button>
						</div>
					</div>
				</form>
			{/if}

			<!-- Expanded: modifiers list -->
			{#if expandedGroupId === group.id}
				<div class="modifiers-list">
					{#if form?.modifierError}
						<div class="error-banner">{form.modifierError}</div>
					{/if}

					{#if (group.modifiers?.length ?? 0) === 0}
						<p class="empty-text">Belum ada modifier di grup ini.</p>
					{/if}

					{#each group.modifiers ?? [] as modifier (modifier.id)}
						<div class="modifier-item">
							{#if editingModifierId === modifier.id}
								<!-- Edit modifier inline -->
								<form method="POST" action="?/updateModifier" use:enhance={() => {
									return async ({ result, update }) => {
										if (result.type === 'success') {
											editingModifierId = null;
										}
										await update();
									};
								}}>
									<input type="hidden" name="modifier_group_id" value={group.id} />
									<input type="hidden" name="id" value={modifier.id} />
									<div class="form-row">
										<div class="form-group flex-1">
											<input name="name" type="text" class="input-field" value={modifier.name} required />
										</div>
										<div class="form-group">
											<input name="price" type="text" class="input-field input-sm" value={modifier.price} placeholder="0.00" />
										</div>
										<div class="form-group">
											<input name="sort_order" type="number" class="input-field input-xs" value={modifier.sort_order} />
										</div>
										<button type="submit" class="btn-primary btn-sm">OK</button>
										<button type="button" class="btn-secondary btn-sm" onclick={() => { editingModifierId = null; }}>X</button>
									</div>
								</form>
							{:else}
								<div class="modifier-info">
									<span class="modifier-name">{modifier.name}</span>
									<span class="modifier-price">
										{parseFloat(modifier.price) !== 0 ? formatRupiah(modifier.price) : '-'}
									</span>
								</div>
								<div class="modifier-actions">
									<button type="button" class="btn-icon" onclick={() => { editingModifierId = modifier.id; }}>Edit</button>
									<form method="POST" action="?/deleteModifier" use:enhance>
										<input type="hidden" name="modifier_group_id" value={group.id} />
										<input type="hidden" name="id" value={modifier.id} />
										<button type="submit" class="btn-icon btn-danger"
											onclick={(e) => { if (!confirm('Hapus modifier "' + modifier.name + '"?')) e.preventDefault(); }}>
											Hapus
										</button>
									</form>
								</div>
							{/if}
						</div>
					{/each}

					<!-- Add modifier form -->
					{#if addingModifierToGroupId === group.id}
						<form method="POST" action="?/createModifier" use:enhance={() => {
							return async ({ result, update }) => {
								if (result.type === 'success') {
									addingModifierToGroupId = null;
								}
								await update();
							};
						}}>
							<input type="hidden" name="modifier_group_id" value={group.id} />
							<div class="form-row modifier-add-form">
								<div class="form-group flex-1">
									<input name="name" type="text" class="input-field" placeholder="Nama modifier" required />
								</div>
								<div class="form-group">
									<input name="price" type="text" class="input-field input-sm" placeholder="0.00" value="0.00" />
								</div>
								<div class="form-group">
									<input name="sort_order" type="number" class="input-field input-xs" value="0" />
								</div>
								<button type="submit" class="btn-primary btn-sm">Tambah</button>
								<button type="button" class="btn-secondary btn-sm" onclick={() => { addingModifierToGroupId = null; }}>X</button>
							</div>
						</form>
					{:else}
						<button type="button" class="btn-add-modifier" onclick={() => { addingModifierToGroupId = group.id; }}>
							+ Tambah Modifier
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

	.hint-text {
		font-size: 11px;
		color: var(--color-text-secondary);
		margin: 4px 0 0;
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

	.group-rules {
		font-size: 11px;
		color: var(--color-text-secondary);
		background-color: var(--color-bg);
		padding: 2px 6px;
		border-radius: 4px;
	}

	.modifier-count {
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

	.modifiers-list {
		padding: 12px 16px;
		border-top: 1px solid var(--color-border);
	}

	.modifier-item {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 8px 0;
		border-bottom: 1px solid var(--color-border);
	}

	.modifier-item:last-of-type {
		border-bottom: none;
	}

	.modifier-info {
		display: flex;
		align-items: center;
		gap: 12px;
	}

	.modifier-name {
		font-size: 13px;
		font-weight: 500;
		color: var(--color-text-primary);
	}

	.modifier-price {
		font-size: 12px;
		color: var(--color-text-secondary);
	}

	.modifier-actions {
		display: flex;
		align-items: center;
		gap: 4px;
	}

	.modifier-add-form {
		padding-top: 8px;
	}

	.btn-add-modifier {
		background: none;
		border: none;
		color: var(--color-primary);
		font-size: 13px;
		font-weight: 600;
		cursor: pointer;
		padding: 8px 0;
	}

	.btn-add-modifier:hover {
		text-decoration: underline;
	}
</style>
