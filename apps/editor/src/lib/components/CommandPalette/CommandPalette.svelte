<!--
  CommandPalette.svelte — global Cmd+K modal.

  Layout
  ------
  ┌────────────────────────────────────────────────────────────────────────┐
  │  [input]                                                               │
  │────────────────────────────────────────────────────────────────────────│
  │                │                                                       │
  │  result list   │           live preview of selected result             │
  │   (40%)        │                     (60%)                             │
  │                │                                                       │
  │────────────────┴───────────────────────────────────────────────────────│
  │  ↑↓ navigate    ↵ open    ⌘P preview    esc close                      │
  └────────────────────────────────────────────────────────────────────────┘

  Behaviour
  ---------
  - `⌘K` / `Ctrl+K` opens and closes the modal (wired in store.ts).
  - Typing with no prefix searches the FTS backend (debounced 120ms).
  - Typing `>` enters command mode, `#` tag mode, `/` path mode.
  - ↑/↓ move the selected result; Enter activates it; Esc closes.
  - When the palette closes the query is cleared so the next open starts fresh.

  Accessibility
  -------------
  - The input is the first focusable element; a focus trap prevents Tab
    from escaping until the modal closes.
  - Overlay gets `role="dialog"` + `aria-modal="true"` + `aria-labelledby`.
  - Results list is `role="listbox"`; rows are `role="option"` with
    `aria-selected`.
  - Announces result count via an sr-only live region.

  Colours
  -------
  Every colour references a CSS variable from tokens.css. The creative agent
  is populating those variables in parallel; we reference them by name even
  though they don't exist at write time.
-->
<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/stores';
  import {
    openStore,
    queryStore,
    resultsStore,
    selectedIndexStore,
    modeStore,
    isLoadingStore,
    errorStore,
    scopeProjectStore,
    initPaletteShortcut,
    closePalette,
    setQuery,
    moveSelection,
    activateSelection,
    setScopeProject,
    type PaletteResult,
    type PaletteSearchHit,
    type PaletteMode,
  } from './store';

  // ---------------------------------------------------------------------------
  // Lifecycle: shortcut + scope sync
  // ---------------------------------------------------------------------------

  let inputEl: HTMLInputElement | undefined = $state(undefined);
  let listEl: HTMLUListElement | undefined = $state(undefined);

  onMount(() => {
    const teardown = initPaletteShortcut();
    return () => teardown();
  });

  // Keep the palette's project scope in sync with the current URL. The
  // Vedox router uses `/projects/:project/docs/…` so we extract the project
  // segment from $page.params or $page.url.pathname as a fallback.
  $effect(() => {
    const project =
      ($page.params?.project as string | undefined) ??
      extractProjectFromPath($page.url?.pathname ?? '');
    setScopeProject(project ?? null);
  });

  function extractProjectFromPath(pathname: string): string | null {
    const match = /^\/projects\/([^/]+)/.exec(pathname);
    return match ? decodeURIComponent(match[1]) : null;
  }

  // ---------------------------------------------------------------------------
  // Focus management: autofocus input on open, restore focus on close
  // ---------------------------------------------------------------------------

  let previouslyFocused: HTMLElement | null = null;

  $effect(() => {
    if ($openStore) {
      previouslyFocused = document.activeElement as HTMLElement | null;
      // Wait for the modal to mount before focusing the input.
      void tick().then(() => {
        inputEl?.focus();
        // Select-all so the next keypress replaces any stale query.
        inputEl?.select();
      });
    } else {
      // Restore focus to whatever had it before the palette opened.
      previouslyFocused?.focus?.();
      previouslyFocused = null;
    }
  });

  // ---------------------------------------------------------------------------
  // Auto-scroll the highlighted row into view
  // ---------------------------------------------------------------------------
  $effect(() => {
    if (!$openStore) return;
    const idx = $selectedIndexStore;
    const host = listEl;
    if (!host) return;
    const row = host.querySelector<HTMLElement>(`[data-result-index="${idx}"]`);
    if (row) {
      row.scrollIntoView({ block: 'nearest' });
    }
  });

  // ---------------------------------------------------------------------------
  // Input handlers
  // ---------------------------------------------------------------------------
  function onInput(e: Event): void {
    const target = e.target as HTMLInputElement;
    setQuery(target.value);
  }

  async function onKeydown(e: KeyboardEvent): Promise<void> {
    switch (e.key) {
      case 'Escape':
        e.preventDefault();
        closePalette();
        setQuery('');
        return;
      case 'ArrowDown':
        e.preventDefault();
        moveSelection(1);
        return;
      case 'ArrowUp':
        e.preventDefault();
        moveSelection(-1);
        return;
      case 'Enter': {
        e.preventDefault();
        await activateSelection((href) => goto(href));
        setQuery('');
        return;
      }
      case 'Tab':
        // Focus trap: our only tabbable element is the input; swallow Tab
        // so focus stays inside the modal. Shift+Tab does the same.
        e.preventDefault();
        inputEl?.focus();
        return;
      default:
        return;
    }
  }

  function onBackdropClick(e: MouseEvent): void {
    // Only dismiss on clicks directly on the backdrop, not on bubble events
    // from inside the modal body.
    if (e.target === e.currentTarget) {
      closePalette();
      setQuery('');
    }
  }

  function onRowClick(index: number): void {
    selectedIndexStore.set(index);
    void activateSelection((href) => goto(href));
    setQuery('');
  }

  // ---------------------------------------------------------------------------
  // Derived view helpers
  // ---------------------------------------------------------------------------
  const placeholder = $derived(placeholderForMode($modeStore));
  const modeLabel = $derived(modeLabelFor($modeStore));
  const selectedResult = $derived<PaletteResult | null>(
    $resultsStore[$selectedIndexStore] ?? null,
  );

  function placeholderForMode(mode: PaletteMode): string {
    switch (mode) {
      case 'command':
        return '> Run a command…';
      case 'tag':
        return '# Filter by tag…';
      case 'path':
        return '/ Jump to path…';
      default:
        return 'Search docs, run commands...';
    }
  }

  function modeLabelFor(mode: PaletteMode): string {
    switch (mode) {
      case 'command':
        return 'Commands';
      case 'tag':
        return 'Tags';
      case 'path':
        return 'Paths';
      default:
        return 'Search';
    }
  }

  function formatScore(score: number): string {
    // Lower BM25 = better. Round to 2 sig figs for a compact badge.
    return Math.abs(score).toFixed(2);
  }

  /** Server already emits <mark> tags inside snippets. We trust its output. */
  function snippetHtml(raw: string): string {
    return raw ?? '';
  }

  function isSearchHit(r: PaletteResult | null): r is PaletteSearchHit {
    return !!r && r.kind === 'search';
  }
