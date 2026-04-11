/**
 * sidebar.ts — SidebarStore
 *
 * Tracks sidebar layout state: collapsed, width, position, and overview mode.
 * All values persist to localStorage so preferences survive navigation and
 * page reloads. The --sidebar-width CSS custom property is synced to
 * document.documentElement whenever width changes, so layout.css container
 * queries can reference it.
 */

import { writable } from "svelte/store";
import { browser } from "$app/environment";

// ---------------------------------------------------------------------------
// Storage keys
// ---------------------------------------------------------------------------

const KEY_COLLAPSED = "vedox:sidebar-collapsed";
const KEY_WIDTH = "vedox:sidebar-width";
const KEY_POSITION = "vedox:sidebar-position";
const KEY_OVERVIEW = "vedox:sidebar-overview";

// ---------------------------------------------------------------------------
// Defaults & constraints
// ---------------------------------------------------------------------------

const DEFAULT_WIDTH = 240;
const MIN_WIDTH = 200;
const MAX_WIDTH = 480;
const RESIZE_STEP = 20;

// ---------------------------------------------------------------------------
// State shape
// ---------------------------------------------------------------------------

export interface SidebarState {
  collapsed: boolean;
  width: number; // px, persisted, default 240
  position: "left" | "right"; // persisted, default 'left'
  overview: boolean; // overview mode at >= 2560px, persisted, default false
}

// ---------------------------------------------------------------------------
// localStorage helpers
// ---------------------------------------------------------------------------

function getStoredBool(key: string, fallback: boolean): boolean {
  if (!browser) return fallback;
  const v = localStorage.getItem(key);
  if (v === null) return fallback;
  return v === "true";
}

function getStoredNumber(key: string, fallback: number): number {
  if (!browser) return fallback;
  const v = localStorage.getItem(key);
  if (v === null) return fallback;
  const n = parseInt(v, 10);
  return isNaN(n) ? fallback : n;
}

function getStoredPosition(): "left" | "right" {
  if (!browser) return "left";
  const v = localStorage.getItem(KEY_POSITION);
  return v === "right" ? "right" : "left";
}

function persist(key: string, value: string): void {
  if (!browser) return;
  try {
    localStorage.setItem(key, value);
  } catch {
    // localStorage unavailable — non-fatal.
  }
}

// ---------------------------------------------------------------------------
// CSS custom property sync
// ---------------------------------------------------------------------------

function syncCssWidth(px: number): void {
  if (!browser) return;
  document.documentElement.style.setProperty("--sidebar-width", `${px}px`);
}

// ---------------------------------------------------------------------------
// Store factory
// ---------------------------------------------------------------------------

function createSidebarStore() {
  const initial: SidebarState = {
    collapsed: getStoredBool(KEY_COLLAPSED, false),
    width: getStoredNumber(KEY_WIDTH, DEFAULT_WIDTH),
    position: getStoredPosition(),
    overview: getStoredBool(KEY_OVERVIEW, false),
  };

  // Sync CSS on init.
  syncCssWidth(initial.collapsed ? 0 : initial.width);

  const { subscribe, set, update } = writable<SidebarState>(initial);

  return {
    subscribe,

    /** Toggle collapsed state. */
    toggle(): void {
      update((s) => {
        const next = !s.collapsed;
        persist(KEY_COLLAPSED, String(next));
        syncCssWidth(next ? 0 : s.width);
        return { ...s, collapsed: next };
      });
    },

    /** Set collapsed explicitly. */
    setCollapsed(value: boolean): void {
      update((s) => {
        persist(KEY_COLLAPSED, String(value));
        syncCssWidth(value ? 0 : s.width);
        return { ...s, collapsed: value };
      });
    },

    /** Set width in px. Clamped to [200, 480]; below 200 auto-collapses. */
    setWidth(px: number): void {
      update((s) => {
        if (px < MIN_WIDTH) {
          persist(KEY_COLLAPSED, "true");
          syncCssWidth(0);
          return { ...s, collapsed: true };
        }
        const clamped = Math.min(Math.max(px, MIN_WIDTH), MAX_WIDTH);
        persist(KEY_WIDTH, String(clamped));
        persist(KEY_COLLAPSED, "false");
        syncCssWidth(clamped);
        return { ...s, width: clamped, collapsed: false };
      });
    },

    /** Increment width by delta px (use +/- 20 for keyboard resize). */
    incrementWidth(delta: number): void {
      update((s) => {
        const target = s.width + delta;
        if (target < MIN_WIDTH) {
          persist(KEY_COLLAPSED, "true");
          syncCssWidth(0);
          return { ...s, collapsed: true };
        }
        const clamped = Math.min(Math.max(target, MIN_WIDTH), MAX_WIDTH);
        persist(KEY_WIDTH, String(clamped));
        persist(KEY_COLLAPSED, "false");
        syncCssWidth(clamped);
        return { ...s, width: clamped, collapsed: false };
      });
    },

    /** Set sidebar position (left or right). */
    setPosition(p: "left" | "right"): void {
      update((s) => {
        persist(KEY_POSITION, p);
        return { ...s, position: p };
      });
    },

    /** Toggle overview mode (for ultra-wide viewports >= 2560px). */
    toggleOverview(): void {
      update((s) => {
        const next = !s.overview;
        persist(KEY_OVERVIEW, String(next));
        return { ...s, overview: next };
      });
    },
  };
}

export const sidebarStore = createSidebarStore();
