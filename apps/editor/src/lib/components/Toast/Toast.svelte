<script lang="ts">
  /**
   * Toast.svelte — single toast notification card.
   *
   * Slides in from the right with a 200ms ease-out.
   * Auto-dismisses after `duration` ms (default 5000). Passing duration=0
   * makes the toast sticky. Hovering pauses the countdown.
   *
   * Accessibility:
   *   - role="status" (polite) for info/success
   *   - role="alert"  (assertive) for warning/error
   *   - Close button has aria-label
   *   - Action buttons use their label as accessible text
   */

  import { onMount, onDestroy } from 'svelte';
  import { dismissToast } from './toastStore';
  import type { ToastProps } from './toastStore';

  interface Props {
    toast: ToastProps;
  }

  let { toast }: Props = $props();

  // ---------------------------------------------------------------------------
  // Auto-dismiss timer
  // ---------------------------------------------------------------------------

  const DURATION = toast.duration ?? 5000;

  let remaining = $state(DURATION);
  let startedAt = 0;
  let timerId: ReturnType<typeof setTimeout> | null = null;
  let paused = $state(false);

  function startTimer(): void {
    if (DURATION === 0) return;
    startedAt = Date.now();
    timerId = setTimeout(() => {
      dismissToast(toast.id);
    }, remaining);
  }

  function pauseTimer(): void {
    if (DURATION === 0 || timerId === null) return;
    paused = true;
    clearTimeout(timerId);
    timerId = null;
    remaining = Math.max(0, remaining - (Date.now() - startedAt));
  }

  function resumeTimer(): void {
    if (DURATION === 0 || remaining <= 0) return;
    paused = false;
    startTimer();
  }

  onMount(() => {
    startTimer();
  });

  onDestroy(() => {
    if (timerId !== null) clearTimeout(timerId);
  });

  // ---------------------------------------------------------------------------
  // Derived ARIA role
  // ---------------------------------------------------------------------------

  const ariaRole = $derived(
    toast.variant === 'warning' || toast.variant === 'error' ? 'alert' : 'status'
  );

  // ---------------------------------------------------------------------------
  // Slide-in animation state
  // ---------------------------------------------------------------------------

  let visible = $state(false);

  onMount(() => {
    // Defer one tick so the initial off-screen position is painted first,
    // giving the CSS transition something to animate from.
    requestAnimationFrame(() => {
      visible = true;
    });
  });
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  class="toast toast--{toast.variant}"
  class:toast--visible={visible}
  role={ariaRole}
  aria-live={ariaRole === 'alert' ? 'assertive' : 'polite'}
  aria-atomic="true"
  onmouseenter={pauseTimer}
  onmouseleave={resumeTimer}
  onfocusin={pauseTimer}
  onfocusout={resumeTimer}
