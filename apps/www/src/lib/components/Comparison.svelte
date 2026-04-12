<script lang="ts">
	import { comparison } from '$lib/content';
	import { reveal } from '$lib/actions/reveal';

	type CellValue = boolean | 'partial' | 'n/a' | string;

	function cellType(val: CellValue): 'yes' | 'no' | 'partial' | 'na' | 'note' {
		if (val === true) return 'yes';
		if (val === false) return 'no';
		if (val === 'partial') return 'partial';
		if (val === 'n/a') return 'na';
		return 'note';
	}

	function cellLabel(val: CellValue): string {
		if (val === true) return 'Yes';
		if (val === false) return 'No';
		if (val === 'partial') return 'Partial';
		if (val === 'n/a') return 'N/A';
		return String(val);
	}
</script>

<section id={comparison.id} class="comparison">
	<div class="container">
		<p class="kicker" use:reveal>{comparison.kicker}</p>
		<h2 use:reveal={{ delay: 60 }}>{comparison.title}</h2>

		<div class="table-frame">
			<!-- svelte-ignore a11y_no_noninteractive_tabindex -->
			<div class="table-scroll" tabindex="0" role="region" aria-label="Feature comparison table — scroll horizontally to see all tools">
				<table>
					<thead>
						<tr>
							<th class="feature-col" scope="col">
								<span class="sr-only">Feature</span>
							</th>
							{#each comparison.tools as tool, i}
								<th scope="col" class:vedox={i === 0}>{tool}</th>
							{/each}
						</tr>
					</thead>
					<tbody>
						{#each comparison.rows as row, ri (row.feature)}
							<tr class:alt={ri % 2 === 1}>
								<td class="feature-col">{row.feature}</td>
								{#each row.values as val, vi}
									{@const type = cellType(val)}
									<td class="{type}" class:vedox={vi === 0}>
										{#if type === 'yes'}
											<svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true" class="icon-yes">
												<path
													d="M5 13l4 4L19 7"
													fill="none"
													stroke="currentColor"
													stroke-width="2.4"
													stroke-linecap="round"
													stroke-linejoin="round"
												/>
											</svg>
											<span class="sr-only">Yes</span>
										{:else if type === 'no'}
											<span class="dash" aria-hidden="true">&mdash;</span>
											<span class="sr-only">No</span>
										{:else if type === 'partial'}
											<span class="tilde" aria-hidden="true">~</span>
											<span class="sr-only">Partial</span>
										{:else if type === 'na'}
											<span class="dash" aria-hidden="true">&mdash;</span>
											<span class="sr-only">Not applicable</span>
										{:else}
											<span class="note-text">{cellLabel(val)}</span>
										{/if}
									</td>
								{/each}
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		</div>
	</div>
</section>

<style>
	.comparison {
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
		max-width: 22ch;
	}

	/* ---- Frame ---- */
	.table-frame {
		background: var(--color-surface-elevated);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-xl);
		overflow: hidden;
	}

	/* ---- Scroll wrapper ---- */
	.table-scroll {
		overflow-x: auto;
		-webkit-overflow-scrolling: touch;
	}
	/* Subtle scroll indicator shadow on the right edge */
	.table-scroll:focus-visible {
		outline: 2px solid var(--color-accent);
		outline-offset: -2px;
	}

	/* ---- Table ---- */
	table {
		width: 100%;
		min-width: 720px;
		border-collapse: collapse;
		table-layout: fixed;
	}

	/* ---- Column sizing ---- */
	.feature-col {
		width: 220px;
		min-width: 180px;
	}
	th:not(.feature-col),
	td:not(.feature-col) {
		width: auto;
		min-width: 100px;
	}

	/* ---- Header ---- */
	thead th {
		padding: var(--space-4) var(--space-5);
		font-size: var(--font-size-sm);
		font-weight: 600;
		color: var(--color-text-primary);
		text-align: center;
		border-bottom: 1px solid var(--color-border);
		white-space: nowrap;
		position: relative;
	}
	thead .feature-col {
		text-align: left;
	}
	thead th.vedox {
		color: var(--color-accent);
		position: relative;
	}
	thead th.vedox::after {
		content: '';
		position: absolute;
		bottom: 0;
		left: var(--space-3);
		right: var(--space-3);
		height: 2px;
		background: var(--color-accent);
		border-radius: 1px;
	}

	/* ---- Body cells ---- */
	td {
		padding: var(--space-3) var(--space-5);
		font-size: var(--font-size-sm);
		text-align: center;
		vertical-align: middle;
		color: var(--color-text-secondary);
		border-bottom: 1px solid var(--color-border);
	}
	tbody tr:last-child td {
		border-bottom: none;
	}
	td.feature-col {
		text-align: left;
		font-weight: 500;
		color: var(--color-text-primary);
		font-size: var(--font-size-sm);
		line-height: 1.4;
	}

	/* ---- Alternating rows ---- */
	tr.alt td {
		background: color-mix(in srgb, var(--color-surface-overlay) 30%, transparent);
	}

	/* ---- Vedox column highlight ---- */
	td.vedox,
	th.vedox {
		background: var(--color-accent-subtle);
	}
	tr.alt td.vedox {
		background: color-mix(in srgb, var(--color-accent-subtle) 80%, var(--color-surface-overlay));
	}

	/* ---- Value types ---- */
	.icon-yes {
		color: var(--color-success);
		display: inline-block;
		vertical-align: middle;
	}
	.dash {
		color: var(--color-text-muted);
		font-size: var(--font-size-base);
		opacity: 0.5;
	}
	.tilde {
		color: var(--color-warning);
		font-weight: 700;
		font-size: var(--font-size-lg);
	}
	.note-text {
		font-size: var(--font-size-xs);
		color: var(--color-text-muted);
		line-height: 1.35;
		display: inline-block;
		max-width: 16ch;
		text-align: center;
	}
	/* Notes in vedox column get accent styling */
	td.vedox .note-text {
		color: var(--color-text-secondary);
	}

	/* ---- Sticky first column on mobile ---- */
	@media (max-width: 880px) {
		.feature-col {
			position: sticky;
			left: 0;
			z-index: 2;
			background: var(--color-surface-elevated);
		}
		thead .feature-col {
			background: var(--color-surface-elevated);
		}
		tr.alt .feature-col {
			background: color-mix(in srgb, var(--color-surface-overlay) 30%, var(--color-surface-elevated));
		}
		/* Drop shadow to hint at horizontal scroll */
		.feature-col::after {
			content: '';
			position: absolute;
			top: 0;
			right: -8px;
			bottom: 0;
			width: 8px;
			background: linear-gradient(to right, rgba(0, 0, 0, 0.06), transparent);
			pointer-events: none;
		}
	}

	/* ---- Screen reader utility ---- */
	.sr-only {
		position: absolute;
		width: 1px;
		height: 1px;
		padding: 0;
		margin: -1px;
		overflow: hidden;
		clip: rect(0, 0, 0, 0);
		white-space: nowrap;
		border: 0;
	}
</style>
