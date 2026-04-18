<script lang="ts">
  /**
   * HistoryEntry.svelte — single timeline card for one history commit.
   *
   * Rendered inside HistoryTimeline. The card shows:
   *   - Date in relative form ("2 days ago") with absolute on hover
   *   - Author avatar: initials circle, colour-coded by authorKind
   *   - Author label + optional agent badge
   *   - Prose summary from the backend
   *   - Word-delta summary (additions / removals)
   *   - Expand/collapse block-level changes (ChangeDiff instances)
   *
   * The timeline rail (vertical line + node dot) is rendered by the parent
   * HistoryTimeline so spacing between entries remains consistent.
   */

  import { onMount } from 'svelte';
  import type { HistoryEntry, AuthorKind } from './types.js';
  import ChangeDiff from './ChangeDiff.svelte';

  interface Props {
    entry: HistoryEntry;
    /** Whether the change list is expanded. Controlled externally. */
    expanded?: boolean;
    /** Called when the user toggles the expanded state. */
    onexpandedchange?: (value: boolean) => void;
  }

  let { entry, expanded = false, onexpandedchange }: Props = $props();

  // ---------------------------------------------------------------------------
  // Relative date
  // ---------------------------------------------------------------------------

  let relativeDate = $state('');
  let absoluteDate = $state('');

  function formatRelative(iso: string): string {
    const now = Date.now();
    const then = new Date(iso).getTime();
    const diffMs = now - then;
    const diffSec = Math.floor(diffMs / 1000);
    const diffMin = Math.floor(diffSec / 60);
    const diffHour = Math.floor(diffMin / 60);
    const diffDay = Math.floor(diffHour / 24);
    const diffWeek = Math.floor(diffDay / 7);
    const diffMonth = Math.floor(diffDay / 30);
    const diffYear = Math.floor(diffDay / 365);

    if (diffSec < 60) return 'just now';
    if (diffMin < 60) return `${diffMin} minute${diffMin === 1 ? '' : 's'} ago`;
    if (diffHour < 24) return `${diffHour} hour${diffHour === 1 ? '' : 's'} ago`;
    if (diffDay === 1) return 'yesterday';
    if (diffDay < 7) return `${diffDay} days ago`;
    if (diffWeek === 1) return 'last week';
    if (diffWeek < 5) return `${diffWeek} weeks ago`;
    if (diffMonth === 1) return 'last month';
    if (diffMonth < 12) return `${diffMonth} months ago`;
    if (diffYear === 1) return 'last year';
    return `${diffYear} years ago`;
  }

  function formatAbsolute(iso: string): string {
    try {
      return new Intl.DateTimeFormat(undefined, {
        year: 'numeric', month: 'short', day: 'numeric',
        hour: '2-digit', minute: '2-digit',
      }).format(new Date(iso));
    } catch {
      return iso;
    }
  }

  onMount(() => {
    relativeDate = formatRelative(entry.date);
    absoluteDate = formatAbsolute(entry.date);
    // Refresh every minute for entries within the last hour.
    const diffMs = Date.now() - new Date(entry.date).getTime();
    if (diffMs < 60 * 60 * 1000) {
      const tid = setInterval(() => {
        relativeDate = formatRelative(entry.date);
      }, 60_000);
      return () => clearInterval(tid);
    }
  });

  // ---------------------------------------------------------------------------
  // Author avatar: initials + colour
  // ---------------------------------------------------------------------------

  const AUTHOR_KIND_COLORS: Record<AuthorKind, string> = {
    'human':       'var(--accent-solid)',
    'claude-code': 'var(--provider-claude)',
    'copilot':     'oklch(72% 0.15 145)',  /* green */
    'codex':       'var(--provider-codex)',
    'gemini':      'var(--provider-gemini)',
    'vedox-agent': 'var(--warning)',
  };

  const AUTHOR_KIND_LABELS: Record<AuthorKind, string | null> = {
    'human':       null,
    'claude-code': 'claude code',
    'copilot':     'copilot',
    'codex':       'codex',
    'gemini':      'gemini',
    'vedox-agent': 'vedox agent',
  };

  const avatarColor = $derived(
    AUTHOR_KIND_COLORS[entry.authorKind] ?? 'var(--color-text-muted)'
  );

  const agentLabel = $derived(AUTHOR_KIND_LABELS[entry.authorKind]);

  const initials = $derived(() => {
    const name = entry.author.trim();
    if (!name) return '?';
    const parts = name.split(/\s+/);
    if (parts.length >= 2) {
      return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
    }
    return name.slice(0, 2).toUpperCase();
  });

  // ---------------------------------------------------------------------------
  // Word delta stats from changes (approximate — character-based proxy)
  // ---------------------------------------------------------------------------

  const wordDelta = $derived.by(() => {
    let added = 0;
    let removed = 0;
    for (const ch of entry.changes) {
      const after  = ch.after?.trim()  ?? '';
      const before = ch.before?.trim() ?? '';
      if (ch.type === 'added')    added   += wordCount(after);
      if (ch.type === 'removed')  removed += wordCount(before);
      if (ch.type === 'modified') {
        added   += wordCount(after);
        removed += wordCount(before);
      }
    }
    return { added, removed };
  });

  function wordCount(text: string): number {
    return text ? text.split(/\s+/).filter(Boolean).length : 0;
  }

  const hasChanges = $derived(entry.changes.length > 0);
  const changeCount = $derived(entry.changes.length);

  function toggleExpanded() {
    onexpandedchange?.(!expanded);
  }
