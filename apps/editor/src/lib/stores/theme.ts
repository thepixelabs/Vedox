/**
 * theme.ts — legacy binary theme shim.
 *
 * The flagship theme system now lives at `$lib/theme/store.ts` and ships
 * five curated named themes (graphite / eclipse / ember / paper / solar)
 * plus a density store. New code should import from `$lib/theme/store`.
 *
 * This shim preserves the old `themeStore` API so the existing
 * `ThemeToggle.svelte`, `/settings` scaffold, and `+layout.svelte`
 * call-sites keep working without edits. It presents a binary
 * `"dark" | "light"` surface that is **projected** from the flagship
 * store:
 *
 *   - `$themeStore` value is the dark/light family of the active theme
 *   - `themeStore.setTheme("dark")` selects Graphite
 *   - `themeStore.setTheme("light")` selects Paper
 *   - `themeStore.toggle()` cycles between the two families via the
 *     flagship store's dark↔light partner map
 *   - `themeStore.sync()` re-reads localStorage via the flagship store
 *
 * The legacy `Theme` type is also re-exported as `"dark" | "light"` so
 * TypeScript call-sites keep their previous shape. Full Theme names
 * are available as `FlagshipTheme` for anyone who wants them without
 * importing from the new path.
 */

import { derived, get } from "svelte/store";
import {
  themeStore as flagshipThemeStore,
  themeMode as flagshipThemeMode,
  type Theme as FlagshipThemeName,
  type ThemeMode,
} from "$lib/theme/store";

// Re-export the flagship type under an explicit name for new consumers
// that still want to go through this path.
export type FlagshipTheme = FlagshipThemeName;

/** Legacy binary type — dark or light. */
export type Theme = "dark" | "light";

// ---------------------------------------------------------------------------
// Legacy themeStore — projects flagship state into binary dark/light
// ---------------------------------------------------------------------------

/**
 * Derived store exposing only the dark/light family of the active
 * flagship theme. `$themeStore` in a Svelte template now resolves to
 * `"dark"` or `"light"`, which is the shape every existing consumer
 * expects.
 */
const legacySubscribable = derived<typeof flagshipThemeMode, Theme>(
  flagshipThemeMode,
  (mode) => mode,
);

export const themeStore = {
  subscribe: legacySubscribable.subscribe,

  /**
   * Accepts the legacy "dark" / "light" values and maps them to the
   * corresponding curated flagship theme. Also tolerates a full flagship
   * theme name for forward-compat, which makes this shim safe to use
   * even after callers upgrade incrementally.
   */
  setTheme(theme: Theme | FlagshipThemeName | string): void {
    flagshipThemeStore.setTheme(theme);
  },

  /**
   * Cycle between the active theme and its dark/light partner
   * (Graphite ↔ Paper, Eclipse ↔ Solar, Ember → Paper).
   */
  toggle(): void {
    flagshipThemeStore.toggle();
  },

  /** Re-sync with localStorage. */
  sync(): void {
    flagshipThemeStore.sync();
  },

  /** Non-reactive getter for the current binary mode. */
  current(): Theme {
    return get(legacySubscribable);
  },
};

// Re-export the flagship surface under fresh names so consumers can
// adopt them without touching this path.
export {
  flagshipThemeStore,
  flagshipThemeMode,
};
export { densityStore, getCurrentTheme, getCurrentDensity, getCurrentMode } from "$lib/theme/store";
export type { ThemeMode, Density } from "$lib/theme/store";
