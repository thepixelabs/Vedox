/**
 * $lib/theme/store.ts — flagship theme + density store
 *
 * Replaces the old binary "dark | light" theme switch with the five-name
 * curated theme system from design-system.md + cmp-dark-mode.md:
 *
 *   graphite (default, dark)
 *   eclipse  (OLED dark with violet accent)
 *   ember    (warm near-black)
 *   paper    (warm off-white light)
 *   solar    (cream + amber light)
 *
 * Also ships a density store with three modes (compact | comfortable | cozy).
 *
 * Both stores:
 *   - are Svelte `writable` stores, so `$themeStore` works in templates
 *   - persist to localStorage under the `vedox:` namespace
 *   - apply their value to `document.documentElement` as `data-theme` /
 *     `data-density` attributes so the CSS cascade in themes.css can react
 *   - are SSR-safe (all browser access is gated on `$app/environment`)
 *
 * Backwards compatibility:
 *   The legacy theme store at $lib/stores/theme.ts is re-exported from here
 *   so existing components keep their import path. The legacy store's
 *   surface (`setTheme("dark" | "light")`, `toggle()`) still works and is
 *   widened to accept the new theme names.
 */

import { writable, derived, get } from "svelte/store";
import { browser } from "$app/environment";

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

/** The five curated theme names. */
export type Theme = "graphite" | "eclipse" | "ember" | "paper" | "solar";

/** The coarse dark/light family a theme belongs to — useful for branching
 *  integrations (Mermaid, legacy components) that only know the binary. */
export type ThemeMode = "dark" | "light";

/** The three information density modes. */
export type Density = "compact" | "comfortable" | "cozy";

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const THEME_KEY = "vedox:theme";
const DENSITY_KEY = "vedox:density";

const DEFAULT_THEME: Theme = "graphite";
const DEFAULT_DENSITY: Density = "comfortable";

const ALL_THEMES: readonly Theme[] = [
  "graphite",
  "eclipse",
  "ember",
  "paper",
  "solar",
] as const;

const ALL_DENSITIES: readonly Density[] = [
  "compact",
  "comfortable",
  "cozy",
] as const;

/** Which family each curated theme belongs to. */
const THEME_MODE: Record<Theme, ThemeMode> = {
  graphite: "dark",
  eclipse: "dark",
  ember: "dark",
  paper: "light",
  solar: "light",
};

/**
 * Pairing used when the user toggles light/dark from a single switch.
 * From a dark theme the toggle goes to its light partner and vice
 * versa — e.g. Graphite ↔ Paper, Solar ↔ Eclipse.
 */
const TOGGLE_PARTNER: Record<Theme, Theme> = {
  graphite: "paper",
  paper: "graphite",
  eclipse: "solar",
  solar: "eclipse",
  ember: "paper",
};

// ---------------------------------------------------------------------------
// Parsing helpers (tolerant of legacy values)
// ---------------------------------------------------------------------------

/**
 * Normalise whatever we read out of localStorage into a valid Theme.
 * Accepts the legacy "dark" | "light" strings so users on the old
 * binary store get a graceful migration to Graphite or Paper.
 */
export function parseTheme(raw: string | null | undefined): Theme {
  if (!raw) return DEFAULT_THEME;
  if ((ALL_THEMES as readonly string[]).includes(raw)) return raw as Theme;
  if (raw === "dark") return "graphite";
  if (raw === "light") return "paper";
  return DEFAULT_THEME;
}

export function parseDensity(raw: string | null | undefined): Density {
  if (!raw) return DEFAULT_DENSITY;
  if ((ALL_DENSITIES as readonly string[]).includes(raw)) return raw as Density;
  return DEFAULT_DENSITY;
}

// ---------------------------------------------------------------------------
// DOM + storage helpers
// ---------------------------------------------------------------------------

function readStoredTheme(): Theme {
  if (!browser) return DEFAULT_THEME;
  try {
    return parseTheme(localStorage.getItem(THEME_KEY));
  } catch {
    return DEFAULT_THEME;
  }
}

function readStoredDensity(): Density {
  if (!browser) return DEFAULT_DENSITY;
  try {
    return parseDensity(localStorage.getItem(DENSITY_KEY));
  } catch {
    return DEFAULT_DENSITY;
  }
}

