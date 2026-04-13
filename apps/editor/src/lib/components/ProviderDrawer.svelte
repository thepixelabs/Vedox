<!--
  ProviderDrawer.svelte — multi-provider config drawer (VDX-PD3-FE).

  A right-anchored 520px panel that hosts four "concern" tabs (Memory,
  Permissions, MCP, Agents) for whichever provider the user has selected
  from the top strip (Claude / Codex / Gemini).

  Layout:
    ┌──────────────────────────────────────┐
    │ Provider Config              [×]     │  header
    ├──────────────────────────────────────┤
    │ [Claude] [Codex] [Gemini]            │  provider strip (pills)
    ├──────────────────────────────────────┤
    │ ⚠ Global — affects all projects      │  (codex only)
    ├──────┬───────────────────────────────┤
    │ Mem  │                               │
    │ Perm │   active concern tab          │  rail + main pane
    │ MCP  │                               │
    │ Agt  │                               │
    └──────┴───────────────────────────────┘

  Keyboard:
    Esc                close drawer
    ArrowDown/ArrowUp  navigate concern rail
-->

<script lang="ts">
  import { onMount } from 'svelte';
  import type { DetectedProviderId } from '$lib/api/client';
  import { providerDrawer } from '$lib/stores/providerConfig.svelte';
  import MemoryTab from './MemoryTab.svelte';
  import PermissionsTab from './PermissionsTab.svelte';
  import McpTab from './McpTab.svelte';
  import AgentsTab from './AgentsTab.svelte';

  interface Props {
    project: string;
    open: boolean;
    onclose: () => void;
  }

  const { project, open, onclose }: Props = $props();

  type ConcernId = 'memory' | 'permissions' | 'mcp' | 'agents';

  const CONCERNS: { id: ConcernId; label: string; icon: string }[] = [
    {
      id: 'memory',
      label: 'Memory',
      icon: '<path d="M12 2a4 4 0 0 0-4 4v1a4 4 0 0 0-2 7.5V18a3 3 0 0 0 6 0v-2"/><path d="M12 2a4 4 0 0 1 4 4v1a4 4 0 0 1 2 7.5V18a3 3 0 0 1-6 0v-2"/>',
    },
    {
      id: 'permissions',
      label: 'Permissions',
      icon: '<rect x="5" y="11" width="14" height="10" rx="2"/><path d="M8 11V7a4 4 0 0 1 8 0v4"/>',
    },
    {
      id: 'mcp',
      label: 'MCP',
      icon: '<rect x="3" y="3" width="7" height="7" rx="1"/><rect x="14" y="3" width="7" height="7" rx="1"/><rect x="3" y="14" width="7" height="7" rx="1"/><rect x="14" y="14" width="7" height="7" rx="1"/>',
    },
    {
      id: 'agents',
      label: 'Agents',
      icon: '<circle cx="9" cy="8" r="3"/><path d="M3 21v-1a5 5 0 0 1 5-5h2a5 5 0 0 1 5 5v1"/><circle cx="17" cy="10" r="2"/><path d="M15 21v-1a3 3 0 0 1 3-3h.5a3 3 0 0 1 3 3v1"/>',
    },
  ];

  let activeConcern = $state<ConcernId>('memory');

  // Reactive view of the store accessors.
  const drawer = providerDrawer;

  // Open the drawer (loads providers) whenever `open` flips to true.
  let lastOpen = $state(false);
  $effect(() => {
    if (open && !lastOpen) {
      void drawer.openDrawer(project);
    }
    if (!open && lastOpen) {
      drawer.closeDrawer();
    }
    lastOpen = open;
  });

  function close() {
    onclose();
  }

  function pickProvider(id: DetectedProviderId) {
    drawer.setActiveProvider(id);
    // Reset to a concern that makes sense for the new provider — Memory is
    // Claude-only, so non-Claude providers should land on MCP.
    if (id !== 'claude') activeConcern = 'mcp';
  }

  function onKeydown(e: KeyboardEvent) {
    if (!open) return;
    if (e.key === 'Escape') {
      e.preventDefault();
      close();
      return;
    }
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault();
      const idx = CONCERNS.findIndex((c) => c.id === activeConcern);
      const delta = e.key === 'ArrowDown' ? 1 : -1;
      const next = (idx + delta + CONCERNS.length) % CONCERNS.length;
      const nextItem = CONCERNS[next];
      if (nextItem) activeConcern = nextItem.id;
    }
  }

  onMount(() => {
    document.addEventListener('keydown', onKeydown);
    return () => document.removeEventListener('keydown', onKeydown);
  });

  const activeProvider = $derived(
    drawer.detectedProviders.find((p) => p.id === drawer.activeProviderId) ?? null,
  );

  // Friendly provider name fallback when the registry has not loaded yet.
  const activeProviderName = $derived(activeProvider?.name ?? 'this provider');
</script>

