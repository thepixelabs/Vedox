/**
 * providerConfig.ts — Provider Drawer state (VDX-PD3-FE)
 *
 * Svelte 5 runes-based state for the multi-provider config drawer. We export
 * a single factory that captures `$state` cells inside a closure and returns
 * accessor functions; consumers import the singleton `providerDrawer` and
 * read fields via `providerDrawer.drawerOpen` etc. This keeps reactivity
 * intact across module boundaries (rune cells are captured by reference).
 *
 * State owned here:
 *   - drawerOpen / activeProviderId — UI shell
 *   - detectedProviders             — probe-based provider list
 *   - claudeConfig / codexConfig    — last-loaded snapshots (with etags)
 *   - drawerLoading / drawerError   — async lifecycle
 *
 * Per-tab dirty/saving state lives inside each tab component — the store
 * only tracks the snapshots and surfaces the etags every PUT needs.
 */

import {
  api,
  ApiError,
  type ClaudeConfig,
  type CodexConfig,
  type DetectedProvider,
  type DetectedProviderId,
} from '$lib/api/client';

function createProviderDrawer() {
  let drawerOpen = $state(false);
  let activeProviderId = $state<DetectedProviderId | null>(null);
  let detectedProviders = $state<DetectedProvider[]>([]);
  let claudeConfig = $state<ClaudeConfig | null>(null);
  let codexConfig = $state<CodexConfig | null>(null);
  let drawerLoading = $state(false);
  let drawerError = $state<string | null>(null);

  function formatErr(err: unknown): string {
    if (err instanceof ApiError) return `[${err.code}] ${err.message}`;
    if (err instanceof Error) return err.message;
    return 'Unknown error';
  }

  async function loadProviders(project: string): Promise<void> {
    drawerLoading = true;
    drawerError = null;
    try {
      detectedProviders = await api.getProviders(project);
    } catch (err) {
      drawerError = formatErr(err);
    } finally {
      drawerLoading = false;
    }
  }

  async function loadClaudeConfig(project: string): Promise<void> {
    drawerLoading = true;
    drawerError = null;
    try {
      claudeConfig = await api.getClaudeConfig(project);
    } catch (err) {
      drawerError = formatErr(err);
    } finally {
      drawerLoading = false;
    }
  }

  async function loadCodexConfig(project: string): Promise<void> {
    drawerLoading = true;
    drawerError = null;
    try {
      codexConfig = await api.getCodexConfig(project);
    } catch (err) {
      drawerError = formatErr(err);
    } finally {
      drawerLoading = false;
    }
  }

  async function openDrawer(project: string, providerId?: DetectedProviderId): Promise<void> {
    drawerOpen = true;
    drawerError = null;
    await loadProviders(project);
    // Pick the requested provider if it is available; otherwise fall back to
    // the first available one so the drawer always opens on a usable tab.
    const requested = providerId
      ? detectedProviders.find((p) => p.id === providerId && p.available)
      : null;
    const fallback = detectedProviders.find((p) => p.available) ?? null;
    activeProviderId = (requested ?? fallback)?.id ?? null;
  }

  function closeDrawer(): void {
    drawerOpen = false;
  }

  function setActiveProvider(id: DetectedProviderId): void {
    activeProviderId = id;
  }

  return {
    // Reactive accessors — use getters so external reads stay live.
    get drawerOpen() { return drawerOpen; },
    set drawerOpen(value: boolean) { drawerOpen = value; },
    get activeProviderId() { return activeProviderId; },
    get detectedProviders() { return detectedProviders; },
    get claudeConfig() { return claudeConfig; },
    set claudeConfig(value: ClaudeConfig | null) { claudeConfig = value; },
    get codexConfig() { return codexConfig; },
    set codexConfig(value: CodexConfig | null) { codexConfig = value; },
    get drawerLoading() { return drawerLoading; },
    get drawerError() { return drawerError; },

    // Actions
    openDrawer,
    closeDrawer,
    loadProviders,
    loadClaudeConfig,
    loadCodexConfig,
    setActiveProvider,
  };
}

export const providerDrawer = createProviderDrawer();
export type ProviderDrawerStore = ReturnType<typeof createProviderDrawer>;
