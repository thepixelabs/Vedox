<script lang="ts">
  /**
   * GraphControls.svelte — toolbar for the doc reference graph view.
   *
   * Provides:
   *   - Zoom in / zoom out buttons
   *   - Fit-to-view button (resets viewport to show all nodes)
   *   - Layout toggle: cose-bilkent (force-directed) ↔ breadthfirst (tree)
   *   - Filter panel: hide broken links, filter by doc type
   *   - Node count / edge count badge (read-only)
   *
   * All interaction is communicated via props + callbacks — this component
   * holds no internal graph state.
   *
   * Keyboard: every button has a meaningful aria-label; focus-visible ring
   * uses --color-accent for WCAG 2.2 AA.
   */

  interface FilterState {
    showBroken: boolean;
    docTypes: Set<string>;
  }

  interface Props {
    /** Current layout algorithm identifier. */
    layout: "cose-bilkent" | "breadthfirst";
    /** Total visible nodes in the current graph render. */
    nodeCount: number;
    /** Total visible edges in the current graph render. */
    edgeCount: number;
    /** Available doc type labels for the filter chips. */
    availableTypes: string[];
    /** Current filter state. */
    filters: FilterState;
    /** Called when user requests zoom in. */
    onZoomIn: () => void;
    /** Called when user requests zoom out. */
    onZoomOut: () => void;
    /** Called when user requests fit-to-view. */
    onFit: () => void;
    /** Called when user toggles the layout algorithm. */
    onLayoutToggle: () => void;
    /** Called when filter state changes. */
    onFiltersChange: (next: FilterState) => void;
  }

  let {
    layout,
    nodeCount,
    edgeCount,
    availableTypes,
    filters,
    onZoomIn,
    onZoomOut,
    onFit,
    onLayoutToggle,
    onFiltersChange,
  }: Props = $props();

  function toggleType(type: string) {
    const next = new Set(filters.docTypes);
    if (next.has(type)) {
      next.delete(type);
    } else {
      next.add(type);
    }
    onFiltersChange({ ...filters, docTypes: next });
  }

  function toggleBroken() {
    onFiltersChange({ ...filters, showBroken: !filters.showBroken });
  }

  const layoutLabel = $derived(
    layout === "cose-bilkent" ? "force" : "tree"
  );
  const layoutNextLabel = $derived(
    layout === "cose-bilkent" ? "switch to tree layout" : "switch to force layout"
  );
</script>

