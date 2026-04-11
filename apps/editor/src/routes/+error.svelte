<script lang="ts">
  /**
   * +error.svelte — application error boundary.
   *
   * Shown when a SvelteKit load function throws an unexpected error.
   * Provides a minimal, helpful error display that matches the design system.
   */

  import { page } from "$app/stores";
</script>

<div class="error-page" role="main">
  <div class="error-page__card">
    <div class="error-page__icon" aria-hidden="true">
      <svg width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
        <circle cx="12" cy="12" r="10"/>
        <line x1="12" y1="8" x2="12" y2="12"/>
        <line x1="12" y1="16" x2="12.01" y2="16"/>
      </svg>
    </div>

    <h1 class="error-page__code">{$page.status}</h1>
    <p class="error-page__message">{$page.error?.message ?? "An unexpected error occurred."}</p>

    <div class="error-page__actions">
      <a href="/" class="error-page__btn">Go home</a>
      <button
        type="button"
        class="error-page__btn error-page__btn--secondary"
        onclick={() => window.location.reload()}
      >
        Reload
      </button>
    </div>
  </div>
</div>

<style>
  .error-page {
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 100vh;
    padding: var(--space-8);
    background-color: var(--color-surface-base);
  }

  .error-page__card {
    display: flex;
    flex-direction: column;
    align-items: center;
    text-align: center;
    max-width: 400px;
    gap: var(--space-3);
  }

  .error-page__icon {
    color: var(--color-error);
    opacity: 0.8;
    margin-bottom: var(--space-2);
  }

  .error-page__code {
    font-size: 48px;
    font-weight: 700;
    color: var(--color-text-primary);
    font-family: var(--font-mono);
    letter-spacing: -0.04em;
    line-height: 1;
  }

  .error-page__message {
    font-size: var(--font-size-base);
    color: var(--color-text-secondary);
    line-height: 1.6;
    max-width: 320px;
  }

  .error-page__actions {
    display: flex;
    gap: var(--space-3);
    margin-top: var(--space-4);
    flex-wrap: wrap;
    justify-content: center;
  }

  .error-page__btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    padding: var(--space-2) var(--space-5);
    background-color: var(--color-accent);
    color: #fff;
    font-size: var(--font-size-sm);
    font-weight: 500;
    font-family: var(--font-sans);
    text-decoration: none;
    border: none;
    border-radius: var(--radius-md);
    cursor: pointer;
    transition: background-color 100ms ease;
  }

  .error-page__btn:hover {
    background-color: var(--color-accent-hover);
    color: #fff;
    text-decoration: none;
  }

  .error-page__btn:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 3px;
  }

  .error-page__btn--secondary {
    background-color: var(--color-surface-elevated);
    color: var(--color-text-primary);
    border: 1px solid var(--color-border);
  }

  .error-page__btn--secondary:hover {
    background-color: var(--color-surface-overlay);
    color: var(--color-text-primary);
  }
</style>
