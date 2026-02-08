<script lang="ts">
	import Sidebar from '$lib/components/Sidebar.svelte';
	import { auth } from '$lib/stores/auth';

	let { data, children } = $props();

	// Sync server-provided user data to client-side auth store (reactive)
	$effect(() => {
		auth.setUser(data.user);
	});
</script>

<div class="app-shell">
	<Sidebar user={data.user} />
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
