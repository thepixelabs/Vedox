<script lang="ts">
	import { roadmap } from '$lib/content';
</script>

<section id={roadmap.id} class="roadmap">
	<div class="container">
		<p class="kicker">{roadmap.kicker}</p>
		<h2>{roadmap.title}</h2>
		<ol>
			{#each roadmap.items as item (item.phase)}
				<li class="item {item.status}">
					<div class="head">
						<span class="phase">{item.phase}</span>
						<span class="chip">
							{#if item.status === 'shipped'}
								<svg viewBox="0 0 24 24" width="14" height="14"
									><path
										d="M5 13l4 4l10-10"
										fill="none"
										stroke="currentColor"
										stroke-width="2.4"
										stroke-linecap="round"
										stroke-linejoin="round"
									/></svg
								>
								Shipped
							{:else if item.status === 'in-progress'}
								<span class="pulse"></span> In progress
							{:else}
								Planned
							{/if}
						</span>
					</div>
					<h3>{item.title}</h3>
					<p>{item.body}</p>
				</li>
			{/each}
		</ol>
	</div>
</section>

<style>
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
		margin-bottom: var(--space-12);
	}
	ol {
		list-style: none;
		display: grid;
		gap: var(--space-5);
		grid-template-columns: 1fr;
	}
	@media (min-width: 880px) {
		ol {
			grid-template-columns: repeat(3, 1fr);
		}
	}
	.item {
		padding: var(--space-8);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-xl);
		background: var(--color-surface-elevated);
	}
	.item.shipped {
		border-color: color-mix(
			in srgb,
			var(--color-success) 40%,
			var(--color-border)
		);
	}
	.item.in-progress {
		border-color: var(--color-accent);
	}
	.head {
		display: flex;
		align-items: center;
		justify-content: space-between;
		margin-bottom: var(--space-4);
	}
	.phase {
		font-family: var(--font-mono);
		font-size: var(--font-size-xs);
		text-transform: uppercase;
		letter-spacing: 0.12em;
		color: var(--color-text-muted);
	}
	.chip {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		font-size: var(--font-size-xs);
		padding: 4px 10px;
		border-radius: 999px;
		background: var(--color-surface-overlay);
		color: var(--color-text-secondary);
	}
	.shipped .chip {
		color: var(--color-success);
		background: color-mix(
			in srgb,
			var(--color-success) 15%,
			transparent
		);
	}
	.in-progress .chip {
		color: var(--color-accent);
		background: var(--color-accent-subtle);
	}
	.pulse {
		width: 8px;
		height: 8px;
		border-radius: 999px;
		background: var(--color-accent);
		box-shadow: 0 0 0 0 var(--color-accent);
		animation: pulse 2s infinite;
	}
	@keyframes pulse {
		0% {
			box-shadow: 0 0 0 0 color-mix(in srgb, var(--color-accent) 50%, transparent);
		}
		70% {
			box-shadow: 0 0 0 8px transparent;
		}
		100% {
			box-shadow: 0 0 0 0 transparent;
		}
	}
	h3 {
		font-size: var(--font-size-xl);
		margin-bottom: var(--space-3);
	}
	p {
		color: var(--color-text-secondary);
	}
</style>
