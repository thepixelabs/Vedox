<script lang="ts">
  /**
   * InstallAgent.svelte — Onboarding step 3.
   *
   * Lets the user pick which AI providers to install the Vedox Doc Agent into.
   * Supported: Claude Code (MCP), Codex, Gemini, Copilot.
   *
   * Each provider calls POST /api/agent/install with { provider, projectId }.
   * Install progress is shown inline per provider.
   *
   * If the daemon is not running, providers show a "daemon offline" state.
   * The user can still proceed (agent can be installed later from settings).
   *
   * Calls `onBack`, `onNext`, or `onSkip`.
   */

  import { onboardingStore } from '$lib/stores/onboarding.svelte';

  interface Props {
    onBack: () => void;
    onNext: () => void;
    onSkip: () => void;
  }

  const { onBack, onNext, onSkip }: Props = $props();

  // ---------------------------------------------------------------------------
  // Provider definitions
  // ---------------------------------------------------------------------------

  interface ProviderDef {
    id: string;
    label: string;
    description: string;
  }

  const PROVIDERS: ProviderDef[] = [
    {
      id: 'claude-code',
      label: 'Claude Code',
      description: 'installs as an MCP server — trigger: "vedox document everything"',
    },
    {
      id: 'codex',
      label: 'OpenAI Codex',
      description: 'installs as a tool function in your codex profile',
    },
    {
      id: 'gemini',
      label: 'Google Gemini',
      description: 'installs via the Gemini CLI extension API',
    },
    {
      id: 'copilot',
      label: 'GitHub Copilot',
      description: 'installs as a Copilot agent extension (preview)',
    },
  ];

  // ---------------------------------------------------------------------------
  // State
  // ---------------------------------------------------------------------------

  type ProviderStatus = 'idle' | 'installing' | 'done' | 'error';

  interface ProviderState {
    selected: boolean;
    status: ProviderStatus;
    error: string | null;
  }

  let providerStates = $state<Record<string, ProviderState>>(
    Object.fromEntries(
      PROVIDERS.map((p) => [
        p.id,
        {
          selected: onboardingStore.selectedProviders.includes(p.id),
          status: 'idle' as ProviderStatus,
          error: null,
        },
      ])
    )
  );

  let globalStatus = $state<'idle' | 'installing' | 'done' | 'error'>('idle');

  const selectedProviders = $derived(
    PROVIDERS.filter((p) => providerStates[p.id]?.selected)
  );

  const allDone = $derived(
    selectedProviders.length > 0 &&
    selectedProviders.every((p) => providerStates[p.id]?.status === 'done')
  );

  const anyInstalling = $derived(
    selectedProviders.some((p) => providerStates[p.id]?.status === 'installing')
  );

  function toggleProvider(id: string) {
    const cur = providerStates[id];
    if (!cur || cur.status === 'installing' || cur.status === 'done') return;
    providerStates = {
      ...providerStates,
      [id]: { ...cur, selected: !cur.selected, status: 'idle', error: null },
    };
  }

  // ---------------------------------------------------------------------------
  // Install
  // ---------------------------------------------------------------------------

  async function installProvider(id: string): Promise<void> {
    providerStates = {
      ...providerStates,
      [id]: { ...providerStates[id], status: 'installing', error: null },
    };

    try {
      const res = await fetch('/api/agent/install', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ provider: id }),
        signal: AbortSignal.timeout(15000),
      });
      if (!res.ok) {
        const body = (await res.json().catch(() => ({}))) as Record<string, unknown>;
        throw new Error((body['message'] as string | undefined) ?? `${res.status}`);
      }
      providerStates = {
        ...providerStates,
        [id]: { ...providerStates[id], status: 'done', error: null },
      };
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      providerStates = {
        ...providerStates,
        [id]: { ...providerStates[id], status: 'error', error: msg },
      };
    }
  }

  async function installAll() {
    if (selectedProviders.length === 0) return;
    globalStatus = 'installing';
    onboardingStore.setSelectedProviders(selectedProviders.map((p) => p.id));

    await Promise.allSettled(selectedProviders.map((p) => installProvider(p.id)));
    globalStatus = 'done';
  }

  function handleNext() {
    onboardingStore.setSelectedProviders(selectedProviders.map((p) => p.id));
    onNext();
  }

  const canInstall = $derived(selectedProviders.length > 0 && !anyInstalling && !allDone);
  const canContinue = $derived(!anyInstalling);
</script>

