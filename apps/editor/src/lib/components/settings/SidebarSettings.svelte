<script lang="ts">
  /**
   * SidebarSettings — Category 3
   *
   * Default panel, collapse behavior, doc tree grouping.
   */

  import { userPrefs, updatePrefs } from '$lib/stores/preferences';
  import { sidebarStore } from '$lib/stores/sidebar';

  interface Props {
    searchQuery?: string;
  }

  let { searchQuery = '' }: Props = $props();

  const panels = [
    { value: 'tree' as const, label: 'Doc tree', description: 'Hierarchical document navigator' },
    { value: 'filter' as const, label: 'Filter', description: 'Contextual filter panel' },
    { value: 'overview' as const, label: 'Overview', description: 'Ultra-wide overview panel' },
  ];

  const groupings = [
    { value: 'type-first' as const, label: 'Type-first', description: 'Group by ADR / how-to / runbook…' },
    { value: 'folder-first' as const, label: 'Folder-first', description: 'Mirror the filesystem hierarchy' },
    { value: 'flat' as const, label: 'Flat', description: 'Alphabetical, ungrouped' },
  ];

  const positions = [
    { value: 'left' as const, label: 'Left' },
    { value: 'right' as const, label: 'Right' },
  ];

  function matches(text: string): boolean {
    if (!searchQuery) return true;
    return text.toLowerCase().includes(searchQuery.toLowerCase());
  }

  const prefs = $derived($userPrefs.sidebar);
</script>

<div class="settings-category">

  <!-- Position -->
  {#if matches('position') || matches('sidebar') || matches('left') || matches('right')}
    <div class="setting-row">
      <div class="setting-row__label">
        <span class="setting-row__name">Sidebar position</span>
        <span class="setting-row__desc">Which edge the sidebar is docked to.</span>
      </div>
      <div class="setting-row__control">
        <div class="seg-buttons" role="group" aria-label="Sidebar position">
          {#each positions as pos (pos.value)}
            <button
              type="button"
              class="seg-btn"
              class:seg-btn--active={$sidebarStore.position === pos.value}
              aria-pressed={$sidebarStore.position === pos.value}
              onclick={() => sidebarStore.setPosition(pos.value)}
            >{pos.label}</button>
          {/each}
        </div>
      </div>
    </div>
  {/if}

  <!-- Default panel -->
  {#if matches('panel') || matches('sidebar') || matches('default')}
    <div class="setting-row">
      <div class="setting-row__label">
        <span class="setting-row__name">Default panel</span>
        <span class="setting-row__desc">Which panel is shown when you open a project.</span>
      </div>
      <div class="setting-row__control">
        <div class="seg-buttons" role="group" aria-label="Default panel">
          {#each panels as p (p.value)}
            <button
              type="button"
              class="seg-btn"
              class:seg-btn--active={prefs.defaultPanel === p.value}
              aria-pressed={prefs.defaultPanel === p.value}
              title={p.description}
              onclick={() => updatePrefs('sidebar', { defaultPanel: p.value })}
            >{p.label}</button>
          {/each}
        </div>
      </div>
    </div>
  {/if}

  <!-- Collapse on open -->
  {#if matches('collapse') || matches('auto-collapse') || matches('sidebar')}
    <div class="setting-row">
      <div class="setting-row__label">
        <span class="setting-row__name">Collapse when opening a doc</span>
        <span class="setting-row__desc">Auto-collapse the sidebar when a document is opened, giving more horizontal space to the editor.</span>
      </div>
      <div class="setting-row__control">
        <button
          type="button"
          role="switch"
          class="toggle-switch"
          class:toggle-switch--on={prefs.collapseOnOpen}
          aria-checked={prefs.collapseOnOpen}
          onclick={() => updatePrefs('sidebar', { collapseOnOpen: !prefs.collapseOnOpen })}
          aria-label="Toggle collapse on open"
        >
          <span class="toggle-switch__thumb" aria-hidden="true"></span>
        </button>
      </div>
    </div>
  {/if}

  <!-- Doc tree grouping -->
  {#if matches('tree') || matches('grouping') || matches('type-first') || matches('folder')}
    <div class="setting-row">
      <div class="setting-row__label">
        <span class="setting-row__name">Doc tree grouping</span>
        <span class="setting-row__desc">How documents are grouped in the sidebar tree.</span>
      </div>
      <div class="setting-row__control">
        <div class="seg-buttons" role="group" aria-label="Doc tree grouping">
          {#each groupings as g (g.value)}
            <button
              type="button"
              class="seg-btn"
              class:seg-btn--active={prefs.docTreeGrouping === g.value}
              aria-pressed={prefs.docTreeGrouping === g.value}
              title={g.description}
              onclick={() => updatePrefs('sidebar', { docTreeGrouping: g.value })}
            >{g.label}</button>
          {/each}
        </div>
      </div>
    </div>
  {/if}

</div>

<style>
  .settings-category {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .setting-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-6);
    padding: var(--space-3) 0;
    border-bottom: 1px solid var(--color-border);
  }

  .setting-row:last-child {
    border-bottom: none;
  }

  .setting-row__label {
    display: flex;
    flex-direction: column;
    gap: 2px;
    flex: 1;
    min-width: 0;
  }

  .setting-row__name {
    font-size: var(--font-size-sm);
    font-weight: 500;
    color: var(--color-text-primary);
  }

  .setting-row__desc {
    font-size: var(--font-size-sm);
    color: var(--color-text-muted);
    line-height: 1.4;
  }

  .setting-row__control {
    flex-shrink: 0;
  }

  .seg-buttons {
    display: flex;
    gap: 2px;
    background-color: var(--color-surface-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: 2px;
  }

  .seg-btn {
    padding: var(--space-1) var(--space-3);
    background: none;
    border: none;
    border-radius: calc(var(--radius-md) - 2px);
    color: var(--color-text-muted);
    font-size: var(--font-size-sm);
    font-family: var(--font-sans);
    cursor: pointer;
    transition: background-color 100ms ease, color 100ms ease;
    white-space: nowrap;
    line-height: 1.4;
  }

  .seg-btn:hover {
    color: var(--color-text-primary);
  }

  .seg-btn--active {
    background-color: var(--color-surface-base);
    color: var(--color-text-primary);
    font-weight: 500;
    box-shadow: var(--shadow-sm);
  }

  .seg-btn:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
  }

  .toggle-switch {
    position: relative;
    display: inline-flex;
    align-items: center;
    width: 40px;
    height: 22px;
    background-color: var(--color-surface-elevated);
    border: 1px solid var(--color-border);
    border-radius: 11px;
    cursor: pointer;
    padding: 0;
    transition: background-color 150ms ease, border-color 150ms ease;
  }

  .toggle-switch--on {
    background-color: var(--color-accent);
    border-color: var(--color-accent);
  }

  .toggle-switch:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
  }

  .toggle-switch__thumb {
    position: absolute;
    left: 2px;
    width: 16px;
    height: 16px;
    background-color: var(--color-text-inverse);
    border-radius: 50%;
    transition: transform 150ms ease;
    box-shadow: var(--shadow-sm);
  }

  .toggle-switch--on .toggle-switch__thumb {
    transform: translateX(18px);
  }
</style>
