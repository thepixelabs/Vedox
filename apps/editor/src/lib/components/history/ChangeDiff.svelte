<script lang="ts">
  /**
   * ChangeDiff.svelte — prose-level visual diff for a single block change.
   *
   * Renders one BlockChange from the history API with document-level visual
   * language: coloured block backgrounds, left border rails, and clear section
   * context. Deliberately NOT a code diff — no line numbers, no unified diff
   * syntax. The goal is "what changed in my document" not "what changed in a
   * text file".
   *
   * Colour semantics:
   *   added    — green background + green rail
   *   removed  — red background + red rail + strikethrough
   *   modified — amber background + amber rail, before/after stacked
   *   moved    — info background + info rail
   */

  import type { BlockChange } from './types.js';

  interface Props {
    change: BlockChange;
  }

  let { change }: Props = $props();

  /** Human-readable label for the change type. */
  const typeLabel: Record<string, string> = {
    added:    'added',
    removed:  'removed',
    modified: 'modified',
    moved:    'moved',
  };

  /** Whether this block has a before version to show. */
  const hasBefore = $derived(change.before.trim().length > 0);
  /** Whether this block has an after version to show. */
  const hasAfter  = $derived(change.after.trim().length > 0);

  /** Readable label for the block kind. */
  function blockKindLabel(kind: string): string {
    const labels: Record<string, string> = {
      frontmatter: 'frontmatter',
      heading:     'heading',
      paragraph:   'paragraph',
      code_fence:  'code block',
      list_item:   'list',
    };
    return labels[kind] ?? kind;
  }
</script>

<div
  class="change-diff"
  class:change-diff--added={change.type === 'added'}
  class:change-diff--removed={change.type === 'removed'}
  class:change-diff--modified={change.type === 'modified'}
  class:change-diff--moved={change.type === 'moved'}
  aria-label="{typeLabel[change.type]} {blockKindLabel(change.blockKind)}{change.section ? ' in ' + change.section : ''}"
