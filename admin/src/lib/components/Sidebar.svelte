<script lang="ts">
	import { page } from '$app/state';
	import type { SessionUser } from '$lib/types/api';

	interface NavItem {
		label: string;
		href: string;
		icon: string;
		/** If set, only these roles can see this item */
		roles?: string[];
	}

	let { user }: { user: SessionUser } = $props();

	const navItems: NavItem[] = [
		{ label: 'Dashboard', href: '/', icon: '##' },
		{ label: 'Menu', href: '/menu', icon: '##' },
		{ label: 'Orders', href: '/orders', icon: '##' },
		{ label: 'Customers', href: '/customers', icon: '##' },
		{ label: 'Reports', href: '/reports', icon: '##', roles: ['OWNER', 'MANAGER'] },
		{ label: 'Settings', href: '/settings', icon: '##', roles: ['OWNER', 'MANAGER'] }
	];

	function isActive(href: string): boolean {
		if (href === '/') return page.url.pathname === '/';
		return page.url.pathname.startsWith(href);
	}

	function canSee(item: NavItem): boolean {
		if (!item.roles) return true;
		return item.roles.includes(user.role);
	}

	/** Get initials from full name (e.g. "John Doe" -> "JD") */
	function getInitials(name: string): string {
		return name
			.split(' ')
			.filter(Boolean)
			.map((part) => part[0])
			.join('')
			.toUpperCase()
			.slice(0, 2);
	}
</script>

<aside class="sidebar">
	<div class="sidebar-header">
		<div class="logo-mark">K</div>
		<span class="logo-text">Kiwari POS</span>
	</div>

	<nav class="sidebar-nav">
		{#each navItems as item}
			{#if canSee(item)}
				<a
					href={item.href}
					class="nav-link"
					class:active={isActive(item.href)}
				>
					<span class="nav-icon">{item.icon}</span>
					<span class="nav-label">{item.label}</span>
				</a>
			{/if}
		{/each}
	</nav>

	<div class="sidebar-footer">
		<div class="user-info">
			<div class="user-avatar">{getInitials(user.full_name)}</div>
			<div class="user-details">
				<span class="user-name">{user.full_name}</span>
				<span class="user-role">{user.role}</span>
			</div>
		</div>
		<a href="/logout" class="logout-link" data-sveltekit-preload-data="off">
			Logout
		</a>
	</div>
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
		flex: 1;
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

	.sidebar-footer {
		border-top: 1px solid var(--color-border);
		padding: 16px 20px;
		display: flex;
		flex-direction: column;
		gap: 12px;
	}

	.user-info {
		display: flex;
		align-items: center;
		gap: 10px;
	}

	.user-avatar {
		width: 32px;
		height: 32px;
		background-color: var(--color-surface);
		color: var(--color-text-secondary);
		border-radius: 50%;
		display: flex;
		align-items: center;
		justify-content: center;
		font-weight: 600;
		font-size: 11px;
		flex-shrink: 0;
	}

	.user-details {
		display: flex;
		flex-direction: column;
		min-width: 0;
	}

	.user-name {
		font-size: 13px;
		font-weight: 600;
		color: var(--color-text-primary);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.user-role {
		font-size: 11px;
		color: var(--color-text-secondary);
		text-transform: capitalize;
	}

	.logout-link {
		font-size: 13px;
		color: var(--color-text-secondary);
		text-decoration: none;
		font-weight: 500;
		transition: color 0.15s ease;
	}

	.logout-link:hover {
		color: var(--color-error);
	}
</style>
