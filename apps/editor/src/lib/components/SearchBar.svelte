<script lang="ts">
  /**
   * SearchBar.svelte
   *
   * Full-text search input for the sidebar. Calls api.search() with a 300ms
   * debounce. Results drop down below the input as a focusable list. Each
   * result links to its document's editor route.
   *
   * Accessibility:
   *   - Input has role="combobox" + aria-expanded + aria-controls
   *   - Results list has role="listbox" with role="option" items
   *   - Escape closes the dropdown and returns focus to the input
   *   - Arrow keys move focus within the dropdown (Down: open/next, Up: prev)
   *   - Click-outside closes the dropdown (using Svelte's on:focusout approach)
   *
   * Props:
   *   project — the current project name (URL slug). When null the component
   *             is hidden (only renders when a project is selected).
   */

  import { onDestroy } from "svelte";
  import { goto } from "$app/navigation";
  import { api, ApiError, type SearchResult } from "$lib/api/client";

  interface Props {
    project: string | null;
  }

  let { project }: Props = $props();

  // ---------------------------------------------------------------------------
  // State
  // ---------------------------------------------------------------------------

  let query = $state("");
  let results: SearchResult[] = $state([]);
  let isOpen = $state(false);
  let isLoading = $state(false);
  let errorMessage = $state("");

  let inputEl: HTMLInputElement | undefined = $state(undefined);
  let listEl: HTMLUListElement | undefined = $state(undefined);
  let containerEl: HTMLDivElement | undefined = $state(undefined);

  const listboxId = "searchbar-results";

  // ---------------------------------------------------------------------------
  // Debounced search
  // ---------------------------------------------------------------------------

  const DEBOUNCE_MS = 300;
  let debounceTimer: ReturnType<typeof setTimeout> | null = null;

  function scheduleSearch(q: string): void {
    if (debounceTimer !== null) clearTimeout(debounceTimer);
    if (!q.trim()) {
      results = [];
      isOpen = false;
      isLoading = false;
      errorMessage = "";
      return;
    }
    isLoading = true;
    debounceTimer = setTimeout(() => runSearch(q), DEBOUNCE_MS);
  }

  async function runSearch(q: string): Promise<void> {
    if (!project) return;
    try {
      results = await api.search(project, q);
      errorMessage = "";
      isOpen = true;
    } catch (err) {
      results = [];
      isOpen = false;
      errorMessage =
        err instanceof ApiError
          ? `Search failed: ${err.message}`
          : err instanceof Error
            ? `Search failed: ${err.message}`
            : "Search failed.";
    } finally {
      isLoading = false;
    }
  }

  function handleInput(): void {
    scheduleSearch(query);
  }

  // ---------------------------------------------------------------------------
  // Keyboard navigation
  // ---------------------------------------------------------------------------

  function handleKeydown(e: KeyboardEvent): void {
    if (e.key === "Escape") {
      close();
      return;
    }
    if (e.key === "ArrowDown") {
      e.preventDefault();
      if (!isOpen && results.length > 0) {
        isOpen = true;
      }
      focusResult(0);
      return;
    }
  }

  function handleResultKeydown(e: KeyboardEvent, index: number): void {
    if (e.key === "Escape") {
      e.preventDefault();
      close();
      inputEl?.focus();
      return;
    }
    if (e.key === "ArrowDown") {
      e.preventDefault();
      focusResult(index + 1);
      return;
    }
    if (e.key === "ArrowUp") {
      e.preventDefault();
      if (index === 0) {
        inputEl?.focus();
      } else {
        focusResult(index - 1);
      }
      return;
    }
    if (e.key === "Enter") {
      e.preventDefault();
      navigate(results[index]);
      return;
    }
  }

  function focusResult(index: number): void {
    const items = listEl?.querySelectorAll<HTMLElement>("[role='option']");
    if (!items) return;
    const clamped = Math.min(index, items.length - 1);
    items[clamped]?.focus();
  }

  // ---------------------------------------------------------------------------
  // Navigation + close helpers
  // ---------------------------------------------------------------------------

  function resultHref(result: SearchResult): string {
    // result.id is the workspace-relative path, e.g. "myproject/docs/adr.md"
    // Strip the leading project segment to get the doc-relative path.
    const prefix = project ? project + "/" : "";
    const docPath = result.id.startsWith(prefix)
      ? result.id.slice(prefix.length)
      : result.id;
    return `/projects/${encodeURIComponent(result.project)}/docs/${docPath}`;
  }

  function navigate(result: SearchResult): void {
    goto(resultHref(result));
    close();
    query = "";
    results = [];
  }

  function close(): void {
    isOpen = false;
    errorMessage = "";
  }

  // ---------------------------------------------------------------------------
  // Click-outside via focusout on the container
  // ---------------------------------------------------------------------------

  function handleContainerFocusout(e: FocusEvent): void {
    // relatedTarget is the element receiving focus next.
    // If it's still inside our container, stay open.
    const next = e.relatedTarget as Node | null;
    if (containerEl && next && containerEl.contains(next)) return;
    close();
  }

  // ---------------------------------------------------------------------------
  // Cleanup
  // ---------------------------------------------------------------------------

  onDestroy(() => {
    if (debounceTimer !== null) clearTimeout(debounceTimer);
  });
</script>

