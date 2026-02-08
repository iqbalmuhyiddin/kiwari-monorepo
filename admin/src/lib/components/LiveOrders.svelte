<!--
  Live active orders panel with polling.
  Fetches active orders from the SvelteKit server endpoint every 10 seconds.
  Shows order cards with status badges, order type chips, and time ago.
-->
<script lang="ts">
	import { onMount } from 'svelte';
	import type { ActiveOrder } from '$lib/types/api';
	import { formatRupiah } from '$lib/utils/format';

	let { initialOrders }: { initialOrders: ActiveOrder[] } = $props();

	// polledOrders starts null; once polling fetches data it takes over.
	let polledOrders = $state<ActiveOrder[] | null>(null);
	let error = $state<string | null>(null);

	let orders = $derived(polledOrders ?? initialOrders);

	const POLL_INTERVAL_MS = 10_000;

	function timeAgo(isoDate: string): string {
		const now = Date.now();
		const then = new Date(isoDate).getTime();
		const diffMs = now - then;
		const diffMin = Math.floor(diffMs / 60_000);

		if (diffMin < 1) return 'baru saja';
		if (diffMin < 60) return `${diffMin} menit lalu`;
		const diffHr = Math.floor(diffMin / 60);
		return `${diffHr} jam lalu`;
	}

	async function fetchOrders() {
		try {
			const res = await fetch('/api/orders/active');
			if (!res.ok) {
				error = `Gagal memuat pesanan (${res.status})`;
				return;
			}
			const data: ActiveOrder[] = await res.json();
			polledOrders = data;
			error = null;
		} catch {
			error = 'Tidak dapat terhubung ke server';
		}
	}

	onMount(() => {
		const interval = setInterval(fetchOrders, POLL_INTERVAL_MS);
		return () => clearInterval(interval);
	});
</script>

<div class="live-orders">
	<div class="panel-header">
		<h3 class="panel-title">Pesanan Aktif</h3>
		<span class="order-count">{orders.length}</span>
	</div>

	{#if error}
		<p class="error-text">{error}</p>
	{/if}

	{#if orders.length === 0}
		<p class="empty-text">Tidak ada pesanan aktif saat ini.</p>
	{:else}
		<div class="order-list">
			{#each orders as order (order.id)}
				<div class="order-card">
					<div class="order-top">
						<span class="order-number">{order.order_number}</span>
						<span class="order-type-chip">{order.order_type}</span>
					</div>
					<div class="order-middle">
						<span class="order-amount">{formatRupiah(order.total_amount)}</span>
						<span class="status-badge" class:status-new={order.status === 'NEW'} class:status-preparing={order.status === 'PREPARING'}>
							{order.status}
						</span>
					</div>
					<div class="order-bottom">
						<span class="order-items-count">
							{order.items?.length ?? 0} item
						</span>
						<span class="order-time">{timeAgo(order.created_at)}</span>
					</div>
				</div>
			{/each}
		</div>
	{/if}
</div>

<style>
	.live-orders {
		background-color: var(--color-bg);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-card);
		padding: 20px;
	}

	.panel-header {
		display: flex;
		align-items: center;
		gap: 8px;
		margin-bottom: 16px;
	}

	.panel-title {
		font-size: 15px;
		font-weight: 600;
		color: var(--color-text-primary);
		margin: 0;
	}

	.order-count {
		background-color: var(--color-surface);
		color: var(--color-text-secondary);
		font-size: 12px;
		font-weight: 600;
		padding: 2px 8px;
		border-radius: var(--radius-chip);
	}

	.error-text {
		color: var(--color-error);
		font-size: 13px;
		margin: 0 0 12px;
	}

	.empty-text {
		color: var(--color-text-secondary);
		font-size: 13px;
		margin: 0;
	}

	.order-list {
		display: flex;
		flex-direction: column;
		gap: 10px;
		max-height: 480px;
		overflow-y: auto;
	}

	.order-card {
		border: 1px solid var(--color-border);
		border-radius: var(--radius-chip);
		padding: 12px;
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	.order-top {
		display: flex;
		align-items: center;
		justify-content: space-between;
	}

	.order-number {
		font-size: 14px;
		font-weight: 700;
		color: var(--color-text-primary);
	}

	.order-type-chip {
		font-size: 11px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.02em;
		color: var(--color-text-secondary);
		background-color: var(--color-surface);
		padding: 2px 8px;
		border-radius: var(--radius-chip);
	}

	.order-middle {
		display: flex;
		align-items: center;
		justify-content: space-between;
	}

	.order-amount {
		font-size: 14px;
		font-weight: 700;
		color: var(--color-text-primary);
	}

	.status-badge {
		font-size: 11px;
		font-weight: 600;
		padding: 2px 8px;
		border-radius: var(--radius-chip);
	}

	.status-new {
		background-color: #fef9c3;
		color: #92400e;
	}

	.status-preparing {
		background-color: #dcfce7;
		color: #166534;
	}

	.order-bottom {
		display: flex;
		align-items: center;
		justify-content: space-between;
	}

	.order-items-count {
		font-size: 12px;
		color: var(--color-text-secondary);
	}

	.order-time {
		font-size: 12px;
		color: var(--color-text-secondary);
	}
</style>
