<script lang="ts">
  /**
   * HistoryTimeline.svelte — vertical doc history timeline.
   *
   * Fetches from GET /api/projects/{project}/docs/{docPath}/history on mount
   * (lazy — only fires when the component is first rendered, i.e. on tab activation).
   *
   * Layout:
   *   Left: vertical rail with node dots on the line
   *   Right: HistoryEntry cards
   *
   * Filter controls at the top:
   *   - Author type (all / human / agent — groups all non-human kinds)
   *   - Date range (since / until — optional)
   *
   * Accessibility:
   *   - The timeline is a <ol> (ordered — most recent first)
   *   - Each entry is a <li> containing an <article>
   *   - Filter controls are labelled form elements
   *   - Loading/empty/error states use role="status" / role="alert"
   */

  import { onMount } from 'svelte';
  import { api } from '$lib/api/client.js';
  import type { HistoryEntry, AuthorKind } from './types.js';
  import HistoryEntryCard from './HistoryEntry.svelte';

  interface Props {
    projectId: string;
    docPath: string;
  }

  let { projectId, docPath }: Props = $props();

  // ---------------------------------------------------------------------------
  // Data fetch state
  // ---------------------------------------------------------------------------

  type LoadState = 'idle' | 'loading' | 'done' | 'error';

  let loadState: LoadState = $state('idle');
  let entries: HistoryEntry[] = $state([]);
  let errorMessage: string = $state('');

  async function loadHistory(): Promise<void> {
    if (loadState === 'loading') return;
    loadState = 'loading';
    errorMessage = '';
    try {
      const path = docPath.split('/').map(encodeURIComponent).join('/');
      const result = await fetch(
        `/api/projects/${encodeURIComponent(projectId)}/docs/${path}/history?limit=50`,
        { headers: { Accept: 'application/json' } },
      );
      if (!result.ok) {
        const body = await result.json().catch(() => ({})) as Record<string, unknown>;
        throw new Error(
          typeof body?.message === 'string' ? body.message : `HTTP ${result.status}`,
        );
      }
      const data = await result.json() as HistoryEntry[];
      entries = Array.isArray(data) ? data : [];
      loadState = 'done';
    } catch (err) {
      errorMessage = err instanceof Error ? err.message : 'Could not load history.';
      loadState = 'error';
    }
  }

  onMount(() => {
    loadHistory();
  });

  // ---------------------------------------------------------------------------
  // Filter state
  // ---------------------------------------------------------------------------

  let filterAuthorKind: 'all' | 'human' | 'agent' = $state('all');
  let filterSince: string = $state('');
  let filterUntil: string = $state('');

  const filteredEntries = $derived.by((): HistoryEntry[] => {
    let result = entries;

    if (filterAuthorKind === 'human') {
      result = result.filter((e) => e.authorKind === 'human');
    } else if (filterAuthorKind === 'agent') {
      result = result.filter((e) => e.authorKind !== 'human');
    }

    if (filterSince) {
      const since = new Date(filterSince).getTime();
      result = result.filter((e) => new Date(e.date).getTime() >= since);
    }

    if (filterUntil) {
      // Use end-of-day for the "until" date.
      const until = new Date(filterUntil);
      until.setHours(23, 59, 59, 999);
      result = result.filter((e) => new Date(e.date).getTime() <= until.getTime());
    }

    return result;
  });

  const hasEntries = $derived(filteredEntries.length > 0);
  const totalCount = $derived(entries.length);
  const filteredCount = $derived(filteredEntries.length);
  const isFiltered = $derived(
    filterAuthorKind !== 'all' || filterSince !== '' || filterUntil !== ''
  );

  function clearFilters(): void {
    filterAuthorKind = 'all';
    filterSince = '';
    filterUntil = '';
  }

  // ---------------------------------------------------------------------------
  // Expand-all / collapse-all
  // ---------------------------------------------------------------------------

  let expandedMap: Record<string, boolean> = $state({});

  function isExpanded(hash: string): boolean {
    return expandedMap[hash] ?? false;
  }

  function setExpanded(hash: string, value: boolean): void {
    expandedMap = { ...expandedMap, [hash]: value };
  }
</script>

