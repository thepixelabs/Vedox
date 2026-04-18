<script lang="ts">
  /**
   * /settings — Vedox personalization hub.
   *
   * Layout: vertical tab rail (left, 200px) + scrollable content area (right).
   * 7 categories: Appearance, Editor, Sidebar, Keyboard, Voice, Agent, Notifications.
   *
   * Search: a filter input at the top of the tab rail scans setting names
   * across ALL categories and shows only matching rows. Each category
   * component receives the query and handles its own filtering.
   *
   * Theme changes apply immediately via the flagship themeStore.
   * Font-size changes apply immediately via CSS custom property.
   *
   * CTO ruling R3: PUT /api/settings = PATCH semantics.
   * Mocked with localStorage via userPrefs store until daemon ships the endpoint.
   */

  import { page } from '$app/stores';
  import { onMount } from 'svelte';
  import { browser } from '$app/environment';

  import AppearanceSettings from '$lib/components/settings/AppearanceSettings.svelte';
  import EditorSettings from '$lib/components/settings/EditorSettings.svelte';
  import SidebarSettings from '$lib/components/settings/SidebarSettings.svelte';
  import KeyboardSettings from '$lib/components/settings/KeyboardSettings.svelte';
  import VoiceSettings from '$lib/components/settings/VoiceSettings.svelte';
  import AgentSettings from '$lib/components/settings/AgentSettings.svelte';
  import NotificationSettings from '$lib/components/settings/NotificationSettings.svelte';

  import { userPrefs } from '$lib/stores/preferences';

  // ---------------------------------------------------------------------------
  // Tab definitions
  // ---------------------------------------------------------------------------

  type TabId = 'appearance' | 'editor' | 'sidebar' | 'keyboard' | 'voice' | 'agent' | 'notifications';

  interface Tab {
    id: TabId;
    label: string;
    description: string;
    /** Flat list of searchable setting names — used for tab-level "has match" hint */
    searchTerms: string[];
  }

  const tabs: Tab[] = [
    {
      id: 'appearance',
      label: 'Appearance',
      description: 'Theme, fonts, density, reading width',
      searchTerms: ['theme', 'font', 'font size', 'line height', 'measure', 'reading width', 'density', 'tree grouping', 'graphite', 'eclipse', 'ember', 'paper', 'solar', 'compact', 'cozy', 'comfortable'],
    },
    {
      id: 'editor',
      label: 'Editor',
      description: 'View mode, auto-save, spell-check',
      searchTerms: ['view', 'split', 'preview', 'source', 'auto-save', 'autosave', 'spell check', 'spellcheck', 'save interval'],
    },
    {
      id: 'sidebar',
      label: 'Sidebar',
      description: 'Position, panels, tree grouping',
      searchTerms: ['sidebar', 'position', 'panel', 'tree', 'collapse', 'grouping', 'type-first', 'folder-first', 'flat'],
    },
    {
      id: 'keyboard',
      label: 'Keyboard',
      description: 'Remappable shortcuts, conflict detection',
      searchTerms: ['shortcut', 'keyboard', 'key', 'remap', 'navigation', 'editor', 'panes', 'view', 'conflict'],
    },
    {
      id: 'voice',
      label: 'Voice',
      description: 'Trigger phrase, mic, push-to-talk',
      searchTerms: ['voice', 'mic', 'microphone', 'trigger', 'phrase', 'push-to-talk', 'ptt', 'wake word'],
    },
    {
      id: 'agent',
      label: 'Agent',
      description: 'Routing repo, dry-run, auto-approve',
      searchTerms: ['agent', 'routing', 'repo', 'dry-run', 'auto-approve', 'provider', 'altergo', 'account'],
    },
    {
      id: 'notifications',
      label: 'Notifications',
      description: 'Toast duration, sound, badge',
      searchTerms: ['notification', 'toast', 'duration', 'sound', 'badge', 'dismiss'],
    },
  ];

  // ---------------------------------------------------------------------------
  // Active tab — URL hash drives it so link-sharing works
  // ---------------------------------------------------------------------------

  let activeTab = $state<TabId>('appearance');

  onMount(() => {
    const hash = window.location.hash.replace('#', '') as TabId;
    if (tabs.some((t) => t.id === hash)) activeTab = hash;

    // Restore font-size from localStorage (done here since the old settings
    // page handled this; the AppearanceSettings component also does it but
    // the page mount guarantees it regardless of which tab is first active).
    const storedSize = localStorage.getItem('vedox:font-size');
    if (storedSize) {
      document.documentElement.style.setProperty('--font-size-override', storedSize);
    }
  });

  function setTab(id: TabId) {
    activeTab = id;
    if (browser) {
      history.replaceState(null, '', `#${id}`);
    }
  }

  // ---------------------------------------------------------------------------
  // Global search across all categories
  // ---------------------------------------------------------------------------

  let searchQuery = $state('');
  let searchInputEl: HTMLInputElement | undefined;
  let searchActive = $derived(searchQuery.trim().length > 0);

  // When search is active, show ALL tabs' content simultaneously, filtered.
  // When search is empty, show only the active tab's content.

  function tabHasMatch(tab: Tab): boolean {
    if (!searchActive) return true;
    const q = searchQuery.toLowerCase();
    return tab.searchTerms.some((t) => t.includes(q)) || tab.label.toLowerCase().includes(q);
  }

  // Quick keyboard: '/' to focus search
  function handlePageKeydown(e: KeyboardEvent) {
    if (
      e.key === '/' &&
      document.activeElement !== searchInputEl &&
      !(document.activeElement instanceof HTMLInputElement) &&
      !(document.activeElement instanceof HTMLTextAreaElement)
    ) {
      e.preventDefault();
      searchInputEl?.focus();
    }
    if (e.key === 'Escape' && searchActive) {
      searchQuery = '';
      searchInputEl?.blur();
    }
  }

  const activeTabMeta = $derived(tabs.find((t) => t.id === activeTab)!);
