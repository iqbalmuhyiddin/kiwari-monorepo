<script lang="ts">
	import { page } from '$app/state';

	interface NavItem {
		label: string;
		href: string;
		icon: string;
	}

	const navItems: NavItem[] = [
		{ label: 'Dashboard', href: '/', icon: '##' },
		{ label: 'Menu', href: '/menu', icon: '##' },
		{ label: 'Orders', href: '/orders', icon: '##' },
		{ label: 'Customers', href: '/customers', icon: '##' },
		{ label: 'Reports', href: '/reports', icon: '##' },
		{ label: 'Settings', href: '/settings', icon: '##' }
	];

	function isActive(href: string): boolean {
		if (href === '/') return page.url.pathname === '/';
		return page.url.pathname.startsWith(href);
	}
</script>

<aside class="sidebar">
	<div class="sidebar-header">
		<div class="logo-mark">K</div>
		<span class="logo-text">Kiwari POS</span>
	</div>

	<nav class="sidebar-nav">
		{#each navItems as item}
			<a
				href={item.href}
				class="nav-link"
				class:active={isActive(item.href)}
			>
				<span class="nav-icon">{item.icon}</span>
				<span class="nav-label">{item.label}</span>
			</a>
		{/each}
	</nav>
</aside>

<style>
	.sidebar {
		width: 240px;
		min-height: 100vh;
		background-color: var(--color-bg);
		border-right: 1px solid var(--color-border);
		display: flex;
		flex-direction: column;
		flex-shrink: 0;
	}

	.sidebar-header {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 24px 20px;
		border-bottom: 1px solid var(--color-border);
	}

	.logo-mark {
		width: 36px;
		height: 36px;
		background-color: var(--color-primary);
		color: white;
		border-radius: var(--radius-chip);
		display: flex;
		align-items: center;
		justify-content: center;
		font-weight: 700;
		font-size: 1rem;
	}

	.logo-text {
		font-weight: 700;
		font-size: var(--text-heading);
		color: var(--color-text-primary);
	}

	.sidebar-nav {
		display: flex;
		flex-direction: column;
		padding: 12px 8px;
		gap: 2px;
	}

	.nav-link {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 10px 12px;
		border-radius: var(--radius-chip);
		text-decoration: none;
		color: var(--color-text-secondary);
		font-size: var(--text-body);
		font-weight: 500;
		transition:
			background-color 0.15s ease,
			color 0.15s ease;
	}

	.nav-link:hover {
		background-color: var(--color-surface);
		color: var(--color-text-primary);
	}

	.nav-link.active {
		background-color: var(--color-surface);
		color: var(--color-primary);
		font-weight: 600;
	}

	.nav-icon {
		width: 20px;
		text-align: center;
		font-size: 1rem;
	}

	.nav-label {
		line-height: 1;
	}
</style>
