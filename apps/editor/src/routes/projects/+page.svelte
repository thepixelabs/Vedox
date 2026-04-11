<script lang="ts">
  /**
   * /projects — project list page.
   *
   * Progressive disclosure empty state (VDX-P2-J):
   *
   *   1. projects.length === 0, no scan:  "No projects yet" + Scan / Create CTAs
   *   2. Scan in progress:                spinner + "Found N projects so far", no CTA
   *   3. Scan done, 0 found:              magnifying glass + "No Git projects found"
   *   4. projects.length > 0:             project grid (unchanged)
   *
   * The scan API (POST /api/scan, GET /api/scan/:id) is implemented by Phase 2-A.
   * This component is written defensively: a connection-refused error surfaces a
   * friendly "Start vedox dev to enable workspace scanning." message instead of
   * crashing or showing a raw error.
   */

  import { onMount, onDestroy } from "svelte";
  import { goto } from "$app/navigation";
  import { projectsStore } from "$lib/stores/projects";
  import { api, ApiError, type ImportResult } from "$lib/api/client";
  import EmptyState from "$lib/components/EmptyState.svelte";
  import ImportDialog from "$lib/components/ImportDialog.svelte";

  // ---------------------------------------------------------------------------
  // Scan state
  // ---------------------------------------------------------------------------

  type ScanPhase =
    | 'idle'         // no scan running, no results yet
    | 'scanning'     // scan in progress — polling
    | 'done-empty'   // scan completed, 0 projects found
    | 'api-offline'; // scan API unreachable

  let scanPhase: ScanPhase = $state('idle');
  let scanFoundCount: number = $state(0);
  let scanError: string | null = $state(null);

  let pollTimer: ReturnType<typeof setInterval> | null = null;
  let currentJobId: string | null = null;

  function stopPolling() {
    if (pollTimer !== null) {
      clearInterval(pollTimer);
      pollTimer = null;
    }
  }

  async function pollJob(jobId: string) {
    try {
      const job = await api.getScanJob(jobId);
      scanFoundCount = job.projects.length;

      if (job.status === 'done' || job.status === 'error') {
        stopPolling();
        if (job.projects.length > 0) {
          // Merge discovered projects into the store so the layout fork
          // switches to the sidebar shell automatically.
          for (const p of job.projects) {
            projectsStore.addProject({
              id: p.name,
              name: p.name,
              docs: [],
              docCount: p.docCount,
            });
          }
          // scanPhase will naturally become irrelevant once hasProjects is true.
        } else {
          scanPhase = 'done-empty';
        }
      }
    } catch {
      // If polling fails mid-scan, stop silently — don't reset the UI yet.
      stopPolling();
    }
  }

  async function handleScanWorkspace() {
    scanError = null;
    scanFoundCount = 0;
    try {
      const { jobId } = await api.startScan();
      currentJobId = jobId;
      scanPhase = 'scanning';
      // Poll every second.
      pollTimer = setInterval(() => {
        if (currentJobId) pollJob(currentJobId);
      }, 1000);
      // Kick off the first poll immediately (don't wait 1 s).
      await pollJob(jobId);
    } catch (err) {
      // TypeError: Failed to fetch  →  API is offline
      const isNetworkError =
        err instanceof TypeError ||
        (err instanceof ApiError && err.status === 0);

      if (isNetworkError) {
        scanPhase = 'api-offline';
      } else {
        scanPhase = 'idle';
        scanError =
          err instanceof ApiError
            ? `[${err.code}] ${err.message}`
            : err instanceof Error
            ? err.message
            : 'Unknown error starting scan.';
      }
    }
  }

  function handleScanDifferentFolder() {
    // Reset to idle so the user can trigger another scan.
    scanPhase = 'idle';
    handleScanWorkspace();
  }

  // ---------------------------------------------------------------------------
  // Import dialog state
  // ---------------------------------------------------------------------------

  let importDialogOpen: boolean = $state(false);
  /**
   * When set, the ImportDialog is pre-populated with this path — used when the
   * user clicks "Import" on a scanner result card.
   */
  let importInitialPath: string = $state('');

  function handleOpenImportDialog(srcPath = '') {
    importInitialPath = srcPath;
    importDialogOpen = true;
  }

  function handleImported(result: ImportResult) {
    // Merge newly imported project into the store so the sidebar updates.
    // The projectName is embedded in each imported path as the first segment.
    if (result.imported.length > 0) {
      const firstImported = result.imported[0];
      const projectName = firstImported.split('/')[0];
      if (projectName) {
        projectsStore.addProject({
          id: projectName,
          name: projectName,
          docs: [],
          docCount: result.imported.length,
        });
      }
    }
  }

  onDestroy(() => {
    stopPolling();
  });

  // ---------------------------------------------------------------------------
  // SVG icon strings — passed to EmptyState as the `icon` prop
  // ---------------------------------------------------------------------------

  const ICON_FOLDER_PLUS = `<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
    <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
    <line x1="12" y1="11" x2="12" y2="17"/>
    <line x1="9" y1="14" x2="15" y2="14"/>
  </svg>`;

  const ICON_SEARCH = `<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
    <circle cx="11" cy="11" r="8"/>
    <line x1="21" y1="21" x2="16.65" y2="16.65"/>
  </svg>`;

  const ICON_WIFI_OFF = `<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
    <line x1="1" y1="1" x2="23" y2="23"/>
    <path d="M16.72 11.06A10.94 10.94 0 0 1 19 12.55"/>
    <path d="M5 12.55a10.94 10.94 0 0 1 5.17-2.39"/>
    <path d="M10.71 5.05A16 16 0 0 1 22.56 9"/>
    <path d="M1.42 9a15.91 15.91 0 0 1 4.7-2.88"/>
    <path d="M8.53 16.11a6 6 0 0 1 6.95 0"/>
    <line x1="12" y1="20" x2="12.01" y2="20"/>
  </svg>`;
