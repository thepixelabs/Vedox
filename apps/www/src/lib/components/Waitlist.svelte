<script lang="ts">
	import { waitlist } from '$lib/content';
	import { track } from '$lib/analytics';
	import { env } from '$env/dynamic/public';

	const PUBLIC_WAITLIST_ENDPOINT = env.PUBLIC_WAITLIST_ENDPOINT;
	import InstallCommand from './InstallCommand.svelte';

	let email = $state('');
	let status = $state<'idle' | 'loading' | 'ok' | 'err' | 'disabled'>(
		PUBLIC_WAITLIST_ENDPOINT ? 'idle' : 'disabled'
	);

	async function onSubmit(e: SubmitEvent) {
		e.preventDefault();
		if (!PUBLIC_WAITLIST_ENDPOINT) {
			status = 'disabled';
			return;
		}
		if (!email || !email.includes('@')) {
			status = 'err';
			return;
		}
		status = 'loading';
		try {
			const res = await fetch(PUBLIC_WAITLIST_ENDPOINT, {
				method: 'POST',
				headers: { 'content-type': 'application/json' },
				body: JSON.stringify({ email, source: 'vedox.dev' }),
			});
			if (!res.ok) throw new Error(String(res.status));
			status = 'ok';
			track('Waitlist Submit');
			email = '';
		} catch {
			status = 'err';
		}
	}
</script>

<section id={waitlist.id} class="waitlist">
	<div class="container inner">
		<p class="kicker">{waitlist.kicker}</p>
		<h2>{waitlist.title}</h2>
		<p class="body">{waitlist.body}</p>

		<div class="install">
			<InstallCommand location="waitlist" />
		</div>

		<form onsubmit={onSubmit} novalidate>
			<label class="sr-only" for="email">Email</label>
			<input
				id="email"
				type="email"
				autocomplete="email"
				placeholder={waitlist.placeholder}
				bind:value={email}
				required
				disabled={status === 'disabled' || status === 'loading'}
			/>
			<button type="submit" disabled={status === 'disabled' || status === 'loading'}>
				{status === 'loading' ? 'Sending...' : waitlist.button}
			</button>
		</form>

		<p class="msg" class:ok={status === 'ok'} class:err={status === 'err'} aria-live="polite">
			{#if status === 'ok'}
				{waitlist.success}
			{:else if status === 'err'}
				{waitlist.failure}
			{:else if status === 'disabled'}
				{waitlist.disabled}
			{:else}
				&nbsp;
			{/if}
		</p>
	</div>
</section>

<style>
	.waitlist {
		text-align: center;
	}
	.inner {
		max-width: 680px;
	}
	.kicker {
		font-family: var(--font-mono);
		font-size: var(--font-size-xs);
		text-transform: uppercase;
		letter-spacing: 0.14em;
		color: var(--color-accent);
		margin-bottom: var(--space-4);
	}
	h2 {
		font-size: clamp(28px, 4vw, 44px);
		margin-bottom: var(--space-4);
	}
	.body {
		margin-bottom: var(--space-8);
	}
	.install {
		display: flex;
		justify-content: center;
		margin-bottom: var(--space-8);
	}
	form {
		display: flex;
		gap: var(--space-3);
		max-width: 480px;
		margin: 0 auto;
		flex-wrap: wrap;
	}
	input {
		flex: 1 1 220px;
		padding: 14px 16px;
		border-radius: var(--radius-lg);
		border: 1px solid var(--color-border);
		background: var(--color-surface-elevated);
		color: var(--color-text-primary);
		font-family: var(--font-sans);
		font-size: var(--font-size-base);
	}
	input:focus-visible {
		border-color: var(--color-accent);
	}
	button {
		padding: 14px 20px;
		border-radius: var(--radius-lg);
		border: 1px solid var(--color-accent);
		background: var(--color-accent);
		color: var(--color-text-inverse);
		font-weight: 600;
		font-size: var(--font-size-base);
		cursor: pointer;
		box-shadow: 0 0 20px color-mix(in srgb, var(--color-accent) 25%, transparent);
	}
	button:hover:not(:disabled) {
		background: var(--color-accent-hover);
		border-color: var(--color-accent-hover);
		box-shadow: 0 0 30px color-mix(in srgb, var(--color-accent) 35%, transparent);
	}
	button:disabled,
	input:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}
	.msg {
		margin-top: var(--space-4);
		font-size: var(--font-size-sm);
		color: var(--color-text-muted);
		min-height: 1.5em;
	}
	.msg.ok {
		color: var(--color-success);
	}
	.msg.err {
		color: var(--color-error);
	}
</style>
