/**
 * store.ts — state for the global command palette.
 *
 * Three orthogonal stores plus a keyboard-shortcut bootstrapper:
 *
 *   openStore          — boolean, drives the modal visibility
 *   queryStore         — the current input value
 *   modeStore          — derived mode from the query prefix
 *   resultsStore       — debounced fetch from /api/search?q=...
 *   selectedIndexStore — integer, which result row is highlighted
 *   isLoadingStore     — boolean, flips during the debounced fetch
 *
 *   togglePalette()    — imperative open/close
 *   openPalette()
 *   closePalette()
 *   initPaletteShortcut() — wires Cmd+K / Ctrl+K globally
 *
 * The palette supports four search modes, selected by prefix:
 *
 *   (no prefix) → 'search'   — full-text doc search via /api/search
 *   '>'         → 'command'  — run a palette command
 *   '#'         → 'tag'      — filter by tag (Tier-1 placeholder)
 *   '/'         → 'path'     — path-based jump
 *
 * Commands are placeholders for this first pass — the architect will resolve
 * the real command registry in a later phase. We ship three no-op commands
 * (Toggle theme, Toggle dark mode, Reload index) so the '>' mode has
 * something to show.
 */

import { derived, get, writable, type Readable, type Writable } from 'svelte/store';
import { api, ApiError, type SearchResult } from '$lib/api/client';
import { themeStore } from '$lib/theme/store';
import { densityStore } from '$lib/theme/store';
import { sidebarStore } from '$lib/stores/sidebar';
import { panesStore } from '$lib/stores/panes';
import { goto } from '$app/navigation';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type PaletteMode = 'search' | 'command' | 'tag' | 'path';

export interface PaletteSearchHit {
  kind: 'search';
  id: string;
  project: string;
  title: string;
  type: string;
  status: string;
  snippet: string;
  score: number;
  /** Route to navigate to when the hit is activated. */
  href: string;
}

export interface PaletteCommandHit {
  kind: 'command';
  id: string;
  title: string;
  description: string;
  /** Stable icon id consumed by the view for a light visual affordance. */
  icon: 'sun' | 'moon' | 'refresh' | 'generic' | 'sidebar' | 'settings' | 'split' | 'palette';
  run: () => void | Promise<void>;
}

export type PaletteResult = PaletteSearchHit | PaletteCommandHit;

// ---------------------------------------------------------------------------
// Stores
// ---------------------------------------------------------------------------

export const openStore: Writable<boolean> = writable(false);
export const queryStore: Writable<string> = writable('');
export const resultsStore: Writable<PaletteResult[]> = writable([]);
export const selectedIndexStore: Writable<number> = writable(0);
export const isLoadingStore: Writable<boolean> = writable(false);
export const errorStore: Writable<string | null> = writable(null);

/**
 * Derived mode based on the current query's first character. Updates live as
 * the user types, which makes the placeholder & hint bar feel responsive.
 */
export const modeStore: Readable<PaletteMode> = derived(queryStore, ($q) => {
  const head = $q.trimStart().charAt(0);
  switch (head) {
    case '>':
      return 'command';
    case '#':
      return 'tag';
    case '/':
      return 'path';
    default:
      return 'search';
  }
});

// ---------------------------------------------------------------------------
// Imperative helpers
// ---------------------------------------------------------------------------

export function openPalette(): void {
  openStore.set(true);
}

export function closePalette(): void {
  openStore.set(false);
}

export function togglePalette(): void {
  openStore.update((v) => !v);
}

export function setQuery(q: string): void {
  queryStore.set(q);
  // Reset the cursor whenever the query changes so arrow-up doesn't
  // point past the end of a freshly shortened list.
  selectedIndexStore.set(0);
}

export function moveSelection(delta: number): void {
  const results = get(resultsStore);
  if (results.length === 0) {
    selectedIndexStore.set(0);
    return;
  }
  selectedIndexStore.update((i) => {
    const next = i + delta;
    if (next < 0) return results.length - 1; // wrap
    if (next >= results.length) return 0; // wrap
    return next;
  });
}

/**
 * Activate the currently-selected result: navigate to a doc, run a command,
 * etc. Closes the palette on success.
 */
export async function activateSelection(
  navigate: (href: string) => void | Promise<void>,
): Promise<void> {
  const results = get(resultsStore);
  const i = get(selectedIndexStore);
  const hit = results[i];
  if (!hit) return;

  try {
    if (hit.kind === 'search') {
      await navigate(hit.href);
    } else {
      await hit.run();
    }
    closePalette();
  } catch (err) {
    // eslint-disable-next-line no-console
    console.warn('[vedox] palette activation failed:', err);
  }
}

// ---------------------------------------------------------------------------
// Project scope
// ---------------------------------------------------------------------------

