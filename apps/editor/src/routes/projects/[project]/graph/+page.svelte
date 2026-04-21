<script lang="ts">
  /**
   * /projects/[project]/graph — per-project doc reference graph.
   *
   * The sibling +page.ts loader fetches the graph via api.getGraph(project)
   * and hands us a fully-typed payload. We pass `data={graph}` into DocGraph
   * so it skips its self-fetch path entirely — the loader owns I/O, the
   * component owns rendering.
   */

  import DocGraph from "$lib/components/graph/DocGraph.svelte";
  import type { PageData } from "./$types";

  let { data }: { data: PageData } = $props();
</script>

<svelte:head>
  <title>graph · {data.project} — Vedox</title>
</svelte:head>

<div class="graph-page">
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
          <circle cx="18" cy="5" r="3" />
          <circle cx="6" cy="12" r="3" />
          <circle cx="18" cy="19" r="3" />
          <line x1="8.59" y1="13.51" x2="15.42" y2="17.49" />
          <line x1="15.41" y1="6.51" x2="8.59" y2="10.49" />
        </svg>
        <h1 class="graph-page__title">graph · {data.project}</h1>
      </div>

      <p class="graph-page__subtitle">
        {data.graph.total_nodes} docs, {data.graph.total_edges} links &mdash; click a node to open
      </p>
    </div>
  </header>

  <div class="graph-page__canvas-area">
    <DocGraph data={data.graph} project={data.project} />
  </div>
</div>

<style>
  .graph-page {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
    overflow: hidden;
  }

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

  .graph-page__canvas-area {
    flex: 1;
    min-height: 0;
    overflow: hidden;
  }

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