{#if project}
  <!-- svelte-ignore a11y-no-static-element-interactions -->
  <div
    bind:this={containerEl}
    class="searchbar"
    onfocusout={handleContainerFocusout}
  >
    <div class="searchbar__input-wrap">
      <svg
        class="searchbar__icon"
        width="13"
        height="13"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"
      >
        <circle cx="11" cy="11" r="8"/>
        <line x1="21" y1="21" x2="16.65" y2="16.65"/>
      </svg>

      <input
        bind:this={inputEl}
        bind:value={query}
        class="searchbar__input"
        type="search"
        role="combobox"
        aria-label="Search documents"
        aria-autocomplete="list"
        aria-expanded={isOpen}
        aria-controls={listboxId}
        aria-busy={isLoading}
        placeholder="Search docs by title or content"
        autocomplete="off"
        spellcheck="false"
        oninput={handleInput}
        onkeydown={handleKeydown}
      />

      {#if isLoading}
        <span class="searchbar__spinner" aria-hidden="true"></span>
      {/if}
    </div>

    {#if errorMessage}
      <p class="searchbar__error" role="alert">{errorMessage}</p>
    {/if}

    {#if isOpen && results.length > 0}
      <ul
        bind:this={listEl}
        id={listboxId}
        class="searchbar__results"
        role="listbox"
        aria-label="Search results"
      >
        {#each results as result, i (result.id)}
          <!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
          <li
            class="searchbar__result"
            role="option"
            aria-selected="false"
            tabindex="-1"
            onkeydown={(e) => handleResultKeydown(e, i)}
            onclick={() => navigate(result)}
          >
            <span class="searchbar__result-title">{result.title || result.id}</span>
            {#if result.snippet}
              <span class="searchbar__result-snippet">{result.snippet}</span>
            {/if}
          </li>
        {/each}
      </ul>
    {:else if isOpen && !isLoading && query.trim().length > 0 && results.length === 0}
      <div class="searchbar__no-results" role="status">
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
          <circle cx="11" cy="11" r="8"/>
          <line x1="21" y1="21" x2="16.65" y2="16.65"/>
          <line x1="8" y1="11" x2="14" y2="11"/>
        </svg>
        <span>No results for "{query}"</span>
      </div>
    {/if}
  </div>
{/if}

<style>
  /* ── Container ───────────────────────────────────────────────────────────── */

  .searchbar {
    position: relative;
    padding: var(--space-2) var(--space-2) 0;
  }

  /* ── Input row ───────────────────────────────────────────────────────────── */

  .searchbar__input-wrap {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    background-color: var(--color-surface-base);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: 0 var(--space-2);
    transition: border-color 80ms var(--ease-out);
  }

  .searchbar__input-wrap:focus-within {
    border-color: var(--color-accent);
  }

  .searchbar__icon {
    color: var(--color-text-muted);
    flex-shrink: 0;
  }

  .searchbar__input {
    flex: 1;
    min-width: 0;
    background: none;
    border: none;
    outline: none;
    color: var(--color-text-primary);
    font-size: var(--font-size-sm);
    font-family: inherit;
    padding: var(--space-2) 0;
    /* Remove the browser's default search-cancel button — we control the UX */
    appearance: none;
  }

  /* Suppress WebKit search input clear button */
  .searchbar__input::-webkit-search-cancel-button {
    display: none;
  }

  .searchbar__input::placeholder {
    color: var(--color-text-muted);
  }

  /* ── Loading spinner ─────────────────────────────────────────────────────── */

  .searchbar__spinner {
    display: inline-block;
    width: 11px;
    height: 11px;
    border: 1.5px solid var(--color-border);
    border-top-color: var(--color-accent);
    border-radius: 50%;
    animation: spin 500ms linear infinite;
    flex-shrink: 0;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  /* ── Error message ───────────────────────────────────────────────────────── */

  .searchbar__error {
    margin: var(--space-1) 0 0;
    padding: 0 var(--space-1);
    font-size: 11px;
    color: var(--color-error, #e53e3e);
    line-height: 1.4;
  }

  /* ── Results dropdown ────────────────────────────────────────────────────── */

  .searchbar__results {
    position: absolute;
    top: calc(100% - var(--space-1));
    left: var(--space-2);
    right: var(--space-2);
    z-index: 200;
    background-color: var(--color-surface-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    box-shadow: var(--shadow-md);
    list-style: none;
    padding: var(--space-1) 0;
    max-height: 320px;
    overflow-y: auto;
    /* Subtle entry animation */
    animation: dropdown-in 120ms ease-out;
  }

  @keyframes dropdown-in {
    from {
      opacity: 0;
      transform: translateY(-4px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }

  /* ── Result item ─────────────────────────────────────────────────────────── */

  .searchbar__result {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: var(--space-2) var(--space-3);
    cursor: pointer;
    outline: none;
    transition: background-color 60ms var(--ease-out);
    border-radius: 0;
  }

  .searchbar__result:hover,
  .searchbar__result:focus {
    background-color: var(--color-surface-overlay);
  }

  .searchbar__result:focus-visible {
    /* Inset outline so it doesn't bleed outside the dropdown border-radius */
    outline: 2px solid var(--color-accent);
    outline-offset: -2px;
  }

  .searchbar__result-title {
    font-size: var(--font-size-sm);
    font-weight: 500;
    color: var(--color-text-primary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .searchbar__result-snippet {
    font-size: 11px;
    color: var(--color-text-muted);
    line-height: 1.4;
    /* Allow two lines of snippet */
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }

  /* ── No results state ───────────────────────────────────────────────────── */

  .searchbar__no-results {
    position: absolute;
    top: calc(100% - var(--space-1));
    left: var(--space-2);
    right: var(--space-2);
    z-index: 200;
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-3) var(--space-3);
    background-color: var(--color-surface-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    box-shadow: var(--shadow-md);
    color: var(--color-text-muted);
    font-size: var(--font-size-sm);
    animation: dropdown-in 120ms ease-out;
  }
</style>
