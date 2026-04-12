<script lang="ts">
  /**
   * /settings — Vedox settings page.
   *
   * Appearance section with the five-theme picker grid, plus About section.
   */

  import { themeStore, densityStore } from "$lib/theme/store";
  import ThemePreviewCard from "$lib/components/ThemePreviewCard.svelte";
  import FontPicker from "$lib/components/FontPicker.svelte";
  import { readingStore } from "$lib/stores/reading";
  import type { ReadingMeasure } from "$lib/stores/reading";
  import { browser } from "$app/environment";
  import { onMount } from "svelte";
  import { shortcuts, shortcutCategories } from "$lib/data/shortcuts-data";
  import { api, type ProviderInfo, type AltergoAccount } from "$lib/api/client";

  const themes = [
    { theme: 'graphite' as const, label: 'Graphite', description: 'Dark neutral, the default' },
    { theme: 'eclipse' as const, label: 'Eclipse', description: 'OLED-black with violet accent' },
    { theme: 'ember' as const, label: 'Ember', description: 'Warm near-black for late-night sessions' },
    { theme: 'paper' as const, label: 'Paper', description: 'Warm off-white light mode' },
    { theme: 'solar' as const, label: 'Solar', description: 'Cream and amber, soft light' },
  ] as const;

  const densities = [
    { value: 'compact' as const, label: 'Compact', description: 'Tighter spacing for power users' },
    { value: 'comfortable' as const, label: 'Comfortable', description: 'Balanced spacing (default)' },
    { value: 'cozy' as const, label: 'Cozy', description: 'Generous spacing for relaxed reading' },
  ] as const;

  const readingWidths: { value: ReadingMeasure; label: string }[] = [
    { value: 'narrow', label: 'Narrow' },
    { value: 'default', label: 'Default' },
    { value: 'wide', label: 'Wide' },
  ];

  const fontSizes = [
    { value: '13px', label: 'Small' },
    { value: '16px', label: 'Default' },
    { value: '18px', label: 'Large' },
  ] as const;

  const FONT_SIZE_KEY = 'vedox:font-size';
  const FONT_SIZE_DEFAULT = '16px';

  let activeFontSize = $state(FONT_SIZE_DEFAULT);

  // Font selection state
  let fontBody = $state(typeof localStorage !== 'undefined' ? (localStorage.getItem('vedox:font-body') || '"Geist Variable", "Geist", system-ui, sans-serif') : '"Geist Variable", "Geist", system-ui, sans-serif');
  let fontDisplay = $state(typeof localStorage !== 'undefined' ? (localStorage.getItem('vedox:font-display') || '"Fraunces Variable", "Fraunces", Georgia, serif') : '"Fraunces Variable", "Fraunces", Georgia, serif');
  let fontMono = $state(typeof localStorage !== 'undefined' ? (localStorage.getItem('vedox:font-mono') || '"JetBrains Mono Variable", "JetBrains Mono", monospace') : '"JetBrains Mono Variable", "JetBrains Mono", monospace');

  function handleFontChange(category: 'body' | 'display' | 'mono', newFamily: string) {
    const cssVar = `--font-${category}`;
    const storageKey = `vedox:font-${category}`;
    document.documentElement.style.setProperty(cssVar, newFamily);
    localStorage.setItem(storageKey, newFamily);
    if (category === 'body') fontBody = newFamily;
    else if (category === 'display') fontDisplay = newFamily;
    else fontMono = newFamily;
  }

  function applyFontSize(size: string): void {
    if (!browser) return;
    document.documentElement.style.setProperty('--font-size-override', size);
    try { localStorage.setItem(FONT_SIZE_KEY, size); } catch { /* quota */ }
    activeFontSize = size;
  }

  // ── AI & Accounts state ─────────────────────────────────────────────────────
  let aiProviders = $state<ProviderInfo[]>([]);
  let altergoAccounts = $state<AltergoAccount[]>([]);
  let altergoAvailable = $state(false);
  let aiProvidersLoaded = $state(false);
  let defaultProvider = $state(
    typeof localStorage !== 'undefined'
      ? (localStorage.getItem('vedox:ai-default-provider') || '')
      : ''
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
    if (browser) localStorage.setItem('vedox:ai-account-order', JSON.stringify(next.map(a => a.name)));
  }

  onMount(async () => {
    const stored = localStorage.getItem(FONT_SIZE_KEY);
    const valid = fontSizes.some((f) => f.value === stored);
    const size = valid ? stored! : FONT_SIZE_DEFAULT;
    activeFontSize = size;
    document.documentElement.style.setProperty('--font-size-override', size);

    // Load AI providers (non-blocking; section shows gracefully while loading).
    try {
      const response = await api.getAIProviders();
      aiProviders = response.providers;
      altergoAccounts = response.altergo.accounts;
      altergoAvailable = response.altergo.available;

      // Restore saved account order.
      const savedOrder = localStorage.getItem('vedox:ai-account-order');
      if (savedOrder) {
        try {
          const order: string[] = JSON.parse(savedOrder);
          const ordered: AltergoAccount[] = [];
          for (const name of order) {
            const found = altergoAccounts.find(a => a.name === name);
            if (found) ordered.push(found);
          }
          // Append any new accounts not in the saved order.
          for (const acct of altergoAccounts) {
            if (!ordered.find(a => a.name === acct.name)) ordered.push(acct);
          }
          altergoAccounts = ordered;
        } catch { /* malformed JSON, ignore */ }
      }

      // Auto-set default provider if not yet configured.
      if (!defaultProvider) {
        const first = aiProviders.find(p => p.available);
        if (first) {
          defaultProvider = first.id;
          localStorage.setItem('vedox:ai-default-provider', first.id);
        }
      }
    } catch { /* backend offline — section renders with empty state */ }

    aiProvidersLoaded = true;
  });
