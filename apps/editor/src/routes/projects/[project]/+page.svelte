<script lang="ts">
  /**
   * /projects/[project] — project home page.
   *
   * Fetches the real document list from the Go backend via api.getProjectDocs().
   * The project metadata (name, docCount) comes from the root layout store;
   * the doc list is fetched on mount so we don't need a separate +page.ts
   * load function (keeps the dependency graph simple for this local-only SPA).
   */

  import { onMount } from "svelte";
  import { page } from "$app/stores";
  import { projectsStore } from "$lib/stores/projects";
  import { api, ApiError, type Doc } from "$lib/api/client";
  import EmptyState from "$lib/components/EmptyState.svelte";
  import TaskBacklog from "$lib/components/TaskBacklog.svelte";

  const projectId = $derived(($page.params as Record<string, string>)["project"]);
  const project = $derived($projectsStore.find((p) => p.id === projectId) ?? null);

  // ---------------------------------------------------------------------------
  // Doc list state
  // ---------------------------------------------------------------------------

  type LoadState = "idle" | "loading" | "done" | "error";

  let loadState: LoadState = $state("idle");
  let docs: Doc[] = $state([]);
  let errorMessage: string = $state("");

  // Task count — fed back from the TaskBacklog component via bind:taskCount
  let taskCount: number = $state(0);

  onMount(async () => {
    if (!projectId) return;
    loadState = "loading";
    try {
      docs = await api.getProjectDocs(projectId);
      loadState = "done";
    } catch (err) {
      loadState = "error";
      if (err instanceof ApiError) {
        errorMessage = `[${err.code}] ${err.message}`;
      } else if (err instanceof Error) {
        errorMessage = err.message;
      } else {
        errorMessage = "Unknown error loading documents.";
      }
    }
  });

  /** Strip the leading project-name segment from the workspace-relative path. */
  function docDisplayPath(docPath: string): string {
    const prefix = projectId + "/";
    return docPath.startsWith(prefix) ? docPath.slice(prefix.length) : docPath;
  }

  /** Derive a display title from path when metadata.title is absent. */
  function docTitle(doc: Doc): string {
    const title = doc.metadata?.["title"];
    if (typeof title === "string" && title.trim()) return title.trim();
    // Fall back to the filename without extension.
    const segments = doc.path.split("/");
    const filename = segments[segments.length - 1] ?? doc.path;
    return filename.replace(/\.md$/i, "");
  }

  /** Route path for the editor — strip the project prefix from the doc path. */
  function docEditorHref(doc: Doc): string {
    return `/projects/${projectId}/docs/${docDisplayPath(doc.path)}`;
  }

  // ---------------------------------------------------------------------------
  // Empty state icon — document with plus sign
  // ---------------------------------------------------------------------------

  const ICON_DOC_PLUS = `<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
    <path d="M14.5 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7.5L14.5 2z"/>
    <polyline points="14 2 14 8 20 8"/>
    <line x1="12" y1="13" x2="12" y2="19"/>
    <line x1="9" y1="16" x2="15" y2="16"/>
  </svg>`;

  function handleNewDocument() {
    window.location.href = `/projects/${projectId}/docs/new`;
  }
</script>

<svelte:head>
  <title>{project?.name ?? "Project"} — Vedox</title>
</svelte:head>

