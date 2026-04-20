/**
 * preferences.ts — userPrefs store
 *
 * Single source of truth for all personalization settings across the 7
 * settings categories. Persists via the daemon GET/PUT /api/settings endpoints
 * (R3 PATCH semantics). Falls back to localStorage when the daemon is offline
 * so the UI remains functional in dev-server mode.
 *
 * Usage:
 *   import { userPrefs, updatePrefs } from '$lib/stores/preferences';
 *   $userPrefs.appearance.theme  // reactive
 *   updatePrefs('editor', { autoSaveInterval: 5000 });
 *
 * Persistence strategy:
 *   1. On load: try GET /api/settings. Merge result with DEFAULT_PREFS and
 *      store in localStorage as a fast-path cache for next load.
 *   2. On write (patch/setAll/reset): try PUT /api/settings (PATCH semantics).
 *      Always mirror to localStorage so a daemon restart doesn't lose recent
 *      edits before the next successful PUT.
 *   3. If GET fails (daemon offline): fall back to localStorage.
 *   4. If PUT fails: localStorage write succeeds; error is logged (non-fatal).
 *      The next successful daemon start will receive the cached prefs on the
 *      next PUT call.
 */

import { writable, get } from 'svelte/store';
import { browser } from '$app/environment';
import { z } from 'zod';

// ---------------------------------------------------------------------------
// Schema
// ---------------------------------------------------------------------------

export const AppearanceSchema = z.object({
  theme: z.enum(['graphite', 'eclipse', 'ember', 'paper', 'solar']).default('graphite'),
  accentColor: z.string().default(''),          // '' = use theme default
  fontSize: z.enum(['13px', '16px', '18px']).default('16px'),
  lineHeight: z.enum(['tight', 'normal', 'relaxed']).default('normal'),
  measure: z.enum(['narrow', 'default', 'wide']).default('default'),
  density: z.enum(['compact', 'comfortable', 'cozy']).default('comfortable'),
  treeGrouping: z.enum(['type-first', 'folder-first', 'flat']).default('type-first'),
  wordmarkFont: z.enum(['display', 'mono']).default('display'),
});

export const EditorSchema = z.object({
  defaultView: z.enum(['split', 'preview', 'source']).default('split'),
  autoSaveInterval: z.number().int().min(0).max(60000).default(3000),  // ms; 0 = off
  spellCheck: z.boolean().default(false),
});

export const SidebarSchema = z.object({
  defaultPanel: z.enum(['tree', 'filter', 'overview']).default('tree'),
  collapseOnOpen: z.boolean().default(false),
  docTreeGrouping: z.enum(['type-first', 'folder-first', 'flat']).default('type-first'),
});

export const KeyboardSchema = z.object({
  /** Map of action id → key string, overrides only (unset = default). */
  overrides: z.record(z.string()).default({}),
});

export const VoiceSchema = z.object({
  triggerPhrase: z.string().default('vedox document everything'),
  micEnabled: z.boolean().default(false),
  pushToTalkKey: z.string().default(''),
});

export const AgentSchema = z.object({
  defaultDocRepo: z.string().default(''),       // repo id from registry
  dryRun: z.boolean().default(false),
  autoApprove: z.boolean().default(false),
});

export const NotificationsSchema = z.object({
  toastDuration: z.number().int().min(500).max(30000).default(4000),  // ms
  soundEnabled: z.boolean().default(false),
  badgeVisible: z.boolean().default(true),
});

export const UserPrefsSchema = z.object({
  appearance: AppearanceSchema,
  editor: EditorSchema,
  sidebar: SidebarSchema,
  keyboard: KeyboardSchema,
  voice: VoiceSchema,
  agent: AgentSchema,
  notifications: NotificationsSchema,
});

export type UserPrefs = z.infer<typeof UserPrefsSchema>;
export type AppearancePrefs = z.infer<typeof AppearanceSchema>;
export type EditorPrefs = z.infer<typeof EditorSchema>;
export type SidebarPrefs = z.infer<typeof SidebarSchema>;
export type KeyboardPrefs = z.infer<typeof KeyboardSchema>;
export type VoicePrefs = z.infer<typeof VoiceSchema>;
export type AgentPrefs = z.infer<typeof AgentSchema>;
export type NotificationsPrefs = z.infer<typeof NotificationsSchema>;

// ---------------------------------------------------------------------------
// Defaults
// ---------------------------------------------------------------------------

export const DEFAULT_PREFS: UserPrefs = UserPrefsSchema.parse({
  appearance: {},
  editor: {},
  sidebar: {},
  keyboard: {},
  voice: {},
  agent: {},
  notifications: {},
});

// ---------------------------------------------------------------------------
// localStorage (offline cache / fallback)
// ---------------------------------------------------------------------------

const STORAGE_KEY = 'vedox-user-prefs';

/**
 * Merge a raw (possibly partial/stale) object with DEFAULT_PREFS and validate.
 * Returns DEFAULT_PREFS on parse failure so a single corrupted key never
 * breaks the entire settings panel.
 */
function mergeWithDefaults(raw: Record<string, unknown>): UserPrefs {
  const result = UserPrefsSchema.safeParse({
    appearance: { ...DEFAULT_PREFS.appearance, ...(raw.appearance ?? {}) },
    editor: { ...DEFAULT_PREFS.editor, ...(raw.editor ?? {}) },
    sidebar: { ...DEFAULT_PREFS.sidebar, ...(raw.sidebar ?? {}) },
    keyboard: { ...DEFAULT_PREFS.keyboard, ...(raw.keyboard ?? {}) },
    voice: { ...DEFAULT_PREFS.voice, ...(raw.voice ?? {}) },
    agent: { ...DEFAULT_PREFS.agent, ...(raw.agent ?? {}) },
    notifications: { ...DEFAULT_PREFS.notifications, ...(raw.notifications ?? {}) },
  });
  return result.success ? result.data : DEFAULT_PREFS;
}

