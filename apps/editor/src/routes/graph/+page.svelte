<script lang="ts">
  /**
   * /graph — project picker landing page.
   *
   * The doc-reference graph is per-project (see /api/graph?project=<name>),
   * so the global "graph" link lands here first. The user picks a project
   * and we navigate to /projects/<slug>/graph for the real canvas.
   *
   * When no projects are registered we show an empty state pointing at
   * onboarding, matching the rest of the app's zero-project affordance.
   */

  import { projectsStore } from "$lib/stores/projects";
</script>

<svelte:head>
  <title>Reference Graph — Vedox</title>
</svelte:head>

<div class="graph-picker">
  <header class="graph-picker__header">
    <div class="graph-picker__title-group">
      <svg
        class="graph-picker__title-icon"
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
        <circle cx="18" cy="5" r="3" />
        <circle cx="6" cy="12" r="3" />
        <circle cx="18" cy="19" r="3" />
        <line x1="8.59" y1="13.51" x2="15.42" y2="17.49" />
        <line x1="15.41" y1="6.51" x2="8.59" y2="10.49" />
      </svg>
      <h1 class="graph-picker__title">reference graph</h1>
    </div>
    <p class="graph-picker__subtitle">
      pick a project to see its doc-to-doc link graph
    </p>
  </header>

  <div class="graph-picker__body">
    {#if $projectsStore.length === 0}
      <div class="graph-picker__empty">
        <p>no projects registered yet.</p>
        <a class="graph-picker__cta" href="/onboarding">./register a project</a>
      </div>
    {:else}
      <ul class="graph-picker__list" aria-label="Projects">
        {#each $projectsStore as project (project.id)}
          <li class="graph-picker__item">
            <a
              class="graph-picker__link"
              href="/projects/{project.id}/graph"
            >
              <span class="graph-picker__link-name">{project.name}</span>
              <span class="graph-picker__link-count">
                {project.docCount ?? 0} docs
              </span>
            </a>
          </li>
        {/each}
      </ul>
    {/if}
  </div>
</div>

<style>
  .graph-picker {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
    overflow: hidden;
  }

  .graph-picker__header {
    flex-shrink: 0;
    padding: var(--space-4) var(--space-6);
    border-bottom: 1px solid var(--color-border);
    background-color: var(--color-surface-elevated);
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .graph-picker__title-group {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }

  .graph-picker__title-icon {
    color: var(--color-text-muted);
    flex-shrink: 0;
  }

  .graph-picker__title {
    font-size: var(--text-xl, 22px);
    font-weight: 600;
    color: var(--color-text-primary);
    letter-spacing: var(--tracking-tight, -0.015em);
    margin: 0;
    font-family: var(--font-mono);
    line-height: 1;
  }

  .graph-picker__subtitle {
    font-size: var(--font-size-sm, 12px);
    color: var(--color-text-muted);
    font-family: var(--font-mono);
    margin: 0;
  }

  .graph-picker__body {
    flex: 1;
    min-height: 0;
    overflow: auto;
    padding: var(--space-6);
  }

  .graph-picker__empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-8);
    color: var(--color-text-muted);
    font-family: var(--font-mono);
    font-size: var(--font-size-sm, 12px);
  }

  .graph-picker__cta {
    color: var(--accent-solid);
    text-decoration: none;
    border-bottom: 1px dashed var(--accent-solid);
  }

  .graph-picker__list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
    gap: var(--space-3);
    max-width: 960px;
  }

  .graph-picker__item {
    margin: 0;
  }

  .graph-picker__link {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
    padding: var(--space-4);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    background-color: var(--color-surface);
    color: var(--color-text-primary);
    text-decoration: none;
    transition: border-color 80ms var(--ease-out), background-color 80ms var(--ease-out);
  }

  .graph-picker__link:hover {
    border-color: var(--accent-solid);
    background-color: var(--color-surface-overlay);
  }

  .graph-picker__link:focus-visible {
    outline: 2px solid var(--accent-solid);
    outline-offset: 2px;
  }

  .graph-picker__link-name {
    font-family: var(--font-mono);
    font-weight: 600;
  }

  .graph-picker__link-count {
    font-family: var(--font-mono);
    font-size: var(--font-size-sm, 12px);
    color: var(--color-text-muted);
  }

  @media (max-width: 640px) {
    .graph-picker__header {
      padding: var(--space-3) var(--space-4);
    }
    .graph-picker__body {
      padding: var(--space-3) var(--space-4);
    }
  }
</style>