/**
 * The command palette searches inside a project scope. The host layout
 * mounts the palette and is responsible for setting the current scope. If
 * no scope is set the palette stays open but search returns empty.
 *
 * We use a writable so the Sidebar/Router can update it when the active
 * project changes, without the palette store having to read $page.
 */
export const scopeProjectStore: Writable<string | null> = writable(null);

export function setScopeProject(project: string | null): void {
  scopeProjectStore.set(project);
}

// ---------------------------------------------------------------------------
// Debounced search driver
// ---------------------------------------------------------------------------

const DEBOUNCE_MS = 120;
let debounceTimer: ReturnType<typeof setTimeout> | null = null;

/**
 * Subscribe the resultsStore to query + mode + scope changes. This is called
 * exactly once by initPaletteShortcut(). It's kept internal so tests can
 * call it directly on module import.
 */
function wireResultsSubscription(): () => void {
  // Composite subscription via multiple `get()` calls inside a single
  // query observer. We derive a synthetic tuple so any of the three
  // stores being updated triggers a re-run.
  const composite = derived(
    [queryStore, scopeProjectStore],
    ([q, project]) => ({ q, project }),
  );
  return composite.subscribe(({ q, project }) => {
    if (debounceTimer) {
      clearTimeout(debounceTimer);
      debounceTimer = null;
    }
    const mode = get(modeStore);
    const trimmed = q.trim();

    // Empty input: show nothing in search/tag/path modes; show all commands
    // in command mode so the user can browse.
    if (!trimmed || trimmed === '>' || trimmed === '#' || trimmed === '/') {
      if (mode === 'command') {
        resultsStore.set(buildCommandResults(''));
        selectedIndexStore.set(0);
      } else {
        resultsStore.set([]);
        selectedIndexStore.set(0);
      }
      isLoadingStore.set(false);
      errorStore.set(null);
      return;
    }

    // Command mode — local filter, no network.
    if (mode === 'command') {
      resultsStore.set(buildCommandResults(trimmed.slice(1).trim()));
      selectedIndexStore.set(0);
      isLoadingStore.set(false);
      errorStore.set(null);
      return;
    }

    // All other modes need the backend. Debounce so rapid typing doesn't
    // fire a query on every keystroke — 120ms is below the 200ms threshold
    // of "feels instant" and gives the kernel a beat to coalesce keydowns.
    isLoadingStore.set(true);
    debounceTimer = setTimeout(() => {
      void runRemoteSearch(trimmed, mode, project);
    }, DEBOUNCE_MS);
  });
}

async function runRemoteSearch(
  query: string,
  mode: PaletteMode,
  project: string | null,
): Promise<void> {
  if (!project) {
    // Without a project scope we can't hit the existing /api/projects/:p/search
    // endpoint. Show a gentle empty state rather than an error.
    resultsStore.set([]);
    selectedIndexStore.set(0);
    isLoadingStore.set(false);
    errorStore.set('Open a project to search its docs.');
    return;
  }

  // Strip the prefix marker from tag/path queries before passing to the
  // backend. The FTS5 endpoint doesn't understand '#' or '/' as operators;
  // we treat them as typed hints today and will wire real operators in a
  // later phase.
  const effectiveQuery = mode === 'tag' || mode === 'path' ? query.slice(1).trim() : query;
  if (!effectiveQuery) {
    resultsStore.set([]);
    selectedIndexStore.set(0);
    isLoadingStore.set(false);
    errorStore.set(null);
    return;
  }

  try {
    const results = await api.search(project, effectiveQuery);
    resultsStore.set(results.map((r) => toSearchHit(r, project)));
    selectedIndexStore.set(0);
    errorStore.set(null);
  } catch (err) {
    resultsStore.set([]);
    errorStore.set(
      err instanceof ApiError
        ? `Search failed: ${err.message}`
        : err instanceof Error
          ? `Search failed: ${err.message}`
          : 'Search failed.',
    );
  } finally {
    isLoadingStore.set(false);
  }
}

/**
 * Transform a raw SearchResult into a palette hit with a concrete href the
 * view can navigate to. The id returned by the backend is the
 * workspace-relative path (`<project>/<doc-path>`), so we strip the project
 * prefix to build the route.
 */
function toSearchHit(r: SearchResult, project: string): PaletteSearchHit {
  const prefix = project + '/';
  const docPath = r.id.startsWith(prefix) ? r.id.slice(prefix.length) : r.id;
  return {
    kind: 'search',
    id: r.id,
    project: r.project,
    title: r.title || r.id,
    type: r.type,
    status: r.status,
    snippet: r.snippet,
    score: r.score,
    href: `/projects/${encodeURIComponent(r.project)}/docs/${docPath}`,
  };
}