<section class="history-timeline" aria-label="Document history">
  <!-- ── Toolbar ──────────────────────────────────────────────────────── -->
  <div class="history-timeline__toolbar" role="group" aria-label="History filters">
    <!-- Author type filter -->
    <div class="history-timeline__filter-group">
      <label class="history-timeline__filter-label" for="ht-author-filter">Author</label>
      <div class="history-timeline__segmented" role="radiogroup" aria-label="Filter by author type">
        {#each [['all', 'All'], ['human', 'Human'], ['agent', 'Agent']] as [val, label] (val)}
          <button
            class="history-timeline__seg-btn"
            class:history-timeline__seg-btn--active={filterAuthorKind === val}
            type="button"
            role="radio"
            aria-checked={filterAuthorKind === val}
            onclick={() => { filterAuthorKind = val as typeof filterAuthorKind; }}
          >
            {label}
          </button>
        {/each}
      </div>
    </div>

    <!-- Date range -->
    <div class="history-timeline__filter-group">
      <label class="history-timeline__filter-label" for="ht-since">Since</label>
      <input
        id="ht-since"
        class="history-timeline__date-input"
        type="date"
        bind:value={filterSince}
        aria-label="Show history since date"
        max={filterUntil || undefined}
      />
    </div>

    <div class="history-timeline__filter-group">
      <label class="history-timeline__filter-label" for="ht-until">Until</label>
      <input
        id="ht-until"
        class="history-timeline__date-input"
        type="date"
        bind:value={filterUntil}
        aria-label="Show history until date"
        min={filterSince || undefined}
      />
    </div>

    {#if isFiltered}
      <button
        class="history-timeline__clear-btn"
        type="button"
        onclick={clearFilters}
        aria-label="Clear all filters"
      >
        clear filters
      </button>
    {/if}

    <!-- Refresh -->
    <button
      class="history-timeline__refresh-btn"
      type="button"
      aria-label="Reload history"
      disabled={loadState === 'loading'}
      onclick={loadHistory}
    >
      <svg
        class="history-timeline__refresh-icon"
        class:history-timeline__refresh-icon--spinning={loadState === 'loading'}
        width="12"
        height="12"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2.5"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"
      >
        <polyline points="23 4 23 10 17 10" />
        <path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10" />
      </svg>
    </button>
  </div>

  <!-- ── Count strip ──────────────────────────────────────────────────── -->
  {#if loadState === 'done' && totalCount > 0}
    <p class="history-timeline__count" aria-live="polite">
      {#if isFiltered}
        {filteredCount} of {totalCount} commit{totalCount === 1 ? '' : 's'}
      {:else}
        {totalCount} commit{totalCount === 1 ? '' : 's'}
      {/if}
    </p>
  {/if}

  <!-- ── Loading state ────────────────────────────────────────────────── -->
  {#if loadState === 'idle' || loadState === 'loading'}
    <div class="history-timeline__loading" role="status" aria-live="polite" aria-busy="true">
      <span class="history-timeline__spinner" aria-hidden="true"></span>
      <span>Loading history…</span>
    </div>

  <!-- ── Error state ──────────────────────────────────────────────────── -->
  {:else if loadState === 'error'}
    <div class="history-timeline__error" role="alert">
      <svg
        width="16"
        height="16"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="1.5"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"
      >
        <circle cx="12" cy="12" r="10"/>
        <line x1="12" y1="8" x2="12" y2="12"/>
        <line x1="12" y1="16" x2="12.01" y2="16"/>
      </svg>
      <span>{errorMessage}</span>
      <button
        class="history-timeline__retry-btn"
        type="button"
        onclick={loadHistory}
      >
        Retry
      </button>
    </div>

  <!-- ── Empty state ──────────────────────────────────────────────────── -->
  {:else if loadState === 'done' && !hasEntries}
    <div class="history-timeline__empty" role="status">
      <svg
        width="32"
        height="32"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="1.2"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"
      >
        <circle cx="12" cy="12" r="10"/>
        <polyline points="12 6 12 12 16 14"/>
      </svg>
      {#if isFiltered}
        <p>No commits match the current filters.</p>
        <button
          class="history-timeline__clear-btn history-timeline__clear-btn--standalone"
          type="button"
          onclick={clearFilters}
        >
          clear filters
        </button>
      {:else}
        <p>No history yet. Commit this document to start tracking changes.</p>
      {/if}
    </div>

  <!-- ── Timeline ─────────────────────────────────────────────────────── -->
  {:else}
    <div class="history-timeline__rail-wrap">
      <!-- Vertical rail line runs behind all entries -->
      <div class="history-timeline__rail" aria-hidden="true"></div>

      <ol
        class="history-timeline__list"
        aria-label="Commit history, most recent first"
      >
        {#each filteredEntries as entry (entry.commitHash)}
          <li class="history-timeline__item">
            <!-- Rail node dot -->
            <div
              class="history-timeline__node"
              class:history-timeline__node--agent={entry.authorKind !== 'human'}
              aria-hidden="true"
            ></div>

            <!-- Entry card -->
            <HistoryEntryCard
              {entry}
              expanded={isExpanded(entry.commitHash)}
              onexpandedchange={(v) => setExpanded(entry.commitHash, v)}
            />
          </li>
        {/each}
      </ol>
    </div>
  {/if}
</section>

<style>
  /* ── Section container ───────────────────────────────────────────────── */

  .history-timeline {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    padding: var(--space-4) var(--space-5);
    min-height: 0;
    overflow-y: auto;
    overflow-x: hidden;
  }

  /* ── Toolbar ─────────────────────────────────────────────────────────── */

  .history-timeline__toolbar {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    flex-wrap: wrap;
    padding-bottom: var(--space-3);
    border-bottom: 1px solid var(--color-border);
  }

  .history-timeline__filter-group {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }

  .history-timeline__filter-label {
    font-size: var(--text-caption);
    font-family: var(--font-mono);
    color: var(--color-text-muted);
    white-space: nowrap;
    font-weight: 500;
    letter-spacing: var(--tracking-wide);
    text-transform: uppercase;
  }

  /* ── Segmented control ───────────────────────────────────────────────── */

  .history-timeline__segmented {
    display: flex;
    align-items: center;
    background-color: var(--color-surface-base);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    overflow: hidden;
  }

  .history-timeline__seg-btn {
    padding: 3px var(--space-3);
    background: none;
    border: none;
    font-size: var(--text-caption);
    font-family: var(--font-mono);
    color: var(--color-text-muted);
    cursor: pointer;
    transition:
      background-color var(--duration-fast) var(--ease-out),
      color var(--duration-fast) var(--ease-out);
    white-space: nowrap;
  }

  .history-timeline__seg-btn + .history-timeline__seg-btn {
    border-left: 1px solid var(--color-border);
  }

  .history-timeline__seg-btn--active {
    background-color: var(--color-accent-subtle);
    color: var(--color-accent);
    font-weight: 600;
  }

  .history-timeline__seg-btn:hover:not(.history-timeline__seg-btn--active) {
    background-color: var(--color-surface-elevated);
    color: var(--color-text-secondary);
  }

  .history-timeline__seg-btn:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: -2px;
  }

  /* ── Date inputs ─────────────────────────────────────────────────────── */

  .history-timeline__date-input {
    height: 26px;
    padding: 0 var(--space-2);
    background-color: var(--color-surface-base);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    color: var(--color-text-secondary);
    font-size: var(--text-caption);
    font-family: var(--font-mono);
    appearance: none;
    transition: border-color var(--duration-fast) var(--ease-out);
    /* Remove browser default date icon in webkit */
    -webkit-appearance: none;
  }

  .history-timeline__date-input:focus {
    outline: none;
    border-color: var(--color-accent);
  }

  .history-timeline__date-input::-webkit-calendar-picker-indicator {
    filter: invert(60%) sepia(0%) saturate(0%) brightness(90%);
    cursor: pointer;
  }

  /* ── Utility buttons ─────────────────────────────────────────────────── */

  .history-timeline__clear-btn {
    padding: 0;
    background: none;
    border: none;
    color: var(--color-accent);
    font-size: var(--text-caption);
    font-family: var(--font-mono);
    cursor: pointer;
    text-decoration: underline;
    text-underline-offset: 2px;
    white-space: nowrap;
  }

  .history-timeline__clear-btn:hover {
    color: var(--accent-solid-hover);
  }

  .history-timeline__clear-btn:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
    border-radius: var(--radius-sm);
  }

  .history-timeline__clear-btn--standalone {
    margin-top: var(--space-1);
  }

  .history-timeline__refresh-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 26px;
    height: 26px;
    padding: 0;
    background-color: var(--color-surface-base);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    color: var(--color-text-muted);
    cursor: pointer;
    transition:
      background-color var(--duration-fast) var(--ease-out),
      color var(--duration-fast) var(--ease-out),
      border-color var(--duration-fast) var(--ease-out);
    margin-left: auto;
  }

  .history-timeline__refresh-btn:hover:not(:disabled) {
    background-color: var(--color-surface-elevated);
    color: var(--color-text-secondary);
    border-color: var(--color-border-strong);
  }

  .history-timeline__refresh-btn:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }

  .history-timeline__refresh-btn:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
  }

  .history-timeline__refresh-icon {
    flex-shrink: 0;
  }

  .history-timeline__refresh-icon--spinning {
    animation: ht-spin 700ms linear infinite;
  }

  @keyframes ht-spin {
    to { transform: rotate(360deg); }
  }

  /* ── Count strip ─────────────────────────────────────────────────────── */

  .history-timeline__count {
    margin: 0;
    font-size: var(--text-caption);
    font-family: var(--font-mono);
    color: var(--color-text-muted);
  }

  /* ── Loading state ───────────────────────────────────────────────────── */

  .history-timeline__loading {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-6) 0;
    color: var(--color-text-muted);
    font-size: var(--text-sm);
    font-family: var(--font-mono);
  }

  .history-timeline__spinner {
    display: inline-block;
    width: 14px;
    height: 14px;
    border: 2px solid var(--color-border);
    border-top-color: var(--color-accent);
    border-radius: 50%;
    animation: ht-spin 600ms linear infinite;
    flex-shrink: 0;
  }

  /* ── Error state ─────────────────────────────────────────────────────── */

  .history-timeline__error {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-4);
    background-color: color-mix(in oklch, var(--error) 8%, transparent);
    border: 1px solid color-mix(in oklch, var(--error) 25%, transparent);
    border-radius: var(--radius-lg);
    font-size: var(--text-sm);
    color: var(--error);
  }

  .history-timeline__error svg {
    flex-shrink: 0;
  }

  .history-timeline__error span {
    flex: 1;
    min-width: 0;
  }

  .history-timeline__retry-btn {
    padding: 2px var(--space-3);
    background-color: color-mix(in oklch, var(--error) 12%, transparent);
    border: 1px solid color-mix(in oklch, var(--error) 30%, transparent);
    border-radius: var(--radius-md);
    color: var(--error);
    font-size: var(--text-caption);
    font-family: var(--font-mono);
    cursor: pointer;
    white-space: nowrap;
    flex-shrink: 0;
    transition:
      background-color var(--duration-fast) var(--ease-out);
  }

  .history-timeline__retry-btn:hover {
    background-color: color-mix(in oklch, var(--error) 22%, transparent);
  }

  .history-timeline__retry-btn:focus-visible {
    outline: 2px solid var(--error);
    outline-offset: 2px;
    border-radius: var(--radius-sm);
  }

  /* ── Empty state ─────────────────────────────────────────────────────── */

  .history-timeline__empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: var(--space-3);
    padding: var(--space-10) var(--space-6);
    text-align: center;
    color: var(--color-text-muted);
  }

  .history-timeline__empty svg {
    opacity: 0.4;
  }

  .history-timeline__empty p {
    margin: 0;
    font-size: var(--text-sm);
    font-family: var(--font-body);
    max-width: 280px;
    line-height: var(--leading-snug);
  }

  /* ── Rail wrapper ────────────────────────────────────────────────────── */

  .history-timeline__rail-wrap {
    position: relative;
  }

  /* The vertical rail line — positioned to the left of all entries */
  .history-timeline__rail {
    position: absolute;
    top: 14px;
    /* 14px = half of the node dot (28px item height) = centred on first node */
    left: 13px;
    /* Half of the 28px avatar column = centred on the dot */
    width: 1px;
    bottom: 14px;
    background-color: var(--color-border);
  }

  /* ── List ────────────────────────────────────────────────────────────── */

  .history-timeline__list {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  /* ── Each item: node dot + entry card ────────────────────────────────── */

  .history-timeline__item {
    display: grid;
    grid-template-columns: 28px 1fr;
    gap: var(--space-3);
    align-items: start;
    position: relative;
  }

  .history-timeline__node {
    width: 10px;
    height: 10px;
    border-radius: var(--radius-full);
    background-color: var(--color-border-strong);
    border: 2px solid var(--color-surface-base);
    flex-shrink: 0;
    /* Centre the dot within the 28px column, align with card's first line */
    margin-top: 9px;
    margin-left: 9px;
    position: relative;
    z-index: 1;
    transition: background-color var(--duration-fast) var(--ease-out);
  }

  /* Agent commits get an accent-tinted node */
  .history-timeline__node--agent {
    background-color: var(--color-accent);
  }

  /* ── Reduced motion ──────────────────────────────────────────────────── */

  @media (prefers-reduced-motion: reduce) {
    .history-timeline__spinner,
    .history-timeline__refresh-icon--spinning {
      animation: none;
      opacity: 0.6;
    }

    .history-timeline__seg-btn,
    .history-timeline__node,
    .history-timeline__refresh-btn,
    .history-timeline__retry-btn {
      transition: none;
    }
  }
</style>
