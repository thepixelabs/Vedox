<script lang="ts">
  /**
   * +layout.svelte — root application layout.
   *
   * Implements the progressive disclosure fork:
   *   - No projects: full-viewport empty state, single CTA, no sidebar chrome
   *   - Has projects: sidebar + main content area layout
   *
   * Also handles pre-hydration theme sync (reads localStorage before first
   * render to prevent a theme flash on page load).
   */

  import { onMount, onDestroy } from "svelte";
  import "../app.css";
  import Sidebar from "$lib/components/Sidebar.svelte";
  import ToastContainer from "$lib/components/Toast/ToastContainer.svelte";
  import WizardDraftCard from "$lib/components/WizardDraftCard.svelte";
  import ImportDialog from "$lib/components/ImportDialog.svelte";
  import CommandPalette from "$lib/components/CommandPalette/CommandPalette.svelte";
  import { page } from "$app/stores";
  import { themeStore } from "$lib/stores/theme";
  import { densityStore } from "$lib/theme/store";
  import { projectsStore, hasProjects } from "$lib/stores/projects";
  import EmptyState from "$lib/components/EmptyState.svelte";
  import { sidebarStore } from "$lib/stores/sidebar";
  import { dispatchShortcut, registerShortcut } from "$lib/keyboard/shortcuts";
  import { panesStore } from "$lib/stores/panes";
  import { readingStore } from "$lib/stores/reading";
  import { get } from "svelte/store";
  import { openPalette, setQuery } from "$lib/components/CommandPalette/store";

  interface Props {
    data: {
      projects: import("$lib/stores/projects").Project[];
      error: string | null;
    };
    children: import("svelte").Snippet;
  }

  let { data, children }: Props = $props();

  const isNewProjectPage = $derived($page.url.pathname === "/projects/new");
  const shouldShowAppShell = $derived($hasProjects || isNewProjectPage);

  // ── Import Dialog ──────────────────────────────────────────────────────────
  let importDialogOpen = $state(false);

  function handleOpenImportDialog(e?: Event) {
    if (e) e.preventDefault();
    importDialogOpen = true;
  }

  function handleImported() {
    // Reload projects from backend or just let the store update if the dialog
    // already handled it (ImportDialog.svelte calls onImported which usually
    // updates the store in the parent, but here we can just refresh).
    window.location.reload();
  }

  // Seed the store from layout load data.
  // The load function already mapped API projects → store shape.
  projectsStore.setProjects(data.projects);

  // ── Keyboard shortcuts ──────────────────────────────────────────────────
  let unregisterShortcuts: (() => void)[] = [];

  onMount(() => {
    // Restore user font preferences before first paint.
    const fontKeys = ['font-body', 'font-display', 'font-mono'] as const;
    for (const key of fontKeys) {
      const val = localStorage.getItem(`vedox:${key}`);
      if (val) {
        document.documentElement.style.setProperty(`--${key}`, val);
      }
    }

    // Sync flagship theme + density stores with localStorage after hydration.
    // The pre-hydration inline script in app.html already set the DOM
    // attributes; these calls reconcile the Svelte store state so reactive
    // consumers see the same value.
    themeStore.sync();
    densityStore.sync();

    // Ensure the theme-ready class is on documentElement — the inline script
    // in app.html adds it, but we set it again here in case the page was
    // hydrated without the inline script running (e.g. dev-server HMR).
    document.documentElement.classList.add("theme-ready");

    // Sync sidebar store with localStorage
    const stored = localStorage.getItem("vedox:sidebar-collapsed");
    if (stored !== null) {
      sidebarStore.setCollapsed(stored === "true");
    }

    // Register global shortcuts
    unregisterShortcuts = [
      registerShortcut({
        key: '\\',
        meta: true,
        shift: false,
        description: 'Split pane vertically',
        handler: () => panesStore.split(),
      }),
      registerShortcut({
        key: 'l',
        meta: true,
        shift: true,
        description: 'Cycle reading measure (narrow/default/wide)',
        handler: () => readingStore.cycle(),
      }),
      registerShortcut({
        key: '[',
        meta: true,
        description: 'Decrease sidebar width',
        handler: () => sidebarStore.incrementWidth(-20),
      }),
      registerShortcut({
        key: ']',
        meta: true,
        description: 'Increase sidebar width',
        handler: () => sidebarStore.incrementWidth(20),
      }),
      registerShortcut({
        key: 'p',
        meta: true,
        shift: false,
        description: 'Quick file open',
        handler: () => {
          setQuery('/');
          openPalette();
        },
      }),
      registerShortcut({
        key: 'w',
        meta: true,
        description: 'Close active pane',
        handler: () => {
          const activeId = get(panesStore.activePaneId);
          if (activeId) panesStore.close(activeId);
        },
      }),
    ];
  });

  onDestroy(() => {
    unregisterShortcuts.forEach((fn) => fn());
  });