function applyThemeToDOM(theme: Theme): void {
  if (!browser) return;
  const html = document.documentElement;

  // Brief transition flag — CSS uses .theme-transition to crossfade.
  html.classList.add("theme-transition");
  html.setAttribute("data-theme", theme);
  // Also project the mode for any component that only branches on dark/light.
  html.setAttribute("data-theme-mode", THEME_MODE[theme]);

  // Remove the transition flag one frame later so subsequent UI
  // interactions do not inherit it.
  requestAnimationFrame(() => {
    requestAnimationFrame(() => {
      html.classList.remove("theme-transition");
    });
  });
}

function applyDensityToDOM(density: Density): void {
  if (!browser) return;
  document.documentElement.setAttribute("data-density", density);
}

function persistTheme(theme: Theme): void {
  if (!browser) return;
  try {
    localStorage.setItem(THEME_KEY, theme);
  } catch {
    /* ignore quota / private-mode errors */
  }
}

function persistDensity(density: Density): void {
  if (!browser) return;
  try {
    localStorage.setItem(DENSITY_KEY, density);
  } catch {
    /* ignore quota / private-mode errors */
  }
}

// ---------------------------------------------------------------------------
// Theme store
// ---------------------------------------------------------------------------

function createThemeStore() {
  const initial = readStoredTheme();
  const { subscribe, set, update } = writable<Theme>(initial);

  // Apply immediately so the first render is correct even if the
  // pre-hydration inline script in app.html did not run.
  applyThemeToDOM(initial);

  /** Coerce legacy dark/light strings into the new Theme union. */
  function normalize(value: Theme | ThemeMode | string): Theme {
    if (value === "dark") return "graphite";
    if (value === "light") return "paper";
    if ((ALL_THEMES as readonly string[]).includes(value as string)) {
      return value as Theme;
    }
    return DEFAULT_THEME;
  }

  return {
    subscribe,

    /** Available theme names in display order. */
    all(): readonly Theme[] {
      return ALL_THEMES;
    },

    /**
     * Explicit setter. Accepts a Theme name OR the legacy "dark"/"light"
     * strings (translated to graphite/paper). This keeps the old
     * `themeStore.setTheme("dark")` call-sites working.
     */
    setTheme(next: Theme | ThemeMode | string): void {
      const theme = normalize(next);
      applyThemeToDOM(theme);
      persistTheme(theme);
      set(theme);
    },

    /**
     * Cycle between a theme and its dark/light partner. This is what
     * the sidebar dock toggle button calls. Long-press / settings open
     * the full picker (out of scope for this store).
     */
    toggle(): void {
      update((current) => {
        const next = TOGGLE_PARTNER[current];
        applyThemeToDOM(next);
        persistTheme(next);
        return next;
      });
    },

    /**
     * Re-sync the store with localStorage. Called on mount after
     * hydration so any pre-hydration script state is picked up.
     */
    sync(): void {
      const stored = readStoredTheme();
      applyThemeToDOM(stored);
      set(stored);
    },
  };
}

// ---------------------------------------------------------------------------
// Density store
// ---------------------------------------------------------------------------

function createDensityStore() {
  const initial = readStoredDensity();
  const { subscribe, set } = writable<Density>(initial);
  applyDensityToDOM(initial);

  return {
    subscribe,

    all(): readonly Density[] {
      return ALL_DENSITIES;
    },

    setDensity(next: Density): void {
      applyDensityToDOM(next);
      persistDensity(next);
      set(next);
    },

    sync(): void {
      const stored = readStoredDensity();
      applyDensityToDOM(stored);
      set(stored);
    },
  };
}

// ---------------------------------------------------------------------------
// Singleton instances + derived helpers
// ---------------------------------------------------------------------------

export const themeStore = createThemeStore();
export const densityStore = createDensityStore();

/**
 * Derived store that exposes the dark/light family of the active theme.
 * Use this for components that only need coarse branching (e.g. Mermaid,
 * legacy ChromeBar logic), while `themeStore` remains the fine-grained
 * source of truth.
 */
export const themeMode = derived<typeof themeStore, ThemeMode>(
  themeStore,
  (theme) => THEME_MODE[theme],
);

/** Convenience getter for non-reactive lookups (e.g. inside effects). */
export function getCurrentTheme(): Theme {
  return get(themeStore);
}

export function getCurrentDensity(): Density {
  return get(densityStore);
}

export function getCurrentMode(): ThemeMode {
  return THEME_MODE[getCurrentTheme()];
}