</script>

<svelte:head>
  <title>Projects — Vedox</title>
</svelte:head>

<div class="projects-page">
  <header class="projects-page__header">
    <h1 class="projects-page__title">Projects</h1>
    <div class="projects-page__header-actions">
      <button
        class="projects-page__new-btn"
        type="button"
        onclick={() => goto('/projects/new')}
      >
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
          <line x1="12" y1="5" x2="12" y2="19"/>
          <line x1="5" y1="12" x2="19" y2="12"/>
        </svg>
        New Project
      </button>
      <button
        class="projects-page__add-btn"
        type="button"
        onclick={() => handleOpenImportDialog()}
      >
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
          <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
          <polyline points="17 8 12 3 7 8"/>
          <line x1="12" y1="3" x2="12" y2="15"/>
        </svg>
        Import Project
      </button>
    </div>
  </header>

  {#if $projectsStore.length > 0}
    <!-- ── Project grid ─────────────────────────────────────────────────────── -->
    <ul class="projects-list" role="list">
      {#each $projectsStore as project (project.id)}
        {@const count = project.docCount ?? project.docs.length}
        <li class="projects-list__item">
          <a class="project-card" href="/projects/{project.id}">
            <span class="project-card__name">{project.name}</span>
            <span class="project-card__meta">
              {count} doc{count === 1 ? "" : "s"}
            </span>
          </a>
        </li>
      {/each}
    </ul>

  {:else if scanPhase === 'idle'}
    <!-- ── Empty: no scan yet ──────────────────────────────────────────────── -->
    <EmptyState
      icon={ICON_FOLDER_PLUS}
      heading="No projects yet"
      body="Scan your workspace to import existing projects, or create a new one from scratch."
      cta={{ label: "Scan Workspace", onClick: handleScanWorkspace }}
      secondary={{ label: "New Project", onClick: () => goto('/projects/new') }}
    />
    {#if scanError}
      <p class="projects-page__scan-error" role="alert">{scanError}</p>
    {/if}

  {:else if scanPhase === 'scanning'}
    <!-- ── Scan in progress: spinner, live count ────────────────────────────── -->
    <EmptyState
      icon=""
      spinning={true}
      heading="Scanning your workspace..."
      body="Found {scanFoundCount} project{scanFoundCount === 1 ? '' : 's'} so far"
    />

  {:else if scanPhase === 'done-empty'}
    <!-- ── Scan done, nothing found ─────────────────────────────────────────── -->
    <EmptyState
      icon={ICON_SEARCH}
      heading="No Git projects found"
      body="Make sure you're running vedox dev from a directory that contains Git repositories."
      cta={{ label: "Scan a different folder", onClick: handleScanDifferentFolder }}
      secondary={{ label: "New Project", onClick: () => goto('/projects/new') }}
    />

  {:else if scanPhase === 'api-offline'}
    <!-- ── Scan API not reachable ───────────────────────────────────────────── -->
    <EmptyState
      icon={ICON_WIFI_OFF}
      heading="Scan unavailable"
      body="Start vedox dev to enable workspace scanning."
      secondary={{ label: "New Project", onClick: () => goto('/projects/new') }}
    />
  {/if}
</div>

<!-- Import dialog — rendered outside the main layout container so it can
     use position:fixed relative to the viewport without clipping issues. -->
<ImportDialog
  bind:open={importDialogOpen}
  initialSrcPath={importInitialPath}
  onImported={handleImported}
/>

<style>
  .projects-page {
    padding: var(--space-8);
    max-width: 800px;
    /* Allow EmptyState to fill vertically when the page has no projects */
    min-height: calc(100vh - var(--space-8) * 2);
    display: flex;
    flex-direction: column;
  }

  /* ── Header ───────────────────────────────────────────────────────────────── */

  .projects-page__header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: var(--space-6);
    flex-shrink: 0;
  }

  .projects-page__title {
    font-size: var(--font-size-xl);
    font-weight: 600;
    color: var(--color-text-primary);
    letter-spacing: -0.02em;
  }

  /* Header action group — holds import + add buttons side by side */
  .projects-page__header-actions {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }

  .projects-page__new-btn {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-4);
    background-color: var(--accent-solid);
    color: var(--accent-contrast);
    font-size: var(--font-size-sm);
    font-weight: 500;
    font-family: var(--font-sans);
    border: none;
    border-radius: var(--radius-md);
    cursor: pointer;
    transition: background-color 100ms ease;
  }

  .projects-page__new-btn:hover {
    background-color: var(--accent-solid-hover);
  }

  .projects-page__new-btn:focus-visible {
    outline: 2px solid var(--accent-solid);
    outline-offset: 3px;
  }

  .projects-page__add-btn {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-4);
    background-color: transparent;
    color: var(--text-2);
    font-size: var(--font-size-sm);
    font-weight: 500;
    font-family: var(--font-sans);
    border: 1px solid var(--border-default);
    border-radius: var(--radius-md);
    cursor: pointer;
    transition: border-color 100ms ease, color 100ms ease, background-color 100ms ease;
  }

  .projects-page__add-btn:hover {
    border-color: var(--border-strong);
    color: var(--text-1);
    background-color: var(--surface-3);
  }

  .projects-page__add-btn:focus-visible {
    outline: 2px solid var(--accent-solid);
    outline-offset: 3px;
  }

  /* ── Project list ─────────────────────────────────────────────────────────── */

  .projects-list {
    list-style: none;
    display: grid;
    gap: var(--space-3);
  }

  .project-card {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--space-4) var(--space-5);
    background-color: var(--color-surface-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    text-decoration: none;
    transition: border-color 100ms ease, background-color 100ms ease, box-shadow 100ms ease;
  }

  .project-card:hover {
    border-color: var(--color-border-strong);
    background-color: var(--color-surface-overlay);
    box-shadow: var(--shadow-sm);
  }

  .project-card:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
  }

  .project-card__name {
    font-size: var(--font-size-base);
    font-weight: 500;
    color: var(--color-text-primary);
  }

  .project-card__meta {
    font-size: var(--font-size-sm);
    color: var(--color-text-muted);
    font-family: var(--font-mono);
  }

  /* ── Scan error inline note ───────────────────────────────────────────────── */

  .projects-page__scan-error {
    display: flex;
    justify-content: center;
    color: var(--color-error);
    font-size: var(--font-size-sm);
    margin-top: var(--space-3);
  }
</style>
