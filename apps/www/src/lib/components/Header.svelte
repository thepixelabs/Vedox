<script lang="ts">
	import { onMount } from 'svelte';
	import { nav, site } from '$lib/content';
	import { getTheme, toggleTheme, type Theme } from '$lib/theme';
	import { track } from '$lib/analytics';

	let theme = $state<Theme>('dark');

	onMount(() => {
		theme = getTheme();
	});

	function onToggle() {
		theme = toggleTheme();
		track('Theme Toggle', { to: theme });
	}
</script>

<header class="site-header">
	<div class="container inner">
		<a class="brand" href="#top" aria-label="Vedox home">
			<img class="logo-img" src="/pixelabs-icon.png" alt="" width="24" height="24" />
			<span>Vedox</span>
		</a>
		<nav aria-label="Section navigation">
			<ul>
				{#each nav.anchors as a (a.id)}
					<li><a href="#{a.id}">{a.label}</a></li>
				{/each}
			</ul>
		</nav>
		<div class="actions">
			<button
				type="button"
				class="icon-btn"
				onclick={onToggle}
				aria-label="Toggle color theme"
				title="Toggle color theme"
			>
				{#if theme === 'dark'}
					<svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true">
						<circle cx="12" cy="12" r="4" fill="currentColor" />
						<g stroke="currentColor" stroke-width="2" stroke-linecap="round">
							<line x1="12" y1="2" x2="12" y2="5" />
							<line x1="12" y1="19" x2="12" y2="22" />
							<line x1="2" y1="12" x2="5" y2="12" />
							<line x1="19" y1="12" x2="22" y2="12" />
							<line x1="4.9" y1="4.9" x2="6.8" y2="6.8" />
							<line x1="17.2" y1="17.2" x2="19.1" y2="19.1" />
							<line x1="4.9" y1="19.1" x2="6.8" y2="17.2" />
							<line x1="17.2" y1="6.8" x2="19.1" y2="4.9" />
						</g>
					</svg>
				{:else}
					<svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true">
						<path
							d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8Z"
							fill="currentColor"
						/>
					</svg>
				{/if}
			</button>
			<a
				class="gh"
				href={site.github}
				onclick={() => track('GitHub Click', { location: 'header' })}
				rel="noopener"
				aria-label="Vedox on GitHub"
			>
				<svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true">
					<path
						fill="currentColor"
						d="M12 .5a12 12 0 0 0-3.8 23.4c.6.1.8-.3.8-.6v-2.2c-3.3.7-4-1.6-4-1.6-.6-1.4-1.4-1.8-1.4-1.8-1.1-.8.1-.8.1-.8 1.3.1 2 1.3 2 1.3 1.1 2 3 1.4 3.7 1.1.1-.8.5-1.4.8-1.7-2.7-.3-5.4-1.3-5.4-6 0-1.3.5-2.4 1.3-3.2-.1-.3-.6-1.6.1-3.2 0 0 1-.3 3.3 1.2a11.5 11.5 0 0 1 6 0c2.3-1.5 3.3-1.2 3.3-1.2.7 1.6.2 2.9.1 3.2.8.8 1.3 1.9 1.3 3.2 0 4.7-2.7 5.7-5.4 6 .4.4.8 1.1.8 2.2v3.3c0 .3.2.7.8.6A12 12 0 0 0 12 .5Z"
					/>
				</svg>
				<span>GitHub</span>
			</a>
		</div>
	</div>
</header>

<style>
	.site-header {
		position: sticky;
		top: 0;
		z-index: 50;
		backdrop-filter: saturate(180%) blur(12px);
		-webkit-backdrop-filter: saturate(180%) blur(12px);
		background: color-mix(
			in srgb,
			var(--color-surface-base) 78%,
			transparent
		);
		border-bottom: 1px solid var(--color-border);
	}
	.inner {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: var(--space-6);
		height: 60px;
	}
	.brand {
		display: inline-flex;
		align-items: center;
		gap: var(--space-2);
		color: var(--color-text-primary);
		font-weight: 700;
		letter-spacing: -0.02em;
		font-size: var(--font-size-lg);
	}
	.brand:hover {
		text-decoration: none;
	}
	.logo-img {
		width: 24px;
		height: 24px;
		border-radius: var(--radius-sm);
	}
	nav ul {
		display: none;
		list-style: none;
		gap: var(--space-6);
	}
	nav a {
		color: var(--color-text-secondary);
		font-size: var(--font-size-sm);
	}
	nav a:hover {
		color: var(--color-text-primary);
		text-decoration: none;
	}
	.actions {
		display: flex;
		align-items: center;
		gap: var(--space-3);
	}
	.icon-btn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 34px;
		height: 34px;
		border-radius: var(--radius-md);
		background: transparent;
		color: var(--color-text-secondary);
		border: 1px solid var(--color-border);
		cursor: pointer;
	}
	.icon-btn:hover {
		color: var(--color-text-primary);
		border-color: var(--color-border-strong);
	}
	.gh {
		display: inline-flex;
		align-items: center;
		gap: var(--space-2);
		color: var(--color-text-secondary);
		font-size: var(--font-size-sm);
		padding: 6px 12px;
		border: 1px solid var(--color-border);
		border-radius: var(--radius-md);
	}
	.gh:hover {
		color: var(--color-text-primary);
		border-color: var(--color-border-strong);
		text-decoration: none;
	}
	@media (min-width: 760px) {
		nav ul {
			display: flex;
		}
	}
</style>
