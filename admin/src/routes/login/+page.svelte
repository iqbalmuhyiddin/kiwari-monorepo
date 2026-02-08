<script lang="ts">
	import { enhance } from '$app/forms';

	let { form } = $props();
	let loading = $state(false);
</script>

<svelte:head>
	<title>Login - Kiwari POS Admin</title>
</svelte:head>

<div class="login-page">
	<div class="login-card">
		<!-- Logo mark -->
		<div class="logo-mark">K</div>

		<h1 class="login-heading">Welcome Back</h1>
		<p class="login-subtitle">Sign in to manage your outlet</p>

		{#if form?.error}
			<div class="error-message">{form.error}</div>
		{/if}

		<form
			method="POST"
			use:enhance={() => {
				loading = true;
				return async ({ update }) => {
					loading = false;
					await update();
				};
			}}
		>
			<div class="form-group">
				<label for="email" class="form-label">Email</label>
				<input
					id="email"
					name="email"
					type="email"
					autocomplete="email"
					value={form?.email ?? ''}
					class="input-field"
					placeholder="you@example.com"
					required
					disabled={loading}
				/>
			</div>

			<div class="form-group">
				<label for="password" class="form-label">Password</label>
				<input
					id="password"
					name="password"
					type="password"
					autocomplete="current-password"
					class="input-field"
					placeholder="Enter your password"
					required
					disabled={loading}
				/>
			</div>

			<button type="submit" class="login-btn btn-primary" disabled={loading}>
				{#if loading}
					<span class="spinner"></span>
					Signing in...
				{:else}
					Login to Admin
				{/if}
			</button>
		</form>
	</div>
</div>

<style>
	.login-page {
		min-height: 100vh;
		background-color: var(--color-primary);
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 24px;
	}

	.login-card {
		background-color: white;
		border-radius: var(--radius-card);
		padding: 40px 32px;
		width: 100%;
		max-width: 400px;
		text-align: center;
	}

	.logo-mark {
		width: 48px;
		height: 48px;
		background-color: var(--color-accent);
		color: var(--color-text-primary);
		border-radius: 10px;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		font-weight: 700;
		font-size: 1.25rem;
		margin-bottom: 20px;
	}

	.login-heading {
		font-size: 18px;
		font-weight: 700;
		color: var(--color-text-primary);
		margin: 0 0 4px 0;
	}

	.login-subtitle {
		font-size: 13px;
		color: var(--color-text-secondary);
		margin: 0 0 24px 0;
	}

	.error-message {
		background-color: var(--color-error-bg);
		color: var(--color-error);
		font-size: 13px;
		font-weight: 500;
		padding: 10px 12px;
		border-radius: var(--radius-chip);
		margin-bottom: 16px;
		text-align: left;
	}

	.form-group {
		margin-bottom: 16px;
		text-align: left;
	}

	.form-label {
		display: block;
		font-size: 13px;
		font-weight: 600;
		color: var(--color-text-primary);
		margin-bottom: 6px;
	}

	.form-group .input-field {
		width: 100%;
		box-sizing: border-box;
	}

	.login-btn {
		width: 100%;
		padding: 12px;
		font-size: 14px;
		font-weight: 700;
		margin-top: 8px;
		border: none;
		cursor: pointer;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		gap: 8px;
	}

	.spinner {
		width: 16px;
		height: 16px;
		border: 2px solid rgba(255, 255, 255, 0.3);
		border-top-color: white;
		border-radius: 50%;
		animation: spin 0.6s linear infinite;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}
</style>
