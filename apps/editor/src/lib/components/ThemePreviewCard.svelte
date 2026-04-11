<!--
  ThemePreviewCard.svelte — miniature Vedox surface preview for a theme.

  Renders a tiny sidebar strip + editor area swatch so the user can
  visually compare themes before selecting. Uses the target theme's
  data-theme attribute scoping to show authentic colours.
-->
<script lang="ts">
  import type { Theme } from '$lib/theme/store';
  import { themeStore } from '$lib/theme/store';

  const { theme, label, description = '' }: {
    theme: Theme;
    label: string;
    description?: string;
  } = $props();

  const isActive = $derived($themeStore === theme);

  function select() {
    themeStore.setTheme(theme);
  }
</script>

<button
  class="theme-card"
  class:theme-card--active={isActive}
  data-theme={theme}
  onclick={select}
  type="button"
  aria-label="Switch to {label} theme"
  aria-pressed={isActive}
>
  <!-- Mini preview: sidebar strip + content area -->
  <div class="theme-card__preview">
    <div class="theme-card__sidebar"></div>
    <div class="theme-card__content">
      <div class="theme-card__line theme-card__line--heading"></div>
      <div class="theme-card__line"></div>
      <div class="theme-card__line theme-card__line--short"></div>
      <div class="theme-card__line theme-card__line--code"></div>
    </div>
  </div>
  <div class="theme-card__label">{label}</div>
  {#if description}
    <div class="theme-card__desc">{description}</div>
  {/if}
</button>

<style>
  .theme-card {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    padding: var(--space-3);
    border-radius: var(--radius-lg);
    border: 1px solid var(--border-default, var(--color-border));
    background: var(--surface-2, var(--color-surface-elevated));
    cursor: pointer;
    text-align: left;
    transition: border-color var(--duration-fast) var(--ease-out), box-shadow var(--duration-fast) var(--ease-out);
    width: 100%;
  }

  .theme-card:hover {
    border-color: var(--border-strong, var(--color-text-muted));
  }

  .theme-card--active {
    border-color: var(--accent-solid, var(--color-accent));
    box-shadow: 0 0 0 2px var(--accent-subtle, color-mix(in srgb, var(--color-accent) 20%, transparent));
  }

  .theme-card__preview {
    display: flex;
    height: 64px;
    border-radius: var(--radius-sm, 6px);
    overflow: hidden;
    border: 1px solid var(--border-hairline, var(--color-border));
  }

  .theme-card__sidebar {
    width: 28%;
    background: var(--surface-3, var(--color-surface-base));
    border-right: 1px solid var(--border-hairline, var(--color-border));
  }

  .theme-card__content {
    flex: 1;
    background: var(--surface-1, var(--color-surface-elevated));
    padding: 6px 8px;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .theme-card__line {
    height: 3px;
    background: var(--text-3, var(--color-text-muted));
    border-radius: 2px;
    opacity: 0.5;
  }

  .theme-card__line--heading {
    background: var(--text-1, var(--color-text-primary));
    height: 5px;
    opacity: 0.9;
    width: 60%;
  }

  .theme-card__line--short {
    width: 45%;
  }

  .theme-card__line--code {
    background: var(--accent-solid, var(--color-accent));
    width: 70%;
    opacity: 0.4;
  }

  .theme-card__label {
    font-size: var(--text-sm, var(--font-size-sm));
    font-weight: 600;
    color: var(--text-1, var(--color-text-primary));
  }

  .theme-card__desc {
    font-size: var(--text-xs, 11px);
    color: var(--text-3, var(--color-text-muted));
  }
</style>
