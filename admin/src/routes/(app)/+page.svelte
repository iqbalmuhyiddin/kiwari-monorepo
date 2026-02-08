<!--
  Dashboard page — KPI cards, hourly sales chart, and live active orders.
  Data loaded server-side from Go API; active orders poll every 10 seconds.
-->
<script lang="ts">
	import StatsCard from '$lib/components/StatsCard.svelte';
	import HourlySalesChart from '$lib/components/HourlySalesChart.svelte';
	import LiveOrders from '$lib/components/LiveOrders.svelte';
	import { formatRupiah } from '$lib/utils/format';

	let { data } = $props();

	function formatDate(dateStr: string): string {
		const d = new Date(dateStr + 'T00:00:00');
		return d.toLocaleDateString('id-ID', { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' });
	}

	let todaySales = $derived(data.dailySales[0] ?? null);
	let revenue = $derived(todaySales ? formatRupiah(todaySales.net_revenue) : 'Rp 0');
	let orderCount = $derived(todaySales ? todaySales.order_count : 0);
	let avgTicket = $derived(
		todaySales && todaySales.order_count > 0
			? formatRupiah(parseFloat(todaySales.net_revenue) / todaySales.order_count)
			: 'Rp 0'
	);

	let paymentBreakdown = $derived(
		data.paymentSummary.length > 0
			? data.paymentSummary.map((p) => `${p.payment_method}: ${p.transaction_count}`).join(', ')
			: '-'
	);
</script>

<svelte:head>
	<title>Dashboard - Kiwari POS</title>
</svelte:head>

<div class="dashboard">
	<div class="page-header">
		<h1 class="page-title">Dashboard</h1>
		<p class="page-subtitle">{formatDate(data.today)}</p>
	</div>

	<!-- KPI Cards -->
	<div class="kpi-grid">
		<StatsCard value={revenue} label="Pendapatan Hari Ini" />
		<StatsCard value={String(orderCount)} label="Jumlah Pesanan" />
		<StatsCard value={avgTicket} label="Rata-rata per Pesanan" />
		<StatsCard value={paymentBreakdown} label="Metode Pembayaran" />
	</div>

	<!-- Chart + Live Orders -->
	<div class="content-grid">
		<div class="chart-section">
			<HourlySalesChart data={data.hourlySales} />
		</div>
		<div class="orders-section">
			<LiveOrders initialOrders={data.activeOrders} />
		</div>
	</div>
</div>

<style>
	.dashboard {
		max-width: 1200px;
	}

	.page-header {
		margin-bottom: 24px;
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
		margin: 4px 0 0;
	}

	.kpi-grid {
		display: grid;
		grid-template-columns: repeat(4, 1fr);
		gap: 16px;
		margin-bottom: 24px;
	}

	.content-grid {
		display: grid;
		grid-template-columns: 1fr 380px;
		gap: 16px;
	}

	/* Responsive: stack on smaller screens */
	@media (max-width: 1024px) {
		.kpi-grid {
			grid-template-columns: repeat(2, 1fr);
		}

		.content-grid {
			grid-template-columns: 1fr;
		}
	}

	@media (max-width: 640px) {
		.kpi-grid {
			grid-template-columns: 1fr;
		}
	}
</style>
