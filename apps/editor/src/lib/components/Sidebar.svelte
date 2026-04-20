<script lang="ts">
  /**
   * Sidebar — the left panel of the Vedox editor.
   *
   * Structure (top to bottom):
   *   1. Collapse toggle button (top edge)
   *   2. ProjectSwitcher (search-first input)
   *   3. DocTree — hierarchical doc navigator with inline filter (Phase 2)
   *   4. SidebarDock — theme, density, settings
   *
   * Phase 2: SearchBar removed; filter is now embedded inside DocTree.
   *
   * Collapsed state persists to localStorage via sidebarStore.
   * Width transitions via CSS — no JS animations.
   *
   * Only renders project-specific content when a project is active.
   * Never renders at all when there are no projects (parent handles this
   * via progressive disclosure in +layout.svelte).
   */

  import { page } from "$app/stores";
  import ProjectSwitcher from "./ProjectSwitcher.svelte";
  import DocTree from "./DocTree.svelte";
  import SidebarDock from "./SidebarDock.svelte";
  import SidebarOverview from "./SidebarOverview.svelte";
  import { sidebarStore } from "$lib/stores/sidebar";
  import type { Project } from "$lib/stores/projects";

  interface Props {
    projects: Project[];
  }

  let { projects }: Props = $props();

  const sidebar = sidebarStore;

  const currentProjectId = $derived(
    ($page.params as Record<string, string>)["project"] ?? null
  );

  const currentProject = $derived(
    projects.find((p) => p.id === currentProjectId) ?? null
  );

</script>

<aside
  class="sidebar"
  class:sidebar--collapsed={$sidebar.collapsed}
  data-sidebar-collapsed={$sidebar.collapsed ? "true" : "false"}
  aria-label="Navigation"
