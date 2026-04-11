<script lang="ts">
	import { workflow } from '$lib/content';
</script>

<section id={workflow.id} class="workflow">
	<div class="container">
		<p class="kicker">{workflow.kicker}</p>
		<h2>{workflow.title}</h2>
		<ol>
			{#each workflow.steps as s (s.n)}
				<li>
					<span class="n">{s.n}</span>
					<div>
						<h3>{s.title}</h3>
						<p>{s.body}</p>
					</div>
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
		max-width: 22ch;
	}
	ol {
		list-style: none;
		display: grid;
		gap: var(--space-6);
		grid-template-columns: 1fr;
	}
	@media (min-width: 880px) {
		ol {
			grid-template-columns: repeat(3, 1fr);
		}
	}
	li {
		padding: var(--space-8);
		background: var(--color-surface-elevated);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-xl);
		position: relative;
	}
	@media (min-width: 880px) {
		li:not(:last-child)::after {
			content: '';
			position: absolute;
			top: 50%;
			right: calc(-1 * var(--space-6) / 2 - 0.5px);
			transform: translateY(-50%);
			width: var(--space-6);
			height: 2px;
			background: linear-gradient(to right, var(--color-accent), var(--color-border));
			z-index: 1;
		}
	}
	.n {
		display: inline-block;
		font-family: var(--font-mono);
		font-size: var(--font-size-xs);
		color: var(--color-accent);
		background: var(--color-accent-subtle);
		padding: 4px 10px;
		border-radius: 999px;
		margin-bottom: var(--space-5);
	}
	h3 {
		font-size: var(--font-size-xl);
		margin-bottom: var(--space-3);
	}
	li p {
		color: var(--color-text-secondary);
	}
</style>
