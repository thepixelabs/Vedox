<script lang="ts">
  /**
   * /projects/[project] — project home page.
   *
   * Fetches the real document list from the Go backend via api.getProjectDocs().
   * The project metadata (name, docCount) comes from the root layout store;
   * the doc list is fetched on mount so we don't need a separate +page.ts
   * load function (keeps the dependency graph simple for this local-only SPA).
   */

  import { page } from "$app/stores";
  import { projectsStore } from "$lib/stores/projects";
  import DocTree from "$lib/components/DocTree.svelte";
  import TaskBacklog from "$lib/components/TaskBacklog.svelte";
  import ProviderDrawer from "$lib/components/ProviderDrawer.svelte";

  const projectId = $derived(($page.params as Record<string, string>)["project"]);
  const project = $derived($projectsStore.find((p) => p.id === projectId) ?? null);

  // Task count — fed back from the TaskBacklog component via bind:taskCount
  let taskCount: number = $state(0);

  // Provider config drawer (VDX-PD3-FE)
  let providerDrawerOpen: boolean = $state(false);
</script>

<svelte:head>
  <title>{project?.name ?? "Project"} — Vedox</title>
</svelte:head>

<div class="project-home">
  {#if project}
    <header class="project-home__header">
      <div class="project-home__header-row">
        <h1 class="project-home__title">{project.name}</h1>
        <button
          type="button"
          class="project-home__config-btn"
          onclick={() => (providerDrawerOpen = true)}
          aria-label="Open provider config"
        >
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <circle cx="12" cy="12" r="3"/>
            <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09a1.65 1.65 0 0 0-1-1.51 1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09a1.65 1.65 0 0 0 1.51-1 1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33h0a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51h0a1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82v0a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/>
          </svg>
          Config
        </button>
      </div>
      <p class="project-home__meta">
        <span class="project-home__meta-item">
          <span class="project-home__meta-value">
            {project.docs.length > 0 ? project.docs.length : (project.docCount ?? "…")}
          </span>
          doc{(project.docs.length > 0 ? project.docs.length : project.docCount) === 1 ? "" : "s"}
        </span>
      </p>
    </header>

    <section class="project-home__section">
      <h2 class="project-home__section-title">Documents</h2>
      <DocTree project={project} />
    </section>

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
    <ProviderDrawer
      project={projectId}
      open={providerDrawerOpen}
      onclose={() => (providerDrawerOpen = false)}
    />
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

  .project-home__config-btn {
    margin-left: auto;
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    padding: var(--space-1) var(--space-3);
    background: var(--color-surface-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    color: var(--color-text-secondary);
    font-family: var(--font-body, inherit);
    font-size: var(--font-size-sm);
    cursor: pointer;
    transition: border-color 80ms, color 80ms, background-color 80ms;
  }
  .project-home__config-btn:hover {
    border-color: var(--color-border-strong);
    color: var(--color-text-primary);
  }
  .project-home__config-btn:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
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
    font-size: var(--text-caption);
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