</script>

<article class="history-entry" aria-label="Commit by {entry.author} {relativeDate || entry.date}">
  <!-- ── Avatar ─────────────────────────────────────────────────────────── -->
  <div
    class="history-entry__avatar"
    style:--avatar-color={avatarColor}
    role="img"
    aria-label="{entry.author}, {entry.authorKind}"
  >
    {initials()}
  </div>

  <!-- ── Card body ──────────────────────────────────────────────────────── -->
  <div class="history-entry__card">
    <!-- ── Author + date row ──────────────────────────────────────────── -->
    <div class="history-entry__header">
      <div class="history-entry__author-row">
        <span class="history-entry__author">{entry.author}</span>
        {#if agentLabel}
          <span class="history-entry__agent-badge" aria-label="AI agent: {agentLabel}">
            <!-- Robot icon -->
            <svg
              width="9"
              height="9"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
              aria-hidden="true"
            >
              <rect x="3" y="8" width="18" height="12" rx="2"/>
              <path d="M12 8V4"/>
              <circle cx="12" cy="4" r="1"/>
              <line x1="8" y1="13" x2="8" y2="14"/>
              <line x1="16" y1="13" x2="16" y2="14"/>
              <line x1="9" y1="17" x2="15" y2="17"/>
            </svg>
            {agentLabel}
          </span>
        {/if}
      </div>

      <time
        class="history-entry__date"
        datetime={entry.date}
        title={absoluteDate}
      >
        {relativeDate || absoluteDate}
      </time>
    </div>

    <!-- ── Prose summary ─────────────────────────────────────────────── -->
    <p class="history-entry__summary">{entry.summary || entry.message}</p>

    <!-- ── Word delta ────────────────────────────────────────────────── -->
    {#if wordDelta.added > 0 || wordDelta.removed > 0}
      <div class="history-entry__delta" aria-label="Word changes">
        {#if wordDelta.added > 0}
          <span class="history-entry__delta-added" aria-label="{wordDelta.added} words added">
            +{wordDelta.added}
          </span>
        {/if}
        {#if wordDelta.removed > 0}
          <span class="history-entry__delta-removed" aria-label="{wordDelta.removed} words removed">
            -{wordDelta.removed}
          </span>
        {/if}
      </div>
    {/if}

    <!-- ── Expand / collapse toggle ──────────────────────────────────── -->
    {#if hasChanges}
      <button
        class="history-entry__toggle"
        type="button"
        aria-expanded={expanded}
        aria-controls="entry-changes-{entry.commitHash}"
        onclick={toggleExpanded}
      >
        <svg
          class="history-entry__toggle-icon"
          class:history-entry__toggle-icon--open={expanded}
          width="10"
          height="10"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2.5"
          stroke-linecap="round"
          stroke-linejoin="round"
          aria-hidden="true"
        >
          <polyline points="9 18 15 12 9 6" />
        </svg>
        {expanded ? 'Hide' : 'View'} {changeCount} change{changeCount === 1 ? '' : 's'}
      </button>

      <!-- ── Change diff list ────────────────────────────────────────── -->
      {#if expanded}
        <ul
          id="entry-changes-{entry.commitHash}"
          class="history-entry__changes"
          aria-label="Block changes in this commit"
        >
          {#each entry.changes as change, i (i)}
            <li class="history-entry__change-item">
              <ChangeDiff {change} />
            </li>
          {/each}
        </ul>
      {/if}
    {/if}
  </div>
</article>

<style>
  /* ── Layout: avatar + card side-by-side ─────────────────────────────── */

  .history-entry {
    display: grid;
    grid-template-columns: 28px 1fr;
    gap: var(--space-3);
    align-items: start;
  }

  /* ── Avatar ──────────────────────────────────────────────────────────── */

  .history-entry__avatar {
    width: 28px;
    height: 28px;
    border-radius: var(--radius-full);
    background-color: color-mix(in oklch, var(--avatar-color) 18%, var(--surface-3));
    border: 1.5px solid color-mix(in oklch, var(--avatar-color) 40%, transparent);
    color: var(--avatar-color);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 10px;
    font-family: var(--font-mono);
    font-weight: 700;
    letter-spacing: 0;
    flex-shrink: 0;
    user-select: none;
    /* Align with the first line of text in the card */
    margin-top: 2px;
  }

  /* ── Card ────────────────────────────────────────────────────────────── */

  .history-entry__card {
    background-color: var(--color-surface-base);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    padding: var(--space-3) var(--space-4);
    min-width: 0;
    transition: border-color var(--duration-fast) var(--ease-out);
  }

  .history-entry__card:hover {
    border-color: var(--color-border-strong);
  }

  /* ── Header ──────────────────────────────────────────────────────────── */

  .history-entry__header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: var(--space-3);
    margin-bottom: var(--space-2);
    flex-wrap: wrap;
  }

  .history-entry__author-row {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex-wrap: wrap;
    min-width: 0;
  }

  .history-entry__author {
    font-size: var(--text-sm);
    font-weight: 600;
    color: var(--color-text-primary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  /* ── Agent badge ─────────────────────────────────────────────────────── */

  .history-entry__agent-badge {
    display: inline-flex;
    align-items: center;
    gap: 3px;
    padding: 1px 6px;
    background-color: var(--color-surface-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-full);
    font-size: 10px;
    font-family: var(--font-mono);
    font-weight: 500;
    color: var(--color-text-muted);
    flex-shrink: 0;
    white-space: nowrap;
  }

  /* ── Date ────────────────────────────────────────────────────────────── */

  .history-entry__date {
    font-size: var(--text-caption);
    font-family: var(--font-mono);
    color: var(--color-text-muted);
    white-space: nowrap;
    flex-shrink: 0;
    cursor: default;
  }

  /* ── Summary ─────────────────────────────────────────────────────────── */

  .history-entry__summary {
    margin: 0 0 var(--space-2) 0;
    font-size: var(--text-sm);
    color: var(--color-text-secondary);
    line-height: var(--leading-snug);
  }

  /* ── Word delta ──────────────────────────────────────────────────────── */

  .history-entry__delta {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    margin-bottom: var(--space-2);
  }

  .history-entry__delta-added {
    font-size: var(--text-caption);
    font-family: var(--font-mono);
    font-weight: 600;
    color: var(--success);
  }

  .history-entry__delta-removed {
    font-size: var(--text-caption);
    font-family: var(--font-mono);
    font-weight: 600;
    color: var(--error);
  }

  /* ── Toggle button ───────────────────────────────────────────────────── */

  .history-entry__toggle {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    padding: 0;
    background: none;
    border: none;
    color: var(--color-accent);
    font-size: var(--text-caption);
    font-family: var(--font-mono);
    cursor: pointer;
    text-decoration: none;
    transition: color var(--duration-fast) var(--ease-out);
  }

  .history-entry__toggle:hover {
    color: var(--accent-solid-hover);
  }

  .history-entry__toggle:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
    border-radius: var(--radius-sm);
  }

  .history-entry__toggle-icon {
    transition: transform var(--duration-fast) var(--ease-out);
    transform: rotate(0deg);
  }

  .history-entry__toggle-icon--open {
    transform: rotate(90deg);
  }

  /* ── Change list ─────────────────────────────────────────────────────── */

  .history-entry__changes {
    list-style: none;
    padding: 0;
    margin: var(--space-3) 0 0 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    animation: entry-expand var(--duration-default) var(--ease-out) both;
  }

  .history-entry__change-item {
    display: block;
  }

  @keyframes entry-expand {
    from {
      opacity: 0;
      transform: translateY(-4px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }

  /* ── Reduced motion ──────────────────────────────────────────────────── */

  @media (prefers-reduced-motion: reduce) {
    .history-entry__toggle-icon,
    .history-entry__card {
      transition: none;
    }

    .history-entry__changes {
      animation: none;
    }
  }
</style>
