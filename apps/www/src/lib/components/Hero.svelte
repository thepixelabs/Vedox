<script lang="ts">
	import { hero, site } from '$lib/content';
	import InstallCommand from './InstallCommand.svelte';
	import { track } from '$lib/analytics';
</script>

<section id="top" class="hero">
	<div class="container hero-inner">
		<p class="eyebrow">{hero.eyebrow}</p>
		<h1>{hero.headline}</h1>
		<p class="sub">{hero.sub}</p>
		<div class="ctas">
			<InstallCommand location="hero" />
			<a
				class="secondary"
				href={hero.secondaryCta.href}
				rel="noopener"
				onclick={() => track('GitHub Click', { location: 'hero' })}
			>
				{hero.secondaryCta.label}
				<svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true">
					<path
						d="M7 17 L17 7 M9 7 H17 V15"
						fill="none"
						stroke="currentColor"
						stroke-width="2"
						stroke-linecap="round"
						stroke-linejoin="round"
					/>
				</svg>
			</a>
		</div>
		<p class="trust">{hero.trustLine}</p>

		<!--
			Hero concept C (static, deadline-safe) chosen by default per
			plan §4. This is a stylized editor screenshot rendered in pure
			SVG + HTML so it stays crisp, themeable, and zero-weight.
		-->
		<div class="screenshot-wrap" aria-hidden="true">
		<div class="screenshot">
			<div class="chrome">
				<span class="dot r"></span>
				<span class="dot y"></span>
				<span class="dot g"></span>
				<span class="tab">README.md</span>
			</div>
			<div class="body">
				<aside class="files">
					<p class="f-title">docs</p>
					<ul>
						<li class="active">README.md</li>
						<li>getting-started.md</li>
						<li>architecture.md</li>
						<li>contributing.md</li>
						<li>adr/001-...md</li>
					</ul>
				</aside>
				<div class="doc">
					<h3>Vedox</h3>
					<p>Documentation that lives in your repo.</p>
					<p class="muted">A local-first docs CMS for developers.</p>
					<pre><span class="kw">$</span> {site.runCommand}</pre>
					<p>Edit. Save. <code>git commit</code>. That's the loop.</p>
				</div>
			</div>
		</div>
		</div>
	</div>
</section>

<style>
	.hero {
		padding-top: clamp(56px, 8vw, 100px);
		padding-bottom: clamp(48px, 7vw, 80px);
	}
	.hero-inner {
		text-align: center;
	}
	.eyebrow {
		display: inline-block;
		font-family: var(--font-mono);
		font-size: var(--font-size-xs);
		letter-spacing: 0.14em;
		text-transform: uppercase;
		color: var(--color-accent);
		background: var(--color-accent-subtle);
		padding: 6px 12px;
		border-radius: 999px;
		margin-bottom: var(--space-6);
	}
	h1 {
		font-size: var(--mkt-display);
		line-height: var(--mkt-display-line);
		max-width: 18ch;
		margin: 0 auto var(--space-6);
	}
	.sub {
		font-size: clamp(16px, 2vw, 20px);
		max-width: 60ch;
		margin: 0 auto var(--space-8);
		color: var(--color-text-secondary);
	}
	.ctas {
		display: flex;
		flex-wrap: wrap;
		gap: var(--space-4);
		justify-content: center;
		align-items: center;
		margin-bottom: var(--space-6);
	}
	.secondary {
		display: inline-flex;
		align-items: center;
		gap: var(--space-2);
		padding: 14px 18px;
		border-radius: var(--radius-lg);
		border: 1px solid var(--color-border-strong);
		color: var(--color-text-primary);
		font-weight: 500;
	}
	.secondary:hover {
		border-color: var(--color-accent);
		text-decoration: none;
	}
	.trust {
		font-size: var(--font-size-sm);
		color: var(--color-text-muted);
		margin-bottom: var(--space-12);
	}
	.screenshot-wrap {
		position: relative;
		margin: 0 auto;
		max-width: 960px;
	}
	.screenshot-wrap::before {
		content: '';
		position: absolute;
		inset: -40px -60px;
		background: radial-gradient(
			ellipse at 50% 50%,
			color-mix(in srgb, var(--color-accent) 15%, transparent),
			transparent 70%
		);
		border-radius: 50%;
		z-index: -1;
		pointer-events: none;
	}
	.screenshot {
		border: 1px solid var(--color-border);
		border-radius: var(--radius-xl);
		overflow: hidden;
		background: var(--color-surface-elevated);
		box-shadow:
			0 16px 48px color-mix(in srgb, var(--color-accent) 12%, transparent),
			0 4px 12px rgba(0, 0, 0, 0.08);
		text-align: left;
	}
	.chrome {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding: 10px var(--space-4);
		background: var(--color-surface-overlay);
		border-bottom: 1px solid var(--color-border);
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
	.body {
		display: grid;
		grid-template-columns: 200px 1fr;
		min-height: 320px;
	}
	.files {
		background: var(--color-surface-base);
		border-right: 1px solid var(--color-border);
		padding: var(--space-4);
	}
	.f-title {
		font-family: var(--font-mono);
		font-size: var(--font-size-xs);
		color: var(--color-text-muted);
		text-transform: uppercase;
		letter-spacing: 0.1em;
		margin-bottom: var(--space-3);
	}
	.files ul {
		list-style: none;
		display: flex;
		flex-direction: column;
		gap: 2px;
	}
	.files li {
		font-family: var(--font-mono);
		font-size: var(--font-size-xs);
		color: var(--color-text-secondary);
		padding: 6px 10px;
		border-radius: var(--radius-sm);
	}
	.files li.active {
		background: var(--color-accent-subtle);
		color: var(--color-accent);
		border-left: 2px solid var(--color-accent);
		padding-left: 8px;
	}
	.doc {
		padding: var(--space-8);
	}
	.doc h3 {
		font-size: 28px;
		margin-bottom: var(--space-3);
	}
	.doc p {
		margin-bottom: var(--space-3);
	}
	.doc .muted {
		color: var(--color-text-muted);
	}
	.doc pre {
		background: var(--color-surface-base);
		border: 1px solid var(--color-border);
		padding: var(--space-3) var(--space-4);
		border-radius: var(--radius-md);
		font-family: var(--font-mono);
		font-size: var(--font-size-sm);
		margin: var(--space-4) 0;
		overflow-x: auto;
	}
	.doc .kw {
		color: var(--color-accent);
		margin-right: 8px;
	}
	@media (max-width: 640px) {
		.body {
			grid-template-columns: 1fr;
		}
		.files {
			display: none;
		}
	}
</style>