<div class="step-agent">
  <header class="step-agent__header">
    <h2 class="step-agent__title">install doc agent</h2>
    <p class="step-agent__desc">
      the vedox doc agent hooks into your ai provider. say "vedox document everything"
      to capture docs automatically. pick the providers you use.
    </p>
  </header>

  <!-- ── Provider list ─────────────────────────────────────────────────────── -->
  <ul class="step-agent__list" role="list" aria-label="AI providers">
    {#each PROVIDERS as provider (provider.id)}
      {@const ps = providerStates[provider.id]}
      <li class="step-agent__provider" class:step-agent__provider--selected={ps?.selected}>
        <label class="step-agent__provider-label">
          <input
            class="step-agent__checkbox"
            type="checkbox"
            checked={ps?.selected ?? false}
            disabled={ps?.status === 'installing' || ps?.status === 'done'}
            onchange={() => toggleProvider(provider.id)}
            aria-label="Select {provider.label}"
          />
          <span class="step-agent__provider-info">
            <span class="step-agent__provider-name">{provider.label}</span>
            <span class="step-agent__provider-desc">{provider.description}</span>
          </span>
          <span class="step-agent__provider-status" aria-live="polite">
            {#if ps?.status === 'installing'}
              <span class="step-agent__spinner" aria-label="Installing..."></span>
            {:else if ps?.status === 'done'}
              <span class="step-agent__done-mark" aria-label="Installed">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                  <polyline points="20 6 9 17 4 12"/>
                </svg>
              </span>
            {:else if ps?.status === 'error'}
              <span class="step-agent__error-mark" title={ps.error ?? 'error'} aria-label="Install failed">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                  <line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/>
                  <circle cx="12" cy="12" r="10"/>
                </svg>
              </span>
            {/if}
          </span>
        </label>
        {#if ps?.status === 'error' && ps.error}
          <p class="step-agent__provider-error" role="alert">{ps.error}</p>
        {/if}
      </li>
    {/each}
  </ul>

  <!-- ── Offline note ──────────────────────────────────────────────────────── -->
  <p class="step-agent__offline-note">
    install runs via the local daemon. if the daemon is not running, agents can be
    installed later from settings &rsaquo; agent.
  </p>

  <!-- ── Actions ──────────────────────────────────────────────────────────── -->
  <footer class="step-agent__footer">
    {#if !allDone}
      <button
        class="step-btn step-btn--primary"
        type="button"
        disabled={!canInstall}
        onclick={installAll}
      >
        {anyInstalling ? 'installing...' : './install selected'}
      </button>
    {:else}
      <button
        class="step-btn step-btn--primary"
        type="button"
        onclick={handleNext}
      >
        ./continue
      </button>
    {/if}
    <button
      class="step-btn step-btn--secondary"
      type="button"
      disabled={!canContinue}
      onclick={onBack}
    >
      ./back
    </button>
    <button
      class="step-btn step-btn--ghost"
      type="button"
      disabled={anyInstalling}
      onclick={onSkip}
    >
      ./skip
    </button>
  </footer>
</div>

<style>
  .step-agent {
    display: flex;
    flex-direction: column;
    gap: var(--space-5);
    min-height: 0;
  }

  /* ── Header ── */

  .step-agent__header {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .step-agent__title {
    margin: 0;
    font-size: var(--font-size-lg, 1.125rem);
    font-weight: 600;
    font-family: var(--font-mono);
    color: var(--color-text-primary);
  }

  .step-agent__desc {
    margin: 0;
    font-size: var(--font-size-sm);
    font-family: var(--font-mono);
    color: var(--color-text-secondary);
    line-height: 1.6;
  }

  /* ── Provider list ── */

  .step-agent__list {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .step-agent__provider {
    border-radius: var(--radius-sm);
    border: 1px solid transparent;
    transition: border-color 80ms var(--ease-out);
  }

  .step-agent__provider--selected {
    border-color: var(--color-accent);
    background-color: var(--color-accent-subtle);
  }

  .step-agent__provider-label {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-3) var(--space-3);
    border-radius: var(--radius-sm);
    cursor: pointer;
  }

  .step-agent__checkbox {
    accent-color: var(--color-accent);
    width: 14px;
    height: 14px;
    flex-shrink: 0;
    cursor: pointer;
  }

  .step-agent__provider-info {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }

  .step-agent__provider-name {
    font-size: var(--font-size-sm);
    font-family: var(--font-mono);
    font-weight: 600;
    color: var(--color-text-primary);
  }

  .step-agent__provider-desc {
    font-size: 11px;
    font-family: var(--font-mono);
    color: var(--color-text-muted);
    line-height: 1.5;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .step-agent__provider-status {
    display: flex;
    align-items: center;
    flex-shrink: 0;
    width: 20px;
    justify-content: center;
  }

  .step-agent__spinner {
    display: inline-block;
    width: 12px;
    height: 12px;
    border: 2px solid var(--color-border);
    border-top-color: var(--color-accent);
    border-radius: 50%;
    animation: spin 600ms linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  .step-agent__done-mark {
    color: var(--color-success, #38a169);
    display: flex;
    align-items: center;
  }

  .step-agent__error-mark {
    color: var(--color-error, #e53e3e);
    display: flex;
    align-items: center;
    cursor: help;
  }

  .step-agent__provider-error {
    margin: 0 0 var(--space-2) calc(var(--space-3) + 14px + var(--space-3));
    font-size: 11px;
    font-family: var(--font-mono);
    color: var(--color-error, #e53e3e);
    line-height: 1.4;
  }

  /* ── Offline note ── */

  .step-agent__offline-note {
    margin: 0;
    font-size: 11px;
    font-family: var(--font-mono);
    color: var(--color-text-muted);
    line-height: 1.5;
  }

  /* ── Footer ── */

  .step-agent__footer {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    flex-wrap: wrap;
    flex-shrink: 0;
  }

  /* ── Reduced motion ── */

  @media (prefers-reduced-motion: reduce) {
    .step-agent__spinner {
      animation: none;
      opacity: 0.5;
    }
    .step-agent__provider {
      transition: none;
    }
  }
</style>