{#if open}
  <!-- Backdrop -->
  <button
    class="provider-drawer__backdrop"
    type="button"
    aria-label="Close provider config"
    onclick={close}
  ></button>

  <div
    class="provider-drawer"
    role="dialog"
    aria-modal="true"
    aria-labelledby="provider-drawer-title"
  >
    <header class="provider-drawer__header">
      <h2 id="provider-drawer-title" class="provider-drawer__title">Provider Config</h2>
      <button
        type="button"
        class="provider-drawer__close"
        aria-label="Close"
        onclick={close}
      >
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <line x1="6" y1="6" x2="18" y2="18"/>
          <line x1="18" y1="6" x2="6" y2="18"/>
        </svg>
      </button>
    </header>

    <!-- Provider selector strip -->
    <div class="provider-strip" role="tablist" aria-label="Detected providers">
      {#each drawer.detectedProviders as p (p.id)}
        <button
          type="button"
          role="tab"
          aria-selected={drawer.activeProviderId === p.id}
          class="provider-pill provider-pill--{p.id}"
          class:provider-pill--active={drawer.activeProviderId === p.id}
          disabled={!p.available}
          title={p.available ? p.name : `${p.name} (not detected)`}
          onclick={() => p.available && pickProvider(p.id)}
        >
          <span class="provider-pill__dot" aria-hidden="true"></span>
          <span class="provider-pill__label">{p.name}</span>
          {#if !p.available}
            <span class="provider-pill__badge">pending</span>
          {/if}
        </button>
      {/each}
      {#if drawer.drawerLoading && drawer.detectedProviders.length === 0}
        <span class="provider-strip__loading">Detecting providers…</span>
      {/if}
    </div>

    <!-- Codex global-scope warning -->
    {#if drawer.activeProviderId === 'codex'}
      <div class="scope-banner" role="note">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
          <path d="M10.3 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/>
          <line x1="12" y1="9" x2="12" y2="13"/>
          <line x1="12" y1="17" x2="12.01" y2="17"/>
        </svg>
        <span>Global config — affects all projects on this machine.</span>
      </div>
    {/if}

    {#if drawer.drawerError}
      <div class="error-banner" role="alert">{drawer.drawerError}</div>
    {/if}

    <!-- Body: rail + main pane -->
    <div class="provider-drawer__body">
      <nav class="concern-rail" aria-label="Configuration concerns">
        {#each CONCERNS as c (c.id)}
          <button
            type="button"
            class="concern-rail__btn"
            class:concern-rail__btn--active={activeConcern === c.id}
            aria-current={activeConcern === c.id ? 'page' : undefined}
            onclick={() => (activeConcern = c.id)}
          >
            <svg
              width="18"
              height="18"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="1.6"
              stroke-linecap="round"
              stroke-linejoin="round"
              aria-hidden="true"
            >
              {@html c.icon}
            </svg>
            <span>{c.label}</span>
          </button>
        {/each}
      </nav>

      <main class="provider-drawer__main">
        {#if !drawer.activeProviderId}
          <p class="empty">No provider selected.</p>
        {:else if activeConcern === 'memory'}
          <MemoryTab
            {project}
            providerId={drawer.activeProviderId}
            providerName={activeProviderName}
          />
        {:else if activeConcern === 'permissions'}
          <PermissionsTab
            {project}
            providerId={drawer.activeProviderId}
            providerName={activeProviderName}
          />
        {:else if activeConcern === 'mcp'}
          <McpTab
            {project}
            providerId={drawer.activeProviderId}
            providerName={activeProviderName}
          />
        {:else if activeConcern === 'agents'}
          <AgentsTab
            {project}
            providerId={drawer.activeProviderId}
            providerName={activeProviderName}
          />
        {/if}
      </main>
    </div>
  </div>
{/if}

<style>
  .provider-drawer__backdrop {
    position: fixed;
    inset: 0;
    background: oklch(0% 0 0 / 0.32);
    border: 0;
    padding: 0;
    cursor: pointer;
    z-index: 90;
    animation: drawer-fade-in 160ms var(--ease-out, ease-out) both;
  }

  .provider-drawer {
    position: fixed;
    top: 0;
    right: 0;
    bottom: 0;
    width: 520px;
    max-width: 100vw;
    background: var(--surface-1);
    border-left: 1px solid var(--border-hairline);
    box-shadow: var(--shadow-overlay);
    display: flex;
    flex-direction: column;
    z-index: 100;
    animation: drawer-slide-in 220ms var(--ease-out, ease-out) both;
  }

  @keyframes drawer-slide-in {
    from { transform: translateX(24px); opacity: 0; }
    to   { transform: translateX(0);    opacity: 1; }
  }
  @keyframes drawer-fade-in {
    from { opacity: 0; }
    to   { opacity: 1; }
  }

  @media (max-width: 640px) {
    .provider-drawer { width: 100vw; }
  }

  .provider-drawer__header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--space-4) var(--space-5);
    border-bottom: 1px solid var(--border-hairline);
    flex-shrink: 0;
  }
  .provider-drawer__title {
    margin: 0;
    font-size: var(--text-base);
    font-weight: 600;
    color: var(--text-1);
  }
  .provider-drawer__close {
    background: none;
    border: 1px solid transparent;
    color: var(--text-3);
    padding: var(--space-1);
    border-radius: var(--radius-sm);
    cursor: pointer;
    display: inline-flex;
    align-items: center;
    justify-content: center;
  }
  .provider-drawer__close:hover { background: var(--surface-3); color: var(--text-1); }
  .provider-drawer__close:focus-visible { outline: 2px solid var(--accent-solid); outline-offset: 2px; }

  .provider-strip {
    display: flex;
    gap: var(--space-2);
    padding: var(--space-3) var(--space-5);
    border-bottom: 1px solid var(--border-hairline);
    flex-shrink: 0;
    flex-wrap: wrap;
  }

  .provider-pill {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-1) var(--space-3);
    border-radius: var(--radius-full, 999px);
    border: 1px solid var(--border-default);
    background: var(--surface-2);
    color: var(--text-3);
    font-family: var(--font-body);
    font-size: var(--text-xs);
    font-weight: 500;
    cursor: pointer;
  }
  .provider-pill:disabled { opacity: 0.55; cursor: not-allowed; }
  .provider-pill:focus-visible { outline: 2px solid var(--accent-solid); outline-offset: 2px; }

  .provider-pill__dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: currentColor;
    flex-shrink: 0;
  }

  .provider-pill--claude { color: var(--provider-claude); }
  .provider-pill--codex  { color: var(--provider-codex); }
  .provider-pill--gemini { color: var(--provider-gemini); }

  .provider-pill--active.provider-pill--claude {
    background: var(--provider-claude-subtle);
    border-color: var(--provider-claude-border);
    color: var(--text-1);
  }
  .provider-pill--active.provider-pill--codex {
    background: var(--provider-codex-subtle);
    border-color: var(--provider-codex-border);
    color: var(--text-1);
  }
  .provider-pill--active.provider-pill--gemini {
    background: var(--provider-gemini-subtle);
    border-color: var(--provider-gemini-border);
    color: var(--text-1);
  }
  .provider-pill--active .provider-pill__dot {
    background: currentColor;
  }
  .provider-pill--active.provider-pill--claude .provider-pill__dot { background: var(--provider-claude); }
  .provider-pill--active.provider-pill--codex  .provider-pill__dot { background: var(--provider-codex); }
  .provider-pill--active.provider-pill--gemini .provider-pill__dot { background: var(--provider-gemini); }

  .provider-pill__badge {
    font-size: 9px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    padding: 1px 5px;
    border-radius: var(--radius-sm);
    background: var(--surface-3);
    color: var(--text-4);
  }

  .provider-strip__loading {
    font-size: var(--text-xs);
    color: var(--text-4);
    padding: var(--space-1) var(--space-2);
  }

  .scope-banner {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-5);
    background: oklch(from var(--warning) l c h / 0.14);
    border-bottom: 1px solid oklch(from var(--warning) l c h / 0.4);
    color: var(--warning);
    font-size: var(--text-xs);
    font-weight: 500;
  }

  .error-banner {
    margin: var(--space-3) var(--space-5);
    padding: var(--space-2) var(--space-3);
    background: oklch(from var(--error) l c h / 0.12);
    border: 1px solid oklch(from var(--error) l c h / 0.4);
    border-radius: var(--radius-sm);
    color: var(--error);
    font-size: var(--text-xs);
  }

  .provider-drawer__body {
    display: flex;
    flex: 1;
    min-height: 0;
  }

  .concern-rail {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    padding: var(--space-3) var(--space-2);
    border-right: 1px solid var(--border-hairline);
    background: var(--surface-2);
    flex-shrink: 0;
    width: 96px;
  }
  .concern-rail__btn {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;
    padding: var(--space-2);
    background: none;
    border: 1px solid transparent;
    border-radius: var(--radius-sm);
    color: var(--text-3);
    font-family: var(--font-body);
    font-size: 11px;
    font-weight: 500;
    cursor: pointer;
  }
  .concern-rail__btn:hover { background: var(--surface-3); color: var(--text-1); }
  .concern-rail__btn:focus-visible { outline: 2px solid var(--accent-solid); outline-offset: 2px; }
  .concern-rail__btn--active {
    background: var(--accent-subtle);
    color: var(--accent-text, var(--text-1));
    border-color: var(--accent-border, var(--accent-solid));
  }

  .provider-drawer__main {
    flex: 1;
    min-width: 0;
    overflow-y: auto;
    padding: var(--space-4) var(--space-5);
  }

  .empty {
    color: var(--text-4);
    font-size: var(--text-sm);
    padding: var(--space-6);
    text-align: center;
  }
</style>
