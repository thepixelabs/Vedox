<script lang="ts">
  /**
   * /graph — doc reference graph page.
   *
   * Renders the full-viewport Cytoscape.js force-directed graph showing
   * which docs reference which. The graph is the primary surface — no
   * surrounding chrome, just the controls toolbar + canvas + legend.
   *
   * Layout:
   *   - Page header: title + refresh button + truncation warning
   *   - DocGraph component (flex: 1 — fills remaining height)
   *
   * The DocGraph component self-fetches from /api/graph on mount.
   * This page does not block on data load.
   *
   * Accessibility:
   *   - Landmark regions: <header>, <main> (provided by layout shell)
   *   - Skip link available from root layout
   *   - Canvas has role="application" + aria-label in DocGraph
   */

  import DocGraph from "$lib/components/graph/DocGraph.svelte";
</script>

<svelte:head>
  <title>Reference Graph — Vedox</title>
</svelte:head>

<div class="graph-page">
  <!-- ── Page header ────────────────────────────────────────────────────── -->
  <header class="graph-page__header">
    <div class="graph-page__title-row">
      <div class="graph-page__title-group">
        <svg
          class="graph-page__title-icon"
          width="16"
          height="16"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
          aria-hidden="true"
        >
          <circle cx="18" cy="5" r="3"/>
          <circle cx="6" cy="12" r="3"/>
          <circle cx="18" cy="19" r="3"/>
          <line x1="8.59" y1="13.51" x2="15.42" y2="17.49"/>
          <line x1="15.41" y1="6.51" x2="8.59" y2="10.49"/>
        </svg>
        <h1 class="graph-page__title">reference graph</h1>
      </div>

      <p class="graph-page__subtitle">
        click a node to open the doc &mdash; hover to trace connections
      </p>
    </div>
  </header>

  <!-- ── Graph canvas (fills remaining height) ──────────────────────────── -->
  <div class="graph-page__canvas-area">
    <DocGraph />
  </div>
</div>

<style>
  /* ── Page shell — fills main area from layout ─────────────────────────── */

  .graph-page {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
    overflow: hidden;
  }

  /* ── Header ───────────────────────────────────────────────────────────── */

  .graph-page__header {
    flex-shrink: 0;
    padding: var(--space-4) var(--space-6);
    border-bottom: 1px solid var(--color-border);
    background-color: var(--color-surface-elevated);
  }

  .graph-page__title-row {
    display: flex;
    align-items: center;
    gap: var(--space-6);
    flex-wrap: wrap;
  }

  .graph-page__title-group {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex-shrink: 0;
  }

  .graph-page__title-icon {
    color: var(--color-text-muted);
    flex-shrink: 0;
  }

  .graph-page__title {
    font-size: var(--text-xl, 22px);
    font-weight: 600;
    color: var(--color-text-primary);
    letter-spacing: var(--tracking-tight, -0.015em);
    margin: 0;
    font-family: var(--font-mono);
    line-height: 1;
  }

  .graph-page__subtitle {
    font-size: var(--font-size-sm, 12px);
    color: var(--color-text-muted);
    font-family: var(--font-mono);
    margin: 0;
  }

  /* ── Canvas area — fills remaining height ─────────────────────────────── */

  .graph-page__canvas-area {
    flex: 1;
    min-height: 0;
    overflow: hidden;
  }

  /* ── Responsive ───────────────────────────────────────────────────────── */

  @media (max-width: 640px) {
    .graph-page__header {
      padding: var(--space-3) var(--space-4);
    }

    .graph-page__title-row {
      gap: var(--space-3);
    }

    .graph-page__subtitle {
      display: none;
    }
  }
</style>