<div class="project-home">
  {#if project}
    <header class="project-home__header">
      <div class="project-home__header-row">
        <h1 class="project-home__title">{project.name}</h1>
      </div>
      <p class="project-home__meta">
        <span class="project-home__meta-item">
          <span class="project-home__meta-value">
            {loadState === "done" ? docs.length : (project.docCount ?? "…")}
          </span>
          doc{(loadState === "done" ? docs.length : project.docCount) === 1 ? "" : "s"}
        </span>
      </p>
    </header>

    {#if loadState === "loading" || loadState === "idle"}
      <div class="project-home__loading" aria-live="polite" aria-busy="true">
        <span class="project-home__spinner" aria-hidden="true"></span>
        Loading documents…
      </div>
    {:else if loadState === "error"}
      <div class="project-home__error" role="alert">
        <p>Could not load documents.</p>
        <p class="project-home__error-detail">{errorMessage}</p>
        <button
          class="project-home__retry"
          type="button"
          onclick={async () => {
            loadState = "loading";
            errorMessage = "";
            try {
              docs = await api.getProjectDocs(projectId);
              loadState = "done";
            } catch (err) {
              loadState = "error";
              errorMessage = err instanceof Error ? err.message : "Unknown error";
            }
          }}
        >
          Retry
        </button>
      </div>
    {:else if docs.length > 0}
      <section class="project-home__section">
        <h2 class="project-home__section-title">Documents</h2>
        <ul class="project-home__doc-list" role="list">
          {#each docs as doc (doc.path)}
            <li>
              <a
                class="project-home__doc-link"
                href={docEditorHref(doc)}
              >
                <span class="project-home__doc-path" aria-hidden="true">
                  <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <path d="M14.5 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7.5L14.5 2z"/>
                    <polyline points="14 2 14 8 20 8"/>
                  </svg>
                </span>
                <span class="project-home__doc-title">{docTitle(doc)}</span>
                <span class="project-home__doc-meta">{docDisplayPath(doc.path)}</span>
              </a>
            </li>
          {/each}
        </ul>
      </section>
    {:else}
      <!-- ── Per-project empty state (VDX-P2-J) ────────────────────────────── -->
      <EmptyState
        icon={ICON_DOC_PLUS}
        heading="No documents yet"
        body="Create your first document using one of the built-in templates."
        cta={{ label: "New Document", onClick: handleNewDocument }}
      />
    {/if}

    <!-- ── Task backlog (VDX-P2-H) ─────────────────────────────────────────── -->
    <section class="project-home__section project-home__section--tasks">
      <h2 class="project-home__section-title">
        Tasks
        {#if taskCount > 0}
          <span class="project-home__count-badge" aria-label="{taskCount} task{taskCount === 1 ? '' : 's'}">{taskCount}</span>
        {/if}
      </h2>
      <TaskBacklog project={projectId} bind:taskCount />
    </section>
  {:else}
    <div class="project-home__not-found">
      <h1>Project not found</h1>
      <a href="/projects">Back to projects</a>
    </div>
  {/if}
</div>

<style>
  .project-home {
    padding: var(--space-8);
    max-width: 800px;
  }

  .project-home__header {
    margin-bottom: var(--space-8);
  }

  .project-home__header-row {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    margin-bottom: var(--space-2);
  }

  /* Override title margin when inside the row */
  .project-home__header-row .project-home__title {
    margin-bottom: 0;
  }

  .project-home__title {
    font-size: var(--font-size-xl);
    font-weight: 600;
    color: var(--color-text-primary);
    letter-spacing: -0.02em;
    margin-bottom: var(--space-2);
  }

  .project-home__meta {
    display: flex;
    gap: var(--space-4);
    color: var(--color-text-muted);
    font-size: var(--font-size-sm);
  }

  .project-home__meta-value {
    font-family: var(--font-mono);
    color: var(--color-text-secondary);
  }

  /* ── Loading state ────────────────────────────────────────────────────────── */

  .project-home__loading {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    color: var(--color-text-muted);
    font-size: var(--font-size-sm);
    padding: var(--space-6) 0;
  }

  .project-home__spinner {
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

  /* ── Error state ──────────────────────────────────────────────────────────── */

  .project-home__error {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    padding: var(--space-5);
    background-color: color-mix(in srgb, var(--color-error, #e53e3e) 8%, transparent);
    border: 1px solid color-mix(in srgb, var(--color-error, #e53e3e) 25%, transparent);
    border-radius: var(--radius-md);
    color: var(--color-error, #e53e3e);
    font-size: var(--font-size-sm);
  }

  .project-home__error-detail {
    font-family: var(--font-mono);
    font-size: 11px;
    color: var(--color-text-muted);
    word-break: break-all;
  }

  .project-home__retry {
    align-self: flex-start;
    padding: var(--space-1) var(--space-3);
    background: none;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    color: var(--color-text-secondary);
    font-size: var(--font-size-sm);
    cursor: pointer;
    font-family: inherit;
    transition: border-color 80ms, color 80ms;
    margin-top: var(--space-1);
  }

  .project-home__retry:hover {
    border-color: var(--color-border-strong);
    color: var(--color-text-primary);
  }

  .project-home__retry:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
  }

  /* ── Document section ─────────────────────────────────────────────────────── */

  .project-home__section {
    margin-bottom: var(--space-6);
  }

  .project-home__section-title {
    font-size: var(--font-size-sm);
    font-weight: 600;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--color-text-muted);
    margin-bottom: var(--space-3);
  }

  .project-home__doc-list {
    list-style: none;
    display: grid;
    gap: 2px;
  }

  .project-home__doc-link {
    display: grid;
    grid-template-columns: 20px 1fr auto;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-3) var(--space-4);
    background-color: var(--color-surface-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    text-decoration: none;
    transition: background-color 80ms ease, border-color 80ms ease;
  }

  .project-home__doc-link:hover {
    background-color: var(--color-surface-overlay);
    border-color: var(--color-border-strong);
  }

  .project-home__doc-link:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
  }

  .project-home__doc-path {
    color: var(--color-text-muted);
    display: flex;
    align-items: center;
  }

  .project-home__doc-title {
    font-size: var(--font-size-sm);
    color: var(--color-text-primary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .project-home__doc-meta {
    font-family: var(--font-mono);
    font-size: 11px;
    color: var(--color-text-muted);
    white-space: nowrap;
  }

  /* ── Not found state ──────────────────────────────────────────────────────── */

  .project-home__not-found {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    padding: var(--space-8) 0;
    color: var(--color-text-secondary);
  }

  /* ── Tasks section ────────────────────────────────────────────────────────── */

  .project-home__section--tasks {
    margin-top: var(--space-8);
  }

  /* Section title flex row so the count badge sits inline with the heading */
  .project-home__section--tasks .project-home__section-title {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }

  .project-home__count-badge {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-width: 18px;
    height: 18px;
    padding: 0 var(--space-1);
    font-family: var(--font-mono);
    font-size: 10px;
    font-weight: 600;
    color: var(--color-text-inverse);
    background-color: var(--color-accent);
    border-radius: var(--radius-sm);
    /* Undo the uppercase + letter-spacing from the parent section-title */
    text-transform: none;
    letter-spacing: 0;
    line-height: 1;
  }
</style>
