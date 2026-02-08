<!--
  Hourly sales bar chart — pure CSS, no dependencies.
  X-axis: hours 6–22 (typical F&B operating hours).
  Each bar shows revenue; hover tooltip shows order count.
-->
<script lang="ts">
	import type { HourlySales } from '$lib/types/api';

	let { data }: { data: HourlySales[] } = $props();

	const START_HOUR = 6;
	const END_HOUR = 22;

	// Build a map from hour -> data for quick lookup
	function buildHourMap(data: HourlySales[]): Map<number, HourlySales> {
		const map = new Map<number, HourlySales>();
		for (const entry of data) {
			map.set(entry.hour, entry);
		}
		return map;
	}

	function formatCompact(amount: number): string {
		if (amount >= 1_000_000) return `${(amount / 1_000_000).toFixed(1)}jt`;
		if (amount >= 1_000) return `${(amount / 1_000).toFixed(0)}rb`;
		return amount.toString();
	}

	// Generate all hours in range, filling in zeroes for missing hours
	let hours = $derived.by(() => {
		const hourMap = buildHourMap(data);
		const result: Array<{
			hour: number;
			revenue: number;
			orderCount: number;
			label: string;
		}> = [];

		for (let h = START_HOUR; h <= END_HOUR; h++) {
			const entry = hourMap.get(h);
			result.push({
				hour: h,
				revenue: entry ? parseFloat(entry.total_revenue) : 0,
				orderCount: entry ? entry.order_count : 0,
				label: `${h.toString().padStart(2, '0')}:00`
			});
		}
		return result;
	});

	let maxRevenue = $derived(Math.max(...hours.map((h) => h.revenue), 1));
</script>

<div class="chart-container">
	<div class="chart-header">
		<h3 class="chart-title">Penjualan Per Jam</h3>
	</div>

	<div class="chart">
		<div class="bars">
			{#each hours as entry}
				<div class="bar-group" title="{entry.label} — {entry.orderCount} pesanan">
					<div class="bar-value">
						{#if entry.revenue > 0}
							{formatCompact(entry.revenue)}
						{/if}
					</div>
					<div class="bar-track">
						<div
							class="bar-fill"
							style="height: {(entry.revenue / maxRevenue) * 100}%"
						></div>
					</div>
					<div class="bar-label">{entry.hour}</div>
				</div>
			{/each}
		</div>
	</div>
</div>

<style>
	.chart-container {
		background-color: var(--color-bg);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-card);
		padding: 20px;
	}

	.chart-header {
		margin-bottom: 16px;
	}

	.chart-title {
		font-size: 15px;
		font-weight: 600;
		color: var(--color-text-primary);
		margin: 0;
	}

	.chart {
		overflow-x: auto;
	}

	.bars {
		display: flex;
		align-items: flex-end;
		gap: 4px;
		height: 200px;
		min-width: 0;
	}

	.bar-group {
		flex: 1;
		display: flex;
		flex-direction: column;
		align-items: center;
		min-width: 28px;
		height: 100%;
	}

	.bar-value {
		font-size: 10px;
		color: var(--color-text-secondary);
		white-space: nowrap;
		height: 16px;
		display: flex;
		align-items: flex-end;
	}

	.bar-track {
		flex: 1;
		width: 100%;
		max-width: 32px;
		display: flex;
		align-items: flex-end;
		padding: 0 2px;
	}

	.bar-fill {
		width: 100%;
		background-color: var(--color-primary);
		border-radius: 3px 3px 0 0;
		min-height: 0;
		transition: height 0.3s ease;
	}

	.bar-label {
		font-size: 11px;
		color: var(--color-text-secondary);
		margin-top: 6px;
		height: 16px;
	}
</style>
