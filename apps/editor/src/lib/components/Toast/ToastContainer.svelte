<script lang="ts">
  /**
   * ToastContainer.svelte — fixed viewport anchor for the toast stack.
   *
   * Positioned at bottom-right. Subscribes to the toasts store and renders
   * up to 5 toasts stacked with 8px gap. Older (lower-index) toasts appear
   * higher in the stack; newest is at the bottom, closest to the anchor point.
   *
   * Mount exactly once in +layout.svelte so toasts are app-wide.
   */

  import { toasts } from './toastStore';
  import Toast from './Toast.svelte';
</script>

<div
  class="toast-container"
  aria-label="Notifications"
  aria-live="off"
>
  {#each $toasts as toast (toast.id)}
    <Toast {toast} />
  {/each}
</div>

<style>
  .toast-container {
    position: fixed;
    bottom: var(--space-4);
    right: var(--space-4);
    z-index: 1000;
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    align-items: flex-end;
    /* Container itself should not intercept pointer events in empty space */
    pointer-events: none;
  }
</style>