</script>

<svelte:window onkeydown={dispatchShortcut} />

<a class="skip-link" href="#main-content">Skip to main content</a>

{#if data.error}
  <!--
    Backend connectivity error banner.
    Shown above the main content regardless of project state.
    Dismissible so it doesn't permanently block the UI.
  -->
  <div class="backend-error" role="alert" aria-live="assertive">
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
      <circle cx="12" cy="12" r="10"/>
      <line x1="12" y1="8" x2="12" y2="12"/>
      <line x1="12" y1="16" x2="12.01" y2="16"/>
    </svg>
    <span class="backend-error__text">
      Backend unavailable — {data.error}
    </span>
  </div>
{/if}

<!--
  Toast container is mounted unconditionally so notifications are visible
  regardless of which branch (empty state vs. full app shell) is active.
-->
<ToastContainer />
<WizardDraftCard />

{#if shouldShowAppShell}
  <!--
    Full layout: sidebar + main content.
    Only rendered once at least one project exists.
  -->
  <div class="app-shell">
    <Sidebar projects={data.projects} />
    <main class="app-shell__main" id="main-content">
      {@render children()}
    </main>
  </div>
{:else}
  <!--
    Empty state: first-run experience.
    Single call-to-action only — no sidebar, no chrome.
    Progressive disclosure: earn the complexity.
  -->
  <div class="layout-empty-wrapper" role="main">
    <EmptyState
      icon={`<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/></svg>`}
      heading="No projects"
      body="Open a folder to start. Your docs stay on disk as plain Markdown."
      cta={{ label: "Open folder", href: "/projects/new", onClick: handleOpenImportDialog }}
    />
  </div>
{/if}

<ImportDialog
  bind:open={importDialogOpen}
  onImported={handleImported}
  onLinked={handleImported}
/>

<!--
  Global command palette. Mounted at the root so Cmd+K works on every route.
  The component is a no-op until the user opens it, so there's no cost to
  always-on mounting.
-->
<CommandPalette />

<style>
  /* ── Skip link (WCAG 2.2 AA: bypass blocks) ───────────────────────────────── */
  .skip-link {
    position: absolute;
    left: -9999px;
    top: auto;
    width: 1px;
    height: 1px;
    overflow: hidden;
    z-index: 2000;
  }

  .skip-link:focus {
    position: fixed;
    left: var(--space-4);
    top: var(--space-4);
    width: auto;
    height: auto;
    padding: var(--space-2) var(--space-4);
    background-color: var(--color-accent);
    color: #fff;
    font-size: var(--font-size-base);
    font-weight: 500;
    border-radius: var(--radius-md);
    text-decoration: none;
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
  }

  /* ── Backend error banner ──────────────────────────────────────────────────── */
  .backend-error {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-4);
    background-color: color-mix(in srgb, var(--color-error, #e53e3e) 12%, transparent);
    border-bottom: 1px solid color-mix(in srgb, var(--color-error, #e53e3e) 30%, transparent);
    color: var(--color-error, #e53e3e);
    font-size: var(--font-size-sm);
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    z-index: 1000;
  }

  .backend-error__text {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  /* ── Full app shell layout ─────────────────────────────────────────────────── */
  .app-shell {
    display: flex;
    height: 100vh;
    overflow: hidden;
    background-color: var(--color-surface-base);
  }

  .app-shell__main {
    flex: 1;
    min-width: 0; /* prevent flex child from overflowing */
    overflow-y: auto;
    background-color: var(--color-surface-base);
  }

  /* ── Empty / first-run state wrapper ──────────────────────────────────────── */
  .layout-empty-wrapper {
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 100vh;
    background-color: var(--color-surface-base);
  }
</style>
