<!--
  Reusable product form — used for both creating and editing products.
  Renders basic info fields: name, price, category, station, prep time, description, is_combo.
  Uses SvelteKit form actions (action="?/saveProduct").
-->
<script lang="ts">
	import { enhance } from '$app/forms';
	import type { Category, Product } from '$lib/types/api';

	let {
		product,
		categories,
		isNew,
		form
	}: {
		product: Product | null;
		categories: Category[];
		isNew: boolean;
		form: Record<string, unknown> | null;
	} = $props();

	const stations = [
		{ value: '', label: '-- Tidak ada --' },
		{ value: 'GRILL', label: 'Grill' },
		{ value: 'BEVERAGE', label: 'Beverage' },
		{ value: 'RICE', label: 'Rice' },
		{ value: 'DESSERT', label: 'Dessert' }
	];

	let loading = $state(false);
	let comboOverride = $state<boolean | null>(null);
	let isCombo = $derived(comboOverride !== null ? comboOverride : (product?.is_combo ?? false));
</script>

<form
	method="POST"
	action="?/saveProduct"
	use:enhance={() => {
		loading = true;
		return async ({ update }) => {
			loading = false;
			await update();
		};
	}}
>
	{#if form?.error}
		<div class="error-message">{form.error}</div>
	{/if}

	{#if form?.success}
		<div class="success-message">Produk berhasil disimpan.</div>
	{/if}

	<div class="form-grid">
		<div class="form-group">
			<label for="name" class="form-label">Nama Produk *</label>
			<input
				id="name"
				name="name"
				type="text"
				class="input-field"
				value={product?.name ?? ''}
				placeholder="Nama produk"
				required
				disabled={loading}
			/>
		</div>

		<div class="form-group">
			<label for="base_price" class="form-label">Harga Dasar (Rp) *</label>
			<input
				id="base_price"
				name="base_price"
				type="text"
				class="input-field"
				value={product?.base_price ?? '0.00'}
				placeholder="25000.00"
				required
				disabled={loading}
			/>
		</div>

		<div class="form-group">
			<label for="category_id" class="form-label">Kategori *</label>
			<select id="category_id" name="category_id" class="input-field" required disabled={loading}>
				<option value="">-- Pilih Kategori --</option>
				{#each categories as cat (cat.id)}
					<option value={cat.id} selected={product?.category_id === cat.id}>{cat.name}</option>
				{/each}
			</select>
		</div>

		<div class="form-group">
			<label for="station" class="form-label">Station</label>
			<select id="station" name="station" class="input-field" disabled={loading}>
				{#each stations as s}
					<option value={s.value} selected={(product?.station ?? '') === s.value}>{s.label}</option>
				{/each}
			</select>
		</div>

		<div class="form-group">
			<label for="preparation_time" class="form-label">Waktu Persiapan (menit)</label>
			<input
				id="preparation_time"
				name="preparation_time"
				type="number"
				class="input-field"
				value={product?.preparation_time ?? 0}
				min="0"
				disabled={loading}
			/>
		</div>

		<div class="form-group">
			<label for="image_url" class="form-label">URL Gambar</label>
			<input
				id="image_url"
				name="image_url"
				type="text"
				class="input-field"
				value={product?.image_url ?? ''}
				placeholder="https://..."
				disabled={loading}
			/>
		</div>

		<div class="form-group full-width">
			<label for="description" class="form-label">Deskripsi</label>
			<textarea
				id="description"
				name="description"
				class="input-field textarea"
				rows="3"
				placeholder="Deskripsi singkat produk"
				disabled={loading}
			>{product?.description ?? ''}</textarea>
		</div>

		<div class="form-group">
			<label class="form-label checkbox-label">
				<input
					type="checkbox"
					checked={isCombo}
					onchange={(e) => { comboOverride = e.currentTarget.checked; }}
					disabled={loading || !isNew}
				/>
				Produk Combo
			</label>
			<input type="hidden" name="is_combo" value={isCombo ? 'true' : 'false'} />
			{#if !isNew && product?.is_combo}
				<p class="hint-text">Status combo tidak dapat diubah setelah dibuat.</p>
			{/if}
		</div>

		<div class="form-group">
			<label class="form-label checkbox-label">
				<input
					type="checkbox"
					name="is_active"
					value="true"
					checked={product?.is_active ?? true}
					disabled={loading}
				/>
				Aktif
			</label>
		</div>
	</div>

	<div class="form-footer">
		<button type="submit" class="btn-primary btn-save" disabled={loading}>
			{#if loading}
				Menyimpan...
			{:else}
				{isNew ? 'Buat Produk' : 'Simpan Perubahan'}
			{/if}
		</button>
	</div>
</form>

<style>
	.error-message {
		background-color: var(--color-error-bg);
		color: var(--color-error);
		font-size: 13px;
		font-weight: 500;
		padding: 10px 12px;
		border-radius: var(--radius-chip);
		margin-bottom: 16px;
	}

	.success-message {
		background-color: color-mix(in srgb, var(--color-primary) 10%, white);
		color: var(--color-primary);
		font-size: 13px;
		font-weight: 500;
		padding: 10px 12px;
		border-radius: var(--radius-chip);
		margin-bottom: 16px;
	}

	.form-grid {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 16px;
	}

	.form-group {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.full-width {
		grid-column: 1 / -1;
	}

	.form-label {
		font-size: 13px;
		font-weight: 600;
		color: var(--color-text-primary);
	}

	.checkbox-label {
		flex-direction: row;
		align-items: center;
		cursor: pointer;
		padding-top: 8px;
	}

	.textarea {
		resize: vertical;
		min-height: 60px;
	}

	.hint-text {
		font-size: 11px;
		color: var(--color-text-secondary);
		margin: 0;
	}

	.form-footer {
		margin-top: 20px;
		display: flex;
		justify-content: flex-end;
	}

	.btn-save {
		padding: 10px 24px;
		font-size: 14px;
		border: none;
		cursor: pointer;
	}

	select.input-field {
		appearance: auto;
	}
</style>
