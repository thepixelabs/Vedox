<script lang="ts">
  /**
   * AgentSettings — Category 6
   *
   * Default doc routing repo, dry-run toggle, auto-approve toggle.
   * The AI & Accounts section (provider picker, AlterGo accounts) from the
   * old monolithic settings page is also included here as it is agent-adjacent.
   */

  import { userPrefs, updatePrefs } from '$lib/stores/preferences';
  import { api, type ProviderInfo, type AltergoAccount } from '$lib/api/client';
  import { browser } from '$app/environment';
  import { onMount } from 'svelte';

  interface Props {
    searchQuery?: string;
  }

  let { searchQuery = '' }: Props = $props();

  // AI provider state (loaded from daemon on mount)
  let aiProviders = $state<ProviderInfo[]>([]);
  let altergoAccounts = $state<AltergoAccount[]>([]);
  let altergoAvailable = $state(false);
  let aiLoaded = $state(false);
  let defaultProvider = $state(
    browser ? (localStorage.getItem('vedox:ai-default-provider') || '') : ''
  );

  function handleDefaultProviderChange(id: string) {
    defaultProvider = id;
    if (browser) localStorage.setItem('vedox:ai-default-provider', id);
  }

  function moveAccount(index: number, direction: -1 | 1) {
    const next = [...altergoAccounts];
    const target = index + direction;
    if (target < 0 || target >= next.length) return;
    [next[index], next[target]] = [next[target], next[index]];
    altergoAccounts = next;
    if (browser) localStorage.setItem('vedox:ai-account-order', JSON.stringify(next.map((a) => a.name)));
  }

  onMount(async () => {
    try {
      const response = await api.getAIProviders();
      aiProviders = response.providers;
      altergoAccounts = response.altergo.accounts;
      altergoAvailable = response.altergo.available;

      const savedOrder = localStorage.getItem('vedox:ai-account-order');
      if (savedOrder) {
        try {
          const order: string[] = JSON.parse(savedOrder);
          const ordered: AltergoAccount[] = [];
          for (const name of order) {
            const found = altergoAccounts.find((a) => a.name === name);
            if (found) ordered.push(found);
          }
          for (const acct of altergoAccounts) {
            if (!ordered.find((a) => a.name === acct.name)) ordered.push(acct);
          }
          altergoAccounts = ordered;
        } catch { /* malformed */ }
      }

      if (!defaultProvider) {
        const first = aiProviders.find((p) => p.available);
        if (first) {
          defaultProvider = first.id;
          localStorage.setItem('vedox:ai-default-provider', first.id);
        }
      }
    } catch { /* daemon offline */ }
    aiLoaded = true;
  });

  function matches(text: string): boolean {
    if (!searchQuery) return true;
    return text.toLowerCase().includes(searchQuery.toLowerCase());
  }

  const prefs = $derived($userPrefs.agent);
</script>