</script>

<svelte:head>
  <title>Settings — Vedox</title>
</svelte:head>

<svelte:window onkeydown={handlePageKeydown} />

<div class="settings-shell">
  <!-- ── Left rail: tab navigation ── -->
  <nav class="settings-nav" aria-label="Settings categories">
    <!-- Search -->
    <div class="settings-nav__search">
      <label class="settings-nav__search-label" for="settings-search">
        <svg
          class="settings-nav__search-icon"
          width="12"
          height="12"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
          aria-hidden="true"
        >
          <circle cx="11" cy="11" r="8"/>
          <line x1="21" y1="21" x2="16.65" y2="16.65"/>
        </svg>
      </label>
      <input
        id="settings-search"
        type="search"
        class="settings-nav__search-input"
        bind:this={searchInputEl}
        bind:value={searchQuery}
        placeholder="Search settings…"
        aria-label="Search settings"
        autocomplete="off"
        spellcheck="false"
      />
      {#if searchActive}
        <button
          type="button"
          class="settings-nav__search-clear"
          onclick={() => { searchQuery = ''; searchInputEl?.focus(); }}
          aria-label="Clear search"
        >
          <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" aria-hidden="true">
            <line x1="18" y1="6" x2="6" y2="18"/>
            <line x1="6" y1="6" x2="18" y2="18"/>
          </svg>
        </button>
      {/if}
    </div>

    <!-- Tab list -->
    <ul class="settings-nav__list" role="tablist" aria-orientation="vertical">
      {#each tabs as tab (tab.id)}
        {@const hasMatch = tabHasMatch(tab)}
        <li role="presentation">
          <button
            type="button"
            role="tab"
            id="tab-{tab.id}"
            class="settings-nav__tab"
            class:settings-nav__tab--active={activeTab === tab.id && !searchActive}
            class:settings-nav__tab--no-match={searchActive && !hasMatch}
            aria-selected={activeTab === tab.id && !searchActive}
            aria-controls="panel-{tab.id}"
            tabindex={activeTab === tab.id ? 0 : -1}
            onclick={() => { setTab(tab.id); searchQuery = ''; }}
            onkeydown={(e) => {
              const idx = tabs.findIndex((t) => t.id === tab.id);
              if (e.key === 'ArrowDown') {
                e.preventDefault();
                const next = tabs[(idx + 1) % tabs.length];
                setTab(next.id);
                searchQuery = '';
              } else if (e.key === 'ArrowUp') {
                e.preventDefault();
                const prev = tabs[(idx - 1 + tabs.length) % tabs.length];
                setTab(prev.id);
                searchQuery = '';
              }
            }}
          >
            <span class="settings-nav__tab-label">{tab.label}</span>
            <span class="settings-nav__tab-desc">{tab.description}</span>
          </button>
        </li>
      {/each}
    </ul>

    <!-- Footer: onboarding + reset all -->
    <div class="settings-nav__footer">
      <a href="/onboarding" class="settings-nav__onboarding-link">
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
          <polyline points="23 4 23 10 17 10"/>
          <path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/>
        </svg>
        Re-run onboarding
      </a>
      <button
        type="button"
        class="settings-nav__reset-btn"
        onclick={() => {
          if (confirm('Reset all settings to defaults? This cannot be undone.')) {
            userPrefs.reset();
          }
        }}
      >Reset to defaults</button>
    </div>
  </nav>

  <!-- ── Right panel: content ── -->
  <div class="settings-content" role="main">
    {#if searchActive}
      <!-- Search mode: show all categories with matching content -->
      <header class="settings-content__header">
        <h1 class="settings-content__title">Search results</h1>
        <p class="settings-content__subtitle">
          Showing settings matching "<strong>{searchQuery}</strong>"
        </p>
      </header>

      {#each tabs as tab (tab.id)}
        {#if tabHasMatch(tab)}
          <section
            class="settings-section"
            aria-labelledby="search-section-{tab.id}"
          >
            <h2 class="settings-section__title" id="search-section-{tab.id}">{tab.label}</h2>
            {#if tab.id === 'appearance'}
              <AppearanceSettings searchQuery={searchQuery} />
            {:else if tab.id === 'editor'}
              <EditorSettings searchQuery={searchQuery} />
            {:else if tab.id === 'sidebar'}
              <SidebarSettings searchQuery={searchQuery} />
            {:else if tab.id === 'keyboard'}
              <KeyboardSettings searchQuery={searchQuery} />
            {:else if tab.id === 'voice'}
              <VoiceSettings searchQuery={searchQuery} />
            {:else if tab.id === 'agent'}
              <AgentSettings searchQuery={searchQuery} />
            {:else if tab.id === 'notifications'}
              <NotificationSettings searchQuery={searchQuery} />
            {/if}
          </section>
        {/if}
      {/each}

    {:else}
      <!-- Normal mode: show active tab only -->
      <header class="settings-content__header">
        <h1 class="settings-content__title">{activeTabMeta.label}</h1>
        <p class="settings-content__subtitle">{activeTabMeta.description}</p>
      </header>

      <div
        id="panel-{activeTab}"
        role="tabpanel"
        aria-labelledby="tab-{activeTab}"
      >
        {#if activeTab === 'appearance'}
          <AppearanceSettings />
        {:else if activeTab === 'editor'}
          <EditorSettings />
        {:else if activeTab === 'sidebar'}
          <SidebarSettings />
        {:else if activeTab === 'keyboard'}
          <KeyboardSettings />
        {:else if activeTab === 'voice'}
          <VoiceSettings />
        {:else if activeTab === 'agent'}
          <AgentSettings />
        {:else if activeTab === 'notifications'}
          <NotificationSettings />
        {/if}
      </div>
    {/if}

    <!-- About (always visible at bottom of content area) -->
    <footer class="settings-about">
      <div class="settings-about__row">
        <span class="settings-about__name">Vedox</span>
        <code class="settings-about__version">v0.1.0</code>
      </div>
      <div class="settings-about__row">
        <span class="settings-about__desc">Local-first, Git-native documentation workspace. Zero outbound network calls.</span>
        <span class="settings-about__badge">No telemetry</span>
      </div>
    </footer>
  </div>
</div>

<style>
  /* ── Shell layout ────────────────────────────────────────────────────────── */

  .settings-shell {
    display: grid;
    grid-template-columns: 200px 1fr;
    height: 100%;
    overflow: hidden;
  }

  /* ── Left nav rail ───────────────────────────────────────────────────────── */

  .settings-nav {
    display: flex;
    flex-direction: column;
    height: 100%;
    border-right: 1px solid var(--color-border);
    background-color: var(--color-surface-base);
    overflow: hidden;
  }

  .settings-nav__search {
    position: relative;
    display: flex;
    align-items: center;
    padding: var(--space-3) var(--space-3);
    border-bottom: 1px solid var(--color-border);
    gap: var(--space-2);
    flex-shrink: 0;
  }

  .settings-nav__search-label {
    display: flex;
    align-items: center;
    color: var(--color-text-muted);
    flex-shrink: 0;
    cursor: default;
  }

  .settings-nav__search-icon {
    flex-shrink: 0;
  }

  .settings-nav__search-input {
    flex: 1;
    min-width: 0;
    border: none;
    background: none;
    color: var(--color-text-primary);
    font-size: var(--font-size-sm);
    font-family: var(--font-sans);
    outline: none;
    line-height: 1.4;
  }

  .settings-nav__search-input::placeholder {
    color: var(--color-text-muted);
  }

  /* Hide browser's default "x" on search inputs */
  .settings-nav__search-input::-webkit-search-cancel-button {
    display: none;
  }

  .settings-nav__search-clear {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 18px;
    height: 18px;
    background: var(--color-surface-elevated);
    border: none;
    border-radius: 50%;
    color: var(--color-text-muted);
    cursor: pointer;
    flex-shrink: 0;
    transition: color 80ms ease;
  }

  .settings-nav__search-clear:hover {
    color: var(--color-text-primary);
  }

  .settings-nav__search-clear:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
  }

  .settings-nav__list {
    list-style: none;
    margin: 0;
    padding: var(--space-2) 0;
    flex: 1;
    overflow-y: auto;
    overflow-x: hidden;
  }

  .settings-nav__tab {
    width: 100%;
    display: flex;
    flex-direction: column;
    gap: 1px;
    padding: var(--space-2) var(--space-3);
    background: none;
    border: none;
    border-radius: var(--radius-sm);
    margin: 1px var(--space-1);
    width: calc(100% - 8px);
    text-align: left;
    cursor: pointer;
    transition: background-color 80ms ease;
    color: var(--color-text-secondary);
  }

  .settings-nav__tab:hover {
    background-color: var(--color-surface-overlay);
  }

  .settings-nav__tab--active {
    background-color: var(--color-surface-overlay);
    color: var(--color-text-primary);
    box-shadow: inset 2px 0 0 var(--color-accent);
  }

  .settings-nav__tab--no-match {
    opacity: 0.35;
  }

  .settings-nav__tab:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: -2px;
  }

  .settings-nav__tab-label {
    font-size: var(--font-size-sm);
    font-weight: 500;
    color: inherit;
    line-height: 1.3;
  }

  .settings-nav__tab--active .settings-nav__tab-label {
    color: var(--color-text-primary);
  }

  .settings-nav__tab-desc {
    font-size: 11px;
    color: var(--color-text-muted);
    line-height: 1.3;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .settings-nav__footer {
    flex-shrink: 0;
    padding: var(--space-3);
    border-top: 1px solid var(--color-border);
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .settings-nav__onboarding-link {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-3);
    font-size: var(--font-size-xs);
    color: var(--color-text-muted);
    text-decoration: none;
    border-radius: var(--radius-md);
    border: 1px solid var(--color-border);
    transition: color 80ms ease, border-color 80ms ease;
    font-family: var(--font-sans);
  }

  .settings-nav__onboarding-link:hover {
    color: var(--color-text-primary);
    border-color: var(--color-text-muted);
  }

  .settings-nav__onboarding-link:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
  }

  .settings-nav__reset-btn {
    width: 100%;
    padding: var(--space-2) var(--space-3);
    font-size: var(--font-size-xs);
    color: var(--color-text-muted);
    background: none;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    cursor: pointer;
    font-family: var(--font-sans);
    text-align: center;
    transition: color 80ms ease, border-color 80ms ease;
  }

  .settings-nav__reset-btn:hover {
    color: var(--color-text-primary);
    border-color: var(--color-text-muted);
  }

  .settings-nav__reset-btn:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
  }

  /* ── Right content area ──────────────────────────────────────────────────── */

  .settings-content {
    display: flex;
    flex-direction: column;
    height: 100%;
    overflow-y: auto;
    overflow-x: hidden;
    padding: var(--space-8) var(--space-8);
    max-width: 640px;
    gap: var(--space-8);
  }

  .settings-content__header {
    flex-shrink: 0;
  }

  .settings-content__title {
    font-size: var(--font-size-lg);
    font-weight: 600;
    color: var(--color-text-primary);
    letter-spacing: -0.02em;
    margin: 0 0 var(--space-1);
  }

  .settings-content__subtitle {
    font-size: var(--font-size-sm);
    color: var(--color-text-muted);
    margin: 0;
    line-height: 1.4;
  }

  .settings-content__subtitle strong {
    color: var(--color-text-primary);
    font-weight: 500;
  }

  /* ── Section header (search mode) ─────────────────────────────────────── */

  .settings-section {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .settings-section__title {
    font-size: var(--font-size-xs);
    font-weight: 600;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--color-text-muted);
    padding-bottom: var(--space-2);
    border-bottom: 1px solid var(--color-border);
  }

  /* ── About footer ────────────────────────────────────────────────────────── */

  .settings-about {
    margin-top: auto;
    padding-top: var(--space-8);
    border-top: 1px solid var(--color-border);
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .settings-about__row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-4);
  }

  .settings-about__name {
    font-size: var(--font-size-sm);
    font-weight: 500;
    color: var(--color-text-primary);
    font-family: var(--font-mono);
  }

  .settings-about__version {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-text-muted);
    background-color: var(--color-surface-elevated);
    padding: 2px var(--space-2);
    border-radius: var(--radius-sm);
    border: 1px solid var(--color-border);
  }

  .settings-about__desc {
    font-size: var(--font-size-xs);
    color: var(--color-text-muted);
    line-height: 1.4;
  }

  .settings-about__badge {
    flex-shrink: 0;
    display: inline-flex;
    align-items: center;
    padding: 2px var(--space-2);
    font-size: 11px;
    font-weight: 500;
    border-radius: var(--radius-sm);
    background-color: var(--color-accent-subtle);
    color: var(--color-accent);
    white-space: nowrap;
  }

  /* ── Responsive: narrow viewports collapse rail ─────────────────────────── */

  @media (max-width: 640px) {
    .settings-shell {
      grid-template-columns: 1fr;
      grid-template-rows: auto 1fr;
    }

    .settings-nav {
      border-right: none;
      border-bottom: 1px solid var(--color-border);
      height: auto;
    }

    .settings-nav__list {
      display: flex;
      flex-direction: row;
      overflow-x: auto;
      padding: var(--space-1) var(--space-2);
    }

    .settings-nav__tab {
      width: auto;
      min-width: max-content;
      margin: 0 2px;
      box-shadow: none;
    }

    .settings-nav__tab--active {
      box-shadow: none;
      border-bottom: 2px solid var(--color-accent);
      border-radius: 0;
    }

    .settings-nav__tab-desc {
      display: none;
    }

    .settings-nav__footer {
      display: none;
    }

    .settings-content {
      padding: var(--space-4);
    }
  }

  /* ── Reduced motion ─────────────────────────────────────────────────────── */

  @media (prefers-reduced-motion: reduce) {
    .settings-nav__tab,
    .settings-nav__reset-btn {
      transition: none;
    }
  }
</style>