// ---------------------------------------------------------------------------
// Command registry (placeholders — architect wires real impls later)
// ---------------------------------------------------------------------------

/** Static command list. The filter arg is a free-text query within '>' mode. */
function buildCommandResults(filter: string): PaletteCommandHit[] {
  const needle = filter.toLowerCase();
  return BUILTIN_COMMANDS.filter((cmd) => {
    if (!needle) return true;
    return (
      cmd.title.toLowerCase().includes(needle) ||
      cmd.description.toLowerCase().includes(needle)
    );
  });
}

const BUILTIN_COMMANDS: PaletteCommandHit[] = [
  {
    kind: 'command',
    id: 'sidebar.toggle',
    title: 'Toggle sidebar',
    description: 'Show or hide the sidebar panel.',
    icon: 'sidebar',
    run: () => {
      sidebarStore.toggle();
    },
  },
  {
    kind: 'command',
    id: 'theme.graphite',
    title: 'Theme: Graphite',
    description: 'Dark neutral, the default.',
    icon: 'sun',
    run: () => {
      themeStore.setTheme('graphite');
    },
  },
  {
    kind: 'command',
    id: 'theme.eclipse',
    title: 'Theme: Eclipse',
    description: 'OLED-black with violet accent.',
    icon: 'moon',
    run: () => {
      themeStore.setTheme('eclipse');
    },
  },
  {
    kind: 'command',
    id: 'theme.ember',
    title: 'Theme: Ember',
    description: 'Warm near-black for late-night sessions.',
    icon: 'sun',
    run: () => {
      themeStore.setTheme('ember');
    },
  },
  {
    kind: 'command',
    id: 'theme.paper',
    title: 'Theme: Paper',
    description: 'Warm off-white light mode.',
    icon: 'sun',
    run: () => {
      themeStore.setTheme('paper');
    },
  },
  {
    kind: 'command',
    id: 'theme.solar',
    title: 'Theme: Solar',
    description: 'Cream and amber, soft light.',
    icon: 'sun',
    run: () => {
      themeStore.setTheme('solar');
    },
  },
  {
    kind: 'command',
    id: 'density.compact',
    title: 'Density: Compact',
    description: 'Tighter spacing for power users.',
    icon: 'generic',
    run: () => {
      densityStore.setDensity('compact');
    },
  },
  {
    kind: 'command',
    id: 'density.comfortable',
    title: 'Density: Comfortable',
    description: 'Balanced spacing (default).',
    icon: 'generic',
    run: () => {
      densityStore.setDensity('comfortable');
    },
  },
  {
    kind: 'command',
    id: 'density.cozy',
    title: 'Density: Cozy',
    description: 'Generous spacing for relaxed reading.',
    icon: 'generic',
    run: () => {
      densityStore.setDensity('cozy');
    },
  },
  {
    kind: 'command',
    id: 'pane.split',
    title: 'Split pane',
    description: 'Open a new empty pane beside the current one.',
    icon: 'split',
    run: () => {
      panesStore.split();
    },
  },
  {
    kind: 'command',
    id: 'nav.settings',
    title: 'Open Settings',
    description: 'Navigate to the settings page.',
    icon: 'settings',
    run: async () => {
      await goto('/settings');
    },
  },
  {
    kind: 'command',
    id: 'index.reload',
    title: 'Reload document index',
    description: 'Re-scan the workspace and rebuild the search index.',
    icon: 'refresh',
    run: () => {
      // eslint-disable-next-line no-console
      console.log('[vedox] command: reload document index (stub — POST /api/projects/.../index)');
    },
  },
];

// ---------------------------------------------------------------------------
// Global keyboard shortcut (Cmd+K / Ctrl+K)
// ---------------------------------------------------------------------------

/**
 * Wire the Cmd+K / Ctrl+K shortcut to togglePalette() on the window. Returns
 * a teardown function that removes the listener and any active subscriptions.
 *
 * This is called from CommandPalette.svelte onMount so only one listener is
 * ever installed — if the component is mounted twice (HMR, tests) the first
 * teardown fires before the second mount.
 */
export function initPaletteShortcut(): () => void {
  const teardownSubscription = wireResultsSubscription();

  const handler = (e: KeyboardEvent): void => {
    // macOS → metaKey, others → ctrlKey. Support both.
    const mod = e.metaKey || e.ctrlKey;
    if (!mod) return;
    if (e.key !== 'k' && e.key !== 'K') return;
    e.preventDefault();
    togglePalette();
  };

  if (typeof window !== 'undefined') {
    window.addEventListener('keydown', handler, { capture: true });
  }

  return () => {
    if (typeof window !== 'undefined') {
      window.removeEventListener('keydown', handler, { capture: true });
    }
    teardownSubscription();
  };
}
