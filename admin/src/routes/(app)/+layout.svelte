<script lang="ts">
	import Sidebar from '$lib/components/Sidebar.svelte';

	let { children } = $props();
</script>

<div class="app-shell">
	<Sidebar />
	<main class="app-content">
		{@render children()}
	</main>
</div>

<style>
	.app-shell {
		display: flex;
		min-height: 100vh;
	}

	.app-content {
		flex: 1;
		padding: 24px;
		background-color: var(--color-bg);
		overflow-y: auto;
	}
</style>