</script>

{#if $openStore}
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
  <div
    class="palette-backdrop"
    role="dialog"
    tabindex="-1"
    aria-modal="true"
    aria-labelledby="palette-title"
    onclick={onBackdropClick}
  >
    <div class="palette" role="document">
      <!-- sr-only title for the dialog accessible name -->
      <h2 id="palette-title" class="sr-only">Command palette</h2>

      <!-- Live region announcing result count to screen readers -->
      <div class="sr-only" aria-live="polite" aria-atomic="true">
        {$resultsStore.length} result{$resultsStore.length === 1 ? '' : 's'} for
        {$queryStore}
      </div>

      <!-- ─── Input row ─────────────────────────────────────────── -->
      <div class="palette__input-row">
        <svg
          class="palette__search-icon"
          width="16"
          height="16"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
          aria-hidden="true"
        >
          <circle cx="11" cy="11" r="8" />
          <line x1="21" y1="21" x2="16.65" y2="16.65" />
        </svg>
        <input
          bind:this={inputEl}
          type="text"
          class="palette__input"
          {placeholder}
          value={$queryStore}
          oninput={onInput}
          onkeydown={onKeydown}
          aria-label="Command palette query"
          aria-autocomplete="list"
          aria-controls="palette-results"
          aria-activedescendant={$resultsStore.length > 0
            ? `palette-result-${$selectedIndexStore}`
            : undefined}
          autocomplete="off"
          spellcheck="false"
          autocorrect="off"
        />
        <span class="palette__mode-badge" aria-hidden="true">{modeLabel}</span>
        {#if $isLoadingStore}
          <span class="palette__spinner" aria-hidden="true"></span>
        {/if}
      </div>

      <!-- ─── Body: list (40%) + preview (60%) ──────────────────── -->
      <div class="palette__body">
        <!-- Result list -->
        <ul
          bind:this={listEl}
          id="palette-results"
          class="palette__results"
          role="listbox"
          aria-label="Search results"
        >
          {#if $errorStore}
            <li class="palette__empty" role="presentation">
              {$errorStore}
            </li>
          {:else if $resultsStore.length === 0}
            <li class="palette__empty" role="presentation">
              {$queryStore.trim()
                ? 'No results. Try a different term.'
                : 'Type to search docs and commands.'}
            </li>
          {:else}
            {#each $resultsStore as result, i (result.kind + ':' + result.id)}
              <!-- svelte-ignore a11y_click_events_have_key_events -->
              <li
                id={`palette-result-${i}`}
                class="palette__result"
                class:palette__result--active={i === $selectedIndexStore}
                role="option"
                aria-selected={i === $selectedIndexStore}
                data-result-index={i}
                onclick={() => onRowClick(i)}
                onmouseenter={() => selectedIndexStore.set(i)}
              >
                {#if result.kind === 'search'}
                  <div class="palette__result-icon" aria-hidden="true">
                    <svg
                      width="14"
                      height="14"
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="currentColor"
                      stroke-width="1.75"
                      stroke-linecap="round"
                      stroke-linejoin="round"
                    >
                      <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
                      <polyline points="14 2 14 8 20 8" />
                    </svg>
                  </div>
                  <div class="palette__result-main">
                    <div class="palette__result-title">{result.title}</div>
                    <div class="palette__result-meta">
                      <span class="palette__result-path">{result.id}</span>
                      {#if result.type}
                        <span class="palette__result-chip">{result.type}</span>
                      {/if}
                      <span class="palette__result-score">{formatScore(result.score)}</span>
                    </div>
                  </div>
                {:else}
                  <div class="palette__result-icon palette__result-icon--cmd" aria-hidden="true">
                    {#if result.icon === 'sun'}
                      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round">
                        <circle cx="12" cy="12" r="5" />
                        <line x1="12" y1="1" x2="12" y2="3" />
                        <line x1="12" y1="21" x2="12" y2="23" />
                        <line x1="4.22" y1="4.22" x2="5.64" y2="5.64" />
                        <line x1="18.36" y1="18.36" x2="19.78" y2="19.78" />
                        <line x1="1" y1="12" x2="3" y2="12" />
                        <line x1="21" y1="12" x2="23" y2="12" />
                        <line x1="4.22" y1="19.78" x2="5.64" y2="18.36" />
                        <line x1="18.36" y1="5.64" x2="19.78" y2="4.22" />
                      </svg>
                    {:else if result.icon === 'moon'}
                      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round">
                        <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
                      </svg>
                    {:else if result.icon === 'refresh'}
                      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round">
                        <polyline points="23 4 23 10 17 10" />
                        <polyline points="1 20 1 14 7 14" />
                        <path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15" />
                      </svg>
                    {:else if result.icon === 'sidebar'}
                      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round">
                        <rect x="3" y="3" width="18" height="18" rx="2" ry="2" />
                        <line x1="9" y1="3" x2="9" y2="21" />
                      </svg>
                    {:else if result.icon === 'settings'}
                      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round">
                        <circle cx="12" cy="12" r="3" />
                        <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
                      </svg>
                    {:else if result.icon === 'split'}
                      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round">
                        <rect x="3" y="3" width="18" height="18" rx="2" ry="2" />
                        <line x1="12" y1="3" x2="12" y2="21" />
                      </svg>
                    {:else}
                      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round">
                        <polyline points="4 17 10 11 4 5" />
                        <line x1="12" y1="19" x2="20" y2="19" />
                      </svg>
                    {/if}
                  </div>
                  <div class="palette__result-main">
                    <div class="palette__result-title">{result.title}</div>
                    <div class="palette__result-meta">
                      <span class="palette__result-path">{result.description}</span>
                    </div>
                  </div>
                {/if}
              </li>
            {/each}
          {/if}
        </ul>

        <!-- Preview pane -->
        <aside class="palette__preview" aria-live="off">
          {#if isSearchHit(selectedResult)}
            <header class="palette__preview-header">
              <div class="palette__preview-title">{selectedResult.title}</div>
              <div class="palette__preview-path">{selectedResult.id}</div>
            </header>
            <div class="palette__preview-body">
              <div class="palette__preview-snippet">
                {@html snippetHtml(selectedResult.snippet)}
              </div>
              <p class="palette__preview-hint">
                Press <kbd>↵</kbd> to open in the editor.
              </p>
            </div>
          {:else if selectedResult && selectedResult.kind === 'command'}
            <header class="palette__preview-header">
              <div class="palette__preview-title">{selectedResult.title}</div>
              <div class="palette__preview-path">Command</div>
            </header>
            <div class="palette__preview-body">
              <p class="palette__preview-desc">{selectedResult.description}</p>
              <p class="palette__preview-hint">
                Press <kbd>↵</kbd> to run.
              </p>
            </div>
          {:else}
            <div class="palette__preview-empty">
              <p>Select a result to preview it here.</p>
            </div>
          {/if}
        </aside>
      </div>

      <!-- ─── Hint bar ──────────────────────────────────────────── -->
      <footer class="palette__hint-bar" aria-hidden="true">
        <span class="palette__hint"><kbd>↑</kbd><kbd>↓</kbd> navigate</span>
        <span class="palette__hint"><kbd>↵</kbd> open</span>
        <span class="palette__hint"><kbd>⌘</kbd><kbd>P</kbd> preview</span>
        <span class="palette__hint"><kbd>esc</kbd> close</span>
        {#if $scopeProjectStore}
          <span class="palette__hint palette__hint--scope">
            scope: <strong>{$scopeProjectStore}</strong>
          </span>
        {/if}
      </footer>
    </div>
  </div>
{/if}

<style>
  /* ── Backdrop ──────────────────────────────────────────────────────────── */
  .palette-backdrop {
    position: fixed;
    inset: 0;
    z-index: 1000;
    display: flex;
    justify-content: center;
    align-items: flex-start;
    padding-top: 12vh;
    background-color: var(
      --palette-backdrop,
      color-mix(in srgb, var(--color-surface-base, #000) 55%, transparent)
    );
    backdrop-filter: blur(6px);
    -webkit-backdrop-filter: blur(6px);
    animation: palette-fade-in var(--duration-fast) var(--ease-out);
  }

  @keyframes palette-fade-in {
    from {
      opacity: 0;
    }
    to {
      opacity: 1;
    }
  }

  /* ── Modal frame ───────────────────────────────────────────────────────── */
  .palette {
    width: min(720px, calc(100vw - 32px));
    max-height: 60vh;
    display: flex;
    flex-direction: column;
    background-color: var(--surface-1, var(--color-surface-elevated, #161b22));
    border: 1px solid var(--border-1, var(--color-border));
    border-radius: var(--radius-lg, 14px);
    box-shadow: var(
      --palette-shadow,
      0 24px 60px rgba(0, 0, 0, 0.45),
      0 2px 10px rgba(0, 0, 0, 0.2)
    );
    color: var(--text-1, var(--color-text-primary));
    overflow: hidden;
    animation: palette-pop 200ms var(--ease-spring) both;
  }

  @keyframes palette-pop {
    from {
      opacity: 0;
      transform: translateY(8px) scale(0.96);
    }
    to {
      opacity: 1;
      transform: translateY(0) scale(1);
    }
  }

  /* ── Input row ─────────────────────────────────────────────────────────── */
  .palette__input-row {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 14px 18px;
    border-bottom: 1px solid var(--border-1, var(--color-border));
  }

  .palette__search-icon {
    color: var(--text-muted, var(--color-text-muted));
    flex-shrink: 0;
  }

  .palette__input {
    flex: 1;
    min-width: 0;
    background: transparent;
    border: none;
    outline: none;
    color: var(--text-1, var(--color-text-primary));
    font-family: var(--font-sans, system-ui, sans-serif);
    font-size: 15px;
    line-height: 1.3;
    padding: 0;
  }

  .palette__input::placeholder {
    color: var(--text-muted, var(--color-text-muted));
  }

  .palette__mode-badge {
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--text-muted, var(--color-text-muted));
    padding: 3px 8px;
    border: 1px solid var(--border-1, var(--color-border));
    border-radius: 999px;
    flex-shrink: 0;
  }

  .palette__spinner {
    width: 12px;
    height: 12px;
    border: 1.5px solid var(--border-1, var(--color-border));
    border-top-color: var(--accent, var(--color-accent));
    border-radius: 50%;
    animation: palette-spin 600ms linear infinite;
    flex-shrink: 0;
  }

  @keyframes palette-spin {
    to {
      transform: rotate(360deg);
    }
  }

  /* ── Body split: results + preview ─────────────────────────────────────── */
  .palette__body {
    display: grid;
    grid-template-columns: 40% 60%;
    min-height: 0;
    flex: 1;
    overflow: hidden;
  }

  .palette__results {
    list-style: none;
    margin: 0;
    padding: 6px 0;
    overflow-y: auto;
    border-right: 1px solid var(--border-1, var(--color-border));
  }

  .palette__empty {
    padding: 18px 20px;
    font-size: 13px;
    color: var(--text-muted, var(--color-text-muted));
    text-align: center;
  }

  .palette__result {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: 10px;
    padding: 10px 16px;
    cursor: pointer;
    transition: background-color 60ms var(--ease-out);
    border-left: 2px solid transparent;
    outline: none;
  }

  .palette__result--active {
    background-color: var(
      --palette-row-active,
      color-mix(in srgb, var(--accent, var(--color-accent)) 10%, transparent)
    );
    border-left-color: var(--accent, var(--color-accent));
  }

  .palette__result-icon {
    width: 20px;
    height: 20px;
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--text-muted, var(--color-text-muted));
    flex-shrink: 0;
    margin-top: 2px;
  }

  .palette__result-icon--cmd {
    color: var(--accent, var(--color-accent));
  }

  .palette__result-main {
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .palette__result-title {
    font-size: 13px;
    font-weight: 500;
    color: var(--text-1, var(--color-text-primary));
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .palette__result-meta {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 11px;
    color: var(--text-muted, var(--color-text-muted));
    min-width: 0;
  }

  .palette__result-path {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    flex: 1;
  }

  .palette__result-chip {
    flex-shrink: 0;
    padding: 1px 6px;
    border: 1px solid var(--border-1, var(--color-border));
    border-radius: 4px;
    font-size: 10px;
    text-transform: lowercase;
  }

  .palette__result-score {
    flex-shrink: 0;
    font-variant-numeric: tabular-nums;
    opacity: 0.6;
  }

  /* ── Preview pane ──────────────────────────────────────────────────────── */
  .palette__preview {
    display: flex;
    flex-direction: column;
    min-width: 0;
    min-height: 0;
    background-color: var(--surface-2, var(--color-surface-base));
  }

  .palette__preview-header {
    padding: 14px 18px 10px;
    border-bottom: 1px solid var(--border-1, var(--color-border));
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .palette__preview-title {
    font-size: 14px;
    font-weight: 600;
    color: var(--text-1, var(--color-text-primary));
  }

  .palette__preview-path {
    font-size: 11px;
    color: var(--text-muted, var(--color-text-muted));
    font-family: var(--font-mono, ui-monospace, 'SF Mono', Menlo, monospace);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .palette__preview-body {
    padding: 14px 18px;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 10px;
    flex: 1;
    min-height: 0;
  }

  .palette__preview-snippet {
    font-size: 13px;
    line-height: 1.6;
    color: var(--text-2, var(--color-text-secondary));
  }

  .palette__preview-snippet :global(mark) {
    background-color: color-mix(
      in srgb,
      var(--accent, var(--color-accent)) 25%,
      transparent
    );
    color: var(--text-1, var(--color-text-primary));
    padding: 1px 2px;
    border-radius: 2px;
  }

  .palette__preview-desc {
    font-size: 13px;
    line-height: 1.6;
    color: var(--text-2, var(--color-text-secondary));
    margin: 0;
  }

  .palette__preview-hint {
    font-size: 11px;
    color: var(--text-muted, var(--color-text-muted));
    margin: 0;
  }

  .palette__preview-empty {
    display: flex;
    align-items: center;
    justify-content: center;
    height: 100%;
    padding: 24px;
    color: var(--text-muted, var(--color-text-muted));
    font-size: 12px;
    text-align: center;
  }

  /* ── Hint bar ──────────────────────────────────────────────────────────── */
  .palette__hint-bar {
    display: flex;
    align-items: center;
    gap: 16px;
    padding: 9px 18px;
    border-top: 1px solid var(--border-1, var(--color-border));
    background-color: var(--surface-2, var(--color-surface-base));
    font-size: 11px;
    color: var(--text-muted, var(--color-text-muted));
  }

  .palette__hint {
    display: inline-flex;
    align-items: center;
    gap: 4px;
  }

  .palette__hint--scope {
    margin-left: auto;
    font-variant: tabular-nums;
  }

  .palette__hint strong {
    color: var(--text-1, var(--color-text-primary));
    font-weight: 600;
  }

  .palette__hint kbd {
    display: inline-block;
    min-width: 18px;
    padding: 2px 5px;
    font-family: var(--font-mono, ui-monospace, 'SF Mono', Menlo, monospace);
    font-size: 10px;
    line-height: 1;
    color: var(--text-1, var(--color-text-primary));
    background-color: var(--surface-1, var(--color-surface-elevated));
    border: 1px solid var(--border-1, var(--color-border));
    border-bottom-width: 2px;
    border-radius: 4px;
    text-align: center;
  }

  /* ── sr-only ───────────────────────────────────────────────────────────── */
  .sr-only {
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

  /* ── Reduced motion ────────────────────────────────────────────────────── */
  @media (prefers-reduced-motion: reduce) {
    .palette-backdrop {
      animation: none;
    }
    .palette {
      animation: none;
      opacity: 1;
      transform: none;
    }
    .palette__spinner {
      animation-duration: 1.5s;
    }
  }
</style>