>
  <!-- Collapse toggle -->
  <button
    class="sidebar__collapse-btn"
    type="button"
    aria-label={$sidebar.collapsed ? "Expand sidebar" : "Collapse sidebar"}
    aria-expanded={!$sidebar.collapsed}
    onclick={() => sidebar.toggle()}
  >
    <svg
      class="sidebar__collapse-icon"
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
      {#if $sidebar.collapsed}
        <!-- Chevron right — expand -->
        <polyline points="9 18 15 12 9 6"/>
      {:else}
        <!-- Chevron left — collapse -->
        <polyline points="15 18 9 12 15 6"/>
      {/if}
    </svg>
  </button>

  {#if !$sidebar.collapsed}
    <div class="sidebar__content">
      <!-- Header: logo wordmark -->
      <div class="sidebar__header">
        <a href="/projects" class="sidebar__wordmark" aria-label="Vedox home">
          <span class="sidebar__wordmark-text">vedox</span>
        </a>
      </div>

      <!-- Project switcher -->
      <div class="sidebar__section sidebar__section--switcher">
        <ProjectSwitcher {projects} />
      </div>

      <!-- Divider -->
      <div class="sidebar__divider" role="separator" aria-hidden="true"></div>

      <!-- Doc tree (includes filter input) — replaces flat list + search bar -->
      <div class="sidebar__section sidebar__section--tree">
        {#if currentProject}
          <div class="sidebar__section-label" aria-hidden="true">Documents</div>
          <DocTree project={currentProject} />
        {:else}
          <div class="sidebar__section-label" aria-hidden="true">Projects</div>
          <nav aria-label="All projects">
            <ul class="sidebar__project-list" role="list">
              {#each projects as project (project.id)}
                <li>
                  <a
                    class="sidebar__project-link"
                    href="/projects/{project.id}"
                    aria-label="Open project {project.name}"
                  >
                    <span class="sidebar__project-icon" aria-hidden="true">
                      <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                        <path d="M3 3h18v18H3z"/>
                        <path d="M3 9h18M9 21V9"/>
                      </svg>
                    </span>
                    {project.name}
                  </a>
                </li>
              {/each}
            </ul>
          </nav>
        {/if}
      </div>

      <!-- Global nav links — graph, analytics, settings -->
      <nav class="sidebar__nav" aria-label="Global navigation">
        <a
          class="sidebar__nav-link"
          class:sidebar__nav-link--active={$page.url.pathname === '/graph'}
          href="/graph"
          aria-label="Node graph"
          aria-current={$page.url.pathname === '/graph' ? 'page' : undefined}
        >
          <span class="sidebar__nav-icon" aria-hidden="true">
            <!-- Share / node graph icon -->
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <circle cx="18" cy="5" r="3"/>
              <circle cx="6" cy="12" r="3"/>
              <circle cx="18" cy="19" r="3"/>
              <line x1="8.59" y1="13.51" x2="15.42" y2="17.49"/>
              <line x1="15.41" y1="6.51" x2="8.59" y2="10.49"/>
            </svg>
          </span>
          <span class="sidebar__nav-label">Graph</span>
        </a>

        <a
          class="sidebar__nav-link"
          class:sidebar__nav-link--active={$page.url.pathname === '/analytics'}
          href="/analytics"
          aria-label="Analytics"
          aria-current={$page.url.pathname === '/analytics' ? 'page' : undefined}
        >
          <span class="sidebar__nav-icon" aria-hidden="true">
            <!-- Bar chart icon -->
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <line x1="18" y1="20" x2="18" y2="10"/>
              <line x1="12" y1="20" x2="12" y2="4"/>
              <line x1="6" y1="20" x2="6" y2="14"/>
              <line x1="2" y1="20" x2="22" y2="20"/>
            </svg>
          </span>
          <span class="sidebar__nav-label">Analytics</span>
        </a>

        <a
          class="sidebar__nav-link"
          class:sidebar__nav-link--active={$page.url.pathname === '/settings'}
          href="/settings"
          aria-label="Settings"
          aria-current={$page.url.pathname === '/settings' ? 'page' : undefined}
        >
          <span class="sidebar__nav-icon" aria-hidden="true">
            <!-- Gear icon -->
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 1 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"/>
              <circle cx="12" cy="12" r="3"/>
            </svg>
          </span>
          <span class="sidebar__nav-label">Settings</span>
        </a>
      </nav>

      <!-- Bottom dock — theme, density, settings, collapse -->
      <SidebarDock />

      <!-- Overview panel — only visible at ultra-wide (>= 2560px) when enabled -->
      {#if $sidebar.overview}
        <div class="sidebar__overview-wrapper">
          <SidebarOverview />
        </div>
      {/if}
    </div>
  {/if}
</aside>

<style>
  .sidebar {
    position: relative;
    display: flex;
    flex-direction: column;
    width: 240px;
    min-width: 240px;
    height: 100vh;
    background-color: var(--surface-3);
    border-right: 1px solid var(--border-default);
    overflow: hidden;
    transition:
      width var(--duration-slow) var(--ease-in-out),
      min-width var(--duration-slow) var(--ease-in-out);
    flex-shrink: 0;
  }

  .sidebar--collapsed {
    width: 40px;
    min-width: 40px;
  }

  .sidebar__collapse-btn {
    position: absolute;
    top: 12px;
    right: 8px;
    z-index: 10;
    display: flex;
    align-items: center;
    justify-content: center;
    width: 24px;
    height: 24px;
    padding: 0;
    background: none;
    border: none;
    border-radius: var(--radius-sm);
    color: var(--color-text-muted);
    cursor: pointer;
    transition: background-color var(--duration-fast) var(--ease-out), color var(--duration-fast) var(--ease-out);
    flex-shrink: 0;
  }

  .sidebar--collapsed .sidebar__collapse-btn {
    right: auto;
    left: 8px;
  }

  .sidebar__collapse-btn:hover {
    background-color: var(--color-surface-overlay);
    color: var(--color-text-primary);
  }

  .sidebar__collapse-btn:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
  }

  .sidebar__content {
    display: flex;
    flex-direction: column;
    height: 100%;
    overflow: hidden;
    padding-top: var(--space-1);
    transition: opacity 60ms var(--ease-out);
  }

  .sidebar--collapsed .sidebar__content {
    opacity: 0;
    pointer-events: none;
  }

  .sidebar__header {
    display: flex;
    align-items: center;
    padding: var(--space-3) var(--space-3);
    padding-right: 40px; /* space for collapse button */
    min-height: 44px;
  }

  .sidebar__wordmark {
    text-decoration: none;
    color: var(--color-text-primary);
  }

  .sidebar__wordmark-text {
    font-family: var(--font-wordmark);
    font-size: 17px;
    font-weight: 700;
    font-variation-settings: "opsz" 12, "wght" 700;
    letter-spacing: -0.02em;
    line-height: 1;
    color: var(--text-1);
  }

  .sidebar__section {
    padding: 0 var(--space-2);
  }

  .sidebar__section--switcher {
    padding-bottom: var(--space-2);
  }

  .sidebar__section--tree {
    flex: 1;
    overflow-y: auto;
    overflow-x: hidden;
    min-height: 0; /* allows flex child to shrink below content height */
    padding-bottom: var(--space-2);
  }

  .sidebar__section-label {
    padding: var(--space-2) var(--space-3) var(--space-2);
    font-size: var(--text-2xs);
    font-weight: 600;
    letter-spacing: 0.02em;
    color: var(--text-4);
    border-bottom: 1px solid var(--border-default);
  }

  .sidebar__divider {
    height: 1px;
    background-color: var(--color-border);
    margin: var(--space-1) 0;
  }

  .sidebar__project-list {
    list-style: none;
    padding: var(--space-1) 0;
  }

  .sidebar__project-link {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-3);
    color: var(--color-text-secondary);
    font-size: var(--font-size-sm);
    text-decoration: none;
    border-radius: var(--radius-sm);
    margin: 1px var(--space-1);
    transition: background-color 80ms var(--ease-out), color 80ms var(--ease-out);
  }

  .sidebar__project-link:hover {
    background-color: var(--color-surface-overlay);
    color: var(--color-text-primary);
  }

  .sidebar__project-link:focus-visible {
    outline: 2px solid var(--accent-solid);
    outline-offset: -2px;
  }

  /* Active project tree item — accent left inset bar, no background fill */
  .sidebar__project-link[aria-current="page"] {
    box-shadow: inset 2px 0 0 var(--accent-solid);
    color: var(--text-1);
  }

  .sidebar__project-icon {
    color: var(--color-text-muted);
    display: flex;
    align-items: center;
    flex-shrink: 0;
  }

  /* ── Global nav links (graph / analytics / settings) ──────────────────── */

  .sidebar__nav {
    flex-shrink: 0;
    padding: var(--space-1) var(--space-2);
    border-top: 1px solid var(--border-default);
  }

  .sidebar__nav-link {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-3);
    color: var(--color-text-secondary);
    font-size: var(--font-size-sm);
    text-decoration: none;
    border-radius: var(--radius-sm);
    margin: 1px var(--space-1);
    transition: background-color 80ms var(--ease-out), color 80ms var(--ease-out);
  }

  .sidebar__nav-link:hover {
    background-color: var(--color-surface-overlay);
    color: var(--color-text-primary);
  }

  .sidebar__nav-link:focus-visible {
    outline: 2px solid var(--accent-solid);
    outline-offset: -2px;
  }

  .sidebar__nav-link--active {
    box-shadow: inset 2px 0 0 var(--accent-solid);
    color: var(--text-1);
    background-color: var(--color-surface-overlay);
  }

  .sidebar__nav-link--active:hover {
    background-color: var(--color-surface-overlay);
    color: var(--text-1);
  }

  .sidebar__nav-icon {
    display: flex;
    align-items: center;
    flex-shrink: 0;
    color: var(--color-text-muted);
  }

  .sidebar__nav-link--active .sidebar__nav-icon {
    color: var(--accent-solid);
  }

  .sidebar__nav-label {
    line-height: 1.4;
  }

  /* ── Legacy bottom-link styles (unused but kept for reference) ───────── */

  .sidebar__bottom {
    flex-shrink: 0;
    padding-bottom: var(--space-2);
  }

  .sidebar__bottom-link {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-3);
    color: var(--color-text-secondary);
    font-size: var(--font-size-sm);
    text-decoration: none;
    border-radius: var(--radius-md);
    margin: 1px var(--space-1);
    transition: background-color var(--duration-fast) var(--ease-out), color var(--duration-fast) var(--ease-out);
  }

  .sidebar__bottom-link:hover {
    background-color: var(--color-surface-overlay);
    color: var(--color-text-primary);
  }

  .sidebar__bottom-link:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: -2px;
  }

  /* Active state — current page link gets accent text */
  .sidebar__bottom-link--active {
    color: var(--color-accent);
    background-color: var(--color-accent-subtle);
  }

  .sidebar__bottom-link--active:hover {
    background-color: var(--color-accent-subtle);
    color: var(--color-accent);
  }

  /* Pending count badge on the Review Queue link */
  .sidebar__queue-badge {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-width: 18px;
    height: 18px;
    padding: 0 4px;
    margin-left: auto;
    background-color: var(--color-accent);
    color: var(--color-text-inverse);
    font-size: var(--font-size-sm);
    font-weight: 600;
    border-radius: 9px;
    font-variant-numeric: tabular-nums;
    flex-shrink: 0;
    /* Subtle pulse to draw attention to new items without being annoying */
    animation: badge-pop 200ms ease both;
  }

  @keyframes badge-pop {
    from { transform: scale(0.7); opacity: 0; }
    to   { transform: scale(1);   opacity: 1; }
  }

  /* ── Overview panel (ultra-wide only) ─────────────────────────────────────── */
  .sidebar__overview-wrapper {
    display: none;
    border-top: 1px solid var(--border-hairline);
    overflow-y: auto;
    flex-shrink: 0;
  }

  @media (min-width: 2560px) {
    .sidebar__overview-wrapper {
      display: block;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .sidebar__queue-badge {
      animation: none;
    }

    .sidebar,
    .sidebar__content {
      transition: none;
    }
  }
</style>
