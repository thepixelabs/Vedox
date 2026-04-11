<!--
  PaneGroup.svelte

  Renders all open panes in a horizontal grid layout.
  Each pane is independently focusable and closable.

  Panes are separated by a 4 px drag handle that lets the user resize
  adjacent columns. The grid template is built dynamically:
    1 pane  → "1fr"
    2 panes → "1fr 4px 1fr"  (one divider)
    N panes → N × "1fr" interleaved with (N-1) × "4px"
  Once the user drags a divider the affected columns switch from "1fr" to
  explicit pixel values so the ratio is preserved.
-->

<script lang="ts">
  import { panesStore } from '$lib/stores/panes';
  import PaneView from './PaneView.svelte';
  import type { DocData } from './PaneView.svelte';

  interface Props {
    /**
     * Pre-loaded document data keyed by docPath.
     * The route passes the initially loaded doc here so the first pane
     * doesn't need to re-fetch. Additional panes will show an empty state
     * until doc loading is wired in a future phase.
     */
    loadedDocs: Record<string, DocData>;
    projectId: string;
    onChange?: (content: string) => void;
    onPublish?: (content: string, message: string) => void;
  }

  let { loadedDocs, projectId, onChange, onPublish }: Props = $props();

  const panes = panesStore.panes;
  const activePaneId = panesStore.activePaneId;

  // ── Drag-to-resize state ────────────────────────────────────────────────

  /** Minimum pane width in px — keeps content readable. */
  const MIN_PANE_WIDTH = 320;

  /** Index of the divider currently being dragged, or null. */
  let dragging = $state<number | null>(null);

  /** clientX at the start of the drag gesture. */
  let dragStartX = $state(0);

  /**
   * Snapshot of resolved pane column widths (px) at drag start.
   * Only pane tracks are included — divider tracks are skipped.
   */
  let startWidths = $state<number[]>([]);

  /**
   * User-set column template override. `null` means "use the default 1fr
   * layout". Once the user drags a divider this is set to explicit pixel
   * widths and stays that way until the pane count changes (which resets
   * it back to the equal-width default).
   */
  let customTemplate = $state<string | null>(null);

  /** Reset custom widths whenever the pane count changes. */
  let lastPaneCount = $state(0);

  /**
   * The grid-template-columns value used by the container.
   * Falls through: customTemplate → equal-width default with dividers.
   */
  let gridTemplate = $derived.by(() => {
    const count = $panes.length;

    // Reset custom widths when pane count changes (open / close / split).
    if (count !== lastPaneCount) {
      lastPaneCount = count;
      customTemplate = null;
    }

    if (customTemplate) return customTemplate;
    if (count <= 1) return '1fr';
    return Array(count).fill('1fr').join(' 4px ');
  });

  // ── Pointer event handlers ──────────────────────────────────────────────

  function onDividerPointerDown(e: PointerEvent, dividerIndex: number) {
    e.preventDefault();
    dragging = dividerIndex;
    dragStartX = e.clientX;

    // Snapshot resolved column widths from the grid container.
    const container = (e.target as HTMLElement).closest('.pane-group') as HTMLElement;
    if (!container) return;

    const cols = getComputedStyle(container).gridTemplateColumns.split(' ');
    // Resolved template alternates: paneWidth dividerWidth paneWidth ...
    // We only want the pane widths (every other value starting at index 0).
    startWidths = cols
      .filter((_c, i) => i % 2 === 0) // even indices = pane columns
      .map((c) => parseFloat(c));

    (e.target as HTMLElement).setPointerCapture(e.pointerId);
  }

  function onPointerMove(e: PointerEvent) {
    if (dragging === null) return;

    const delta = e.clientX - dragStartX;
    const newWidths = [...startWidths];

    // Redistribute space between the two adjacent panes.
    const left = Math.max(MIN_PANE_WIDTH, startWidths[dragging] + delta);
    const right = Math.max(MIN_PANE_WIDTH, startWidths[dragging + 1] - delta);
    newWidths[dragging] = left;
    newWidths[dragging + 1] = right;

    customTemplate = newWidths.map((w) => `${w}px`).join(' 4px ');
  }

  function onPointerUp() {
    dragging = null;
  }
</script>

<div
  class="pane-group"
  style:grid-template-columns={gridTemplate}
  onpointermove={onPointerMove}
  onpointerup={onPointerUp}
>
  {#each $panes as pane, i (pane.id)}
    <PaneView
      {pane}
      isActive={$activePaneId === pane.id}
      docData={pane.docPath ? loadedDocs[pane.docPath] ?? null : null}
      {projectId}
      {onChange}
      {onPublish}
      onFocus={() => panesStore.focus(pane.id)}
      onClose={() => panesStore.close(pane.id)}
    />
    {#if i < $panes.length - 1}
      <div
        class="pane-divider"
        class:pane-divider--dragging={dragging === i}
        role="separator"
        aria-orientation="vertical"
        onpointerdown={(e) => onDividerPointerDown(e, i)}
      ></div>
    {/if}
  {/each}
</div>

<style>
  .pane-group {
    display: grid;
    height: 100%;
    width: 100%;
    overflow: hidden;
  }

  .pane-divider {
    width: 4px;
    height: 100%;
    background: var(--border-hairline);
    cursor: col-resize;
    transition: background-color var(--duration-fast) var(--ease-out);
    touch-action: none;
  }

  .pane-divider:hover,
  .pane-divider--dragging {
    background: var(--accent-solid);
  }

  @media (prefers-reduced-motion: reduce) {
    .pane-divider {
      transition: none;
    }
  }
</style>