>
  <!-- Variant accent strip -->
  <div class="toast__accent" aria-hidden="true"></div>

  <!-- Content -->
  <div class="toast__body">
    <p class="toast__title">{toast.title}</p>
    {#if toast.body}
      <p class="toast__description">{toast.body}</p>
    {/if}

    {#if toast.actions && toast.actions.length > 0}
      <div class="toast__actions">
        {#each toast.actions as action (action.label)}
          <button
            type="button"
            class="toast__action-btn toast__action-btn--{action.variant ?? 'ghost'}"
            onclick={() => {
              action.onClick();
              dismissToast(toast.id);
            }}
          >
            {action.label}
          </button>
        {/each}
      </div>
    {/if}
  </div>

  <!-- Close button -->
  <button
    type="button"
    class="toast__close"
    aria-label="Dismiss notification"
    onclick={() => dismissToast(toast.id)}
  >
    <svg width="12" height="12" viewBox="0 0 12 12" fill="none" aria-hidden="true">
      <path d="M1 1l10 10M11 1L1 11" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
    </svg>
  </button>
</div>

<style>
  /* ── Base card ──────────────────────────────────────────────────────────── */

  .toast {
    display: flex;
    align-items: stretch;
    background-color: var(--color-surface-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    box-shadow: var(--shadow-md);
    width: 340px;
    max-width: calc(100vw - 32px);
    overflow: hidden;
    position: relative;
    /* Slide up + fade entry animation */
    animation: toast-in var(--duration-default) var(--ease-spring) both;
    /* Ensure pointer events work even while animating */
    pointer-events: auto;
  }

  @keyframes toast-in {
    from { opacity: 0; transform: translateY(8px) scale(0.97); }
    to   { opacity: 1; transform: translateY(0) scale(1); }
  }

  /* ── Variant accent strip (4px left edge) ────────────────────────────── */

  .toast__accent {
    width: 4px;
    flex-shrink: 0;
    border-radius: var(--radius-md) 0 0 var(--radius-md);
  }

  .toast--info .toast__accent    { background-color: var(--color-info); }
  .toast--success .toast__accent { background-color: var(--color-success); }
  .toast--warning .toast__accent { background-color: var(--color-warning); }
  .toast--error .toast__accent   { background-color: var(--color-error); }

  /* ── Content area ────────────────────────────────────────────────────── */

  .toast__body {
    flex: 1;
    min-width: 0;
    padding: var(--space-4);
    padding-right: var(--space-2);
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .toast__title {
    font-size: var(--font-size-base);
    font-weight: 500;
    color: var(--color-text-primary);
    line-height: 1.4;
    margin: 0;
    /* Leave room for the close button so text doesn't underlap it */
    padding-right: var(--space-4);
  }

  .toast__description {
    font-size: var(--font-size-sm);
    color: var(--color-text-secondary);
    line-height: 1.5;
    margin: 0;
  }

  /* ── Action buttons ──────────────────────────────────────────────────── */

  .toast__actions {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: var(--space-2);
    margin-top: var(--space-2);
    flex-wrap: wrap;
  }

  .toast__action-btn {
    display: inline-flex;
    align-items: center;
    padding: var(--space-1) var(--space-3);
    border-radius: var(--radius-sm);
    font-size: var(--font-size-sm);
    font-weight: 500;
    font-family: var(--font-sans);
    cursor: pointer;
    transition: background-color var(--duration-fast) var(--ease-out), color var(--duration-fast) var(--ease-out), border-color var(--duration-fast) var(--ease-out);
    white-space: nowrap;
    line-height: 1.5;
  }

  .toast__action-btn--primary {
    background-color: var(--color-accent);
    color: var(--color-text-inverse);
    border: 1px solid transparent;
  }

  .toast__action-btn--primary:hover {
    background-color: var(--color-accent-hover);
  }

  .toast__action-btn--primary:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
  }

  .toast__action-btn--ghost {
    background-color: transparent;
    color: var(--color-text-secondary);
    border: 1px solid var(--color-border);
  }

  .toast__action-btn--ghost:hover {
    background-color: var(--color-surface-overlay);
    color: var(--color-text-primary);
    border-color: var(--color-border-strong);
  }

  .toast__action-btn--ghost:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
  }

  /* ── Close button ────────────────────────────────────────────────────── */

  .toast__close {
    position: absolute;
    top: var(--space-2);
    right: var(--space-2);
    display: flex;
    align-items: center;
    justify-content: center;
    width: 20px;
    height: 20px;
    background: none;
    border: none;
    padding: 0;
    color: var(--color-text-muted);
    cursor: pointer;
    border-radius: var(--radius-sm);
    transition: color var(--duration-fast) var(--ease-out), background-color var(--duration-fast) var(--ease-out);
    flex-shrink: 0;
  }

  .toast__close:hover {
    color: var(--color-text-primary);
    background-color: var(--color-surface-overlay);
  }

  .toast__close:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 1px;
  }

  @media (prefers-reduced-motion: reduce) {
    .toast { animation: none; }
  }
</style>
