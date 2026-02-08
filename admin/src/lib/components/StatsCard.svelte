<!--
  Reusable KPI card for the dashboard.
  Shows a large value with a label beneath it.
-->
<script lang="ts">
	let { value, label }: { value: string; label: string } = $props();
</script>

<div class="stats-card">
	<span class="stats-value">{value}</span>
	<span class="stats-label">{label}</span>
</div>

<style>
	.stats-card {
		background-color: var(--color-bg);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-card);
		padding: 20px;
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.stats-value {
		font-size: 20px;
		font-weight: 700;
		color: var(--color-text-primary);
		line-height: 1.2;
	}

	.stats-label {
		font-size: 13px;
		color: var(--color-text-secondary);
		line-height: 1.4;
	}
</style>
