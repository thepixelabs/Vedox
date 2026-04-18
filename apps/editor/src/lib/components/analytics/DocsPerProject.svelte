<script lang="ts">
  /**
   * DocsPerProject.svelte — CSS-only horizontal bar chart.
   *
   * Renders one bar per project, sorted descending by doc count.
   * No chart library. Bars are sized via inline `width` percentages
   * derived from the max doc count.
   *
   * Future (v2.1): two-tone stacked bar (agent vs. human).
   *
   * Props:
   *   data    — record of { [projectName]: docCount }
   *   loading — show skeleton rows
   */

  interface Props {
    data: Record<string, number>;
    loading?: boolean;
  }

  let { data, loading = false }: Props = $props();

  interface Row {
    project: string;
    count: number;
    pct: number;
  }

  const rows = $derived.by((): Row[] => {
    const entries = Object.entries(data).map(([project, count]) => ({ project, count }));
    entries.sort((a, b) => b.count - a.count);
    const max = entries[0]?.count ?? 1;
    return entries.map((e) => ({
      ...e,
      pct: Math.round((e.count / max) * 100),
    }));
  });

  const isEmpty = $derived(!loading && rows.length === 0);
</script>

<section class="dpp" aria-labelledby="dpp-heading">
  <header class="dpp__header">
    <h2 class="dpp__heading" id="dpp-heading">docs per project</h2>
    {#if !loading && rows.length > 0}
      <span class="dpp__count">{rows.length} project{rows.length === 1 ? "" : "s"}</span>
    {/if}
  </header>

  {#if loading}
    <ul class="dpp__list" aria-busy="true" aria-label="Loading docs per project">
      {#each { length: 4 } as _, i (i)}
        <li class="dpp__row dpp__row--skeleton">
          <span class="dpp__skeleton dpp__skeleton--label" aria-hidden="true"></span>
          <span class="dpp__track">
            <span
              class="dpp__bar dpp__bar--skeleton"
              style="width: {[75, 55, 35, 20][i]}%"
              aria-hidden="true"
            ></span>
          </span>
          <span class="dpp__skeleton dpp__skeleton--count" aria-hidden="true"></span>
        </li>
      {/each}
    </ul>
  {:else if isEmpty}
    <p class="dpp__empty">no projects indexed yet.</p>
  {:else}
    <ul class="dpp__list" aria-label="Doc count per project">
      {#each rows as row (row.project)}
        <li class="dpp__row">
          <span class="dpp__label" title={row.project}>{row.project}</span>
          <div class="dpp__track" role="presentation">
            <div
              class="dpp__bar"
              style="width: {row.pct}%"
              aria-hidden="true"
            ></div>
          </div>
          <span class="dpp__count-val" aria-label="{row.count} docs">{row.count}</span>
        </li>
      {/each}
    </ul>
  {/if}
</section>

<style>
  /* ── Section ───────────────────────────────────────────────────────────── */

  .dpp {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .dpp__header {
    display: flex;
    align-items: baseline;
    gap: var(--space-3);
  }

  .dpp__heading {
    font-size: var(--text-2xs, 11px);
    font-weight: 600;
    letter-spacing: var(--tracking-wider, 0.06em);
    text-transform: uppercase;
    color: var(--color-text-muted);
    font-family: var(--font-mono);
    margin: 0;
  }

  .dpp__count {
    font-size: var(--font-size-sm, 12px);
    color: var(--color-text-muted);
    font-family: var(--font-mono);
  }

  /* ── List ──────────────────────────────────────────────────────────────── */

  .dpp__list {
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    padding: 0;
    margin: 0;
  }

  .dpp__row {
    display: grid;
    grid-template-columns: 140px 1fr 36px;
    align-items: center;
    gap: var(--space-3);
  }

  @media (max-width: 480px) {
    .dpp__row {
      grid-template-columns: 100px 1fr 28px;
    }
  }

  /* ── Label ─────────────────────────────────────────────────────────────── */

  .dpp__label {
    font-size: var(--font-size-sm, 12px);
    font-family: var(--font-mono);
    color: var(--color-text-secondary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    text-align: right;
  }

  /* ── Track + bar ───────────────────────────────────────────────────────── */

  .dpp__track {
    position: relative;
    height: 8px;
    background-color: var(--color-surface-overlay);
    border-radius: var(--radius-full);
    overflow: hidden;
  }

  .dpp__bar {
    position: absolute;
    inset: 0 auto 0 0;
    background-color: var(--color-accent);
    border-radius: var(--radius-full);
    transition: width 400ms var(--ease-out);
  }

  /* ── Count value ───────────────────────────────────────────────────────── */

  .dpp__count-val {
    font-size: var(--font-size-sm, 12px);
    font-family: var(--font-mono);
    color: var(--color-text-muted);
    font-variant-numeric: tabular-nums;
    text-align: right;
  }

  /* ── Empty state ───────────────────────────────────────────────────────── */

  .dpp__empty {
    font-size: var(--font-size-sm, 12px);
    font-family: var(--font-mono);
    color: var(--color-text-muted);
    padding: var(--space-4) 0;
  }

  /* ── Skeleton rows ─────────────────────────────────────────────────────── */

  .dpp__skeleton {
    display: block;
    background-color: var(--color-surface-overlay);
    border-radius: var(--radius-sm);
    animation: dpp-shimmer 1.4s ease-in-out infinite;
  }

  .dpp__skeleton--label {
    width: 80px;
    height: 12px;
    margin-left: auto;
  }

  .dpp__skeleton--count {
    width: 24px;
    height: 12px;
  }

  .dpp__bar--skeleton {
    background-color: var(--color-surface-overlay);
    animation: dpp-shimmer 1.4s ease-in-out infinite;
  }

  @keyframes dpp-shimmer {
    0%   { opacity: 0.5; }
    50%  { opacity: 0.9; }
    100% { opacity: 0.5; }
  }

  /* ── Reduced motion ────────────────────────────────────────────────────── */

  @media (prefers-reduced-motion: reduce) {
    .dpp__bar {
      transition: none;
    }

    .dpp__skeleton,
    .dpp__bar--skeleton {
      animation: none;
      opacity: 0.6;
    }
  }
</style>
