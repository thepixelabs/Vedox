<script lang="ts">
  /**
   * /projects/[project]/docs/[...path] — document editor page.
   *
   * Receives a pre-loaded Doc (or error) from +page.ts.
   * Mounts Editor.svelte with the loaded content and wires:
   *   - onChange  → api.saveDoc() (called by Editor after its 800ms debounce)
   *   - onPublish → api.publishDoc() with the commit message from the dialog
   *
   * Save and publish errors are surfaced as inline status messages rather than
   * crashing the editor — users can keep writing even if the backend is slow.
   */

  import { page } from "$app/stores";
  import { onMount } from "svelte";
  import { projectsStore } from "$lib/stores/projects";
  import { api, ApiError } from "$lib/api/client";
  import PaneGroup from "$lib/components/PaneGroup.svelte";
  import type { DocData } from "$lib/components/PaneView.svelte";
  import { panesStore } from "$lib/stores/panes";
  import HistoryTimeline from "$lib/components/history/HistoryTimeline.svelte";
  import type { DocPageData } from "./+page.js";

  interface Props {
    data: DocPageData;
  }

  let { data }: Props = $props();

  // ---------------------------------------------------------------------------
  // Derived state from URL params + layout store
  // ---------------------------------------------------------------------------

  const projectId = $derived(($page.params as Record<string, string>)["project"]);
  const docPath = $derived(($page.params as Record<string, string>)["path"] ?? "");

  const project = $derived($projectsStore.find((p) => p.id === projectId) ?? null);

  // ---------------------------------------------------------------------------
  // Save / publish status
  // ---------------------------------------------------------------------------

  type OpState = "idle" | "saving" | "saved" | "publishing" | "error";

  let opState: OpState = $state("idle");
  let opError: string = $state("");
  let opErrorTimer: ReturnType<typeof setTimeout> | null = null;

  function clearError(): void {
    opState = "idle";
    opError = "";
  }

  function scheduleErrorClear(ms = 6000): void {
    if (opErrorTimer !== null) clearTimeout(opErrorTimer);
    opErrorTimer = setTimeout(clearError, ms);
  }

  // ---------------------------------------------------------------------------
  // Editor callbacks
  // ---------------------------------------------------------------------------

  /**
   * Called by Editor after its internal 800ms debounce fires.
   * We do NOT add another debounce — the Editor already handles that.
   *
   * Read-only docs (symlinked via SymlinkAdapter) skip the API call entirely —
   * the backend would return VDX-011 anyway, but we avoid the round-trip and
   * give an immediate, clear message.
   */
  async function handleChange(content: string): Promise<void> {
    if (isReadOnly) {
      opState = "error";
      opError = "This document is read-only. Use Import & Migrate to edit it.";
      scheduleErrorClear(6000);
      return;
    }
    if (opState === "publishing") return; // don't auto-save while publishing
    opState = "saving";
    opError = "";
    try {
      await api.saveDoc(projectId, docPath, content);
      opState = "saved";
      // Fade the "saved" badge after 2 s so it doesn't clutter the UI.
      scheduleErrorClear(2000);
    } catch (err) {
      opState = "error";
      opError = err instanceof ApiError
        ? `Save failed: [${err.code}] ${err.message}`
        : err instanceof Error ? `Save failed: ${err.message}` : "Save failed.";
      scheduleErrorClear(8000);
    }
  }

  async function handlePublish(content: string, message: string): Promise<void> {
    if (isReadOnly) {
      opState = "error";
      opError = "This document is read-only. Use Import & Migrate to edit it.";
      scheduleErrorClear(6000);
      return;
    }
    opState = "publishing";
    opError = "";
    try {
      await api.publishDoc(projectId, docPath, message);
      opState = "idle";
    } catch (err) {
      opState = "error";
      opError = err instanceof ApiError
        ? `Publish failed: [${err.code}] ${err.message}`
        : err instanceof Error ? `Publish failed: ${err.message}` : "Publish failed.";
      scheduleErrorClear(10000);
    }
  }

  // ---------------------------------------------------------------------------
  // Read-only banner (symlinked docs)
  // ---------------------------------------------------------------------------

  /**
   * True when the backend SymlinkAdapter injected _editable = false into the
   * document metadata. Symlinked docs cannot be edited in Vedox; the user must
   * Import & Migrate to gain write access.
   */
  const isReadOnly = $derived(
    data.doc?.metadata?.["_editable"] === false
  );

  const sourcePath = $derived(
    typeof data.doc?.metadata?.["_source_path"] === "string"
      ? data.doc.metadata["_source_path"] as string
      : null
  );

  // ---------------------------------------------------------------------------
  // Pane initialization
  // ---------------------------------------------------------------------------

  /**
   * Pre-loaded docs keyed by docPath for PaneGroup.
   * Currently only the route-loaded doc is available; additional panes
   * will show empty state until multi-doc loading is wired.
   */
  const loadedDocs: Record<string, DocData> = $derived(
    data.doc
      ? { [docPath]: { content: data.doc.content, metadata: data.doc.metadata ?? {} } }
      : {}
  );

  // Open the current doc in a pane on mount.
  // We gate this on mount so it only runs client-side and once per navigation.
  onMount(() => {
    if (data.doc && docPath) {
      panesStore.open(docPath);
    }
  });

  // ---------------------------------------------------------------------------
  // History panel
  // ---------------------------------------------------------------------------

  let showHistory = $state(false);

  function toggleHistory(): void {
    showHistory = !showHistory;
  }

  // ---------------------------------------------------------------------------
  // Derived title
  // ---------------------------------------------------------------------------

  const pageTitle = $derived(
    data.doc
      ? `${String(data.doc.metadata?.["title"] ?? docPath)} — ${project?.name ?? "Vedox"}`
      : `${docPath} — Vedox`
  );

