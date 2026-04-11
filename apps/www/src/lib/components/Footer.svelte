<script lang="ts">
	import { footer, site } from '$lib/content';
	import { track } from '$lib/analytics';
</script>

<footer class="site-footer">
	<div class="container inner">
		<div class="brand">
			<img class="logo-img" src="/pixelabs-icon.png" alt="" width="18" height="18" />
			<span>{site.name}</span>
		</div>
		<p class="copy">{footer.copy}</p>
		<ul>
			{#each footer.links as l (l.label)}
				<li>
					<a
						href={l.href}
						rel="noopener"
						onclick={() => track('Footer Link', { label: l.label })}>{l.label}</a
					>
				</li>
			{/each}
		</ul>
	</div>
</footer>

<style>
	.site-footer {
		border-top: none;
		padding: var(--space-10) 0;
		margin-top: var(--space-20);
		position: relative;
	}
	.site-footer::before {
		content: '';
		position: absolute;
		top: 0;
		left: var(--mkt-gutter);
		right: var(--mkt-gutter);
		height: 1px;
		background: linear-gradient(to right, transparent, var(--color-accent), transparent);
	}
	.inner {
		display: flex;
		flex-wrap: wrap;
		gap: var(--space-6);
		align-items: center;
		justify-content: space-between;
	}
	.brand {
		display: inline-flex;
		align-items: center;
		gap: var(--space-2);
		color: var(--color-text-primary);
		font-weight: 700;
	}
	.logo-img {
		width: 18px;
		height: 18px;
		border-radius: 3px;
	}
	.copy {
		font-size: var(--font-size-sm);
		color: var(--color-text-muted);
	}
	ul {
		list-style: none;
		display: flex;
		gap: var(--space-5);
	}
	ul a {
		color: var(--color-text-secondary);
		font-size: var(--font-size-sm);
	}
	ul a:hover {
		color: var(--color-text-primary);
		text-decoration: none;
	}
</style>
