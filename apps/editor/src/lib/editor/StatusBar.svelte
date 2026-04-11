<!--
  StatusBar.svelte

  Bottom 24px status strip for the editor. Shows:
    - Left: doc path
    - Center: word count + reading time
    - Right: cursor position + git branch (dirty indicator)

  Uses JetBrains Mono at 11px with tabular figures. Hairline top border.

  Props:
    content: string         — full markdown including frontmatter
    cursorLine?: number
    cursorCol?: number
    projectId?: string
    docPath?: string
-->

<script lang="ts">
  import { onMount } from 'svelte';
  import { fetchGitStatus, type GitStatus } from '$lib/api/git-status';

  interface Props {
    content: string;
    cursorLine?: number;
    cursorCol?: number;
    projectId?: string;
    docPath?: string;
  }

  let {
    content,
    cursorLine = 1,
    cursorCol = 1,
    projectId = '',
    docPath = ''
  }: Props = $props();

  // ---------------------------------------------------------------------------
  // Derived: word count (excludes frontmatter and code blocks)
  // ---------------------------------------------------------------------------

  const wordCount = $derived.by(() => {
    let text = content;
    // Strip frontmatter
    text = text.replace(/^---\n[\s\S]*?\n---\n?/, '');
    // Strip fenced code blocks
    text = text.replace(/```[\s\S]*?```/g, '');
    // Strip inline code
    text = text.replace(/`[^`]*`/g, '');
    // Strip markdown syntax (minimal)
    text = text.replace(/[#>*_\[\]()]/g, ' ');
    // Count whitespace-separated runs
    const words = text.trim().split(/\s+/).filter((w) => w.length > 0);
    return words.length;
  });

  // ---------------------------------------------------------------------------
  // Derived: reading time (~200 words/min)
  // ---------------------------------------------------------------------------

  const readingTime = $derived(() => {
    const minutes = Math.max(1, Math.round(wordCount / 200));
    return `${minutes} min`;
  });

  // ---------------------------------------------------------------------------
  // Git status (fetched on mount + on projectId change)
  // ---------------------------------------------------------------------------

  let gitStatus = $state<GitStatus | null>(null);
  let gitError = $state(false);

  async function loadGitStatus(): Promise<void> {
    if (!projectId) return;
    try {
      gitStatus = await fetchGitStatus(projectId);
      gitError = false;
    } catch {
      gitError = true;
      gitStatus = null;
    }
  }

  $effect(() => {
    // Refetch whenever projectId changes
    if (projectId) {
      loadGitStatus();
    }
  });

  onMount(() => {
    loadGitStatus();
    // Refetch every 30s to pick up branch changes from outside the app
    const interval = setInterval(loadGitStatus, 30_000);
    return () => clearInterval(interval);
  });
</script>

<div class="status-bar" role="contentinfo" aria-label="Document status">
  <!-- Left: doc path -->
  <div class="status-bar__section status-bar__section--left">
    {#if docPath}
      <span class="status-bar__path" title={docPath}>{docPath}</span>
    {/if}
  </div>

  <!-- Center: word count + reading time -->
  <div class="status-bar__section status-bar__section--center">
    <span class="status-bar__item">
      <span class="status-bar__value">{wordCount.toLocaleString()}</span>
      <span class="status-bar__label">words</span>
    </span>
    <span class="status-bar__divider" aria-hidden="true">·</span>
    <span class="status-bar__item">
      <span class="status-bar__value">{readingTime()}</span>
      <span class="status-bar__label">read</span>
    </span>
  </div>

  <!-- Right: cursor + git -->
  <div class="status-bar__section status-bar__section--right">
    <span class="status-bar__item" title="Cursor position">
      <span class="status-bar__label">Ln</span>
      <span class="status-bar__value">{cursorLine}</span>
      <span class="status-bar__label">Col</span>
      <span class="status-bar__value">{cursorCol}</span>
    </span>

    {#if gitStatus}
      <span class="status-bar__divider" aria-hidden="true">·</span>
      <span
        class="status-bar__item status-bar__git"
        class:status-bar__git--dirty={gitStatus.dirty}
        title={gitStatus.dirty
          ? `${gitStatus.branch} (uncommitted changes)`
          : gitStatus.branch}
      >
        <svg
          width="11"
          height="11"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
          aria-hidden="true"
        >
          <circle cx="12" cy="18" r="3" />
          <circle cx="6" cy="6" r="3" />
          <path d="M6 21V9a9 9 0 0 0 9 9" />
        </svg>
        <span class="status-bar__value">{gitStatus.branch}</span>
        {#if gitStatus.dirty}<span class="status-bar__dirty-dot" aria-hidden="true"></span>{/if}
        {#if gitStatus.ahead > 0}
          <span class="status-bar__ahead" title="Commits ahead of remote">↑{gitStatus.ahead}</span>
        {/if}
        {#if gitStatus.behind > 0}
          <span class="status-bar__behind" title="Commits behind remote">↓{gitStatus.behind}</span>
        {/if}
      </span>
    {:else if gitError}
      <span class="status-bar__divider" aria-hidden="true">·</span>
      <span class="status-bar__item status-bar__git--error" title="Git status unavailable">
        no git
      </span>
    {/if}
  </div>
</div>

<style>
  .status-bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    height: 24px;
    flex-shrink: 0;
    padding: 0 var(--space-4, 16px);
    background: var(--surface-2, #1a1a1a);
    border-top: 1px solid var(--border-hairline, rgba(255, 255, 255, 0.06));
    font-family: var(--font-mono, 'JetBrains Mono', ui-monospace, monospace);
    font-size: 11px;
    line-height: 1;
    color: var(--text-3, rgba(255, 255, 255, 0.5));
    font-feature-settings: 'tnum' 1, 'zero' 1, 'ss01' 1;
    user-select: none;
    gap: var(--space-3, 12px);
  }

  .status-bar__section {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
  }

  .status-bar__section--left {
    flex: 1;
    min-width: 0;
  }

  .status-bar__section--center {
    flex-shrink: 0;
  }

  .status-bar__section--right {
    flex: 1;
    justify-content: flex-end;
    min-width: 0;
  }

  .status-bar__path {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    color: var(--text-3, rgba(255, 255, 255, 0.5));
  }

  .status-bar__item {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    white-space: nowrap;
  }

  .status-bar__value {
    color: var(--text-2, rgba(255, 255, 255, 0.7));
    font-variant-numeric: tabular-nums;
  }

  .status-bar__label {
    color: var(--text-3, rgba(255, 255, 255, 0.4));
  }

  .status-bar__divider {
    color: var(--text-3, rgba(255, 255, 255, 0.3));
    opacity: 0.5;
  }

  .status-bar__git {
    color: var(--text-2, rgba(255, 255, 255, 0.7));
  }

  .status-bar__git--dirty {
    color: var(--warning, #f59e0b);
  }

  .status-bar__git--error {
    color: var(--text-3, rgba(255, 255, 255, 0.3));
    font-style: italic;
  }

  .status-bar__dirty-dot {
    display: inline-block;
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--warning, #f59e0b);
    margin-left: 2px;
  }

  .status-bar__ahead,
  .status-bar__behind {
    color: var(--text-3, rgba(255, 255, 255, 0.5));
    font-variant-numeric: tabular-nums;
    margin-left: 2px;
  }
</style>
