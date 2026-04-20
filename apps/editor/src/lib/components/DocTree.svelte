<script lang="ts">
  /**
   * DocTree.svelte — hierarchical document navigator.
   *
   * Replaces the flat doc list from Phase 1 (ProjectTree.svelte).
   *
   * Grouping strategy (Diataxis-first):
   *   type group (how-to, adr, explanation, …)
   *     └── folder (filesystem path relative to project root)
   *           └── doc
   *
   * Features:
   *   - As-you-type filter input at the top (replaces the SearchBar in the sidebar)
   *   - Count badge per type group (fades when group is expanded)
   *   - Expand/collapse per type group, persisted to localStorage
   *   - Active doc highlighted with accent bar (carries over from ProjectTree)
   *   - Cmd+click opens doc in a split pane
   *   - Full WAI-ARIA tree pattern (role="tree" / role="treeitem")
   *   - Arrow key navigation across all visible treeitems
   *   - Reduced-motion: no CSS transitions
   *
   * Props:
   *   project — the current Project (id + docs[])
   */

  import { onMount } from "svelte";
  import { page } from "$app/stores";
  import { browser } from "$app/environment";
  import type { Project, DocEntry, DocType } from "$lib/stores/projects";
  import { projectsStore } from "$lib/stores/projects";
  import { panesStore } from "$lib/stores/panes";
  import { api } from "$lib/api/client";
  import EmptyState from "./EmptyState.svelte";

  interface Props {
    project: Project;
  }

  let { project }: Props = $props();

  // ---------------------------------------------------------------------------
  // Lazy-load docs when the tree is mounted but docs are empty.
  // This handles the case where a user navigates directly to a doc URL
  // without visiting the project page first (which normally calls setProjectDocs).
  // ---------------------------------------------------------------------------

  let lazyLoadState: "idle" | "loading" | "done" | "error" = $state("idle");

  onMount(async () => {
    if (project.docs.length === 0 && project.id) {
      lazyLoadState = "loading";
      try {
        const docs = await api.getProjectDocs(project.id);
        projectsStore.setProjectDocs(project.id, docs);
        lazyLoadState = "done";
      } catch {
        lazyLoadState = "error";
      }
    } else {
      lazyLoadState = "done";
    }
  });

  // ---------------------------------------------------------------------------
  // Filter state
  // ---------------------------------------------------------------------------

  let filterQuery = $state("");

  const normalizedFilter = $derived(filterQuery.trim().toLowerCase());

  // ---------------------------------------------------------------------------
  // Active doc path from URL
  // ---------------------------------------------------------------------------

  const currentPath = $derived(
    ($page.params as Record<string, string>)["path"] ?? null
  );

  // ---------------------------------------------------------------------------
  // Type group ordering and display labels
  // ---------------------------------------------------------------------------

  const TYPE_ORDER: DocType[] = [
    "how-to",
    "tutorial",
    "explanation",
    "reference",
    "adr",
    "runbook",
    "readme",
    "other",
  ];

  const TYPE_LABELS: Record<DocType, string> = {
    "how-to": "How-to",
    tutorial: "Tutorial",
    explanation: "Explanation",
    reference: "Reference",
    adr: "ADR",
    runbook: "Runbook",
    readme: "Readme",
    other: "Other",
  };

  // ---------------------------------------------------------------------------
  // Expand/collapse state — persisted per project in localStorage
  // ---------------------------------------------------------------------------

  const storageKey = $derived(
    browser ? `vedox:tree:${project.id}:expanded` : ""
  );

  function loadExpanded(): Set<string> {
    if (!browser) return new Set(TYPE_ORDER); // all expanded server-side
    try {
      const raw = localStorage.getItem(storageKey);
      if (raw) {
        const arr = JSON.parse(raw) as string[];
        return new Set(arr);
      }
    } catch {
      // ignore parse errors
    }
    // First visit: all groups expanded.
    return new Set(TYPE_ORDER);
  }

  let expanded: Set<string> = $state(loadExpanded());

  function saveExpanded(set: Set<string>) {
    if (!browser) return;
    try {
      localStorage.setItem(storageKey, JSON.stringify([...set]));
    } catch {
      // localStorage unavailable
    }
  }

  function toggleGroup(type: string) {
    const next = new Set(expanded);
    if (next.has(type)) {
      next.delete(type);
    } else {
      next.add(type);
    }
    expanded = next;
    saveExpanded(next);
  }

  // ---------------------------------------------------------------------------
  // Build grouped tree from docs + filter
  // ---------------------------------------------------------------------------

  interface FolderGroup {
    folder: string;      // "" means root (no folder)
    docs: DocEntry[];
  }

  interface TypeGroup {
    type: DocType;
    label: string;
    folders: FolderGroup[];
    totalCount: number;  // count before filter
    matchCount: number;  // count after filter
  }

  const groups = $derived.by((): TypeGroup[] => {
    const q = normalizedFilter;

    // 1. Filter docs
    const filteredDocs = q
      ? project.docs.filter(
          (d) =>
            d.title.toLowerCase().includes(q) ||
            d.path.toLowerCase().includes(q)
        )
      : project.docs;

    // 2. Build a map: type → folder → docs
    const typeMap = new Map<DocType, Map<string, DocEntry[]>>();

    // Pre-populate in display order so iteration order is stable
    for (const t of TYPE_ORDER) {
      typeMap.set(t, new Map());
    }

    for (const doc of filteredDocs) {
      const typeEntry = typeMap.get(doc.type)!;
      const existing = typeEntry.get(doc.folder) ?? [];
      existing.push(doc);
      typeEntry.set(doc.folder, existing);
    }

    // 3. Count totals per type (before filter, for the badge)
    const totalsByType = new Map<DocType, number>();
    for (const doc of project.docs) {
      totalsByType.set(doc.type, (totalsByType.get(doc.type) ?? 0) + 1);
    }

    // 4. Build result, drop empty types
    const result: TypeGroup[] = [];
    for (const type of TYPE_ORDER) {
      const folderMap = typeMap.get(type)!;
      if (folderMap.size === 0) continue;

      // Sort folders: root first, then alphabetical
      const sortedFolders: FolderGroup[] = [];
      const rootDocs = folderMap.get("") ?? [];
      if (rootDocs.length > 0) {
        sortedFolders.push({
          folder: "",
          docs: [...rootDocs].sort((a, b) => a.title.localeCompare(b.title)),
        });
      }
      const nonRootFolders = [...folderMap.entries()]
        .filter(([f]) => f !== "")
        .sort(([a], [b]) => a.localeCompare(b));
      for (const [folder, docs] of nonRootFolders) {
        sortedFolders.push({
          folder,
          docs: [...docs].sort((a, b) => a.title.localeCompare(b.title)),
        });
      }

      const matchCount = [...folderMap.values()].reduce(
        (sum, docs) => sum + docs.length,
        0
      );

      result.push({
        type,
        label: TYPE_LABELS[type],
        folders: sortedFolders,
        totalCount: totalsByType.get(type) ?? 0,
        matchCount,
      });
    }
    return result;
  });

  const hasAnyDocs = $derived(project.docs.length > 0);
  const hasFilterResults = $derived(groups.length > 0);

  // ---------------------------------------------------------------------------
  // URL builder
  // ---------------------------------------------------------------------------

  function getDocUrl(docPath: string): string {
    return `/projects/${project.id}/docs/${docPath}`;
  }

  // ---------------------------------------------------------------------------
  // Click handler — Cmd/Ctrl+click = split pane
  // ---------------------------------------------------------------------------

  function handleDocClick(e: MouseEvent, docPath: string) {
    if (e.metaKey || e.ctrlKey) {
      e.preventDefault();
      panesStore.split();
      panesStore.open(docPath);
    }
  }

  // ---------------------------------------------------------------------------
  // Keyboard navigation
  // ---------------------------------------------------------------------------

  let treeEl: HTMLElement | undefined = $state();

  function getAllTreeItems(): HTMLElement[] {
    if (!treeEl) return [];
    return Array.from(
      treeEl.querySelectorAll<HTMLElement>('[role="treeitem"]')
    );
  }

  function handleTreeKeydown(event: KeyboardEvent) {
    const items = getAllTreeItems();
    if (items.length === 0) return;

    const focused = document.activeElement as HTMLElement | null;
    const currentIdx = focused ? items.indexOf(focused) : -1;

    switch (event.key) {
      case "ArrowDown":
        event.preventDefault();
        if (currentIdx < items.length - 1) {
          items[currentIdx + 1]?.focus();
        }
        break;

      case "ArrowUp":
        event.preventDefault();
        if (currentIdx > 0) {
          items[currentIdx - 1]?.focus();
        }
        break;

      case "Home":
        event.preventDefault();
        items[0]?.focus();
        break;

      case "End":
        event.preventDefault();
        items[items.length - 1]?.focus();
        break;

      case "Enter":
      case " ":
        event.preventDefault();
        if (focused && focused !== treeEl) {
          focused.click();
        }
        break;
    }
  }

  // ---------------------------------------------------------------------------
  // Filter input — focus / clear
  // ---------------------------------------------------------------------------

  let filterInputEl: HTMLInputElement | undefined = $state();

  function clearFilter() {
    filterQuery = "";
    filterInputEl?.focus();
  }
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="doctree" bind:this={treeEl} onkeydown={handleTreeKeydown}>
  <!-- ── Filter input ───────────────────────────────────────────────────────── -->
  <div class="doctree__filter">
    <div
      class="doctree__filter-wrap"
      class:doctree__filter-wrap--active={filterQuery.length > 0}
    >
      <svg
        class="doctree__filter-icon"
        width="11"
        height="11"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2.5"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"
      >
        <circle cx="11" cy="11" r="8" />
        <line x1="21" y1="21" x2="16.65" y2="16.65" />
      </svg>
      <input
        bind:this={filterInputEl}
        bind:value={filterQuery}
        class="doctree__filter-input"
        type="search"
        placeholder="filter docs"
        aria-label="Filter documents"
        autocomplete="off"
        spellcheck={false}
      />
      {#if filterQuery.length > 0}
        <button
          class="doctree__filter-clear"
          type="button"
          aria-label="Clear filter"
          onclick={clearFilter}
        >
          <svg
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
            <line x1="18" y1="6" x2="6" y2="18" />
            <line x1="6" y1="6" x2="18" y2="18" />
          </svg>
        </button>
      {/if}
    </div>
  </div>

  <!-- ── Tree body ──────────────────────────────────────────────────────────── -->
  {#if lazyLoadState === "loading"}
    <div class="doctree__loading" aria-live="polite" aria-busy="true">
      <span class="doctree__spinner" aria-hidden="true"></span>
    </div>
  {:else if !hasAnyDocs}
    <EmptyState
      icon={`<svg width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M14.5 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7.5L14.5 2z"/><polyline points="14 2 14 8 20 8"/></svg>`}
      heading="Empty folder"
      body="Create a doc with ⌘N."
    />
  {:else if !hasFilterResults}
    <div class="doctree__no-results" role="status" aria-live="polite">
      <span>no match for "{filterQuery}"</span>
      <button class="doctree__no-results-clear" type="button" onclick={clearFilter}>
        clear
      </button>
    </div>
  {:else}
    <nav aria-label="Documents in {project.name}">
      <ul
        class="doctree__list"
        role="tree"
        aria-label="Documents"
      >
        {#each groups as group (group.type)}
          {@const isExpanded = expanded.has(group.type)}
          <!-- ── Type group header ─────────────────────────────────────────── -->
          <li class="doctree__group" role="none">
            <button
              class="doctree__group-btn"
              type="button"
              aria-expanded={isExpanded}
              aria-selected="false"
              aria-label="{group.label} ({group.totalCount} doc{group.totalCount === 1 ? '' : 's'})"
              role="treeitem"
              tabindex="-1"
              onclick={() => toggleGroup(group.type)}
            >
              <!-- Chevron -->
              <svg
                class="doctree__chevron"
                class:doctree__chevron--open={isExpanded}
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

              <span class="doctree__group-label">{group.label}</span>

              <!-- Count badge — visible, shows matched when filtering -->
              <span
                class="doctree__badge"
                class:doctree__badge--filtered={normalizedFilter.length > 0}
                aria-hidden="true"
              >
                {normalizedFilter.length > 0 ? group.matchCount : group.totalCount}
              </span>
            </button>

            <!-- ── Folder + doc items ──────────────────────────────────────── -->
            {#if isExpanded}
              <ul class="doctree__group-items" role="group">
                {#each group.folders as folderGroup (folderGroup.folder)}
                  {#if folderGroup.folder && group.folders.length > 1}
                    <!-- Folder sub-header — only shown when there are multiple folders -->
                    <li class="doctree__folder" role="none">
                      <span class="doctree__folder-label" aria-hidden="true">
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
                          <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z" />
                        </svg>
                        {folderGroup.folder}
                      </span>
                    </li>
                  {/if}

                  <!-- Doc items -->
                  {#each folderGroup.docs as doc (doc.path)}
                    {@const isActive = currentPath === doc.path}
                    <li
                      class="doctree__item"
                      class:doctree__item--nested={folderGroup.folder !== "" && group.folders.length > 1}
                      role="none"
                    >
                      <a
                        class="doctree__link"
                        class:doctree__link--active={isActive}
                        href={getDocUrl(doc.path)}
                        role="treeitem"
                        aria-selected={isActive}
                        tabindex="-1"
                        title="Click to open · ⌘Click to open in split pane"
                        onclick={(e) => handleDocClick(e, doc.path)}
                      >
                        <span class="doctree__item-icon" aria-hidden="true">
                          <svg
                            width="11"
                            height="11"
                            viewBox="0 0 24 24"
                            fill="none"
                            stroke="currentColor"
                            stroke-width="2"
                            stroke-linecap="round"
                            stroke-linejoin="round"
                          >
                            <path d="M14.5 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7.5L14.5 2z" />
                            <polyline points="14 2 14 8 20 8" />
                          </svg>
                        </span>
                        <span class="doctree__item-label">{doc.title}</span>
                      </a>
                    </li>
                  {/each}
                {/each}
              </ul>
            {/if}
          </li>
        {/each}
      </ul>
    </nav>
  {/if}
</div>

<style>
  /* ── Container ───────────────────────────────────────────────────────────── */

  .doctree {
    display: flex;
    flex-direction: column;
    min-height: 0;
  }

  /* ── Filter row ──────────────────────────────────────────────────────────── */

  .doctree__filter {
    padding: var(--space-2) var(--space-2) var(--space-1);
    flex-shrink: 0;
  }

  .doctree__filter-wrap {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    background-color: var(--color-surface-base);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    padding: 0 var(--space-2);
    transition: border-color 80ms var(--ease-out);
  }

  .doctree__filter-wrap:focus-within,
  .doctree__filter-wrap--active {
    border-color: var(--color-accent);
  }

  .doctree__filter-icon {
    color: var(--color-text-muted);
    flex-shrink: 0;
  }

  .doctree__filter-input {
    flex: 1;
    min-width: 0;
    background: none;
    border: none;
    outline: none;
    color: var(--color-text-primary);
    font-size: var(--font-size-sm);
    font-family: var(--font-body);
    padding: 5px 0;
    /* Remove browser default search-cancel button */
    appearance: none;
  }

  .doctree__filter-input::-webkit-search-cancel-button {
    display: none;
  }

  .doctree__filter-input::placeholder {
    color: var(--color-text-muted);
    font-family: var(--font-body);
  }

  .doctree__filter-clear {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 16px;
    height: 16px;
    padding: 0;
    background: none;
    border: none;
    border-radius: var(--radius-sm);
    color: var(--color-text-muted);
    cursor: pointer;
    flex-shrink: 0;
    transition: color 60ms var(--ease-out);
  }

  .doctree__filter-clear:hover {
    color: var(--color-text-primary);
  }

  .doctree__filter-clear:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 1px;
  }

  /* ── Tree list ───────────────────────────────────────────────────────────── */

  .doctree__list {
    list-style: none;
    padding: var(--space-1) 0;
    margin: 0;
  }

  /* ── Type group ──────────────────────────────────────────────────────────── */

  .doctree__group {
    margin: 1px 0;
  }

  .doctree__group-btn {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    width: 100%;
    padding: var(--space-2) var(--space-3);
    background: none;
    border: none;
    border-radius: var(--radius-sm);
    margin: 0 var(--space-1);
    width: calc(100% - var(--space-2));
    color: var(--color-text-muted);
    cursor: pointer;
    font-size: var(--font-size-sm);
    font-family: inherit;
    text-align: left;
    transition:
      background-color 80ms var(--ease-out),
      color 80ms var(--ease-out);
  }

  .doctree__group-btn:hover {
    background-color: var(--color-surface-overlay);
    color: var(--color-text-primary);
  }

  .doctree__group-btn:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: -2px;
  }

  .doctree__chevron {
    flex-shrink: 0;
    color: var(--color-text-muted);
    transition: transform 120ms var(--ease-out);
    transform: rotate(0deg);
  }

  .doctree__chevron--open {
    transform: rotate(90deg);
  }

  .doctree__group-label {
    flex: 1;
    font-size: var(--text-2xs, 11px);
    font-weight: 600;
    letter-spacing: 0.02em;
    font-family: var(--font-body);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  /* ── Count badge ─────────────────────────────────────────────────────────── */

  .doctree__badge {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-width: 18px;
    height: 18px;
    padding: 0 4px;
    background-color: var(--color-surface-overlay);
    border-radius: 9px;
    font-size: var(--text-caption);
    font-family: var(--font-mono);
    font-weight: 500;
    color: var(--color-text-muted);
    flex-shrink: 0;
    font-variant-numeric: tabular-nums;
    transition: background-color 80ms var(--ease-out), color 80ms var(--ease-out);
  }

  .doctree__badge--filtered {
    background-color: var(--color-accent-subtle);
    color: var(--color-accent);
  }

  /* ── Group items (expanded content) ─────────────────────────────────────── */

  .doctree__group-items {
    list-style: none;
    padding: 0;
    margin: 0;
    /* Subtle entry animation — respects reduced-motion */
    animation: group-expand 120ms var(--ease-out) both;
  }

  @keyframes group-expand {
    from {
      opacity: 0;
      transform: translateY(-4px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }

  /* ── Folder label ────────────────────────────────────────────────────────── */

  .doctree__folder {
    margin-top: var(--space-1);
  }

  .doctree__folder-label {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    padding: 3px var(--space-3) 3px calc(var(--space-3) + 14px);
    font-size: var(--text-caption);
    font-family: var(--font-mono);
    font-weight: 500;
    color: var(--color-text-muted);
    letter-spacing: 0.04em;
    text-transform: uppercase;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    user-select: none;
  }

  /* ── Doc item ────────────────────────────────────────────────────────────── */

  .doctree__item {
    display: contents;
  }

  .doctree__link {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-3) var(--space-2) calc(var(--space-3) + 14px);
    color: var(--color-text-secondary);
    font-size: var(--font-size-sm);
    text-decoration: none;
    border-radius: var(--radius-sm);
    margin: 1px var(--space-1);
    transition:
      background-color 80ms var(--ease-out),
      color 80ms var(--ease-out);
    white-space: nowrap;
    overflow: hidden;
  }

  /* Nested docs (when folder sub-header is shown) get extra left indent */
  .doctree__item--nested .doctree__link {
    padding-left: calc(var(--space-3) + 28px);
  }

  .doctree__link:hover {
    background-color: var(--color-surface-overlay);
    color: var(--color-text-primary);
  }

  .doctree__link--active {
    background-color: var(--color-accent-subtle);
    color: var(--color-accent);
    font-weight: 500;
    /* Inset left accent bar */
    box-shadow: inset 2px 0 0 var(--color-accent);
  }

  .doctree__link--active:hover {
    background-color: var(--color-accent-subtle);
    color: var(--color-accent);
  }

  .doctree__link:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: -2px;
  }

  .doctree__item-icon {
    flex-shrink: 0;
    color: var(--color-text-muted);
    display: flex;
    align-items: center;
  }

  .doctree__link--active .doctree__item-icon {
    color: var(--color-accent);
  }

  .doctree__item-label {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-family: var(--font-body);
    font-size: var(--font-size-sm);
  }

  /* ── Loading state ───────────────────────────────────────────────────────── */

  .doctree__loading {
    display: flex;
    align-items: center;
    justify-content: center;
    padding: var(--space-4) 0;
  }

  .doctree__spinner {
    display: inline-block;
    width: 14px;
    height: 14px;
    border: 2px solid var(--color-border);
    border-top-color: var(--color-accent);
    border-radius: 50%;
    animation: doctree-spin 600ms linear infinite;
  }

  @keyframes doctree-spin {
    to { transform: rotate(360deg); }
  }

  /* ── No results state ────────────────────────────────────────────────────── */

  .doctree__no-results {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-3) var(--space-3);
    font-size: var(--font-size-sm);
    font-family: var(--font-body);
    color: var(--color-text-muted);
  }

  .doctree__no-results-clear {
    background: none;
    border: none;
    padding: 0;
    color: var(--color-accent);
    font-size: var(--font-size-sm);
    font-family: var(--font-body);
    cursor: pointer;
    text-decoration: underline;
    text-underline-offset: 2px;
  }

  .doctree__no-results-clear:hover {
    color: var(--color-accent-hover, var(--color-accent));
  }

  .doctree__no-results-clear:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
    border-radius: var(--radius-sm);
  }

  /* ── Reduced motion ──────────────────────────────────────────────────────── */

  @media (prefers-reduced-motion: reduce) {
    .doctree__chevron {
      transition: none;
    }

    .doctree__group-items {
      animation: none;
    }

    .doctree__spinner {
      animation: none;
      opacity: 0.5;
    }

    .doctree__link,
    .doctree__group-btn,
    .doctree__badge,
    .doctree__filter-wrap {
      transition: none;
    }
  }
</style>