<div class="settings-category">

  <!-- Default doc repo -->
  {#if matches('repo') || matches('routing') || matches('agent') || matches('default')}
    <div class="setting-row setting-row--block">
      <div class="setting-row__label">
        <span class="setting-row__name">Default documentation repo</span>
        <span class="setting-row__desc">
          Which registered repo the Doc Agent routes private documentation to when no explicit repo is specified.
          Set up repos in the Onboarding flow or the registry (coming soon).
        </span>
      </div>
      <div class="setting-row__input-wrap">
        <input
          type="text"
          class="text-input"
          value={prefs.defaultDocRepo}
          oninput={(e) => updatePrefs('agent', { defaultDocRepo: (e.target as HTMLInputElement).value })}
          placeholder="repo-id or leave blank to prompt on each run"
          aria-label="Default documentation repo"
          spellcheck="false"
        />
      </div>
    </div>
  {/if}

  <!-- Dry-run -->
  {#if matches('dry run') || matches('dry-run') || matches('preview') || matches('agent')}
    <div class="setting-row">
      <div class="setting-row__label">
        <span class="setting-row__name">Dry-run mode</span>
        <span class="setting-row__desc">
          Preview the documentation the agent would generate without writing or committing anything.
          Useful during onboarding and experimentation.
        </span>
      </div>
      <div class="setting-row__control">
        <button
          type="button"
          role="switch"
          class="toggle-switch"
          class:toggle-switch--on={prefs.dryRun}
          aria-checked={prefs.dryRun}
          onclick={() => updatePrefs('agent', { dryRun: !prefs.dryRun })}
          aria-label="Toggle dry-run mode"
        >
          <span class="toggle-switch__thumb" aria-hidden="true"></span>
        </button>
      </div>
    </div>
  {/if}

  <!-- Auto-approve -->
  {#if matches('auto-approve') || matches('approve') || matches('agent') || matches('commit')}
    <div class="setting-row">
      <div class="setting-row__label">
        <span class="setting-row__name">Auto-approve agent output</span>
        <span class="setting-row__desc">
          Automatically commit agent-generated documentation without showing a review diff.
          Leave off until you trust the agent's output for your project.
        </span>
      </div>
      <div class="setting-row__control">
        <button
          type="button"
          role="switch"
          class="toggle-switch"
          class:toggle-switch--on={prefs.autoApprove}
          aria-checked={prefs.autoApprove}
          onclick={() => updatePrefs('agent', { autoApprove: !prefs.autoApprove })}
          aria-label="Toggle auto-approve"
        >
          <span class="toggle-switch__thumb" aria-hidden="true"></span>
        </button>
      </div>
    </div>
  {/if}

  <!-- Divider -->
  <div class="section-divider" role="separator" aria-hidden="true"></div>

  <!-- AI Providers -->
  {#if matches('provider') || matches('ai') || matches('claude') || matches('openai') || matches('copilot') || matches('default provider')}
    <div class="subsection">
      <h3 class="subsection__title">AI providers</h3>

      {#if !aiLoaded}
        <p class="loading-text">Loading provider information…</p>
      {:else}
        <div class="setting-row">
          <div class="setting-row__label">
            <span class="setting-row__name">Default provider</span>
            <span class="setting-row__desc">Used in the New Project wizard when no provider is explicitly selected.</span>
          </div>
          <div class="setting-row__control">
            <select
              class="select-control"
              value={defaultProvider}
              onchange={(e) => handleDefaultProviderChange((e.target as HTMLSelectElement).value)}
              aria-label="Default AI provider"
            >
              {#if aiProviders.length === 0}
                <option value="">No providers found</option>
              {:else}
                {#each aiProviders as p}
                  <option value={p.id} disabled={!p.available}>
                    {p.name}{p.available ? '' : ' (not installed)'}
                  </option>
                {/each}
              {/if}
            </select>
          </div>
        </div>
      {/if}
    </div>
  {/if}

  <!-- AlterGo accounts -->
  {#if matches('altergo') || matches('account') || matches('fallback') || matches('usage limit')}
    <div class="subsection">
      <h3 class="subsection__title">AlterGo accounts</h3>

      {#if !aiLoaded}
        <p class="loading-text">Loading…</p>
      {:else if !altergoAvailable}
        <p class="setting-desc">
          <a
            href="https://github.com/thepixelabs/altergo"
            target="_blank"
            rel="noopener noreferrer"
            class="link"
          >Install AlterGo</a> to enable multi-account support. Vedox automatically falls back
          to the next account when one hits its usage limit.
        </p>
      {:else if altergoAccounts.length === 0}
        <p class="setting-desc">
          No accounts found. Run <code class="code-inline">altergo --setup</code> to create one.
        </p>
      {:else}
        <p class="setting-desc">
          Accounts are tried in order when the active account hits its usage limit.
        </p>
        <ul class="accounts-list" role="list">
          {#each altergoAccounts as account, i (account.name)}
            <li class="account-row">
              <div class="account-row__info">
                <span class="account-row__name">{account.name}</span>
                {#if account.providers?.length}
                  <div class="account-row__badges">
                    {#each account.providers as prov}
                      <span class="badge">{prov}</span>
                    {/each}
                  </div>
                {/if}
              </div>
              <div class="account-row__actions">
                <button
                  type="button"
                  class="move-btn"
                  onclick={() => moveAccount(i, -1)}
                  disabled={i === 0}
                  aria-label="Move {account.name} up"
                >↑</button>
                <button
                  type="button"
                  class="move-btn"
                  onclick={() => moveAccount(i, 1)}
                  disabled={i === altergoAccounts.length - 1}
                  aria-label="Move {account.name} down"
                >↓</button>
              </div>
            </li>
          {/each}
        </ul>
      {/if}
    </div>
  {/if}

</div>

<style>
  .settings-category {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .setting-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-6);
    padding: var(--space-3) 0;
    border-bottom: 1px solid var(--color-border);
  }

  .setting-row:last-child {
    border-bottom: none;
  }

  .setting-row--block {
    flex-direction: column;
    align-items: stretch;
    gap: var(--space-3);
  }

  .setting-row__label {
    display: flex;
    flex-direction: column;
    gap: 2px;
    flex: 1;
    min-width: 0;
  }

  .setting-row__name {
    font-size: var(--font-size-sm);
    font-weight: 500;
    color: var(--color-text-primary);
  }

  .setting-row__desc {
    font-size: var(--font-size-sm);
    color: var(--color-text-muted);
    line-height: 1.4;
  }

  .setting-row__control {
    flex-shrink: 0;
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }

  .setting-row__input-wrap {
    width: 100%;
  }

  .text-input {
    width: 100%;
    padding: var(--space-2) var(--space-3);
    background: var(--color-surface-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    color: var(--color-text-primary);
    font-size: var(--font-size-sm);
    font-family: var(--font-mono);
    transition: border-color 100ms ease;
    box-sizing: border-box;
  }

  .text-input:focus {
    outline: none;
    border-color: var(--color-accent);
    box-shadow: 0 0 0 3px var(--color-accent-subtle);
  }

  .toggle-switch {
    position: relative;
    display: inline-flex;
    align-items: center;
    width: 40px;
    height: 22px;
    background-color: var(--color-surface-elevated);
    border: 1px solid var(--color-border);
    border-radius: 11px;
    cursor: pointer;
    padding: 0;
    transition: background-color 150ms ease, border-color 150ms ease;
  }

  .toggle-switch--on {
    background-color: var(--color-accent);
    border-color: var(--color-accent);
  }

  .toggle-switch:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
  }

  .toggle-switch__thumb {
    position: absolute;
    left: 2px;
    width: 16px;
    height: 16px;
    background-color: var(--color-text-inverse);
    border-radius: 50%;
    transition: transform 150ms ease;
    box-shadow: var(--shadow-sm);
  }

  .toggle-switch--on .toggle-switch__thumb {
    transform: translateX(18px);
  }

  .section-divider {
    height: 1px;
    background: var(--color-border);
    margin: var(--space-4) 0;
  }

  .subsection {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .subsection__title {
    font-size: var(--font-size-sm);
    font-weight: 600;
    color: var(--color-text-primary);
  }

  .loading-text,
  .setting-desc {
    font-size: var(--font-size-sm);
    color: var(--color-text-muted);
    line-height: 1.5;
    margin: 0;
  }

  .link {
    color: var(--color-accent);
    text-decoration: underline;
    text-underline-offset: 2px;
  }

  .code-inline {
    font-family: var(--font-mono);
    font-size: 12px;
    background: var(--color-surface-elevated);
    padding: 1px 4px;
    border-radius: var(--radius-sm);
    color: var(--color-text-primary);
  }

  .select-control {
    padding: var(--space-2) var(--space-3);
    background: var(--color-surface-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    color: var(--color-text-primary);
    font-size: var(--font-size-sm);
    font-family: var(--font-sans);
    cursor: pointer;
    min-width: 160px;
    transition: border-color 100ms ease;
  }

  .select-control:focus {
    outline: none;
    border-color: var(--color-accent);
    box-shadow: 0 0 0 3px var(--color-accent-subtle);
  }

  .accounts-list {
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: 0;
    margin: 0;
  }

  .account-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--space-3) var(--space-4);
    background: var(--color-surface-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    gap: var(--space-4);
  }

  .account-row__info {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    flex: 1;
    min-width: 0;
  }

  .account-row__name {
    font-size: var(--font-size-sm);
    font-weight: 500;
    color: var(--color-text-primary);
    font-family: var(--font-mono);
  }

  .account-row__badges {
    display: flex;
    gap: var(--space-1);
    flex-wrap: wrap;
  }

  .badge {
    font-size: 10px;
    font-weight: 500;
    padding: 1px 6px;
    border-radius: var(--radius-full);
    background: var(--color-accent-subtle);
    color: var(--color-accent);
    letter-spacing: 0.03em;
    text-transform: capitalize;
  }

  .account-row__actions {
    display: flex;
    gap: var(--space-1);
  }

  .move-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 28px;
    height: 28px;
    background: none;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    color: var(--color-text-muted);
    font-size: 12px;
    cursor: pointer;
    transition: border-color 80ms ease, color 80ms ease;
  }

  .move-btn:hover:not(:disabled) {
    border-color: var(--color-text-muted);
    color: var(--color-text-primary);
  }

  .move-btn:disabled {
    opacity: 0.3;
    cursor: not-allowed;
  }

  .move-btn:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
  }
</style>
