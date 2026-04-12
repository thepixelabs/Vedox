<script lang="ts">
	import { features } from '$lib/content';
	import { reveal } from '$lib/actions/reveal';

	const icons: Record<string, string> = {
		edit: 'M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7 M18.5 2.5a2.121 2.121 0 1 1 3 3L12 15l-4 1 1-4 9.5-9.5z',
		search: 'M11 3a8 8 0 1 0 0 16 8 8 0 0 0 0-16z M21 21l-4.35-4.35',
		palette: 'M12 2a10 10 0 0 0 0 20c.55 0 1-.45 1-1v-.5c0-.25.1-.5.28-.68a.94.94 0 0 1 .72-.32h1.5A4.5 4.5 0 0 0 20 15c0-5.52-3.58-10-8-10z M6.5 11.5a1.5 1.5 0 1 0 0 3 1.5 1.5 0 0 0 0-3z M9 7a1.5 1.5 0 1 0 0 3 1.5 1.5 0 0 0 0-3z M15 7a1.5 1.5 0 1 0 0 3 1.5 1.5 0 0 0 0-3z',
		bolt: 'M13 2L4 14h6l-1 8 9-12h-6z',
		terminal: 'M4 17l6-5-6-5 M12 19h8',
	};

	const accents: Record<string, string> = {
		edit: 'var(--color-accent)',
		search: 'var(--color-info)',
		palette: 'var(--color-warning)',
		bolt: 'var(--color-success)',
		terminal: 'var(--color-accent)',
	};
</script>

<section id={features.id} class="features">
	<div class="container">
		<p class="kicker" use:reveal>{features.kicker}</p>
		<h2 use:reveal={{ delay: 60 }}>{features.title}</h2>

		{#each features.groups as group (group.category)}
			<div class="group" style="--group-accent: {accents[group.icon] || 'var(--color-accent)'}">
				<header class="group-header">
					<div class="group-icon" aria-hidden="true">
						<svg viewBox="0 0 24 24" width="18" height="18">
							<path
								d={icons[group.icon]}
								fill="none"
								stroke="currentColor"
								stroke-width="1.8"
								stroke-linecap="round"
								stroke-linejoin="round"
							/>
						</svg>
					</div>
					<h3>{group.category}</h3>
					<span class="group-count">{group.items.length}</span>
				</header>

				<div class="items">
					{#each group.items as item (item.title)}
						<div class="item">
							<h4>{item.title}</h4>
							<p>{item.body}</p>
						</div>
					{/each}
				</div>
			</div>
		{/each}
	</div>
</section>

<style>
	.features {
		padding: var(--mkt-section-pad) 0;
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
		margin-bottom: var(--space-12);
		max-width: 20ch;
	}

	/* ---- Group ---- */
	.group {
		margin-bottom: var(--space-10);
		padding: var(--space-6) var(--space-8);
		background: var(--color-surface-elevated);
		border: 1px solid var(--color-border);
		border-left: 3px solid var(--group-accent, var(--color-accent));
		border-radius: var(--radius-xl);
	}
	.group:last-child {
		margin-bottom: 0;
	}
	.group-header {
		display: flex;
		align-items: center;
		gap: var(--space-3);
		margin-bottom: var(--space-5);
		padding-bottom: var(--space-4);
		border-bottom: 1px solid var(--color-border);
	}
	.group-icon {
		width: 32px;
		height: 32px;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		background: color-mix(in srgb, var(--group-accent) 12%, transparent);
		color: var(--group-accent);
		border-radius: var(--radius-md);
		flex-shrink: 0;
	}
	h3 {
		font-size: var(--font-size-lg);
		font-weight: 600;
		color: var(--color-text-primary);
		letter-spacing: -0.01em;
		flex: 1;
	}
	.group-count {
		font-family: var(--font-mono);
		font-size: var(--font-size-xs);
		color: var(--color-text-muted);
		background: var(--color-surface-overlay);
		padding: 2px 8px;
		border-radius: 999px;
	}

	/* ---- Items grid ---- */
	.items {
		display: grid;
		gap: var(--space-2);
		grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
	}

	/* ---- Individual item ---- */
	.item {
		padding: var(--space-4) var(--space-5);
		border-radius: var(--radius-md);
		border: 1px solid transparent;
		transition:
			border-color 160ms ease,
			background-color 160ms ease;
	}
	.item:hover {
		border-color: var(--color-border);
		background: var(--color-surface-overlay);
	}
	h4 {
		font-size: var(--font-size-sm);
		font-weight: 600;
		color: var(--color-text-primary);
		margin-bottom: var(--space-1);
		line-height: 1.4;
	}
	.item p {
		font-size: var(--font-size-sm);
		color: var(--color-text-secondary);
		line-height: 1.5;
	}

	/* Tighten grid on smaller screens */
	@media (max-width: 640px) {
		.group {
			padding: var(--space-5) var(--space-5);
		}
		.items {
			grid-template-columns: 1fr;
		}
	}
</style>
