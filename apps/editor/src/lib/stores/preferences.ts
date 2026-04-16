/**
 * preferences.ts — userPrefs store
 *
 * Single source of truth for all personalization settings across the 7
 * settings categories. Persists to localStorage at `vedox-user-prefs`
 * (JSON blob). Typed with Zod v3. Default values match the binding
 * rulings in master-vision.md (Graphite theme, JetBrains Mono, etc.).
 *
 * Usage:
 *   import { userPrefs, updatePrefs } from '$lib/stores/preferences';
 *   $userPrefs.appearance.theme  // reactive
 *   updatePrefs('editor', { autoSaveInterval: 5000 });
 *
 * CTO ruling R3: PUT /api/settings uses PATCH semantics.
 * Frontend mocks this with localStorage until the daemon implements the
 * endpoint. When the endpoint is ready, replace the `persistPrefs` helper
 * with a fetch call.
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
// Storage
// ---------------------------------------------------------------------------

const STORAGE_KEY = 'vedox-user-prefs';

function loadPrefs(): UserPrefs {
  if (!browser) return DEFAULT_PREFS;
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return DEFAULT_PREFS;
    const parsed = JSON.parse(raw);
    // Use safeParse so a partial/stale blob merges cleanly with new defaults.
    const result = UserPrefsSchema.safeParse({
      appearance: { ...DEFAULT_PREFS.appearance, ...(parsed.appearance ?? {}) },
      editor: { ...DEFAULT_PREFS.editor, ...(parsed.editor ?? {}) },
      sidebar: { ...DEFAULT_PREFS.sidebar, ...(parsed.sidebar ?? {}) },
      keyboard: { ...DEFAULT_PREFS.keyboard, ...(parsed.keyboard ?? {}) },
      voice: { ...DEFAULT_PREFS.voice, ...(parsed.voice ?? {}) },
      agent: { ...DEFAULT_PREFS.agent, ...(parsed.agent ?? {}) },
      notifications: { ...DEFAULT_PREFS.notifications, ...(parsed.notifications ?? {}) },
    });
    return result.success ? result.data : DEFAULT_PREFS;
  } catch {
    return DEFAULT_PREFS;
  }
}

function persistPrefs(prefs: UserPrefs): void {
  if (!browser) return;
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(prefs));
  } catch {
    // quota / private-mode — non-fatal
  }
  // TODO(R3): When daemon implements GET/PUT /api/settings, send a PATCH here:
  // fetch('/api/settings', { method: 'PUT', body: JSON.stringify(prefs) })
}

// ---------------------------------------------------------------------------
// Store
// ---------------------------------------------------------------------------

function createUserPrefsStore() {
  const { subscribe, set, update } = writable<UserPrefs>(loadPrefs());

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
