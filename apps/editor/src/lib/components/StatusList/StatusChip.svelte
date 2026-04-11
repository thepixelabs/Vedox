<script lang="ts">
  /**
   * StatusChip — standalone status pill primitive.
   *
   * Used internally by StatusList, but exported as a standalone component
   * so Phase 3 agent review queue can import chips without the full list.
   *
   * Design: pill shape, 12px uppercase text, token-only colors.
   * Five statuses map to five semantic color families.
   */

  import type { StatusListItem } from './index.js'

  interface Props {
    status: StatusListItem['status']
    label?: string
  }

  let { status, label }: Props = $props()

  const defaultLabels: Record<StatusListItem['status'], string> = {
    'todo': 'Todo',
    'in-progress': 'In Progress',
    'done': 'Done',
    'review': 'Review',
    'rejected': 'Rejected',
  }

  const displayLabel = $derived(label ?? defaultLabels[status])
</script>

<span
  class="chip chip--{status}"
  aria-label="Status: {displayLabel}"
>
  {displayLabel}
</span>

<style>
  .chip {
    display: inline-flex;
    align-items: center;
    padding: 4px var(--space-2);
    border-radius: var(--radius-lg);
    font-family: var(--font-sans);
    font-size: var(--font-size-sm);
    font-weight: 500;
    letter-spacing: 0.04em;
    text-transform: uppercase;
    line-height: 1;
    white-space: nowrap;
    border: 1px solid transparent;
    /* Prevent chips from being a tab stop — they are labelled by aria-label
       on the parent item. The chip itself is purely presentational. */
    user-select: none;
  }

  /* todo — neutral: muted text, border outline, no fill */
  .chip--todo {
    color: var(--color-text-muted);
    border-color: var(--color-border);
    background-color: transparent;
  }

  /* in-progress — accent family */
  .chip--in-progress {
    color: var(--color-accent);
    background-color: var(--color-accent-subtle);
    border-color: transparent;
  }

  /* done — success tint via currentColor alpha trick on elevated surface */
  .chip--done {
    color: var(--color-success);
    background-color: color-mix(in srgb, var(--color-success) 12%, transparent);
    border-color: transparent;
  }

  /* review — warning tint */
  .chip--review {
    color: var(--color-warning);
    background-color: color-mix(in srgb, var(--color-warning) 12%, transparent);
    border-color: transparent;
  }

  /* rejected — error tint */
  .chip--rejected {
    color: var(--color-error);
    background-color: color-mix(in srgb, var(--color-error) 12%, transparent);
    border-color: transparent;
  }
</style>
