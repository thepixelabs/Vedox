<script lang="ts">
  /**
   * StatusList — accessible, draggable list of status-tagged items.
   *
   * Used for the Phase 2 task backlog view and Phase 3 agent review queue.
   * Drag-to-reorder uses the HTML5 Drag and Drop API — no library dependency.
   *
   * Keyboard drag flow:
   *   1. Focus drag handle → Tab/click
   *   2. Space → "picks up" the item (enters keyboard drag mode)
   *   3. Arrow Up / Arrow Down → moves item in list
   *   4. Space → drops item at current position
   *   5. Escape → cancels, restores original order
   *
   * Announcements via aria-live="polite" region on the container.
   */

  import StatusChip from './StatusChip.svelte'
  import type { StatusListItem } from './index.js'

  interface Props {
    items?: StatusListItem[]
    /** When true, drag handles appear on hover and keyboard reorder is enabled. */
    draggable?: boolean
    onReorder?: (newOrder: string[]) => void
    emptyMessage?: string
    loading?: boolean
  }

  let {
    items = [],
    draggable: isDraggable = false,
    onReorder = () => {},
    emptyMessage = 'No items',
    loading = false,
  }: Props = $props()

  // Local alias avoids shadowing the HTML global `draggable` attribute name
  // in event handler closures and template expressions. $derived keeps it
  // reactive so parent toggling isDraggable flows through correctly.
  const draggable = $derived(isDraggable)

  // ── Local mutable copy for drag reordering ───────────────────────────────
  let orderedItems = $derived([...items])

  // ── HTML5 drag state ─────────────────────────────────────────────────────
  let dragSourceId: string | null = $state(null)
  let dropTargetId: string | null = $state(null)
  let dropPosition: 'above' | 'below' | null = $state(null)

  function handleDragStart(event: DragEvent, item: StatusListItem) {
    dragSourceId = item.id
    if (event.dataTransfer) {
      event.dataTransfer.effectAllowed = 'move'
      event.dataTransfer.setData('text/plain', item.id)
    }
  }

  function handleDragOver(event: DragEvent, item: StatusListItem) {
    event.preventDefault()
    if (event.dataTransfer) {
      event.dataTransfer.dropEffect = 'move'
    }
    if (item.id === dragSourceId) {
      dropTargetId = null
      dropPosition = null
      return
    }
    dropTargetId = item.id
    // Determine insertion line position: above if cursor is in top half
    const rect = (event.currentTarget as HTMLElement).getBoundingClientRect()
    dropPosition = event.clientY < rect.top + rect.height / 2 ? 'above' : 'below'
  }

  function handleDragLeave(event: DragEvent) {
    // Only clear if we're leaving the item entirely (not entering a child)
    const relatedTarget = event.relatedTarget as Node | null
    const currentTarget = event.currentTarget as Node
    if (relatedTarget && currentTarget.contains(relatedTarget)) return
    dropTargetId = null
    dropPosition = null
  }

  function handleDrop(event: DragEvent, targetItem: StatusListItem) {
    event.preventDefault()
    if (!dragSourceId || dragSourceId === targetItem.id) {
      clearDragState()
      return
    }

    const newOrder = reorder(orderedItems, dragSourceId, targetItem.id, dropPosition ?? 'below')
    onReorder(newOrder.map(i => i.id))
    clearDragState()
  }

  function handleDragEnd() {
    clearDragState()
  }

  function clearDragState() {
    dragSourceId = null
    dropTargetId = null
    dropPosition = null
  }

  // ── Keyboard drag state ──────────────────────────────────────────────────
  let keyboardDragId: string | null = $state(null)
  let announcement = $state('')

  function handleHandleKeydown(event: KeyboardEvent, item: StatusListItem) {
    if (!draggable) return

    if (event.key === ' ') {
      event.preventDefault()
      if (keyboardDragId === null) {
        // Pick up
        keyboardDragId = item.id
        announcement = `Picked up ${item.title}. Use Arrow Up and Arrow Down to move, Space to drop, Escape to cancel.`
      } else if (keyboardDragId === item.id) {
        // Drop in place
        announcement = `Dropped ${item.title}.`
        keyboardDragId = null
      }
      return
    }

    if (keyboardDragId === item.id) {
      if (event.key === 'ArrowUp') {
        event.preventDefault()
        const newOrder = moveItem(orderedItems, item.id, -1)
        if (newOrder) {
          onReorder(newOrder.map(i => i.id))
          const newIdx = newOrder.findIndex(i => i.id === item.id)
          announcement = `${item.title} moved to position ${newIdx + 1} of ${newOrder.length}.`
        }
        return
      }
      if (event.key === 'ArrowDown') {
        event.preventDefault()
        const newOrder = moveItem(orderedItems, item.id, 1)
        if (newOrder) {
          onReorder(newOrder.map(i => i.id))
          const newIdx = newOrder.findIndex(i => i.id === item.id)
          announcement = `${item.title} moved to position ${newIdx + 1} of ${newOrder.length}.`
        }
        return
      }
      if (event.key === 'Escape') {
        event.preventDefault()
        announcement = `Cancelled. ${item.title} returned to original position.`
        keyboardDragId = null
        return
      }
    }
  }

  // ── Pure reorder helpers ─────────────────────────────────────────────────
  function reorder(
    list: StatusListItem[],
    sourceId: string,
    targetId: string,
    position: 'above' | 'below',
  ): StatusListItem[] {
    const next = list.filter(i => i.id !== sourceId)
    const source = list.find(i => i.id === sourceId)!
    const targetIdx = next.findIndex(i => i.id === targetId)
    const insertAt = position === 'above' ? targetIdx : targetIdx + 1
    next.splice(insertAt, 0, source)
    return next
  }

  function moveItem(
    list: StatusListItem[],
    id: string,
    delta: -1 | 1,
  ): StatusListItem[] | null {
    const idx = list.findIndex(i => i.id === id)
    const nextIdx = idx + delta
    if (nextIdx < 0 || nextIdx >= list.length) return null
    const next = [...list]
    ;[next[idx], next[nextIdx]] = [next[nextIdx], next[idx]]
    return next
  }