</script>

<svelte:head>
  <title>{pageTitle}</title>
</svelte:head>

<div class="doc-view">
  <!-- ── Header / breadcrumb ──────────────────────────────────────────────── -->
  <header class="doc-view__header">
    <nav class="doc-view__breadcrumb" aria-label="Document location">
      <a href="/projects" class="doc-view__breadcrumb-item">Projects</a>
      <span class="doc-view__breadcrumb-sep" aria-hidden="true">/</span>
      {#if project}
        <a
          href="/projects/{project.id}"
          class="doc-view__breadcrumb-item"
        >{project.name}</a>
        <span class="doc-view__breadcrumb-sep" aria-hidden="true">/</span>
      {/if}
      <span
        class="doc-view__breadcrumb-item doc-view__breadcrumb-item--current"
        aria-current="page"
      >
        {data.doc
          ? String(data.doc.metadata?.["title"] ?? docPath.split("/").pop())
          : docPath}
      </span>
    </nav>

    <div class="doc-view__path-row">
      <div class="doc-view__path" aria-label="File path">
        <code>{docPath}</code>
      </div>

      <!-- History toggle -->
      <button
        class="doc-view__history-btn"
        class:doc-view__history-btn--active={showHistory}
        type="button"
        aria-pressed={showHistory}
        aria-label="{showHistory ? 'Hide' : 'Show'} document history"
        title="Toggle history panel (⌘⇧H)"
        onclick={toggleHistory}
      >
        <svg
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
          <circle cx="12" cy="12" r="10"/>
          <polyline points="12 6 12 12 16 14"/>
        </svg>
        History
      </button>

      <!-- Op status badge -->
      {#if opState !== "idle"}
        <div
          class="doc-view__op-badge"
          class:doc-view__op-badge--saving={opState === "saving"}
          class:doc-view__op-badge--saved={opState === "saved"}
          class:doc-view__op-badge--publishing={opState === "publishing"}
          class:doc-view__op-badge--error={opState === "error"}
          role="status"
          aria-live="polite"
        >
          {#if opState === "saving"}
            Saving…
          {:else if opState === "saved"}
            Saved
          {:else if opState === "publishing"}
            Publishing…
          {:else if opState === "error"}
            {opError}
            <button
              class="doc-view__op-badge-dismiss"
              type="button"
              aria-label="Dismiss error"
              onclick={clearError}
            >×</button>
          {/if}
        </div>
      {/if}
    </div>
  </header>

  <!-- ── Read-only banner (symlinked docs) ───────────────────────────────── -->
  {#if isReadOnly}
    <div class="doc-view__readonly-banner" role="note" aria-label="Read-only document">
      <svg
        class="doc-view__readonly-icon"
        width="14"
        height="14"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"
      >
        <rect x="3" y="11" width="18" height="11" rx="2" ry="2"/>
        <path d="M7 11V7a5 5 0 0 1 10 0v4"/>
      </svg>
      <span class="doc-view__readonly-text">
        This document is read-only (symlinked from
        {#if sourcePath}
          <code class="doc-view__readonly-path">{sourcePath}</code>
        {:else}
          an external location
        {/if}).
      </span>
      <a
        href="/projects"
        class="doc-view__readonly-cta"
        title="Open the Add Project dialog and choose Import &amp; Migrate to edit this document"
      >Import to edit &rarr;</a>
    </div>
  {/if}

  <!-- ── History panel ────────────────────────────────────────────────────── -->
  {#if showHistory}
    <div
      class="doc-view__history-panel"
      role="region"
      aria-label="Document history"
    >
      <HistoryTimeline {projectId} {docPath} />
    </div>
  {/if}

  <!-- ── Editor area ──────────────────────────────────────────────────────── -->
  <div
    class="doc-view__editor-area"
    class:doc-view__editor-area--split={showHistory}
    role="region"
    aria-label="Document editor"
  >
    {#if data.error}
      <!-- Error state — doc failed to load -->
      <div class="doc-view__error" role="alert">
        <svg
          class="doc-view__error-icon"
          width="20"
          height="20"
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
        <h2 class="doc-view__error-heading">Could not load document</h2>
        <p class="doc-view__error-detail">{data.error}</p>
        <a href="/projects/{projectId}" class="doc-view__error-back">
          Back to project
        </a>
      </div>
    {:else if data.doc === null}
      <!-- Loading state — should be brief since +page.ts pre-loads -->
      <div class="doc-view__loading" aria-live="polite" aria-busy="true">
        <span class="doc-view__spinner" aria-hidden="true"></span>
        Loading document…
      </div>
    {:else}
      <!-- Pane group — wraps Editor instances, supports split panes -->
      <PaneGroup
        {loadedDocs}
        {projectId}
        onChange={handleChange}
        onPublish={handlePublish}
      />
    {/if}
  </div>
</div>

<style>
  /* ── Layout ─────────────────────────────────────────────────────────────── */

  .doc-view {
    display: flex;
    flex-direction: column;
    height: 100%;
  }

  .doc-view__header {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    padding: var(--space-5) var(--space-8) var(--space-4);
    border-bottom: 1px solid var(--color-border);
    background-color: var(--color-surface-base);
    flex-shrink: 0;
  }

  .doc-view__breadcrumb {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    flex-wrap: wrap;
  }

  .doc-view__breadcrumb-item {
    font-size: var(--font-size-sm);
    color: var(--color-text-muted);
    text-decoration: none;
    transition: color 80ms ease;
  }

  .doc-view__breadcrumb-item:hover {
    color: var(--color-text-secondary);
    text-decoration: none;
  }

  .doc-view__breadcrumb-item:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
    border-radius: var(--radius-sm);
  }

  .doc-view__breadcrumb-item--current {
    color: var(--color-text-secondary);
    font-weight: 500;
    cursor: default;
  }

  .doc-view__breadcrumb-item--current:hover {
    color: var(--color-text-secondary);
  }

  .doc-view__breadcrumb-sep {
    color: var(--color-border-strong);
    font-size: var(--font-size-sm);
    user-select: none;
  }

  .doc-view__path-row {
    display: flex;
    align-items: center;
    gap: var(--space-4);
    min-width: 0;
  }

  .doc-view__path {
    font-family: var(--font-mono);
    font-size: var(--font-size-sm);
    color: var(--color-text-muted);
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  /* ── Op status badge ─────────────────────────────────────────────────────── */

  .doc-view__op-badge {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    padding: 2px var(--space-3);
    border-radius: var(--radius-sm);
    font-size: 11px;
    font-weight: 500;
    white-space: nowrap;
    flex-shrink: 0;
    transition: opacity 200ms ease;
  }

  .doc-view__op-badge--saving {
    color: var(--color-text-muted);
    background-color: var(--color-surface-elevated);
    border: 1px solid var(--color-border);
  }

  .doc-view__op-badge--saved {
    color: var(--color-success, #38a169);
    background-color: color-mix(in srgb, var(--color-success, #38a169) 10%, transparent);
    border: 1px solid color-mix(in srgb, var(--color-success, #38a169) 25%, transparent);
  }

  .doc-view__op-badge--publishing {
    color: var(--color-accent);
    background-color: color-mix(in srgb, var(--color-accent) 10%, transparent);
    border: 1px solid color-mix(in srgb, var(--color-accent) 25%, transparent);
  }

  .doc-view__op-badge--error {
    color: var(--color-error, #e53e3e);
    background-color: color-mix(in srgb, var(--color-error, #e53e3e) 10%, transparent);
    border: 1px solid color-mix(in srgb, var(--color-error, #e53e3e) 25%, transparent);
    max-width: 360px;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .doc-view__op-badge-dismiss {
    background: none;
    border: none;
    padding: 0 0 0 var(--space-1);
    color: inherit;
    cursor: pointer;
    font-size: 14px;
    line-height: 1;
    opacity: 0.7;
    flex-shrink: 0;
  }

  .doc-view__op-badge-dismiss:hover {
    opacity: 1;
  }

  /* ── Read-only banner ────────────────────────────────────────────────────── */

  .doc-view__readonly-banner {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-8);
    background-color: color-mix(in srgb, var(--color-warning, #f59e0b) 10%, transparent);
    border-bottom: 1px solid color-mix(in srgb, var(--color-warning, #f59e0b) 30%, transparent);
    font-size: var(--font-size-sm);
    color: var(--color-text-secondary);
    flex-shrink: 0;
    flex-wrap: wrap;
    gap: var(--space-1) var(--space-2);
  }

  .doc-view__readonly-icon {
    color: var(--color-warning, #f59e0b);
    flex-shrink: 0;
  }

  .doc-view__readonly-text {
    flex: 1;
    min-width: 0;
  }

  .doc-view__readonly-path {
    font-family: var(--font-mono);
    font-size: 0.9em;
    word-break: break-all;
  }

  .doc-view__readonly-cta {
    color: var(--color-accent);
    text-decoration: none;
    font-weight: 500;
    white-space: nowrap;
    flex-shrink: 0;
  }

  .doc-view__readonly-cta:hover {
    text-decoration: underline;
  }

  .doc-view__readonly-cta:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
    border-radius: var(--radius-sm);
  }

  /* ── History toggle button ───────────────────────────────────────────────── */

  .doc-view__history-btn {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    padding: 3px var(--space-3);
    background-color: var(--color-surface-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    color: var(--color-text-muted);
    font-size: 11px;
    font-family: var(--font-mono);
    cursor: pointer;
    white-space: nowrap;
    flex-shrink: 0;
    transition:
      background-color 80ms var(--ease-out),
      color 80ms var(--ease-out),
      border-color 80ms var(--ease-out);
  }

  .doc-view__history-btn:hover {
    background-color: var(--color-surface-overlay);
    color: var(--color-text-secondary);
    border-color: var(--color-border-strong);
  }

  .doc-view__history-btn--active {
    background-color: var(--color-accent-subtle);
    color: var(--color-accent);
    border-color: var(--accent-border, var(--color-accent));
  }

  .doc-view__history-btn--active:hover {
    background-color: color-mix(in oklch, var(--color-accent) 20%, transparent);
    color: var(--color-accent);
  }

  .doc-view__history-btn:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
  }

  /* ── History panel ───────────────────────────────────────────────────────── */

  .doc-view__history-panel {
    flex-shrink: 0;
    max-height: 42vh;
    min-height: 240px;
    overflow-y: auto;
    border-bottom: 1px solid var(--color-border);
    background-color: var(--color-surface-base);
  }

  /* ── Editor container ────────────────────────────────────────────────────── */

  .doc-view__editor-area {
    flex: 1;
    min-height: 0;
    overflow: hidden;
    display: flex;
    flex-direction: column;
  }

  /* When history panel is open, give editor a minimum breathing room */
  .doc-view__editor-area--split {
    min-height: 200px;
  }

  /* ── Loading state ───────────────────────────────────────────────────────── */

  .doc-view__loading {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-8);
    color: var(--color-text-muted);
    font-size: var(--font-size-sm);
  }

  .doc-view__spinner {
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

  /* ── Error state ─────────────────────────────────────────────────────────── */

  .doc-view__error {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: var(--space-4);
    padding: var(--space-12, var(--space-8));
    text-align: center;
    color: var(--color-text-secondary);
    flex: 1;
  }

  .doc-view__error-icon {
    color: var(--color-text-muted);
    opacity: 0.6;
  }

  .doc-view__error-heading {
    font-size: var(--font-size-base);
    font-weight: 600;
    color: var(--color-text-primary);
  }

  .doc-view__error-detail {
    font-family: var(--font-mono);
    font-size: var(--font-size-sm);
    color: var(--color-text-muted);
    max-width: 480px;
  }

  .doc-view__error-back {
    display: inline-flex;
    align-items: center;
    padding: var(--space-2) var(--space-4);
    background-color: var(--color-surface-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    color: var(--color-text-secondary);
    font-size: var(--font-size-sm);
    text-decoration: none;
    transition: border-color 80ms, color 80ms;
  }

  .doc-view__error-back:hover {
    border-color: var(--color-border-strong);
    color: var(--color-text-primary);
    text-decoration: none;
  }

  .doc-view__error-back:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
  }
</style>
