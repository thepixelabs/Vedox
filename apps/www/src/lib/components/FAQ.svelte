<script lang="ts">
	import { faq } from '$lib/content';
	import { trackFaqExpand } from '$lib/analytics';
</script>

<section id={faq.id} class="faq">
	<div class="container">
		<p class="kicker">{faq.kicker}</p>
		<h2>{faq.title}</h2>
		<div class="list">
			{#each faq.items as item, i (item.q)}
				<details open={i === 0} ontoggle={(e: Event) => { if ((e.target as HTMLDetailsElement).open) trackFaqExpand(item.q); }}>
					<summary>
						<span>{item.q}</span>
						<svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true">
							<path
								d="M6 9l6 6l6-6"
								fill="none"
								stroke="currentColor"
								stroke-width="2"
								stroke-linecap="round"
								stroke-linejoin="round"
							/>
						</svg>
					</summary>
					<p>{item.a}</p>
				</details>
			{/each}
		</div>
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
		margin-bottom: var(--space-10);
	}
	.list {
		max-width: 780px;
	}
	details {
		border-top: 1px solid var(--color-border);
		padding: var(--space-5) 0;
		border-left: 3px solid transparent;
		padding-left: var(--space-5);
		transition: border-left-color 200ms ease;
	}
	details[open] {
		border-left-color: var(--color-accent);
	}
	details:last-child {
		border-bottom: 1px solid var(--color-border);
	}
	summary {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: var(--space-4);
		cursor: pointer;
		list-style: none;
		font-weight: 600;
		color: var(--color-text-primary);
		font-size: var(--font-size-lg);
	}
	summary::-webkit-details-marker {
		display: none;
	}
	summary svg {
		color: var(--color-text-muted);
		transition: transform 180ms ease;
		flex-shrink: 0;
	}
	details[open] summary svg {
		transform: rotate(180deg);
	}
	details p {
		margin-top: var(--space-3);
		color: var(--color-text-secondary);
		max-width: 68ch;
	}
</style>
