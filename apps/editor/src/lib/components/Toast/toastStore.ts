/**
 * toastStore.ts — Svelte store for managing toast notification state.
 *
 * Keeps a list of active ToastProps. Components call showToast() to
 * enqueue a notification and dismissToast() to remove one by id.
 * The ToastContainer subscribes to this store and renders the list.
 */

import { writable } from 'svelte/store';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface ToastAction {
  label: string;
  onClick: () => void;
  variant?: 'primary' | 'ghost';
}

export interface ToastProps {
  id: string;
  variant: 'info' | 'success' | 'warning' | 'error';
  title: string;
  body?: string;
  actions?: ToastAction[];
  /**
   * Milliseconds before auto-dismiss. 0 = sticky (never auto-dismisses).
   * Defaults to 5000 when not provided.
   */
  duration?: number;
  onDismiss?: () => void;
}

// ---------------------------------------------------------------------------
// Internal counter for deterministic ids
// ---------------------------------------------------------------------------

let _counter = 0;

function nextId(): string {
  _counter += 1;
  return `toast-${Date.now()}-${_counter}`;
}

// ---------------------------------------------------------------------------
// Store
// ---------------------------------------------------------------------------

export const toasts = writable<ToastProps[]>([]);

/**
 * Add a new toast. Returns the generated id so callers can dismiss
 * programmatically (e.g. replace a pending toast with a success one).
 */
export function showToast(props: Omit<ToastProps, 'id'>): string {
  const id = nextId();
  toasts.update((list) => {
    // Keep max 5 visible — drop the oldest when over limit.
    const next = [...list, { ...props, id }];
    return next.length > 5 ? next.slice(next.length - 5) : next;
  });
  return id;
}

/**
 * Remove a toast by id. Triggers onDismiss callback if present.
 */
export function dismissToast(id: string): void {
  toasts.update((list) => {
    const target = list.find((t) => t.id === id);
    if (target?.onDismiss) {
      try { target.onDismiss(); } catch { /* swallow */ }
    }
    return list.filter((t) => t.id !== id);
  });
}