</script>

<svelte:head>
  <title>Settings — Vedox</title>
</svelte:head>

<div class="settings-page">
  <header class="settings-page__header">
    <h1 class="settings-page__title">Settings</h1>
  </header>

  <div class="settings-sections">
    <!-- Appearance: Theme picker -->
    <section class="settings-section" aria-labelledby="section-appearance">
      <h2 class="settings-section__title" id="section-appearance">Appearance</h2>
      <div class="settings-section__body">
        <div class="settings-row settings-row--block">
          <div class="settings-row__label">
            <span class="settings-row__name">Theme</span>
            <span class="settings-row__desc">
              Controls the visual theme and color palette.
            </span>
          </div>
          <div class="theme-grid" role="radiogroup" aria-label="Theme selection">
            {#each themes as t (t.theme)}
              <ThemePreviewCard theme={t.theme} label={t.label} description={t.description} />
            {/each}
          </div>
        </div>

      </div>
    </section>

    <!-- ── Typography ──────────────────────────────────────────────────────────── -->
    <section class="settings-section" aria-labelledby="typography-heading">
      <h2 class="settings-section__title" id="typography-heading">Typography</h2>
      <p class="settings-section__desc">
        Choose fonts for different parts of the interface. Changes apply immediately.
      </p>

      <div class="typography-pickers">
        <FontPicker
          category="body"
          value={fontBody}
          onChange={(v) => handleFontChange('body', v)}
        />
        <FontPicker
          category="display"
          value={fontDisplay}
          onChange={(v) => handleFontChange('display', v)}
        />
        <FontPicker
          category="mono"
          value={fontMono}
          onChange={(v) => handleFontChange('mono', v)}
        />
      </div>
    </section>

    <!-- Reading -->
    <section class="settings-section" aria-labelledby="section-reading">
      <h2 class="settings-section__title" id="section-reading">Reading</h2>
      <div class="settings-section__body">
        <div class="settings-row">
          <div class="settings-row__label">
            <span class="settings-row__name">Reading width</span>
            <span class="settings-row__desc">
              Maximum line length in the editor.
            </span>
          </div>
          <div class="settings-row__control">
            <div class="density-buttons" role="group" aria-label="Reading width">
              {#each readingWidths as rw (rw.value)}
                <button
                  type="button"
                  class="density-btn"
                  class:density-btn--active={$readingStore === rw.value}
                  aria-pressed={$readingStore === rw.value}
                  onclick={() => readingStore.setMeasure(rw.value)}
                >
                  {rw.label}
                </button>
              {/each}
            </div>
          </div>
        </div>

        <div class="settings-row">
          <div class="settings-row__label">
            <span class="settings-row__name">Information density</span>
            <span class="settings-row__desc">
              Spacing between UI elements.
            </span>
          </div>
          <div class="settings-row__control">
            <div class="density-buttons" role="group" aria-label="Information density">
              {#each densities as d (d.value)}
                <button
                  type="button"
                  class="density-btn"
                  class:density-btn--active={$densityStore === d.value}
                  aria-pressed={$densityStore === d.value}
                  onclick={() => densityStore.setDensity(d.value)}
                  title={d.description}
                >
                  {d.label}
                </button>
              {/each}
            </div>
          </div>
        </div>

        <div class="settings-row">
          <div class="settings-row__label">
            <span class="settings-row__name">Font size</span>
            <span class="settings-row__desc">
              Base text size across the UI.
            </span>
          </div>
          <div class="settings-row__control">
            <div class="density-buttons" role="group" aria-label="Font size">
              {#each fontSizes as fs (fs.value)}
                <button
                  type="button"
                  class="density-btn"
                  class:density-btn--active={activeFontSize === fs.value}
                  aria-pressed={activeFontSize === fs.value}
                  onclick={() => applyFontSize(fs.value)}
                >
                  {fs.label}
                </button>
              {/each}
            </div>
          </div>
        </div>
      </div>
    </section>

    <!-- Keyboard shortcuts -->
    <section class="settings-section" aria-labelledby="section-shortcuts">
      <h2 class="settings-section__title" id="section-shortcuts">Keyboard shortcuts</h2>
      {#each shortcutCategories as category}
        <div class="shortcuts-group">
          <h3 class="shortcuts-category">{category}</h3>
          <dl class="shortcuts-list">
            {#each shortcuts.filter((s) => s.category === category) as s (s.key)}
              <div class="shortcut-row">
                <dt class="shortcut-desc">{s.description}</dt>
                <dd class="shortcut-key"><kbd>{s.key}</kbd></dd>
              </div>
            {/each}
          </dl>
        </div>
      {/each}
    </section>

    <!-- AI & Accounts -->
    <section class="settings-section" aria-labelledby="section-ai-accounts">
      <h2 class="settings-section__title" id="section-ai-accounts">AI &amp; Accounts</h2>
      <div class="settings-section__body">

        {#if !aiProvidersLoaded}
          <p class="settings-section__desc">Loading provider information…</p>
        {:else}

          <!-- Default provider -->
          <div class="settings-row">
            <div class="settings-row__label">
              <span class="settings-row__name">Default AI provider</span>
              <span class="settings-row__desc">Used in the New Project wizard when no provider is explicitly selected.</span>
            </div>
            <div class="settings-row__control">
              <select
                class="ai-settings-select"
                value={defaultProvider}
                onchange={(e) => handleDefaultProviderChange((e.target as HTMLSelectElement).value)}
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

          <!-- AlterGo accounts -->
          <div class="settings-row settings-row--block">
            <div class="settings-row__label">
              <span class="settings-row__name">AlterGo accounts</span>
              <span class="settings-row__desc">
                {#if altergoAvailable}
                  Vedox tries accounts in order when a provider hits its usage limit. Drag or use the arrows to reorder.
                {:else}
                  <a href="https://github.com/thepixelabs/altergo" target="_blank" rel="noopener noreferrer" class="ai-settings-link">Install AlterGo</a> to enable multi-account support. With multiple accounts, Vedox automatically falls back to the next account when one hits its usage limit.
                {/if}
              </span>
            </div>

            {#if altergoAvailable}
              {#if altergoAccounts.length === 0}
                <p class="ai-settings-empty">No AlterGo accounts found. Run <code class="ai-settings-code">altergo --setup</code> to create one.</p>
              {:else}
                <ul class="ai-accounts-list" role="list">
                  {#each altergoAccounts as account, i (account.name)}
                    <li class="ai-account-row">
                      <div class="ai-account-row__info">
                        <span class="ai-account-row__name">{account.name}</span>
                        {#if account.providers?.length}
                          <div class="ai-account-row__badges">
                            {#each account.providers as prov}
                              <span class="ai-account-badge">{prov}</span>
                            {/each}
                          </div>
                        {/if}
                      </div>
                      <div class="ai-account-row__actions">
                        <button
                          type="button"
                          class="ai-account-move-btn"
                          onclick={() => moveAccount(i, -1)}
                          disabled={i === 0}
                          aria-label="Move {account.name} up"
                          title="Move up"
                        >↑</button>
                        <button
                          type="button"
                          class="ai-account-move-btn"
                          onclick={() => moveAccount(i, 1)}
                          disabled={i === altergoAccounts.length - 1}
                          aria-label="Move {account.name} down"
                          title="Move down"
                        >↓</button>
                      </div>
                    </li>
                  {/each}
                </ul>
                <p class="ai-settings-note">
                  Account 1 is used first. When it hits its limit, Vedox tries account 2, and so on.
                </p>
              {/if}
            {/if}

          </div>

        {/if}
      </div>
    </section>

    <!-- About -->
    <section class="settings-section" aria-labelledby="section-about">
      <h2 class="settings-section__title" id="section-about">About</h2>
      <div class="settings-section__body">
        <div class="settings-row">
          <div class="settings-row__label">
            <span class="settings-row__name">Vedox</span>
            <span class="settings-row__desc">
              Local-first, Git-native documentation workspace.
            </span>
          </div>
          <div class="settings-row__control">
            <code class="settings-version">v0.1.0</code>
          </div>
        </div>
        <div class="settings-row">
          <div class="settings-row__label">
            <span class="settings-row__name">Network policy</span>
            <span class="settings-row__desc">
              Zero outbound network calls. No telemetry. No version checks.
            </span>
          </div>
          <div class="settings-row__control">
            <span class="settings-badge settings-badge--success">Enforced</span>
          </div>
        </div>
      </div>
    </section>
  </div>
</div>

<style>
  .settings-page {
    padding: var(--space-8);
    max-width: 720px;
  }

  .settings-page__header {
    margin-bottom: var(--space-8);
  }

  .settings-page__title {
    font-size: var(--font-size-xl);
    font-weight: 600;
    color: var(--color-text-primary);
    letter-spacing: -0.02em;
  }

  .settings-sections {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
  }

  .settings-section {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .settings-section__title {
    font-size: var(--font-size-sm);
    font-weight: 600;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--color-text-muted);
    padding-bottom: var(--space-3);
    border-bottom: 1px solid var(--color-border);
  }

  .settings-section__body {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .settings-section__desc {
    font-size: var(--font-size-sm);
    color: var(--color-text-muted);
    line-height: 1.5;
    margin: 0;
  }

  .typography-pickers {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
  }

  .settings-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-6);
    padding: var(--space-3) 0;
    border-bottom: 1px solid var(--color-border);
  }

  .settings-row:last-child {
    border-bottom: none;
  }

  .settings-row--block {
    flex-direction: column;
    align-items: stretch;
    gap: var(--space-4);
  }

  .settings-row__label {
    display: flex;
    flex-direction: column;
    gap: 2px;
    flex: 1;
    min-width: 0;
  }

  .settings-row__name {
    font-size: var(--font-size-sm);
    font-weight: 500;
    color: var(--color-text-primary);
  }

  .settings-row__desc {
    font-size: var(--font-size-sm);
    color: var(--color-text-muted);
    line-height: 1.4;
  }

  .settings-row__control {
    flex-shrink: 0;
  }

  /* Theme grid */
  .theme-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
    gap: var(--space-3);
  }

  /* Density toggle buttons */
  .density-buttons {
    display: flex;
    gap: 2px;
    background-color: var(--color-surface-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: 2px;
  }

  .density-btn {
    padding: var(--space-1) var(--space-3);
    background: none;
    border: none;
    border-radius: calc(var(--radius-md) - 2px);
    color: var(--color-text-muted);
    font-size: var(--font-size-sm);
    font-family: var(--font-sans);
    cursor: pointer;
    transition: background-color 100ms ease, color 100ms ease;
    white-space: nowrap;
  }

  .density-btn:hover {
    color: var(--color-text-primary);
  }

  .density-btn--active {
    background-color: var(--color-surface-base);
    color: var(--color-text-primary);
    font-weight: 500;
    box-shadow: var(--shadow-sm);
  }

  .density-btn:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
  }

  /* Misc controls */
  .settings-version {
    font-family: var(--font-mono);
    font-size: var(--font-size-sm);
    color: var(--color-text-muted);
    background-color: var(--color-surface-elevated);
    padding: var(--space-1) var(--space-2);
    border-radius: var(--radius-sm);
    border: 1px solid var(--color-border);
  }

  .settings-badge {
    display: inline-flex;
    align-items: center;
    padding: 2px var(--space-2);
    font-size: 11px;
    font-weight: 500;
    border-radius: var(--radius-sm);
  }

  .settings-badge--success {
    background-color: var(--color-accent-subtle);
    color: var(--color-success);
  }

  /* AI & Accounts section */
  .ai-settings-select {
    padding: var(--space-2) var(--space-3);
    background: var(--surface-2);
    border: 1px solid var(--border-default);
    border-radius: var(--radius-md);
    color: var(--text-1);
    font-size: var(--font-size-sm);
    font-family: var(--font-sans);
    cursor: pointer;
    min-width: 160px;
  }

  .ai-settings-select:focus {
    outline: none;
    border-color: var(--accent-solid);
    box-shadow: 0 0 0 3px var(--accent-subtle);
  }

  .ai-settings-link {
    color: var(--accent-text);
    text-decoration: underline;
    text-underline-offset: 2px;
  }

  .ai-settings-empty {
    font-size: var(--font-size-sm);
    color: var(--color-text-muted);
    font-style: italic;
  }

  .ai-settings-code {
    font-family: var(--font-mono);
    font-size: 12px;
    background: var(--surface-3);
    padding: 1px 4px;
    border-radius: var(--radius-sm);
    color: var(--text-2);
  }

  .ai-settings-note {
    font-size: var(--font-size-sm);
    color: var(--color-text-muted);
    margin-top: var(--space-2);
  }

  .ai-accounts-list {
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .ai-account-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--space-3) var(--space-4);
    background: var(--surface-2);
    border: 1px solid var(--border-default);
    border-radius: var(--radius-md);
    gap: var(--space-4);
  }

  .ai-account-row__info {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    flex: 1;
    min-width: 0;
  }

  .ai-account-row__name {
    font-size: var(--font-size-sm);
    font-weight: 500;
    color: var(--color-text-primary);
    font-family: var(--font-mono);
  }

  .ai-account-row__badges {
    display: flex;
    gap: var(--space-1);
    flex-wrap: wrap;
  }

  .ai-account-badge {
    font-size: 10px;
    font-weight: 500;
    padding: 1px 6px;
    border-radius: var(--radius-full);
    background: var(--accent-subtle);
    color: var(--accent-text);
    letter-spacing: 0.03em;
    text-transform: capitalize;
  }

  .ai-account-row__actions {
    display: flex;
    gap: var(--space-1);
  }

  .ai-account-move-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 28px;
    height: 28px;
    background: none;
    border: 1px solid var(--border-default);
    border-radius: var(--radius-sm);
    color: var(--text-3);
    font-size: 12px;
    cursor: pointer;
    transition: border-color var(--duration-fast) var(--ease-out), color var(--duration-fast) var(--ease-out);
  }

  .ai-account-move-btn:hover:not(:disabled) {
    border-color: var(--border-strong);
    color: var(--text-1);
  }

  .ai-account-move-btn:disabled {
    opacity: 0.3;
    cursor: not-allowed;
  }

  .ai-account-move-btn:focus-visible {
    outline: 2px solid var(--accent-solid);
    outline-offset: 2px;
  }

  /* Keyboard shortcuts */
  .shortcuts-group {
    margin-bottom: var(--space-4);
  }

  .shortcuts-group:last-child {
    margin-bottom: 0;
  }

  .shortcuts-category {
    font-size: var(--font-size-xs);
    font-weight: 600;
    letter-spacing: 0.04em;
    text-transform: uppercase;
    color: var(--color-text-muted);
    margin-bottom: var(--space-2);
  }

  .shortcuts-list {
    margin: 0;
    padding: 0;
  }

  .shortcut-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--space-2) 0;
    border-bottom: 1px solid var(--color-border);
  }

  .shortcut-row:last-child {
    border-bottom: none;
  }

  .shortcut-desc {
    font-size: var(--font-size-sm);
    color: var(--color-text-primary);
  }

  .shortcut-key {
    margin: 0;
  }

  .shortcut-key kbd {
    display: inline-flex;
    align-items: center;
    padding: 2px 6px;
    border-radius: var(--radius-sm);
    border: 1px solid var(--color-border);
    background: var(--color-surface-elevated);
    color: var(--color-text-muted);
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    line-height: 1.4;
    white-space: nowrap;
    box-shadow: 0 1px 0 var(--color-border);
  }
</style>
