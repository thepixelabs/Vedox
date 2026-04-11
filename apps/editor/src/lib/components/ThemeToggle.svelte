<script lang="ts">
  import { themeStore } from "$lib/stores/theme";

  const theme = themeStore;
</script>

<!--
  ThemeToggle — a button that switches between dark and light themes.
  Lives in the sidebar bottom bar.
  Uses aria-pressed to communicate current state to screen readers.
-->
<button
  class="theme-toggle"
  type="button"
  aria-label={$theme === "dark" ? "Switch to light theme" : "Switch to dark theme"}
  aria-pressed={$theme === "light"}
  onclick={() => theme.toggle()}
>
  <span class="theme-toggle__icon" aria-hidden="true">
    {#if $theme === "dark"}
      <!-- Sun icon — clicking will go to light -->
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <circle cx="12" cy="12" r="4"/>
        <path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M6.34 17.66l-1.41 1.41M19.07 4.93l-1.41 1.41"/>
      </svg>
    {:else}
      <!-- Moon icon — clicking will go to dark -->
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/>
      </svg>
    {/if}
  </span>
  <span class="theme-toggle__label">
    {$theme === "dark" ? "Light mode" : "Dark mode"}
  </span>
</button>

<style>
  .theme-toggle {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    width: 100%;
    padding: var(--space-2) var(--space-3);
    background: none;
    border: none;
    border-radius: var(--radius-md);
    color: var(--color-text-secondary);
    font-family: var(--font-sans);
    font-size: var(--font-size-sm);
    cursor: pointer;
    text-align: left;
    transition: background-color var(--duration-fast) var(--ease-out), color var(--duration-fast) var(--ease-out);
  }

  .theme-toggle:hover {
    background-color: var(--color-surface-overlay);
    color: var(--color-text-primary);
  }

  .theme-toggle:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
  }

  .theme-toggle__icon {
    display: flex;
    align-items: center;
    flex-shrink: 0;
    color: var(--color-text-muted);
  }

  .theme-toggle:hover .theme-toggle__icon {
    color: var(--color-text-secondary);
  }
</style>