>
  <!-- ── Header row: type badge + section context ──────────────────────── -->
  <div class="change-diff__meta">
    <span
      class="change-diff__badge"
      class:change-diff__badge--added={change.type === 'added'}
      class:change-diff__badge--removed={change.type === 'removed'}
      class:change-diff__badge--modified={change.type === 'modified'}
      class:change-diff__badge--moved={change.type === 'moved'}
      aria-hidden="true"
    >
      {typeLabel[change.type]}
    </span>

    <span class="change-diff__kind" aria-hidden="true">
      {blockKindLabel(change.blockKind)}
    </span>

    {#if change.section}
      <span class="change-diff__section" aria-hidden="true">
        <svg
          width="10"
          height="10"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
          aria-hidden="true"
        >
          <polyline points="9 18 15 12 9 6" />
        </svg>
        {change.section}
      </span>
    {/if}
  </div>

  <!-- ── Content body ──────────────────────────────────────────────────── -->
  <div class="change-diff__body">
    {#if change.type === 'modified' && hasBefore && hasAfter}
      <!-- Modified: before stacked over after with clear labels -->
      <div class="change-diff__block change-diff__block--before" aria-label="Before">
        <span class="change-diff__block-label" aria-hidden="true">before</span>
        <p class="change-diff__text change-diff__text--strikethrough">{change.before}</p>
      </div>
      <div class="change-diff__block change-diff__block--after" aria-label="After">
        <span class="change-diff__block-label" aria-hidden="true">after</span>
        <p class="change-diff__text">{change.after}</p>
      </div>
    {:else if change.type === 'removed' && hasBefore}
      <p class="change-diff__text change-diff__text--strikethrough">{change.before}</p>
    {:else if change.type === 'added' && hasAfter}
      <p class="change-diff__text">{change.after}</p>
    {:else if change.type === 'moved'}
      <!-- Moved blocks show destination content (after), before dimly in meta -->
      {#if hasBefore && hasAfter && change.before !== change.after}
        <div class="change-diff__block change-diff__block--before" aria-label="From">
          <span class="change-diff__block-label" aria-hidden="true">from</span>
          <p class="change-diff__text change-diff__text--muted">{change.before}</p>
        </div>
        <div class="change-diff__block change-diff__block--after" aria-label="To">
          <span class="change-diff__block-label" aria-hidden="true">to</span>
          <p class="change-diff__text">{change.after}</p>
        </div>
      {:else}
        <p class="change-diff__text">{change.after || change.before}</p>
      {/if}
    {/if}
  </div>
</div>

<style>
  /* ── Container ───────────────────────────────────────────────────────── */

  .change-diff {
    border-radius: var(--radius-md);
    border-left: 3px solid transparent;
    padding: var(--space-2) var(--space-3);
    background-color: var(--color-surface-overlay);
    transition:
      background-color var(--duration-fast) var(--ease-out),
      border-color var(--duration-fast) var(--ease-out);
  }

  /* Type-specific backgrounds and rails */
  .change-diff--added {
    background-color: color-mix(in oklch, var(--success) 9%, transparent);
    border-left-color: var(--success);
  }

  .change-diff--removed {
    background-color: color-mix(in oklch, var(--error) 9%, transparent);
    border-left-color: var(--error);
  }

  .change-diff--modified {
    background-color: color-mix(in oklch, var(--warning) 9%, transparent);
    border-left-color: var(--warning);
  }

  .change-diff--moved {
    background-color: color-mix(in oklch, var(--info) 9%, transparent);
    border-left-color: var(--info);
  }

  /* ── Meta row ────────────────────────────────────────────────────────── */

  .change-diff__meta {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    margin-bottom: var(--space-2);
    flex-wrap: wrap;
  }

  /* ── Type badge ──────────────────────────────────────────────────────── */

  .change-diff__badge {
    display: inline-flex;
    align-items: center;
    padding: 1px 6px;
    border-radius: var(--radius-full);
    font-size: 10px;
    font-family: var(--font-mono);
    font-weight: 600;
    letter-spacing: var(--tracking-wide);
    text-transform: uppercase;
    flex-shrink: 0;
  }

  .change-diff__badge--added {
    background-color: color-mix(in oklch, var(--success) 18%, transparent);
    color: var(--success);
  }

  .change-diff__badge--removed {
    background-color: color-mix(in oklch, var(--error) 18%, transparent);
    color: var(--error);
  }

  .change-diff__badge--modified {
    background-color: color-mix(in oklch, var(--warning) 18%, transparent);
    color: var(--warning);
  }

  .change-diff__badge--moved {
    background-color: color-mix(in oklch, var(--info) 18%, transparent);
    color: var(--info);
  }

  /* ── Kind label ──────────────────────────────────────────────────────── */

  .change-diff__kind {
    font-size: var(--text-caption);
    font-family: var(--font-mono);
    color: var(--color-text-muted);
    flex-shrink: 0;
  }

  /* ── Section context ─────────────────────────────────────────────────── */

  .change-diff__section {
    display: inline-flex;
    align-items: center;
    gap: 3px;
    font-size: var(--text-caption);
    font-family: var(--font-mono);
    color: var(--color-text-muted);
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .change-diff__section svg {
    color: var(--color-text-subtle);
    flex-shrink: 0;
  }

  /* ── Content body ────────────────────────────────────────────────────── */

  .change-diff__body {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  /* ── Sub-blocks for modified / moved ─────────────────────────────────── */

  .change-diff__block {
    position: relative;
  }

  .change-diff__block--before {
    opacity: 0.8;
  }

  .change-diff__block--after {
    opacity: 1;
  }

  .change-diff__block-label {
    display: block;
    font-size: 9px;
    font-family: var(--font-mono);
    font-weight: 600;
    letter-spacing: var(--tracking-widest);
    text-transform: uppercase;
    color: var(--color-text-subtle);
    margin-bottom: 2px;
  }

  /* ── Text content ────────────────────────────────────────────────────── */

  .change-diff__text {
    margin: 0;
    font-size: var(--text-sm);
    font-family: var(--font-body);
    color: var(--color-text-secondary);
    line-height: var(--leading-snug);
    /* Clamp long text to 4 lines; user can see full text in the commit view */
    display: -webkit-box;
    -webkit-line-clamp: 4;
    line-clamp: 4;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }

  .change-diff__text--strikethrough {
    text-decoration: line-through;
    color: var(--color-text-muted);
    opacity: 0.7;
  }

  .change-diff__text--muted {
    color: var(--color-text-muted);
    opacity: 0.7;
  }

  /* ── Reduced motion ──────────────────────────────────────────────────── */

  @media (prefers-reduced-motion: reduce) {
    .change-diff {
      transition: none;
    }
  }
</style>
