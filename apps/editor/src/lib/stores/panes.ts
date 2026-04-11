/**
 * panes.ts — pane tree store
 *
 * A pane is a single document view. Multiple panes can be open simultaneously.
 * v1: flat array (no nested splits), horizontal layout only.
 *
 * Each pane tracks its document identity and scroll position. The actual
 * document content is NOT stored here — it lives in the Editor instance.
 * The pane store is a coordination layer: which panes exist, which is active,
 * and viewport-aware capacity limits.
 */

import { writable, derived, get } from 'svelte/store';

export type PaneMode = 'rich' | 'source'; // rich = Tiptap (wysiwyg), source = CodeMirror (code)

export interface Pane {
  id: string;
  /** null = empty/picker state (split with no doc selected yet) */
  docPath: string | null;
  mode: PaneMode;
  scrollTop: number;
  readingMeasure: 'narrow' | 'default' | 'wide';
}

function createId(): string {
  return Math.random().toString(36).slice(2, 9);
}

function createPanesStore() {
  const panes = writable<Pane[]>([]);
  const activePaneId = writable<string | null>(null);

  /**
   * Viewport-aware max pane count.
   * Conservative thresholds — each pane needs ~600px minimum to be usable.
   */
  function getMaxPanes(): number {
    if (typeof window === 'undefined') return 1;
    const w = window.innerWidth;
    if (w >= 3840) return 4;
    if (w >= 2560) return 3;
    if (w >= 1440) return 2;
    return 1;
  }

  /**
   * Open a document in a pane.
   *
   * Strategy (in priority order):
   * 1. If the active pane has no doc (empty state), fill it.
   * 2. If at capacity, replace the active pane's doc in-place.
   * 3. Otherwise, open a new pane.
   *
   * Returns the pane ID that received the document.
   */
  function open(docPath: string, mode: PaneMode = 'rich'): string {
    const max = getMaxPanes();
    const current = get(panes);
    const activeId = get(activePaneId);

    // Fill empty active pane
    const active = current.find((p) => p.id === activeId);
    if (active && active.docPath === null) {
      panes.update((ps) =>
        ps.map((p) => (p.id === activeId ? { ...p, docPath, mode } : p))
      );
      return activeId!;
    }

    // At capacity — replace active pane
    if (current.length >= max) {
      if (activeId) {
        panes.update((ps) =>
          ps.map((p) => (p.id === activeId ? { ...p, docPath, mode } : p))
        );
        return activeId;
      }
    }

    // Room for a new pane
    const id = createId();
    const pane: Pane = {
      id,
      docPath,
      mode,
      scrollTop: 0,
      readingMeasure: 'default',
    };
    panes.update((ps) => [...ps, pane]);
    activePaneId.set(id);
    return id;
  }

  /**
   * Close a pane. If the closed pane was active, focus shifts to the last
   * remaining pane.
   */
  function close(id: string): void {
    panes.update((ps) => ps.filter((p) => p.id !== id));

    // If we closed the active pane, activate the last remaining one
    activePaneId.update((current) => {
      if (current !== id) return current;
      const remaining = get(panes);
      return remaining.length > 0
        ? remaining[remaining.length - 1].id
        : null;
    });
  }

  /** Focus a pane (make it the active target for keyboard shortcuts, etc.) */
  function focus(id: string): void {
    activePaneId.set(id);
  }

  /**
   * Split: add an empty pane next to the current one.
   * If already at max capacity, returns the current active pane ID (no-op).
   */
  function split(): string {
    const max = getMaxPanes();
    const current = get(panes);
    if (current.length >= max) return get(activePaneId) ?? '';

    const id = createId();
    const pane: Pane = {
      id,
      docPath: null,
      mode: 'rich',
      scrollTop: 0,
      readingMeasure: 'default',
    };
    panes.update((ps) => [...ps, pane]);
    activePaneId.set(id);
    return id;
  }

  function updateScrollTop(id: string, scrollTop: number): void {
    panes.update((ps) =>
      ps.map((p) => (p.id === id ? { ...p, scrollTop } : p))
    );
  }

  function setMode(id: string, mode: PaneMode): void {
    panes.update((ps) =>
      ps.map((p) => (p.id === id ? { ...p, mode } : p))
    );
  }

  function setReadingMeasure(
    id: string,
    measure: Pane['readingMeasure']
  ): void {
    panes.update((ps) =>
      ps.map((p) => (p.id === id ? { ...p, readingMeasure: measure } : p))
    );
  }

  const activePane = derived(
    [panes, activePaneId],
    ([$panes, $activeId]) =>
      $panes.find((p) => p.id === $activeId) ?? null
  );

  return {
    panes: { subscribe: panes.subscribe },
    activePaneId: { subscribe: activePaneId.subscribe },
    activePane,
    open,
    close,
    focus,
    split,
    updateScrollTop,
    setMode,
    setReadingMeasure,
  };
}

export const panesStore = createPanesStore();
