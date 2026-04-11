<script lang="ts">
	import { pitch } from '$lib/content';
</script>

<section id={pitch.id} class="pitch">
	<div class="container grid">
		<div class="copy">
			<p class="kicker">{pitch.kicker}</p>
			<h2>{pitch.title}</h2>
			<p class="body">{pitch.body}</p>
		</div>
		<div class="terminal" role="img" aria-label="Terminal running vedox dev">
			<div class="chrome">
				<span class="dot r"></span>
				<span class="dot y"></span>
				<span class="dot g"></span>
				<span class="tab">zsh</span>
			</div>
			<pre>
{#each pitch.terminal as line, i (i)}<span class={line.kind}>{#if line.kind === 'prompt'}<span class="arrow">›</span> {/if}{line.text}</span>
{/each}</pre>
		</div>
	</div>
</section>

<style>
	.pitch {
		padding-top: var(--mkt-section-pad);
	}
	.grid {
		display: grid;
		gap: var(--space-12);
		grid-template-columns: 1fr;
		align-items: center;
	}
	@media (min-width: 880px) {
		.grid {
			grid-template-columns: 1fr 1.1fr;
		}
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
		margin-bottom: var(--space-5);
		max-width: 18ch;
	}
	.body {
		font-size: var(--font-size-lg);
		max-width: 48ch;
	}
	.terminal {
		border-radius: var(--radius-xl);
		background: var(--color-surface-elevated);
		border: 1px solid var(--color-border);
		box-shadow:
			0 12px 40px rgba(0, 0, 0, 0.12),
			-20px 0 60px -20px color-mix(in srgb, var(--color-accent) 10%, transparent);
		overflow: hidden;
	}
	.chrome {
		display: flex;
		gap: var(--space-2);
		padding: 10px var(--space-4);
		background: var(--color-surface-overlay);
		border-bottom: 1px solid var(--color-border);
		align-items: center;
	}
	.dot {
		width: 10px;
		height: 10px;
		border-radius: 999px;
	}
	.dot.r {
		background: #ff5f57;
	}
	.dot.y {
		background: #febc2e;
	}
	.dot.g {
		background: #28c840;
	}
	.tab {
		margin-left: var(--space-4);
		font-family: var(--font-mono);
		font-size: var(--font-size-xs);
		color: var(--color-text-muted);
	}
	pre {
		padding: var(--space-6);
		font-family: var(--font-mono);
		font-size: var(--font-size-sm);
		line-height: 1.8;
		color: var(--color-text-primary);
		white-space: pre-wrap;
	}
	.prompt {
		color: var(--color-text-primary);
	}
	.arrow {
		color: var(--color-accent);
	}
	.output {
		color: var(--color-text-muted);
	}
</style>
