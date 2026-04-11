<script lang="ts">
  /**
   * SidebarDock — bottom dock bar for the Sidebar.
   *
   * Four 28px-square icon buttons:
   *   1. Theme toggle (cycles through 5 themes)
   *   2. Density toggle (compact / comfortable / cozy)
   *   3. Settings link (/settings)
   *   4. Collapse button
   *
   * Imports theme + density stores from $lib/theme/store.ts.
   */

  import { themeStore, densityStore } from "$lib/theme/store";
  import type { Theme, Density } from "$lib/theme/store";
  import { sidebarStore } from "$lib/stores/sidebar";

  const sidebar = sidebarStore;

  const ALL_THEMES: readonly Theme[] = themeStore.all();
  const ALL_DENSITIES: readonly Density[] = densityStore.all();

  function cycleTheme(): void {
    const current = ALL_THEMES.indexOf($themeStore);
    const next = ALL_THEMES[(current + 1) % ALL_THEMES.length];
    themeStore.setTheme(next);
  }

  function cycleDensity(): void {
    const current = ALL_DENSITIES.indexOf($densityStore);
    const next = ALL_DENSITIES[(current + 1) % ALL_DENSITIES.length];
    densityStore.setDensity(next);
  }

  /** Capitalise first letter for tooltip labels. */
  function cap(s: string): string {
    return s.charAt(0).toUpperCase() + s.slice(1);
  }
</script>

<div class="dock" role="toolbar" aria-label="Sidebar actions">
  <!-- Theme cycle -->
  <button
    class="dock__btn"
    type="button"
    title="Theme: {cap($themeStore)}"
    aria-label="Cycle theme (currently {$themeStore})"
    onclick={cycleTheme}
  >
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
      <circle cx="12" cy="12" r="4"/>
      <path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M6.34 17.66l-1.41 1.41M19.07 4.93l-1.41 1.41"/>
    </svg>
  </button>

  <!-- Density cycle -->
  <button
    class="dock__btn"
    type="button"
    title="Density: {cap($densityStore)}"
    aria-label="Cycle density (currently {$densityStore})"
    onclick={cycleDensity}
  >
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
      <line x1="3" y1="6" x2="21" y2="6"/>
      <line x1="3" y1="12" x2="21" y2="12"/>
      <line x1="3" y1="18" x2="21" y2="18"/>
    </svg>
  </button>

  <!-- Settings link -->
  <a
    class="dock__btn"
    href="/settings"
    title="Settings"
    aria-label="Settings"
  >
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
      <path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"/>
      <circle cx="12" cy="12" r="3"/>
    </svg>
  </a>

  <!-- Spacer -->
  <span class="dock__spacer"></span>

  <!-- Collapse -->
  <button
    class="dock__btn"
    type="button"
    title={$sidebar.collapsed ? "Expand sidebar" : "Collapse sidebar"}
    aria-label={$sidebar.collapsed ? "Expand sidebar" : "Collapse sidebar"}
    aria-expanded={!$sidebar.collapsed}
    onclick={() => sidebar.toggle()}
  >
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
      {#if $sidebar.collapsed}
        <polyline points="9 18 15 12 9 6"/>
      {:else}
        <polyline points="15 18 9 12 15 6"/>
      {/if}
    </svg>
  </button>
</div>

<style>
  .dock {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    padding: var(--space-2);
    border-top: 1px solid var(--border-hairline);
    flex-shrink: 0;
  }

  .dock__btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 28px;
    height: 28px;
    border-radius: var(--radius-sm);
    color: var(--text-3);
    background: transparent;
    border: none;
    cursor: pointer;
    text-decoration: none;
    transition: background-color var(--duration-fast) var(--ease-out), color var(--duration-fast) var(--ease-out);
  }

  .dock__btn:hover {
    background: var(--surface-4);
    color: var(--text-1);
  }

  .dock__btn:focus-visible {
    outline: 2px solid var(--accent-solid);
    outline-offset: 2px;
  }

  .dock__spacer {
    flex: 1;
  }
</style>
