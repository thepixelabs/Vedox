/**
 * shortcuts.ts — centralized keyboard shortcut dispatcher
 *
 * Provides a register/unregister API so shortcuts can be added from any
 * component (layout, editor, dialogs) without global coupling.
 *
 * Usage:
 *   1. Bind `dispatchShortcut` to `<svelte:window onkeydown={...} />`
 *      in the root layout (done once).
 *   2. Call `registerShortcut(...)` in `onMount` and save the returned
 *      unregister function for cleanup.
 *
 * Only the first matching shortcut fires (no bubbling within this system).
 * Matching is exact: if a shortcut requires meta, the event must have meta.
 */

type ShortcutHandler = (event: KeyboardEvent) => void;

export interface Shortcut {
  /** The `event.key` value to match (case-insensitive). */
  key: string;
  /** Require Cmd (Mac) or Ctrl (Windows/Linux). */
  meta?: boolean;
  /** Require Shift. */
  shift?: boolean;
  /** Require Alt/Option. */
  alt?: boolean;
  /** Human-readable description for help menus / command palette. */
  description: string;
  handler: ShortcutHandler;
}

const shortcuts: Shortcut[] = [];

/**
 * Register a keyboard shortcut. Returns an unregister function.
 */
export function registerShortcut(s: Shortcut): () => void {
  shortcuts.push(s);
  return () => {
    const idx = shortcuts.indexOf(s);
    if (idx !== -1) shortcuts.splice(idx, 1);
  };
}

/**
 * Dispatch a keydown event against all registered shortcuts.
 * Call this from `<svelte:window onkeydown={dispatchShortcut} />`.
 */
export function dispatchShortcut(event: KeyboardEvent): void {
  // Normalize meta key: Cmd on Mac, Ctrl elsewhere
  const isMac =
    typeof navigator !== 'undefined' &&
    /Mac|iPhone|iPad/.test(navigator.platform);
  const metaPressed = isMac ? event.metaKey : event.ctrlKey;

  for (const shortcut of shortcuts) {
    const keyMatch =
      event.key.toLowerCase() === shortcut.key.toLowerCase();
    if (!keyMatch) continue;

    // Each modifier must match exactly when specified
    if (shortcut.meta !== undefined && metaPressed !== shortcut.meta)
      continue;
    if (shortcut.shift !== undefined && event.shiftKey !== shortcut.shift)
      continue;
    if (shortcut.alt !== undefined && event.altKey !== shortcut.alt)
      continue;

    event.preventDefault();
    shortcut.handler(event);
    return; // first match wins
  }
}

/**
 * Get a snapshot of all currently registered shortcuts.
 * Useful for rendering a help/shortcut overview.
 */
export function getShortcutList(): ReadonlyArray<Shortcut> {
  return [...shortcuts];
}
