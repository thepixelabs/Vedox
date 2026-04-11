<!--
  ReviewQueue.svelte — AI review suggestions panel (v1 stub)

  Renders pending suggestions from the reviewQueueStore with
  accept / reject / dismiss actions. Designed to mount as a
  slide-out right drawer from the editor toolbar.
-->

<script lang="ts">
  import { reviewQueueStore } from '$lib/stores/reviewQueue';
  import type { ReviewSuggestion } from '$lib/stores/reviewQueue';

  const queue = reviewQueueStore;

  const pendingCount = reviewQueueStore.pendingCount;

  const pendingSuggestions = $derived(
    ($queue).filter((s: ReviewSuggestion) => s.status === 'pending')
  );

  const TYPE_LABELS: Record<ReviewSuggestion['type'], string> = {
    grammar: 'Grammar',
    clarity: 'Clarity',
    structure: 'Structure',
    style: 'Style',
  };

  function formatRelative(iso: string): string {
    const diff = Date.now() - new Date(iso).getTime();
    const min = Math.floor(diff / 60000);
    if (min < 1) return 'just now';
    if (min < 60) return `${min}m ago`;
    return `${Math.floor(min / 60)}h ago`;
  }
</script>

<div class="review-queue">
  <div class="review-queue__header">
    <h2 class="review-queue__title">
      Writing review
      {#if $pendingCount > 0}
        <span class="review-queue__badge">{$pendingCount}</span>
      {/if}
    </h2>
    <p class="review-queue__subtitle">AI suggestions for clarity, grammar, and structure</p>
  </div>

  {#if pendingSuggestions.length === 0}
    <div class="review-queue__empty">
      <p>Queue is clear</p>
      <p class="review-queue__empty-hint">The AI checks your writing as you type.</p>
    </div>
  {:else}
    <ul class="review-queue__list" role="list">
      {#each pendingSuggestions as suggestion (suggestion.id)}
        <li class="review-card">
          <div class="review-card__meta">
            <span class="review-card__type">{TYPE_LABELS[suggestion.type]}</span>
            <span class="review-card__time">{formatRelative(suggestion.createdAt)}</span>
          </div>
          <div class="review-card__diff">
            <del class="review-card__original">{suggestion.original}</del>
            <ins class="review-card__suggested">{suggestion.suggested}</ins>
          </div>
          <p class="review-card__reason">{suggestion.reason}</p>
          <div class="review-card__actions">
            <button
              class="review-card__btn review-card__btn--accept"
              type="button"
              onclick={() => queue.accept(suggestion.id)}
            >Accept</button>
            <button
              class="review-card__btn review-card__btn--reject"
              type="button"
              onclick={() => queue.reject(suggestion.id)}
            >Reject</button>
            <button
              class="review-card__btn"
              type="button"
              onclick={() => queue.dismiss(suggestion.id)}
            >Dismiss</button>
          </div>
        </li>
      {/each}
    </ul>
  {/if}
</div>

<style>
  .review-queue {
    display: flex;
    flex-direction: column;
    height: 100%;
    overflow: hidden;
  }

  .review-queue__header {
    padding: var(--space-4) var(--space-5);
    border-bottom: 1px solid var(--border-hairline);
    flex-shrink: 0;
  }

  .review-queue__title {
    font-size: var(--text-base);
    font-weight: 600;
    color: var(--text-1);
    margin: 0;
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }

  .review-queue__badge {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 20px;
    height: 20px;
    border-radius: var(--radius-full);
    background: var(--accent-solid);
    color: var(--accent-contrast);
    font-size: var(--text-2xs);
    font-weight: 700;
  }

  .review-queue__subtitle {
    font-size: var(--text-xs);
    color: var(--text-4);
    margin: var(--space-1) 0 0 0;
  }

  .review-queue__list {
    list-style: none;
    padding: var(--space-3);
    margin: 0;
    overflow-y: auto;
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .review-queue__empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    flex: 1;
    padding: var(--space-10);
    text-align: center;
    color: var(--text-4);
    font-size: var(--text-sm);
    gap: var(--space-2);
  }

  .review-queue__empty p { margin: 0; }
  .review-queue__empty-hint { font-size: var(--text-xs); }

  .review-card {
    padding: var(--space-4);
    background: var(--surface-2);
    border: 1px solid var(--border-hairline);
    border-radius: var(--radius-md);
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    animation: review-slide-in 180ms var(--ease-out) both;
  }

  .review-card:nth-child(1) { animation-delay: 0ms; }
  .review-card:nth-child(2) { animation-delay: 30ms; }
  .review-card:nth-child(3) { animation-delay: 60ms; }
  .review-card:nth-child(4) { animation-delay: 90ms; }
  .review-card:nth-child(5) { animation-delay: 120ms; }

  @keyframes review-slide-in {
    from { opacity: 0; transform: translateY(-4px); }
    to   { opacity: 1; transform: translateY(0); }
  }

  .review-card__meta {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .review-card__type {
    font-size: var(--text-2xs);
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--accent-text);
    background: var(--accent-subtle);
    padding: 2px 6px;
    border-radius: var(--radius-full);
  }

  .review-card__time {
    font-size: var(--text-2xs);
    color: var(--text-4);
    font-feature-settings: "tnum" 1;
  }

  .review-card__diff {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    font-size: var(--text-sm);
  }

  .review-card__original {
    color: var(--error);
    text-decoration: line-through;
    opacity: 0.8;
  }

  .review-card__suggested {
    color: var(--success);
    text-decoration: none;
  }

  .review-card__reason {
    font-size: var(--text-xs);
    color: var(--text-3);
    margin: 0;
    line-height: var(--leading-relaxed);
  }

  .review-card__actions {
    display: flex;
    gap: var(--space-2);
    margin-top: var(--space-1);
  }

  .review-card__btn {
    padding: var(--space-1) var(--space-3);
    border-radius: var(--radius-sm);
    font-size: var(--text-xs);
    font-weight: 500;
    cursor: pointer;
    border: 1px solid var(--border-default);
    background: var(--surface-3);
    color: var(--text-2);
    transition: background-color var(--duration-fast) var(--ease-out);
    font-family: var(--font-body);
  }

  .review-card__btn:hover { background: var(--surface-4); color: var(--text-1); }

  .review-card__btn--accept {
    background: oklch(from var(--success) l c h / 0.15);
    color: var(--success);
    border-color: oklch(from var(--success) l c h / 0.3);
  }

  .review-card__btn--accept:hover {
    background: oklch(from var(--success) l c h / 0.25);
  }

  .review-card__btn--reject {
    background: oklch(from var(--error) l c h / 0.15);
    color: var(--error);
    border-color: oklch(from var(--error) l c h / 0.3);
  }

  .review-card__btn--reject:hover {
    background: oklch(from var(--error) l c h / 0.25);
  }
</style>
