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

	// Tier 1 hero items per category (by title)
	const heroItems: Record<string, string> = {
		Editor: 'Dual-mode WYSIWYG',
		'Navigation & search': 'Command palette',
		'Design system': '5 curated themes',
		'AI & agents': 'Agent-safe API',
		'Developer experience': 'Locked down by default',
	};

	// Tier 2 standard items per category
	const tier2Items: Record<string, string[]> = {
		Editor: ['Round-trip fidelity', 'Mermaid diagrams', 'Shiki syntax highlighting'],
		'Navigation & search': ['Full-text search', 'Multi-pane layout'],
		'Design system': ['3 density modes', 'Variable fonts'],
		'AI & agents': ['AI review queue', 'Provider config drawer'],
		'Developer experience': ['Single binary', 'Zero config start', 'Workspace scanner'],
	};

	// Status badges (sparse — only 3)
	const statusBadges: Record<string, { label: string; variant: 'success' | 'warning' }> = {
		'AI review queue': { label: 'shipped', variant: 'success' },
		'Provider config drawer': { label: 'shipped', variant: 'success' },
		'Agent-safe API': { label: 'beta', variant: 'warning' },
	};

	// Theme swatches for "5 curated themes"
	const themeSwatches = [
		{ name: 'Graphite', color: '#3a3d45' },
		{ name: 'Eclipse', color: '#1a1a2e' },
		{ name: 'Ember', color: '#c2410c' },
		{ name: 'Paper', color: '#f5f0e8' },
		{ name: 'Solar', color: '#fbbf24' },
	];

	// Security checklist for "Locked down by default"
	const securityChecks = [
		'loopback-only',
		'path traversal protection',
		'secret file blocklist',
		'DOMPurify',
		'CSP headers',
	];

	let activeGroupIndex = $state(0);
	let panelVisible = $state(true);
	let switching = $state(false);

	const prefersReduced =
		typeof window !== 'undefined'
			? window.matchMedia('(prefers-reduced-motion: reduce)').matches
			: false;

	function switchGroup(i: number) {
		if (i === activeGroupIndex) return;

		if (prefersReduced) {
			activeGroupIndex = i;
			return;
		}

		switching = true;
		panelVisible = false;

		setTimeout(() => {
			activeGroupIndex = i;
			switching = false;
			requestAnimationFrame(() => {
				panelVisible = true;
			});
		}, 180);
	}

	function onTabKeydown(e: KeyboardEvent) {
		const groups = features.groups;
		const idx = activeGroupIndex;
		let next = idx;
		if (e.key === 'ArrowRight') next = (idx + 1) % groups.length;
		else if (e.key === 'ArrowLeft') next = (idx - 1 + groups.length) % groups.length;
		else return;
		e.preventDefault();
		switchGroup(next);
		const btn = (e.currentTarget as HTMLElement)?.querySelector<HTMLButtonElement>(
			`[data-group-index="${next}"]`
		);
		btn?.focus();
	}

	function mdInline(text: string): string {
		return text.replace(/`([^`]+)`/g, '<code>$1</code>');
	}

</script>

<section id={features.id} class="features">
	<!-- Faint aurora ellipse — bottom-center, desktop only -->
	<svg class="aurora" aria-hidden="true">
		<defs>
			<filter id="fg-aurora-blur"><feGaussianBlur stdDeviation="65"/></filter>
		</defs>
		<ellipse class="fg-aurora-e1" cx="50%" cy="90%" rx="60%" ry="35%" fill="#818cf8" opacity="0.09" filter="url(#fg-aurora-blur)"/>
	</svg>

	<div class="container">
		<p class="kicker" use:reveal>{features.kicker}</p>
		<h2 use:reveal={{ delay: 60 }}>{features.title}</h2>
		<hr class="heading-rule" aria-hidden="true" />

		<!-- Tab bar -->
		<div
			class="tab-bar"
			role="tablist"
			aria-label="Feature categories"
			tabindex="-1"
			use:reveal={{ delay: 180 }}
			onkeydown={onTabKeydown}
		>
			{#each features.groups as group, i (group.category)}
				<button
					type="button"
					role="tab"
					class="tab-btn"
					class:active={activeGroupIndex === i}
					aria-selected={activeGroupIndex === i}
					aria-controls="feature-panel"
					data-group-index={i}
					tabindex={activeGroupIndex === i ? 0 : -1}
					style="--tab-accent: {accents[group.icon] || 'var(--color-accent)'}"
					onclick={() => switchGroup(i)}
				>
					<svg viewBox="0 0 24 24" width="14" height="14" aria-hidden="true" class="tab-icon">
						<path
							d={icons[group.icon]}
							fill="none"
							stroke="currentColor"
							stroke-width="1.8"
							stroke-linecap="round"
							stroke-linejoin="round"
						/>
					</svg>
					<span class="tab-name">{group.category}</span>
					<span class="tab-count">{group.items.length}</span>
				</button>
			{/each}
		</div>

		<!-- Panel -->
		<div
			id="feature-panel"
			role="tabpanel"
			class="panel"
			class:panel-visible={panelVisible}
			class:panel-switching={switching}
			use:reveal={{ delay: 280 }}
			style="--group-accent: {accents[features.groups[activeGroupIndex]?.icon] || 'var(--color-accent)'}"
		>
			{#each features.groups as group (group.category)}
				{@const isActive = features.groups[activeGroupIndex]?.category === group.category}
				{@const heroTitle = heroItems[group.category]}
				{@const heroItem = group.items.find(it => it.title === heroTitle)}
				{@const tier2Titles = tier2Items[group.category] ?? []}
				{@const tier2List = group.items.filter(it => tier2Titles.includes(it.title))}
				{@const tier3List = group.items.filter(it => it.title !== heroTitle && !tier2Titles.includes(it.title))}

				{#if isActive}
					<div class="tiered-grid">
						<!-- Tier 1 hero card -->
						{#if heroItem}
							<article
								class="item tier-1"
								class:panel-item-animate={panelVisible && !switching}
								style="--stagger: 0; --card-accent: {accents[group.icon] || 'var(--color-accent)'}"
							>
								<div class="tier1-header">
									<!-- svelte-ignore a11y_no_noninteractive_tabindex -->
									<h4 tabindex="0">
										{heroItem.title}
										<!-- Per-item enhancements for hero titles -->
										{#if heroItem.title === 'Agent-safe API'}
											<span class="badge badge-warning">beta</span>
										{/if}
									</h4>
									{#if heroItem.title === 'Command palette'}
										<div class="cmd-pills">
											<kbd class="kbd-pill">Cmd K</kbd>
											<span class="mode-pill">search</span>
											<span class="mode-pill">&gt; commands</span>
											<span class="mode-pill"># tags</span>
											<span class="mode-pill">/ path</span>
										</div>
									{/if}
									{#if heroItem.title === 'Dual-mode WYSIWYG'}
										<span class="mode-toggle-tag">wysiwyg <span class="mode-arrow">&harr;</span> source</span>
									{/if}
									{#if heroItem.title === '5 curated themes'}
										<div class="swatches" aria-label="Theme color swatches">
											{#each themeSwatches as swatch}
												<span
													class="swatch"
													style="background: {swatch.color}"
													title={swatch.name}
													aria-label={swatch.name}
												></span>
											{/each}
										</div>
									{/if}
								</div>
								{#if heroItem.title === 'Locked down by default'}
									<ul class="security-checklist" aria-label="Security features">
										{#each securityChecks as check}
											<li><span class="check-mark" aria-hidden="true">&#10003;</span><code class="security-code">{check}</code></li>
										{/each}
									</ul>
								{:else}
									<p>{@html mdInline(heroItem.body)}</p>
								{/if}
							</article>
						{/if}

						<!-- Tier 2 standard cards -->
						{#each tier2List as item, i}
							<article
								class="item tier-2"
								class:panel-item-animate={panelVisible && !switching}
								style="--stagger: {i + 1}; --card-accent: {accents[group.icon] || 'var(--color-accent)'}"
							>
								<div class="tier2-header">
									<!-- svelte-ignore a11y_no_noninteractive_tabindex -->
									<h4 tabindex="0">{item.title}</h4>
									{#if statusBadges[item.title]}
										{@const badge = statusBadges[item.title]}
										<span
											class="badge"
											class:badge-success={badge.variant === 'success'}
											class:badge-warning={badge.variant === 'warning'}
										>{badge.label}</span>
									{/if}
								</div>
								<p>{@html mdInline(item.body)}</p>
							</article>
						{/each}

						<!-- Tier 3 compact rows -->
						{#if tier3List.length > 0}
							<div class="tier3-zone">
								{#each tier3List as item}
									<div
										class="tier3-row"
										style="--card-accent: {accents[group.icon] || 'var(--color-accent)'}"
									>
										<span class="tier3-title">{item.title}</span>
										<span class="tier3-body">{@html mdInline(item.body)}</span>
									</div>
								{/each}
							</div>
						{/if}
					</div>
				{/if}
			{/each}
		</div>
	</div>
</section>

<style>
	.features {
		padding: var(--mkt-section-pad) 0;
		position: relative;
		overflow: hidden;
	}

	/* ---- Aurora (desktop only) ---- */
	.aurora {
		position: absolute;
		inset: -10% -5%;
		width: 110%;
		height: 120%;
		pointer-events: none;
		z-index: 0;
	}
	.fg-aurora-e1 {
		animation: fg-aurora-drift 28s ease-in-out infinite alternate;
		animation-delay: -3s;
	}
	@keyframes fg-aurora-drift {
		from { transform: translate(0, 0) scale(1); }
		to   { transform: translate(2%, -1%) scale(1.03); }
	}
	@media (prefers-reduced-motion: reduce) {
		.fg-aurora-e1 {
			animation: none;
			transform: translate(1%, -0.5%) scale(1.015);
		}
	}
	@media (max-width: 1023px) {
		.aurora { display: none; }
	}

	.container {
		position: relative;
		z-index: 1;
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
		margin-bottom: var(--space-8);
		max-width: 20ch;
	}

	/* ---- Heading rule ---- */
	.heading-rule {
		border: none;
		border-top: 1px solid var(--color-border);
		margin: 0 0 var(--space-8) 0;
		padding: 0;
	}

	/* ---- Tab bar ---- */
	.tab-bar {
		display: flex;
		gap: 0;
		border-bottom: 1px solid var(--color-border);
		margin-bottom: var(--space-8);
		overflow-x: auto;
		-webkit-overflow-scrolling: touch;
		scrollbar-width: none;
	}
	.tab-bar::-webkit-scrollbar { display: none; }

	.tab-btn {
		display: inline-flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-3) var(--space-4);
		background: transparent;
		border: none;
		border-bottom: 2px solid transparent;
		cursor: pointer;
		font-family: var(--font-mono);
		font-size: var(--font-size-xs);
		text-transform: uppercase;
		letter-spacing: 0.1em;
		color: var(--color-text-muted);
		white-space: nowrap;
		flex-shrink: 0;
		transition: color 150ms ease, border-color 150ms ease;
		margin-bottom: -1px;
	}
	.tab-btn:hover { color: var(--color-text-secondary); }
	.tab-btn.active {
		color: var(--tab-accent, var(--color-accent));
		border-bottom-color: var(--tab-accent, var(--color-accent));
	}
	.tab-icon {
		color: currentColor;
		flex-shrink: 0;
	}
	.tab-count {
		font-family: var(--font-mono);
		font-size: 10px;
		font-variant-numeric: tabular-nums;
		color: var(--color-text-muted);
		background: var(--color-surface-overlay);
		padding: 1px 6px;
		border-radius: 999px;
	}
	.tab-btn.active .tab-count {
		color: var(--tab-accent, var(--color-accent));
		background: color-mix(in srgb, var(--tab-accent, var(--color-accent)) 12%, transparent);
	}

	/* ---- Panel transitions ---- */
	.panel {
		opacity: 0;
		min-height: 560px;
		transition: opacity 260ms cubic-bezier(0.16, 1, 0.3, 1);
	}
	.panel-visible { opacity: 1; }
	.panel-switching {
		opacity: 0;
		transition: opacity 180ms ease-in;
	}
	@media (prefers-reduced-motion: reduce) {
		.panel { opacity: 1; transition: opacity 100ms ease; }
		.panel-switching { opacity: 1; transition: opacity 100ms ease; }
	}

	/* ---- Tiered grid ---- */
	.tiered-grid {
		display: grid;
		gap: var(--space-3);
		grid-template-columns: repeat(3, 1fr);
	}

	/* ---- Item cards (shared) ---- */
	.item {
		border-radius: var(--radius-md);
		border: 1px solid var(--color-border);
		border-left: 3px solid var(--card-accent, var(--color-accent));
		background: var(--color-surface-elevated);
		cursor: default;
		opacity: 0;
		transform: translateY(8px);
		transition:
			border-color 200ms ease,
			background-color 200ms ease,
			transform 200ms ease;
	}
	.panel-item-animate {
		animation: item-enter 260ms cubic-bezier(0.16, 1, 0.3, 1) both;
		animation-delay: calc(var(--stagger) * 30ms + 180ms);
		animation-fill-mode: forwards;
	}
	@keyframes item-enter {
		from { opacity: 0; transform: translateY(8px); }
		to   { opacity: 1; transform: translateY(0); }
	}
	.item:hover,
	.item:focus-within {
		background: var(--color-surface-overlay);
		transform: translateY(-1px);
	}
	.item:focus-visible {
		outline: 2px solid var(--group-accent, var(--color-accent));
		outline-offset: 2px;
	}
	@media (prefers-reduced-motion: reduce) {
		.item {
			opacity: 1;
			transform: none;
			animation: none !important;
		}
	}

	/* ---- Tier 1 hero card ---- */
	.tier-1 {
		grid-column: span 2;
		padding: var(--space-8) var(--space-8);
	}
	.tier1-header {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: var(--space-3);
		margin-bottom: var(--space-4);
	}
	.tier-1 h4 {
		font-size: var(--font-size-lg);
		font-weight: 600;
		color: var(--color-text-primary);
		line-height: 1.3;
		display: flex;
		align-items: center;
		gap: var(--space-2);
		margin: 0;
	}
	.tier-1 p {
		font-size: var(--font-size-sm);
		color: var(--color-text-secondary);
		line-height: 1.65;
	}

	/* ---- Cmd+K pills ---- */
	.cmd-pills {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: var(--space-2);
	}
	.kbd-pill {
		font-family: var(--font-mono);
		font-size: 10px;
		font-variant-numeric: tabular-nums;
		padding: 3px 8px;
		border: 1px solid var(--color-border);
		border-radius: 4px;
		background: var(--color-surface-overlay);
		color: var(--color-text-primary);
		letter-spacing: 0.04em;
	}
	.mode-pill {
		font-family: var(--font-mono);
		font-size: 10px;
		font-variant-numeric: tabular-nums;
		padding: 3px 8px;
		border: 1px solid var(--color-border);
		border-radius: 999px;
		background: color-mix(in srgb, var(--group-accent, var(--color-accent)) 6%, transparent);
		color: var(--color-text-muted);
	}

	/* ---- Mode toggle tag (WYSIWYG card) ---- */
	.mode-toggle-tag {
		font-family: var(--font-mono);
		font-size: 11px;
		padding: 3px 10px;
		border: 1px solid var(--color-border);
		border-radius: 999px;
		background: color-mix(in srgb, #818cf8 10%, transparent);
		color: #818cf8;
		white-space: nowrap;
	}
	.mode-arrow {
		color: #818cf8;
	}

	/* ---- Theme swatches ---- */
	.swatches {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		margin-top: var(--space-1);
	}
	.swatch {
		display: inline-block;
		width: 14px;
		height: 14px;
		border-radius: 50%;
		border: 1px solid color-mix(in srgb, currentColor 20%, transparent);
		flex-shrink: 0;
	}

	/* ---- Security checklist ---- */
	.security-checklist {
		list-style: none;
		display: flex;
		flex-direction: column;
		gap: var(--space-2);
		margin: 0;
		padding: 0;
	}
	.security-checklist li {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		font-size: var(--font-size-sm);
		color: var(--color-text-secondary);
	}
	.check-mark {
		color: var(--color-success);
		font-size: 13px;
		flex-shrink: 0;
	}
	.security-code {
		font-family: var(--font-mono);
		font-size: 0.85em;
		padding: 1px 6px;
		background: color-mix(in srgb, var(--color-accent) 8%, transparent);
		border: 1px solid var(--color-border);
		border-radius: 3px;
		color: var(--color-text-primary);
	}

	/* ---- Status badges ---- */
	.badge {
		font-family: var(--font-mono);
		font-size: 10px;
		font-variant-numeric: tabular-nums;
		padding: 2px 8px;
		border-radius: 999px;
		line-height: 1.4;
		flex-shrink: 0;
	}
	.badge-success {
		color: var(--color-success);
		background: color-mix(in srgb, var(--color-success) 14%, transparent);
		border: 1px solid color-mix(in srgb, var(--color-success) 25%, transparent);
	}
	.badge-warning {
		color: var(--color-warning);
		background: color-mix(in srgb, var(--color-warning) 14%, transparent);
		border: 1px solid color-mix(in srgb, var(--color-warning) 25%, transparent);
	}

	/* ---- Tier 2 standard card ---- */
	.tier-2 {
		padding: var(--space-5) var(--space-6);
	}
	.tier2-header {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		margin-bottom: var(--space-2);
		flex-wrap: wrap;
	}
	.tier-2 h4 {
		font-size: var(--font-size-base);
		font-weight: 600;
		color: var(--color-text-primary);
		line-height: 1.35;
		margin: 0;
	}
	.tier-2 p {
		font-size: var(--font-size-sm);
		color: var(--color-text-secondary);
		line-height: 1.65;
	}

	/* ---- Inline code in body text (shared across tiers) ---- */
	.tier-1 p :global(code),
	.tier-2 p :global(code) {
		font-family: var(--font-mono);
		font-size: 0.85em;
		padding: 1px 6px;
		background: color-mix(in srgb, var(--color-accent) 8%, transparent);
		border: 1px solid var(--color-border);
		border-radius: 3px;
		color: var(--color-text-primary);
	}

	/* ---- Tier 3 compact zone ---- */
	.tier3-zone {
		grid-column: 1 / -1;
		display: grid;
		grid-template-columns: repeat(3, 1fr);
		gap: var(--space-2);
	}
	.tier3-row {
		display: flex;
		flex-direction: column;
		gap: 2px;
		padding: var(--space-3) var(--space-4);
		border-left: 1px solid var(--card-accent, var(--color-accent));
		opacity: 0.85;
	}
	.tier3-title {
		font-size: var(--font-size-xs);
		font-weight: 600;
		color: var(--color-text-primary);
		line-height: 1.3;
	}
	.tier3-body {
		font-size: var(--font-size-xs);
		color: var(--color-text-muted);
		line-height: 1.5;
	}
	.tier3-body :global(code) {
		font-family: var(--font-mono);
		font-size: 0.85em;
		padding: 1px 5px;
		background: color-mix(in srgb, var(--color-accent) 8%, transparent);
		border: 1px solid var(--color-border);
		border-radius: 3px;
		color: var(--color-text-primary);
	}

	/* ---- Responsive ---- */
	/* iPad 768–1023px: 2-col grid, Tier 1 spans full width */
	@media (min-width: 768px) and (max-width: 1023px) {
		.tiered-grid {
			grid-template-columns: repeat(2, 1fr);
		}
		.tier-1 {
			grid-column: 1 / -1;
		}
		.tier3-zone {
			grid-template-columns: repeat(2, 1fr);
		}
	}

	/* Mobile <768px: single column */
	@media (max-width: 767px) {
		.tiered-grid {
			grid-template-columns: 1fr;
		}
		.tier-1 {
			grid-column: span 1;
		}
		.tier3-zone {
			grid-template-columns: 1fr;
		}
		/* Tier 3 mobile: plain left-bordered rows, no card chrome */
		.tier3-row {
			background: none;
			border-radius: 0;
			border-top: none;
			border-right: none;
			border-bottom: none;
		}
	}
</style>
