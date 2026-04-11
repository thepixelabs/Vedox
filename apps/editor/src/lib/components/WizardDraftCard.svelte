<script lang="ts">
  import { goto } from '$app/navigation';
  import { page } from '$app/stores';
  import { draft, clearDraft } from '$lib/stores/wizardDraft';
  import { scale } from 'svelte/transition';
  import { cubicOut, cubicInOut } from 'svelte/easing';

  // Hide the card on the wizard page itself — the full form is visible there.
  const isWizardPage = $derived($page.url.pathname === '/projects/new');
  const visible = $derived($draft !== null && !isWizardPage);

  const displayName = $derived(
    $draft?.projectName?.trim() || 'Untitled project'
  );

  let cardEl: HTMLDivElement | undefined;
  let expanding = $state(false);

  function handleExpand() {
    if (expanding) return;
    expanding = true;
    // Brief CSS class-driven scale-up before navigation gives the "card expands"
    // illusion. 150ms matches the scale transition duration below.
    setTimeout(() => goto('/projects/new'), 150);
  }

  function handleDiscard(e: MouseEvent) {
    e.stopPropagation();
    clearDraft();
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      handleExpand();
    }
  }
</script>

{#if visible}
  <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
  <div
    bind:this={cardEl}
    class="wizard-draft-card"
    class:wizard-draft-card--expanding={expanding}
    role="button"
    tabindex="0"
    aria-label="Resume new project: {displayName}"
    onclick={handleExpand}
    onkeydown={handleKeydown}
    transition:scale={{ duration: 220, start: 0.65, opacity: 0, easing: cubicInOut }}
  >
    <div class="wizard-draft-card__icon" aria-hidden="true">
      <svg width="16" height="16" viewBox="0 0 48 48" fill="none" xmlns="http://www.w3.org/2000/svg">
        <circle cx="24" cy="24" r="22" stroke="currentColor" stroke-width="1.5" opacity="0.4"/>
        <path d="M16 24 L24 14 L32 24 L24 34 Z" stroke="currentColor" stroke-width="1.5" fill="none" opacity="0.8"/>
        <circle cx="24" cy="24" r="3" fill="currentColor"/>
      </svg>
    </div>

    <div class="wizard-draft-card__body">
      <span class="wizard-draft-card__label">New project</span>
      <span class="wizard-draft-card__name">{displayName}</span>
    </div>

    <button
      type="button"
      class="wizard-draft-card__discard"
      aria-label="Discard draft"
      onclick={handleDiscard}
      onkeydown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          e.stopPropagation();
          clearDraft();
        }
      }}
    >
      <svg width="10" height="10" viewBox="0 0 12 12" fill="none" aria-hidden="true">
        <path d="M1 1l10 10M11 1L1 11" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
      </svg>
    </button>
  </div>
{/if}

<style>
  .wizard-draft-card {
    position: fixed;
    top: var(--space-4, 16px);
    right: var(--space-4, 16px);
    z-index: 50; /* above sidebar, below modals */
    display: flex;
    align-items: center;
    gap: var(--space-3, 12px);
    padding: var(--space-3, 12px) var(--space-4, 16px);
    background: var(--surface-4, #2a2a2a);
    border: 1px solid var(--accent-border, oklch(60% 0.15 250 / 0.4));
    border-radius: var(--radius-xl, 14px);
    box-shadow:
      0 4px 24px oklch(0% 0 0 / 0.3),
      0 1px 4px oklch(0% 0 0 / 0.2);
    cursor: pointer;
    max-width: 260px;
    color: var(--text-1);
    user-select: none;
    /* Entry: animate scale in from top-right (transform-origin top right) */
    transform-origin: top right;
    transition:
      border-color var(--duration-fast, 100ms) var(--ease-out, ease-out),
      background-color var(--duration-fast, 100ms) var(--ease-out, ease-out),
      transform 150ms var(--ease-snap, cubic-bezier(0.85, 0, 0.15, 1));
  }

  .wizard-draft-card:hover {
    border-color: var(--accent-solid);
    background: color-mix(in oklch, var(--surface-4, #2a2a2a) 85%, var(--accent-solid) 15%);
  }

  .wizard-draft-card:focus-visible {
    outline: 2px solid var(--accent-solid);
    outline-offset: 3px;
  }

  /* Expanding: scale up slightly before goto() fires */
  .wizard-draft-card--expanding {
    transform: scale(1.06);
    opacity: 0.8;
  }

  .wizard-draft-card__icon {
    color: var(--accent-solid);
    flex-shrink: 0;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .wizard-draft-card__body {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
    flex: 1;
  }

  .wizard-draft-card__label {
    font-size: 10px;
    font-weight: 700;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--accent-text);
    line-height: 1;
  }

  .wizard-draft-card__name {
    font-size: 13px;
    font-weight: 500;
    color: var(--text-1);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    line-height: 1.3;
  }

  .wizard-draft-card__discard {
    flex-shrink: 0;
    width: 22px;
    height: 22px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: none;
    border: none;
    border-radius: var(--radius-full, 9999px);
    color: var(--text-3);
    cursor: pointer;
    padding: 0;
    transition:
      color var(--duration-fast, 100ms) var(--ease-out, ease-out),
      background-color var(--duration-fast, 100ms) var(--ease-out, ease-out);
  }

  .wizard-draft-card__discard:hover {
    color: var(--text-1);
    background-color: var(--surface-3, oklch(25% 0 0));
  }

  .wizard-draft-card__discard:focus-visible {
    outline: 2px solid var(--accent-solid);
    outline-offset: 2px;
    border-radius: var(--radius-full, 9999px);
  }

  @media (prefers-reduced-motion: reduce) {
    .wizard-draft-card {
      transition: none;
    }
    .wizard-draft-card--expanding {
      transform: none;
    }
  }
</style>