<div class="graph-controls" role="toolbar" aria-label="Graph view controls">
  <!-- ── Zoom / fit cluster ──────────────────────────────────────────────── -->
  <div class="graph-controls__group" aria-label="Zoom controls">
    <button
      class="graph-controls__btn"
      type="button"
      aria-label="Zoom in"
      title="Zoom in"
      onclick={onZoomIn}
    >
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        <circle cx="11" cy="11" r="8"/>
        <line x1="21" y1="21" x2="16.65" y2="16.65"/>
        <line x1="11" y1="8" x2="11" y2="14"/>
        <line x1="8" y1="11" x2="14" y2="11"/>
      </svg>
    </button>

    <button
      class="graph-controls__btn"
      type="button"
      aria-label="Zoom out"
      title="Zoom out"
      onclick={onZoomOut}
    >
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        <circle cx="11" cy="11" r="8"/>
        <line x1="21" y1="21" x2="16.65" y2="16.65"/>
        <line x1="8" y1="11" x2="14" y2="11"/>
      </svg>
    </button>

    <button
      class="graph-controls__btn"
      type="button"
      aria-label="Fit all nodes in view"
      title="Fit to view"
      onclick={onFit}
    >
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        <polyline points="15 3 21 3 21 9"/>
        <polyline points="9 21 3 21 3 15"/>
        <line x1="21" y1="3" x2="14" y2="10"/>
        <line x1="3" y1="21" x2="10" y2="14"/>
      </svg>
    </button>
  </div>

  <div class="graph-controls__divider" role="separator" aria-hidden="true"></div>

  <!-- ── Layout toggle ──────────────────────────────────────────────────── -->
  <div class="graph-controls__group" aria-label="Layout controls">
    <button
      class="graph-controls__btn graph-controls__btn--label"
      type="button"
      aria-label={layoutNextLabel}
      title={layoutNextLabel}
      onclick={onLayoutToggle}
      aria-pressed={layout === "breadthfirst"}
    >
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        {#if layout === "cose-bilkent"}
          <!-- Force/scatter icon -->
          <circle cx="12" cy="12" r="2"/>
          <line x1="12" y1="2" x2="12" y2="6"/>
          <line x1="12" y1="18" x2="12" y2="22"/>
          <line x1="2" y1="12" x2="6" y2="12"/>
          <line x1="18" y1="12" x2="22" y2="12"/>
          <line x1="4.93" y1="4.93" x2="7.76" y2="7.76"/>
          <line x1="16.24" y1="16.24" x2="19.07" y2="19.07"/>
          <line x1="19.07" y1="4.93" x2="16.24" y2="7.76"/>
          <line x1="7.76" y1="16.24" x2="4.93" y2="19.07"/>
        {:else}
          <!-- Tree icon -->
          <rect x="2" y="3" width="5" height="4" rx="1"/>
          <rect x="8.5" y="10" width="5" height="4" rx="1"/>
          <rect x="8.5" y="17" width="5" height="4" rx="1"/>
          <rect x="15" y="3" width="5" height="4" rx="1"/>
          <line x1="4.5" y1="7" x2="4.5" y2="18.5"/>
          <line x1="4.5" y1="18.5" x2="8.5" y2="18.5"/>
          <line x1="4.5" y1="11.5" x2="8.5" y2="11.5"/>
          <line x1="17.5" y1="7" x2="17.5" y2="18.5"/>
        {/if}
      </svg>
      <span class="graph-controls__layout-label">{layoutLabel}</span>
    </button>
  </div>

  <div class="graph-controls__divider" role="separator" aria-hidden="true"></div>

  <!-- ── Filters ────────────────────────────────────────────────────────── -->
  <div class="graph-controls__group graph-controls__group--filters" aria-label="Node filters">
    <!-- Broken link toggle -->
    <button
      class="graph-controls__chip"
      class:graph-controls__chip--active={filters.showBroken}
      type="button"
      role="switch"
      aria-checked={filters.showBroken}
      aria-label={filters.showBroken ? "Hide broken links" : "Show broken links"}
      onclick={toggleBroken}
    >
      broken
    </button>

    <!-- Doc type chips -->
    {#each availableTypes as type (type)}
      <button
        class="graph-controls__chip graph-controls__chip--type"
        class:graph-controls__chip--active={filters.docTypes.has(type)}
        type="button"
        role="switch"
        aria-checked={filters.docTypes.has(type)}
        aria-label={filters.docTypes.has(type) ? `Hide ${type} docs` : `Show ${type} docs`}
        onclick={() => toggleType(type)}
        data-type={type}
      >
        {type}
      </button>
    {/each}
  </div>

  <!-- ── Stats badge ────────────────────────────────────────────────────── -->
  <div class="graph-controls__stats" aria-live="polite" aria-label="Graph statistics">
    <span class="graph-controls__stat">{nodeCount} nodes</span>
    <span class="graph-controls__stat-sep" aria-hidden="true">·</span>
    <span class="graph-controls__stat">{edgeCount} edges</span>
  </div>
</div>

<style>
  /* ── Toolbar shell ─────────────────────────────────────────────────────── */

  .graph-controls {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-3);
    background-color: var(--color-surface-elevated);
    border-bottom: 1px solid var(--color-border);
    flex-wrap: wrap;
    min-height: 40px;
  }

  /* ── Button group ─────────────────────────────────────────────────────── */

  .graph-controls__group {
    display: flex;
    align-items: center;
    gap: 2px;
  }

  .graph-controls__group--filters {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--space-1);
  }

  /* ── Icon buttons ─────────────────────────────────────────────────────── */

  .graph-controls__btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-1);
    width: 28px;
    height: 28px;
    padding: 0;
    background: none;
    border: 1px solid transparent;
    border-radius: var(--radius-md);
    color: var(--color-text-muted);
    cursor: pointer;
    transition:
      background-color var(--duration-fast) var(--ease-out),
      color var(--duration-fast) var(--ease-out),
      border-color var(--duration-fast) var(--ease-out);
  }

  .graph-controls__btn--label {
    width: auto;
    padding: 0 var(--space-2);
  }

  .graph-controls__btn:hover {
    background-color: var(--color-surface-overlay);
    color: var(--color-text-primary);
  }

  .graph-controls__btn:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
    border-color: transparent;
  }

  .graph-controls__btn[aria-pressed="true"] {
    background-color: var(--color-accent-subtle);
    color: var(--color-accent);
    border-color: var(--color-accent-subtle);
  }

  /* ── Layout label ─────────────────────────────────────────────────────── */

  .graph-controls__layout-label {
    font-size: var(--font-size-sm, 12px);
    font-family: var(--font-mono);
    color: inherit;
  }

  /* ── Divider ──────────────────────────────────────────────────────────── */

  .graph-controls__divider {
    width: 1px;
    height: 18px;
    background-color: var(--color-border);
    flex-shrink: 0;
  }

  /* ── Filter chips ─────────────────────────────────────────────────────── */

  .graph-controls__chip {
    display: inline-flex;
    align-items: center;
    padding: 2px var(--space-2);
    font-size: var(--font-size-sm, 12px);
    font-family: var(--font-mono);
    color: var(--color-text-muted);
    background-color: transparent;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-full);
    cursor: pointer;
    transition:
      background-color var(--duration-fast) var(--ease-out),
      color var(--duration-fast) var(--ease-out),
      border-color var(--duration-fast) var(--ease-out);
  }

  .graph-controls__chip:hover {
    color: var(--color-text-primary);
    border-color: var(--color-border-strong);
  }

  .graph-controls__chip:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
  }

  .graph-controls__chip--active {
    background-color: var(--color-accent-subtle);
    color: var(--color-accent);
    border-color: var(--color-accent-subtle);
  }

  /* Doc-type specific chip tints using OKLCH */
  .graph-controls__chip--type[data-type="adr"] {
    --chip-hue: 265;
  }
  .graph-controls__chip--type[data-type="how-to"] {
    --chip-hue: 162;
  }
  .graph-controls__chip--type[data-type="tutorial"] {
    --chip-hue: 220;
  }
  .graph-controls__chip--type[data-type="reference"] {
    --chip-hue: 80;
  }
  .graph-controls__chip--type[data-type="runbook"] {
    --chip-hue: 25;
  }
  .graph-controls__chip--type[data-type="explanation"] {
    --chip-hue: 305;
  }

  .graph-controls__chip--type.graph-controls__chip--active {
    background-color: oklch(62% 0.14 var(--chip-hue, 265) / 0.18);
    color: oklch(80% 0.14 var(--chip-hue, 265));
    border-color: oklch(62% 0.14 var(--chip-hue, 265) / 0.35);
  }

  /* ── Stats badge ──────────────────────────────────────────────────────── */

  .graph-controls__stats {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    margin-left: auto;
    flex-shrink: 0;
  }

  .graph-controls__stat {
    font-size: var(--font-size-sm, 12px);
    font-family: var(--font-mono);
    color: var(--color-text-muted);
  }

  .graph-controls__stat-sep {
    color: var(--color-text-subtle);
    font-size: var(--font-size-sm, 12px);
  }

  /* ── Responsive ───────────────────────────────────────────────────────── */

  @media (max-width: 640px) {
    .graph-controls__stats {
      margin-left: 0;
      width: 100%;
    }
  }

  /* ── Reduced motion ───────────────────────────────────────────────────── */

  @media (prefers-reduced-motion: reduce) {
    .graph-controls__btn,
    .graph-controls__chip {
      transition: none;
    }
  }
</style>
