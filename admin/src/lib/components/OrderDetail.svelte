<!--
  Order detail panel — slide-in from right showing full order info.
  Includes items with variants/modifiers, payments, timeline, and status actions.
-->
<script lang="ts">
	import { enhance } from '$app/forms';
	import { formatRupiah } from '$lib/utils/format';
	import {
		formatDateTime,
		formatDate,
		getOrderTypeLabel,
		getStatusLabel,
		getCateringStatusLabel
	} from '$lib/utils/labels';
	import OrderTimeline from '$lib/components/OrderTimeline.svelte';
	import type { Order, OrderStatus } from '$lib/types/api';

	interface Props {
		order: Order;
		onClose: () => void;
		statusError?: string | null;
	}

	let { order, onClose, statusError = null }: Props = $props();

	let submitting = $state(false);

	function getKitchenStatusLabel(status: string): string {
		const labels: Record<string, string> = {
			PENDING: 'Menunggu',
			PREPARING: 'Dimasak',
			READY: 'Siap'
		};
		return labels[status] ?? status;
	}

	function getPaymentMethodLabel(method: string): string {
		const labels: Record<string, string> = {
			CASH: 'Tunai',
			QRIS: 'QRIS',
			TRANSFER: 'Transfer'
		};
		return labels[method] ?? method;
	}

	/** Determine valid next statuses based on current status */
	function getNextActions(status: OrderStatus): { label: string; value: OrderStatus; variant: 'primary' | 'cancel' }[] {
		const actions: { label: string; value: OrderStatus; variant: 'primary' | 'cancel' }[] = [];
		switch (status) {
			case 'NEW':
				actions.push({ label: 'Mulai Proses', value: 'PREPARING', variant: 'primary' });
				actions.push({ label: 'Batalkan', value: 'CANCELLED', variant: 'cancel' });
				break;
			case 'PREPARING':
				actions.push({ label: 'Siap Diambil', value: 'READY', variant: 'primary' });
				actions.push({ label: 'Batalkan', value: 'CANCELLED', variant: 'cancel' });
				break;
			case 'READY':
				actions.push({ label: 'Selesai', value: 'COMPLETED', variant: 'primary' });
				actions.push({ label: 'Batalkan', value: 'CANCELLED', variant: 'cancel' });
				break;
		}
		return actions;
	}

	let nextActions = $derived(getNextActions(order.status));
	let totalPaid = $derived(
		(order.payments ?? []).reduce((sum, p) => sum + parseFloat(p.amount), 0)
	);
	let remainingBalance = $derived(parseFloat(order.total_amount) - totalPaid);
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="overlay" onclick={onClose} onkeydown={(e) => { if (e.key === 'Escape') onClose(); }}>
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div class="detail-panel" onclick={(e) => e.stopPropagation()} onkeydown={() => {}}>
		<!-- Header -->
		<div class="panel-header">
			<div class="header-info">
				<h2 class="order-number">{order.order_number}</h2>
				<div class="header-badges">
					<span class="status-badge status-{order.status.toLowerCase()}">{getStatusLabel(order.status)}</span>
					<span class="type-badge">{getOrderTypeLabel(order.order_type)}</span>
				</div>
				<span class="order-date">{formatDateTime(order.created_at)}</span>
			</div>
			<button type="button" class="btn-close" onclick={onClose}>&times;</button>
		</div>

		<div class="panel-body">
			<!-- Timeline -->
			<div class="section">
				<h3 class="section-title">Status</h3>
				<OrderTimeline status={order.status} createdAt={order.created_at} completedAt={null} />
			</div>

			<!-- Customer info -->
			{#if order.customer_id}
				<div class="section">
					<h3 class="section-title">Pelanggan</h3>
					<p class="info-text">{order.customer_name ?? order.customer_id}</p>
					{#if order.customer_phone}
						<p class="info-text">Tel: {order.customer_phone}</p>
					{/if}
				</div>
			{/if}

			<!-- Table number -->
			{#if order.table_number}
				<div class="section">
					<h3 class="section-title">Meja</h3>
					<p class="info-text">{order.table_number}</p>
				</div>
			{/if}

			<!-- Catering info -->
			{#if order.order_type === 'CATERING'}
				<div class="section">
					<h3 class="section-title">Info Katering</h3>
					<div class="info-grid">
						{#if order.catering_date}
							<div class="info-item">
								<span class="info-label">Tanggal</span>
								<span class="info-value">{formatDate(order.catering_date)}</span>
							</div>
						{/if}
						{#if order.catering_status}
							<div class="info-item">
								<span class="info-label">Status Katering</span>
								<span class="catering-badge catering-{order.catering_status.toLowerCase()}">{getCateringStatusLabel(order.catering_status)}</span>
							</div>
						{/if}
						{#if order.catering_dp_amount}
							<div class="info-item">
								<span class="info-label">Uang Muka (DP)</span>
								<span class="info-value">{formatRupiah(order.catering_dp_amount)}</span>
							</div>
						{/if}
						{#if order.delivery_address}
							<div class="info-item">
								<span class="info-label">Alamat</span>
								<span class="info-value">{order.delivery_address}</span>
							</div>
						{/if}
					</div>
				</div>
			{/if}

			<!-- Delivery info -->
			{#if order.order_type === 'DELIVERY' && order.delivery_platform}
				<div class="section">
					<h3 class="section-title">Info Pengiriman</h3>
					<div class="info-grid">
						<div class="info-item">
							<span class="info-label">Platform</span>
							<span class="info-value">{order.delivery_platform}</span>
						</div>
						{#if order.delivery_address}
							<div class="info-item">
								<span class="info-label">Alamat</span>
								<span class="info-value">{order.delivery_address}</span>
							</div>
						{/if}
					</div>
				</div>
			{/if}

			<!-- Order notes -->
			{#if order.notes}
				<div class="section">
					<h3 class="section-title">Catatan</h3>
					<p class="info-text">{order.notes}</p>
				</div>
			{/if}

			<!-- Items -->
			<div class="section">
				<h3 class="section-title">Item ({order.items?.length ?? 0})</h3>
				{#if order.items && order.items.length > 0}
					<div class="items-list">
						{#each order.items as item (item.id)}
							<div class="item-row">
								<div class="item-main">
									<div class="item-header">
										<span class="item-qty">{item.quantity}x</span>
										<span class="item-name">{item.product_name ?? 'Produk tidak dikenal'}</span>
										<span class="kitchen-badge kitchen-{item.status.toLowerCase()}">{getKitchenStatusLabel(item.status)}</span>
									</div>
									{#if item.variant_name || item.variant_id}
										<span class="item-variant">{item.variant_name ?? 'Varian'}</span>
									{/if}
									{#if item.modifiers && item.modifiers.length > 0}
										<div class="item-modifiers">
											{#each item.modifiers as mod (mod.id)}
												<span class="modifier-tag">+ {mod.modifier_name ?? 'Modifier'}{#if mod.quantity > 1} x{mod.quantity}{/if}{#if parseFloat(mod.unit_price) > 0} ({formatRupiah(mod.unit_price)}){/if}</span>
											{/each}
										</div>
									{/if}
									{#if item.notes}
										<span class="item-notes">{item.notes}</span>
									{/if}
								</div>
								<div class="item-price">
									<span class="item-subtotal">{formatRupiah(item.subtotal)}</span>
									{#if parseFloat(item.discount_amount) > 0}
										<span class="item-discount">-{formatRupiah(item.discount_amount)}</span>
									{/if}
								</div>
							</div>
						{/each}
					</div>
				{:else}
					<p class="empty-text">Tidak ada item.</p>
				{/if}
			</div>

			<!-- Order summary -->
			<div class="section summary-section">
				<div class="summary-row">
					<span>Subtotal</span>
					<span>{formatRupiah(order.subtotal)}</span>
				</div>
				{#if parseFloat(order.discount_amount) > 0}
					<div class="summary-row discount">
						<span>Diskon{#if order.discount_type === 'PERCENTAGE' && order.discount_value} ({order.discount_value}%){/if}</span>
						<span>-{formatRupiah(order.discount_amount)}</span>
					</div>
				{/if}
				{#if parseFloat(order.tax_amount) > 0}
					<div class="summary-row">
						<span>Pajak</span>
						<span>{formatRupiah(order.tax_amount)}</span>
					</div>
				{/if}
				<div class="summary-row total">
					<span>Total</span>
					<span>{formatRupiah(order.total_amount)}</span>
				</div>
			</div>

			<!-- Payments -->
			<div class="section">
				<h3 class="section-title">Pembayaran ({order.payments?.length ?? 0})</h3>
				{#if order.payments && order.payments.length > 0}
					<div class="payments-list">
						{#each order.payments as payment (payment.id)}
							<div class="payment-row">
								<div class="payment-info">
									<span class="payment-method method-{payment.payment_method.toLowerCase()}">{getPaymentMethodLabel(payment.payment_method)}</span>
									{#if payment.reference_number}
										<span class="payment-ref">Ref: {payment.reference_number}</span>
									{/if}
									<span class="payment-time">{formatDateTime(payment.processed_at)}</span>
								</div>
								<span class="payment-amount">{formatRupiah(payment.amount)}</span>
							</div>
						{/each}
					</div>
					<div class="payment-summary">
						<div class="summary-row">
							<span>Total Dibayar</span>
							<span>{formatRupiah(totalPaid)}</span>
						</div>
						{#if remainingBalance > 0}
							<div class="summary-row remaining">
								<span>Sisa</span>
								<span>{formatRupiah(remainingBalance)}</span>
							</div>
						{/if}
					</div>
				{:else}
					<p class="empty-text">Belum ada pembayaran.</p>
				{/if}
			</div>

			<!-- Status error -->
			{#if statusError}
				<div class="error-banner">{statusError}</div>
			{/if}

			<!-- Status actions -->
			{#if nextActions.length > 0}
				<div class="section actions-section">
					{#each nextActions as action (action.value)}
						{#if action.variant === 'cancel'}
							<form method="POST" action="?/cancelOrder" use:enhance={() => {
								submitting = true;
								return async ({ update }) => {
									submitting = false;
									await update();
								};
							}}>
								<input type="hidden" name="order_id" value={order.id} />
								<button
									type="submit"
									class="btn-action btn-cancel"
									disabled={submitting}
									onclick={(e) => { if (!confirm('Batalkan pesanan ' + order.order_number + '?')) e.preventDefault(); }}
								>
									{action.label}
								</button>
							</form>
						{:else}
							<form method="POST" action="?/updateStatus" use:enhance={() => {
								submitting = true;
								return async ({ update }) => {
									submitting = false;
									await update();
								};
							}}>
								<input type="hidden" name="order_id" value={order.id} />
								<input type="hidden" name="status" value={action.value} />
								<button type="submit" class="btn-action btn-primary" disabled={submitting}>
									{action.label}
								</button>
							</form>
						{/if}
					{/each}
				</div>
			{/if}
		</div>
	</div>
</div>

<style>
	.overlay {
		position: fixed;
		inset: 0;
		background-color: rgba(0, 0, 0, 0.3);
		z-index: 100;
		display: flex;
		justify-content: flex-end;
	}

	.detail-panel {
		width: 480px;
		max-width: 100vw;
		background-color: var(--color-bg);
		height: 100vh;
		overflow-y: auto;
		box-shadow: -4px 0 24px rgba(0, 0, 0, 0.1);
		display: flex;
		flex-direction: column;
	}

	.panel-header {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		padding: 20px;
		border-bottom: 1px solid var(--color-border);
		flex-shrink: 0;
	}

	.header-info {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.order-number {
		font-size: 18px;
		font-weight: 700;
		color: var(--color-text-primary);
		margin: 0;
	}

	.header-badges {
		display: flex;
		gap: 6px;
		align-items: center;
	}

	.order-date {
		font-size: 12px;
		color: var(--color-text-secondary);
	}

	.btn-close {
		background: none;
		border: none;
		font-size: 24px;
		color: var(--color-text-secondary);
		cursor: pointer;
		padding: 0 4px;
		line-height: 1;
	}

	.btn-close:hover {
		color: var(--color-text-primary);
	}

	.panel-body {
		padding: 0 20px 20px;
		flex: 1;
		overflow-y: auto;
	}

	.section {
		padding: 16px 0;
		border-bottom: 1px solid var(--color-border);
	}

	.section:last-child {
		border-bottom: none;
	}

	.section-title {
		font-size: 13px;
		font-weight: 600;
		color: var(--color-text-primary);
		margin: 0 0 10px;
		text-transform: uppercase;
		letter-spacing: 0.02em;
	}

	/* Status badges */
	.status-badge {
		font-size: 11px;
		font-weight: 600;
		padding: 2px 8px;
		border-radius: var(--radius-chip);
	}

	.status-new {
		background-color: #dbeafe;
		color: #1e40af;
	}

	.status-preparing {
		background-color: #fef3c7;
		color: #92400e;
	}

	.status-ready {
		background-color: #dcfce7;
		color: #166534;
	}

	.status-completed {
		background-color: var(--color-surface);
		color: var(--color-text-secondary);
	}

	.status-cancelled {
		background-color: var(--color-error-bg);
		color: var(--color-error);
	}

	.type-badge {
		font-size: 11px;
		font-weight: 500;
		color: var(--color-text-secondary);
		background-color: var(--color-surface);
		padding: 2px 8px;
		border-radius: var(--radius-chip);
	}

	/* Kitchen status badges */
	.kitchen-badge {
		font-size: 10px;
		font-weight: 600;
		padding: 1px 6px;
		border-radius: 4px;
	}

	.kitchen-pending {
		background-color: #dbeafe;
		color: #1e40af;
	}

	.kitchen-preparing {
		background-color: #fef3c7;
		color: #92400e;
	}

	.kitchen-ready {
		background-color: #dcfce7;
		color: #166534;
	}

	/* Catering badges */
	.catering-badge {
		font-size: 11px;
		font-weight: 600;
		padding: 2px 8px;
		border-radius: var(--radius-chip);
	}

	.catering-booked {
		background-color: #dbeafe;
		color: #1e40af;
	}

	.catering-dp_paid {
		background-color: #fef3c7;
		color: #92400e;
	}

	.catering-settled {
		background-color: #dcfce7;
		color: #166534;
	}

	/* Payment method badges */
	.payment-method {
		font-size: 11px;
		font-weight: 600;
		padding: 2px 8px;
		border-radius: var(--radius-chip);
	}

	.method-cash {
		background-color: #dcfce7;
		color: #166534;
	}

	.method-qris {
		background-color: #dbeafe;
		color: #1e40af;
	}

	.method-transfer {
		background-color: #f3e8ff;
		color: #6b21a8;
	}

	/* Info grid */
	.info-grid {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	.info-item {
		display: flex;
		justify-content: space-between;
		align-items: center;
	}

	.info-label {
		font-size: 12px;
		color: var(--color-text-secondary);
	}

	.info-value {
		font-size: 13px;
		color: var(--color-text-primary);
		font-weight: 500;
	}

	.info-text {
		font-size: 13px;
		color: var(--color-text-primary);
		margin: 0;
	}

	/* Items */
	.items-list {
		display: flex;
		flex-direction: column;
		gap: 10px;
	}

	.item-row {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
		padding: 8px;
		background-color: var(--color-surface);
		border-radius: var(--radius-chip);
	}

	.item-main {
		display: flex;
		flex-direction: column;
		gap: 3px;
		flex: 1;
		min-width: 0;
	}

	.item-header {
		display: flex;
		align-items: center;
		gap: 6px;
		flex-wrap: wrap;
	}

	.item-qty {
		font-size: 13px;
		font-weight: 700;
		color: var(--color-text-primary);
		flex-shrink: 0;
	}

	.item-name {
		font-size: 13px;
		font-weight: 600;
		color: var(--color-text-primary);
	}

	.item-variant {
		font-size: 12px;
		color: var(--color-text-secondary);
		padding-left: 24px;
	}

	.item-modifiers {
		display: flex;
		flex-direction: column;
		gap: 2px;
		padding-left: 24px;
	}

	.modifier-tag {
		font-size: 11px;
		color: var(--color-text-secondary);
	}

	.item-notes {
		font-size: 11px;
		color: var(--color-text-secondary);
		font-style: italic;
		padding-left: 24px;
	}

	.item-price {
		display: flex;
		flex-direction: column;
		align-items: flex-end;
		flex-shrink: 0;
		margin-left: 12px;
	}

	.item-subtotal {
		font-size: 13px;
		font-weight: 600;
		color: var(--color-text-primary);
	}

	.item-discount {
		font-size: 11px;
		color: var(--color-error);
	}

	/* Summary */
	.summary-section {
		background-color: var(--color-surface);
		border-radius: var(--radius-chip);
		padding: 12px 16px !important;
		margin: 16px 0 0;
	}

	.summary-row {
		display: flex;
		justify-content: space-between;
		font-size: 13px;
		color: var(--color-text-primary);
		padding: 3px 0;
	}

	.summary-row.discount {
		color: var(--color-error);
	}

	.summary-row.total {
		font-weight: 700;
		font-size: 15px;
		border-top: 1px solid var(--color-border);
		padding-top: 8px;
		margin-top: 4px;
	}

	.summary-row.remaining {
		color: var(--color-error);
		font-weight: 600;
	}

	/* Payments */
	.payments-list {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	.payment-row {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
		padding: 8px;
		background-color: var(--color-surface);
		border-radius: var(--radius-chip);
	}

	.payment-info {
		display: flex;
		flex-direction: column;
		gap: 3px;
	}

	.payment-ref {
		font-size: 11px;
		color: var(--color-text-secondary);
	}

	.payment-time {
		font-size: 11px;
		color: var(--color-text-secondary);
	}

	.payment-amount {
		font-size: 14px;
		font-weight: 700;
		color: var(--color-text-primary);
	}

	.payment-summary {
		margin-top: 8px;
		padding-top: 8px;
		border-top: 1px solid var(--color-border);
	}

	/* Error banner */
	.error-banner {
		background-color: var(--color-error-bg);
		color: var(--color-error);
		font-size: 13px;
		font-weight: 500;
		padding: 8px 12px;
		border-radius: var(--radius-chip);
		margin-top: 12px;
	}

	.empty-text {
		font-size: 13px;
		color: var(--color-text-secondary);
		margin: 0;
	}

	/* Action buttons */
	.actions-section {
		display: flex;
		gap: 8px;
		border-bottom: none;
		padding-bottom: 0;
	}

	.btn-action {
		padding: 10px 20px;
		font-size: 14px;
		font-weight: 600;
		border: none;
		border-radius: var(--radius-btn);
		cursor: pointer;
		transition: background-color 0.15s ease;
	}

	.btn-action:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.btn-action.btn-primary {
		background-color: var(--color-primary);
		color: white;
	}

	.btn-action.btn-primary:hover:not(:disabled) {
		background-color: var(--color-primary-pressed);
	}

	.btn-cancel {
		background-color: var(--color-error-bg);
		color: var(--color-error);
	}

	.btn-cancel:hover:not(:disabled) {
		background-color: var(--color-error);
		color: white;
	}

	@media (max-width: 640px) {
		.detail-panel {
			width: 100vw;
		}
	}
</style>
