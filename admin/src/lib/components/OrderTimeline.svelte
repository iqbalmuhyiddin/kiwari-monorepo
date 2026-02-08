<!--
  Order status timeline — visual lifecycle from Created to Completed/Cancelled.
  Highlights the current status step and shows timestamps where available.
-->
<script lang="ts">
	import type { OrderStatus } from '$lib/types/api';

	interface Props {
		status: OrderStatus;
		createdAt: string;
		completedAt: string | null;
	}

	let { status, createdAt, completedAt }: Props = $props();

	const steps: { key: OrderStatus; label: string }[] = [
		{ key: 'NEW', label: 'Baru' },
		{ key: 'PREPARING', label: 'Diproses' },
		{ key: 'READY', label: 'Siap' },
		{ key: 'COMPLETED', label: 'Selesai' }
	];

	const statusIndex: Record<OrderStatus, number> = {
		NEW: 0,
		PREPARING: 1,
		READY: 2,
		COMPLETED: 3,
		CANCELLED: -1
	};

	let currentIndex = $derived(statusIndex[status]);
	let isCancelled = $derived(status === 'CANCELLED');

	function formatTimestamp(iso: string): string {
		const d = new Date(iso);
		return d.toLocaleString('id-ID', {
			day: 'numeric',
			month: 'short',
			hour: '2-digit',
			minute: '2-digit',
			timeZone: 'Asia/Jakarta'
		});
	}

	function getStepTimestamp(index: number): string | null {
		if (index === 0) return formatTimestamp(createdAt);
		if (index === 3 && completedAt) return formatTimestamp(completedAt);
		return null;
	}
</script>

<div class="timeline" class:cancelled={isCancelled}>
	{#if isCancelled}
		<div class="cancelled-banner">
			<span class="cancelled-label">Dibatalkan</span>
			<span class="cancelled-time">{formatTimestamp(createdAt)}</span>
		</div>
	{:else}
		<div class="steps">
			{#each steps as step, i (step.key)}
				<div class="step" class:done={i < currentIndex} class:active={i === currentIndex} class:pending={i > currentIndex}>
					<div class="step-dot">
						{#if i < currentIndex}
							<span class="dot-check">&#10003;</span>
						{:else if i === currentIndex}
							<span class="dot-current"></span>
						{:else}
							<span class="dot-empty"></span>
						{/if}
					</div>
					<div class="step-info">
						<span class="step-label">{step.label}</span>
						{#if getStepTimestamp(i)}
							<span class="step-time">{getStepTimestamp(i)}</span>
						{/if}
					</div>
					{#if i < steps.length - 1}
						<div class="step-line" class:line-done={i < currentIndex}></div>
					{/if}
				</div>
			{/each}
		</div>
	{/if}
</div>

<style>
	.timeline {
		padding: 12px 0;
	}

	.cancelled-banner {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 8px 12px;
		background-color: var(--color-error-bg);
		border-radius: var(--radius-chip);
	}

	.cancelled-label {
		font-size: 13px;
		font-weight: 600;
		color: var(--color-error);
	}

	.cancelled-time {
		font-size: 12px;
		color: var(--color-text-secondary);
	}

	.steps {
		display: flex;
		align-items: flex-start;
		gap: 0;
	}

	.step {
		display: flex;
		flex-direction: column;
		align-items: center;
		position: relative;
		flex: 1;
	}

	.step-dot {
		width: 24px;
		height: 24px;
		border-radius: 50%;
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 1;
		flex-shrink: 0;
	}

	.step.done .step-dot {
		background-color: var(--color-primary);
	}

	.step.active .step-dot {
		background-color: var(--color-primary);
		box-shadow: 0 0 0 4px color-mix(in srgb, var(--color-primary) 20%, transparent);
	}

	.step.pending .step-dot {
		background-color: var(--color-border);
	}

	.dot-check {
		color: white;
		font-size: 12px;
		font-weight: 700;
		line-height: 1;
	}

	.dot-current {
		width: 8px;
		height: 8px;
		background-color: white;
		border-radius: 50%;
	}

	.dot-empty {
		width: 8px;
		height: 8px;
		background-color: var(--color-bg);
		border-radius: 50%;
	}

	.step-info {
		display: flex;
		flex-direction: column;
		align-items: center;
		margin-top: 6px;
	}

	.step-label {
		font-size: 11px;
		font-weight: 600;
		color: var(--color-text-secondary);
	}

	.step.done .step-label,
	.step.active .step-label {
		color: var(--color-text-primary);
	}

	.step-time {
		font-size: 10px;
		color: var(--color-text-secondary);
		margin-top: 2px;
	}

	.step-line {
		position: absolute;
		top: 12px;
		left: calc(50% + 12px);
		right: calc(-50% + 12px);
		height: 2px;
		background-color: var(--color-border);
	}

	.step-line.line-done {
		background-color: var(--color-primary);
	}
</style>
