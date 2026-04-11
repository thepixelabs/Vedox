<script lang="ts">
  /**
   * ProjectTree — document list for the currently active project.
   *
   * ARIA: role="tree" / role="treeitem" pattern.
   * Keyboard: Arrow keys navigate items, Enter follows link.
   *
   * Phase 1 scope: flat doc list (no nested folders yet).
   * Phase 2 will add hierarchical tree expansion when the workspace
   * scanner produces folder structure.
   */

  import { page } from "$app/stores";
  import type { Project } from "$lib/stores/projects";
  import { panesStore } from "$lib/stores/panes";
  import EmptyState from "./EmptyState.svelte";

  interface Props {
    project: Project;
  }

  let { project }: Props = $props();

  let treeEl: HTMLUListElement | undefined = $state();

  const currentPath = $derived(
    ($page.params as Record<string, string>)["path"] ?? null
  );

  function getDocUrl(docPath: string): string {
    return `/projects/${project.id}/docs/${docPath}`;
  }

  function handleKeydown(event: KeyboardEvent) {
    if (!treeEl) return;
    const items = Array.from(
      treeEl.querySelectorAll<HTMLElement>('[role="treeitem"]')
    );
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
        } else {
          // Wrap focus back to the tree container
          treeEl.focus();
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
          (focused as HTMLElement).click();
        }
        break;
    }
  }

  /**
   * Cmd+click (Mac) / Ctrl+click (Windows) opens the doc in a new split pane
   * instead of navigating in-place. Normal clicks fall through to default
   * anchor behaviour (SvelteKit client-side navigation).
   */
  function handleDocClick(e: MouseEvent, docPath: string) {
    if (e.metaKey || e.ctrlKey) {
      e.preventDefault();
      panesStore.split();
      panesStore.open(docPath);
    }
  }
</script>

<nav aria-label="Documents in {project.name}">
  {#if project.docs.length === 0}
    <EmptyState
      icon={`<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M14.5 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7.5L14.5 2z"/><polyline points="14 2 14 8 20 8"/></svg>`}
      heading="Empty folder"
      body="Create a doc with ⌘N."
    />
  {:else}
    <ul
      bind:this={treeEl}
      class="tree"
      role="tree"
      aria-label="Documents"
      tabindex="0"
      onkeydown={handleKeydown}
    >
      {#each project.docs as doc (doc.path)}
        {@const isActive = currentPath === doc.path}
        <li
          class="tree__item"
          role="none"
        >
          <a
            class="tree__link"
            class:tree__link--active={isActive}
            href={getDocUrl(doc.path)}
            role="treeitem"
            aria-selected={isActive}
            tabindex="-1"
            title="Click to open · ⌘Click to open in split pane"
            onclick={(e) => handleDocClick(e, doc.path)}
          >
            <!-- File icon -->
            <span class="tree__icon" aria-hidden="true">
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M14.5 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7.5L14.5 2z"/>
                <polyline points="14 2 14 8 20 8"/>
              </svg>
            </span>
            <span class="tree__label">{doc.title}</span>
          </a>
        </li>
      {/each}
    </ul>
  {/if}
</nav>

<style>
  .tree__empty {
    padding: var(--space-3) var(--space-3);
    color: var(--color-text-muted);
    font-size: var(--font-size-sm);
  }

  .tree {
    list-style: none;
    padding: var(--space-1) 0;
  }

  /* Invisible focus on the tree container itself (navigated past via arrow keys) */
  .tree:focus {
    outline: none;
  }

  .tree__item {
    display: contents;
  }

  .tree__link {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: 5px var(--space-3);
    color: var(--color-text-secondary);
    font-size: var(--font-size-sm);
    text-decoration: none;
    border-radius: var(--radius-sm);
    margin: 1px var(--space-1);
    transition: background-color 80ms var(--ease-out), color 80ms var(--ease-out);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .tree__link:hover {
    background-color: var(--color-surface-overlay);
    color: var(--color-text-primary);
  }

  .tree__link--active {
    background-color: var(--color-accent-subtle);
    color: var(--color-accent);
    font-weight: 500;
  }

  .tree__link--active:hover {
    background-color: var(--color-accent-subtle);
    color: var(--color-accent-hover);
  }

  /* Keyboard focus indicator — not just browser default */
  .tree__link:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: -2px;
  }

  .tree__icon {
    flex-shrink: 0;
    color: var(--color-text-muted);
    display: flex;
    align-items: center;
  }

  .tree__link--active .tree__icon {
    color: var(--color-accent);
  }

  .tree__label {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-family: var(--font-mono);
    font-size: var(--font-size-sm);
  }
</style>