</script>

<!-- aria-live region is always mounted so the browser registers it before we write to it -->
<div class="status-list__announcer" aria-live="polite" aria-atomic="true">
  {announcement}
</div>

<div
  class="status-list"
  class:status-list--loading={loading}
  aria-busy={loading}
>
  {#if loading}
    <div class="status-list__loading" aria-label="Loading items">
      <span class="status-list__spinner" aria-hidden="true"></span>
      <span class="status-list__loading-text">Loading…</span>
    </div>
  {:else if orderedItems.length === 0}
    <div class="status-list__empty" aria-label={emptyMessage}>
      <span class="status-list__empty-text">{emptyMessage}</span>
    </div>
  {:else}
    <ul class="status-list__list" role="list">
      {#each orderedItems as item (item.id)}
        {@const isDragging = dragSourceId === item.id}
        {@const isDropTarget = dropTargetId === item.id}
        {@const isKeyboardDragging = keyboardDragId === item.id}

        <li
          class="status-list__item"
          class:status-list__item--dragging={isDragging}
          class:status-list__item--drop-above={isDropTarget && dropPosition === 'above'}
          class:status-list__item--drop-below={isDropTarget && dropPosition === 'below'}
          class:status-list__item--keyboard-dragging={isKeyboardDragging}
          role="listitem"
          draggable={draggable ? 'true' : undefined}
          ondragstart={draggable ? (e) => handleDragStart(e, item) : undefined}
          ondragover={draggable ? (e) => handleDragOver(e, item) : undefined}
          ondragleave={draggable ? handleDragLeave : undefined}
          ondrop={draggable ? (e) => handleDrop(e, item) : undefined}
          ondragend={draggable ? handleDragEnd : undefined}
        >
          <!-- Drag handle — only renders when draggable prop is true -->
          {#if draggable}
            <button
              class="status-list__drag-handle"
              type="button"
              aria-label="Drag to reorder {item.title}"
              tabindex="0"
              aria-pressed={isKeyboardDragging}
              onkeydown={(e) => handleHandleKeydown(e, item)}
            >
              <!-- Braille pattern dots 123456 — universal drag grip icon -->
              <svg
                width="10"
                height="14"
                viewBox="0 0 10 14"
                fill="currentColor"
                aria-hidden="true"
              >
                <circle cx="2.5" cy="2" r="1.25"/>
                <circle cx="7.5" cy="2" r="1.25"/>
                <circle cx="2.5" cy="7" r="1.25"/>
                <circle cx="7.5" cy="7" r="1.25"/>
                <circle cx="2.5" cy="12" r="1.25"/>
                <circle cx="7.5" cy="12" r="1.25"/>
              </svg>
            </button>
          {/if}

          <!-- Item body -->
          <div class="status-list__body">
            <!-- Top row: title + chip -->
            <div class="status-list__row status-list__row--top">
              <div class="status-list__title-wrap">
                {#if item.href}
                  <a
                    class="status-list__title status-list__title--link"
                    href={item.href}
                  >{item.title}</a>
                {:else}
                  <span class="status-list__title">{item.title}</span>
                {/if}
              </div>
              <StatusChip status={item.status} />
            </div>

            <!-- Description — optional -->
            {#if item.description}
              <p class="status-list__description">{item.description}</p>
            {/if}

            <!-- Bottom row: meta + actions -->
            {#if item.meta || (item.actions && item.actions.length > 0)}
              <div class="status-list__row status-list__row--bottom">
                {#if item.meta}
                  <span class="status-list__meta">{item.meta}</span>
                {/if}

                {#if item.actions && item.actions.length > 0}
                  <div class="status-list__actions" role="group" aria-label="Actions for {item.title}">
                    {#each item.actions as action}
                      <button
                        class="status-list__action"
                        class:status-list__action--primary={action.variant === 'primary'}
                        class:status-list__action--danger={action.variant === 'danger'}
                        type="button"
                        onclick={action.onClick}
                      >{action.label}</button>
                    {/each}
                  </div>
                {/if}
              </div>
            {/if}
          </div>
        </li>
      {/each}
    </ul>
  {/if}
</div>

<style>
  /* ── Screen-reader only announcer ──────────────────────────────────────── */
  .status-list__announcer {
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

  /* ── Container ──────────────────────────────────────────────────────────── */
  .status-list {
    font-family: var(--font-sans);
    font-size: var(--font-size-base);
    color: var(--color-text-primary);
    width: 100%;
  }

  /* ── Loading state ──────────────────────────────────────────────────────── */
  .status-list__loading {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-5) var(--space-4);
    color: var(--color-text-muted);
    font-size: var(--font-size-sm);
  }

  .status-list__spinner {
    display: inline-block;
    width: 14px;
    height: 14px;
    border: 2px solid var(--color-border);
    border-top-color: var(--color-accent);
    border-radius: 50%;
    animation: spin 600ms linear infinite;
    flex-shrink: 0;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  .status-list__loading-text {
    color: var(--color-text-muted);
  }

  /* ── Empty state ────────────────────────────────────────────────────────── */
  .status-list__empty {
    display: flex;
    align-items: center;
    justify-content: center;
    padding: var(--space-7) var(--space-4);
  }

  .status-list__empty-text {
    font-size: var(--font-size-sm);
    color: var(--color-text-muted);
    text-align: center;
  }

  /* ── List ───────────────────────────────────────────────────────────────── */
  .status-list__list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 1px; /* hairline gap between items via surface color */
  }

  /* ── Item ───────────────────────────────────────────────────────────────── */
  .status-list__item {
    display: flex;
    align-items: flex-start;
    gap: var(--space-2);
    padding: var(--space-3) var(--space-3);
    background-color: var(--color-surface-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    transition:
      background-color 80ms ease,
      border-color 80ms ease,
      opacity 120ms ease;
    position: relative;
  }

  .status-list__item:hover {
    background-color: var(--color-surface-overlay);
    border-color: var(--color-border-strong);
  }

  /* Dragging: fade source item */
  .status-list__item--dragging {
    opacity: 0.4;
  }

  /* Keyboard drag active — subtle ring to show item is "held" */
  .status-list__item--keyboard-dragging {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
    background-color: var(--color-accent-subtle);
  }

  /* Drop target insertion lines */
  .status-list__item--drop-above::before,
  .status-list__item--drop-below::after {
    content: '';
    position: absolute;
    left: 0;
    right: 0;
    height: 2px;
    background-color: var(--color-accent);
    border-radius: 1px;
    pointer-events: none;
  }

  .status-list__item--drop-above::before {
    top: -2px;
  }

  .status-list__item--drop-below::after {
    bottom: -2px;
  }

  /* ── Drag handle ────────────────────────────────────────────────────────── */
  .status-list__drag-handle {
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
    width: 20px;
    height: 20px;
    margin-top: 1px; /* optical align with first line of title */
    padding: 0;
    background: none;
    border: none;
    border-radius: var(--radius-sm);
    color: var(--color-text-muted);
    cursor: grab;
    opacity: 0;
    transition:
      opacity 100ms ease,
      color 100ms ease,
      background-color 100ms ease;
  }

  /* Show handle on item hover or when any item in list is focused */
  .status-list__item:hover .status-list__drag-handle,
  .status-list__drag-handle:focus-visible {
    opacity: 1;
  }

  .status-list__drag-handle:hover {
    color: var(--color-text-secondary);
    background-color: var(--color-surface-overlay);
  }

  .status-list__drag-handle:active {
    cursor: grabbing;
  }

  .status-list__drag-handle:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
    color: var(--color-text-primary);
  }

  /* ── Item body ──────────────────────────────────────────────────────────── */
  .status-list__body {
    flex: 1;
    min-width: 0; /* allow text truncation */
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  /* ── Rows ───────────────────────────────────────────────────────────────── */
  .status-list__row {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }

  .status-list__row--top {
    justify-content: space-between;
    align-items: flex-start;
  }

  .status-list__row--bottom {
    justify-content: space-between;
    align-items: center;
    margin-top: var(--space-1);
  }

  /* ── Title ──────────────────────────────────────────────────────────────── */
  .status-list__title-wrap {
    flex: 1;
    min-width: 0;
  }

  .status-list__title {
    font-size: var(--font-size-base);
    font-weight: 500;
    color: var(--color-text-primary);
    line-height: 1.4;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }

  .status-list__title--link {
    text-decoration: none;
    color: var(--color-text-primary);
    transition: color 80ms var(--ease-out);
  }

  .status-list__title--link:hover {
    color: var(--color-accent);
    text-decoration: underline;
    text-underline-offset: 2px;
  }

  .status-list__title--link:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
    border-radius: var(--radius-sm);
  }

  /* ── Description ────────────────────────────────────────────────────────── */
  .status-list__description {
    margin: 0;
    font-size: var(--font-size-sm);
    color: var(--color-text-secondary);
    line-height: 1.5;
    display: -webkit-box;
    -webkit-line-clamp: 3;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }

  /* ── Meta ───────────────────────────────────────────────────────────────── */
  .status-list__meta {
    font-size: var(--font-size-sm);
    color: var(--color-text-muted);
    font-family: var(--font-mono);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    min-width: 0;
  }

  /* ── Actions ────────────────────────────────────────────────────────────── */
  .status-list__actions {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    flex-shrink: 0;
  }

  .status-list__action {
    display: inline-flex;
    align-items: center;
    padding: 3px var(--space-2);
    background: none;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    font-family: var(--font-sans);
    font-size: var(--font-size-sm);
    font-weight: 500;
    color: var(--color-text-secondary);
    cursor: pointer;
    transition:
      background-color 80ms ease,
      border-color 80ms ease,
      color 80ms ease;
  }

  .status-list__action:hover {
    background-color: var(--color-surface-overlay);
    border-color: var(--color-border-strong);
    color: var(--color-text-primary);
  }

  .status-list__action:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
  }

  /* Primary variant */
  .status-list__action--primary {
    background-color: var(--color-accent);
    border-color: var(--color-accent);
    color: var(--color-text-inverse);
  }

  .status-list__action--primary:hover {
    background-color: var(--color-accent-hover);
    border-color: var(--color-accent-hover);
    color: var(--color-text-inverse);
  }

  /* Danger variant */
  .status-list__action--danger {
    color: var(--color-error);
    border-color: color-mix(in srgb, var(--color-error) 30%, transparent);
  }

  .status-list__action--danger:hover {
    background-color: color-mix(in srgb, var(--color-error) 10%, transparent);
    border-color: var(--color-error);
    color: var(--color-error);
  }

  /* ── Responsive ─────────────────────────────────────────────────────────── */
  @media (max-width: 480px) {
    .status-list__row--top {
      flex-wrap: wrap;
      gap: var(--space-1);
    }

    .status-list__row--bottom {
      flex-direction: column;
      align-items: flex-start;
      gap: var(--space-1);
    }
  }

  /* ── Reduced motion ─────────────────────────────────────────────────────── */
  @media (prefers-reduced-motion: reduce) {
    .status-list__item,
    .status-list__drag-handle,
    .status-list__action,
    .status-list__title--link {
      transition: none;
    }

    .status-list__spinner {
      animation: none;
      /* Show a static partial arc so the spinner is still visible */
      border-top-color: var(--color-accent);
    }
  }
</style>