function loadFromLocalStorage(): UserPrefs {
  if (!browser) return DEFAULT_PREFS;
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return DEFAULT_PREFS;
    return mergeWithDefaults(JSON.parse(raw) as Record<string, unknown>);
  } catch {
    return DEFAULT_PREFS;
  }
}

function saveToLocalStorage(prefs: UserPrefs): void {
  if (!browser) return;
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(prefs));
  } catch {
    // quota / private-mode — non-fatal; daemon copy is the source of truth
  }
}

// ---------------------------------------------------------------------------
// Daemon API helpers
// ---------------------------------------------------------------------------

const SETTINGS_URL = '/api/settings';

/**
 * Fetch prefs from the daemon. Returns null when the daemon is unreachable
 * (network error, timeout, non-200 status) so callers can fall back to
 * localStorage without treating offline as an error.
 */
async function fetchPrefsFromDaemon(): Promise<UserPrefs | null> {
  try {
    const resp = await fetch(SETTINGS_URL, {
      method: 'GET',
      headers: { Accept: 'application/json' },
      // Short timeout: if the daemon isn't there we don't want to stall
      // the initial render. AbortSignal.timeout is available in all
      // evergreen browsers and Node ≥ 17.
      signal: AbortSignal.timeout(2000),
    });
    if (!resp.ok) return null;
    const raw: Record<string, unknown> = await resp.json();
    // The daemon may return {} on first run — that's fine; mergeWithDefaults
    // fills in all the defaults.
    return mergeWithDefaults(raw);
  } catch {
    // Daemon offline or network error — silent fallback.
    return null;
  }
}

/**
 * Push prefs to the daemon using PATCH semantics (R3). Only the supplied
 * category keys are overwritten on the server side; all other stored keys
 * are preserved. Returns true on success.
 *
 * Failure is non-fatal: the prefs are already mirrored to localStorage
 * before this is called, so the user's changes are not lost.
 */
async function pushPrefsToDaemon(prefs: UserPrefs): Promise<boolean> {
  try {
    const resp = await fetch(SETTINGS_URL, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(prefs),
      signal: AbortSignal.timeout(3000),
    });
    return resp.ok;
  } catch {
    return false;
  }
}

// ---------------------------------------------------------------------------
// Store
// ---------------------------------------------------------------------------

function createUserPrefsStore() {
  // Initialise synchronously from localStorage so the UI renders immediately
  // without a flash of default values, then hydrate from the daemon in the
  // background.
  const { subscribe, set, update } = writable<UserPrefs>(loadFromLocalStorage());

  // Hydrate from daemon asynchronously. We do this once at module load time
  // (when running in the browser) rather than in a component onMount so that
  // any component importing this store gets the daemon-sourced values even if
  // it mounts before the first tick where the async result arrives.
  if (browser) {
    fetchPrefsFromDaemon().then((daemonPrefs) => {
      if (daemonPrefs) {
        // Daemon is the authoritative source — update the store and refresh
        // the localStorage cache so the next synchronous load is warm.
        saveToLocalStorage(daemonPrefs);
        set(daemonPrefs);
      }
      // If daemonPrefs is null the store already holds the localStorage value,
      // which is correct for offline/dev-server mode.
    });
  }

  /**
   * Persist to both localStorage (synchronous, never fails visibly) and the
   * daemon (async, non-fatal on failure).
   */
  function persistPrefs(prefs: UserPrefs): void {
    // localStorage first — fast and synchronous. Even if the daemon call
    // fails, the user's changes are preserved for the next page load.
    saveToLocalStorage(prefs);
    // Daemon call is fire-and-forget from the store's perspective.
    // Callers that need to handle the result (e.g. to show an error toast)
    // can call pushPrefsToDaemon directly.
    if (browser) {
      pushPrefsToDaemon(prefs).catch(() => {
        // Swallow — already logged inside pushPrefsToDaemon.
      });
    }
  }

  return {
    subscribe,

    /**
     * Partially update one category. Merges into existing prefs and persists.
     *
     * @example
     *   userPrefs.patch('editor', { spellCheck: true });
     */
    patch<K extends keyof UserPrefs>(category: K, partial: Partial<UserPrefs[K]>): void {
      update((current) => {
        const next: UserPrefs = {
          ...current,
          [category]: { ...current[category], ...partial },
        };
        persistPrefs(next);
        return next;
      });
    },

    /** Replace all prefs at once (e.g. import/reset). */
    setAll(prefs: UserPrefs): void {
      persistPrefs(prefs);
      set(prefs);
    },

    /** Reset to factory defaults and persist. */
    reset(): void {
      persistPrefs(DEFAULT_PREFS);
      set(DEFAULT_PREFS);
    },

    /**
     * Explicitly re-fetch from the daemon and sync the store. Useful after
     * the daemon restarts or after an external write to user-prefs.json.
     * Returns true if the daemon responded successfully.
     */
    async syncFromDaemon(): Promise<boolean> {
      const daemonPrefs = await fetchPrefsFromDaemon();
      if (daemonPrefs) {
        saveToLocalStorage(daemonPrefs);
        set(daemonPrefs);
        return true;
      }
      return false;
    },
  };
}

export const userPrefs = createUserPrefsStore();

/**
 * Convenience wrapper so call-sites can write:
 *   updatePrefs('editor', { spellCheck: true })
 * instead of importing both `userPrefs` and calling `.patch()`.
 */
export function updatePrefs<K extends keyof UserPrefs>(
  category: K,
  partial: Partial<UserPrefs[K]>,
): void {
  userPrefs.patch(category, partial);
}
