<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { api, type GenerationParams, type RefinementInput, type ProviderInfo, type AltergoAccount } from '$lib/api/client';
  import type { WizardAiPanelDraft } from '$lib/stores/wizardDraft';

  interface Props {
    /** Called when the user selects a name — auto-fills the project name input */
    onNameSelected: (name: string) => void;
    /** Seed state from a previously saved draft (optional). */
    initialState?: Partial<WizardAiPanelDraft>;
    /** Called after any meaningful state change so the parent can persist it. */
    onStateChange?: (state: WizardAiPanelDraft) => void;
  }

  let { onNameSelected, initialState, onStateChange }: Props = $props();

  // ── Provider discovery ──────────────────────────────────────────────────────
  let providers = $state<ProviderInfo[]>([]);
  let altergoAccounts = $state<AltergoAccount[]>([]);
  let altergoAvailable = $state(false);

  // Helper: snapshot current state for persistence.
  function currentSnapshot(): WizardAiPanelDraft {
    return {
      open: false, // managed by the parent <details> element
      selectedCategories: Array.from(selectedCategories),
      selectedPlatform,
      selectedOS,
      selectedInterface,
      selectedAudience,
      selectedTone,
      selectedLength,
      selectedLanguageStyle,
      selectedProvider,
      selectedAccount,
      nameCount,
      generatedNames,
      selectedName,
      phase,
    };
  }

  function notifyStateChange() {
    onStateChange?.(currentSnapshot());
  }

  onMount(async () => {
    // Restore from draft before fetching providers so UI reflects prior state.
    if (initialState) {
      if (initialState.selectedCategories) {
        selectedCategories = new Set(initialState.selectedCategories);
      }
      if (initialState.selectedPlatform) selectedPlatform = initialState.selectedPlatform;
      if (initialState.selectedOS) selectedOS = initialState.selectedOS;
      if (initialState.selectedInterface) selectedInterface = initialState.selectedInterface;
      if (initialState.selectedAudience) selectedAudience = initialState.selectedAudience;
      if (initialState.selectedTone) selectedTone = initialState.selectedTone;
      if (initialState.selectedLength) selectedLength = initialState.selectedLength;
      if (initialState.selectedLanguageStyle) selectedLanguageStyle = initialState.selectedLanguageStyle;
      if (initialState.selectedAccount) selectedAccount = initialState.selectedAccount;
      if (initialState.nameCount) nameCount = initialState.nameCount;
      if (initialState.generatedNames) generatedNames = initialState.generatedNames;
      if (initialState.selectedName) selectedName = initialState.selectedName;
      if (initialState.phase && initialState.phase !== 'loading') phase = initialState.phase;
    }

    try {
      const response = await api.getAIProviders();
      providers = response.providers;
      altergoAccounts = response.altergo.accounts;
      altergoAvailable = response.altergo.available;
      // Restore saved provider, or auto-select first available.
      if (initialState?.selectedProvider && providers.some(p => p.id === initialState!.selectedProvider && p.available)) {
        selectedProvider = initialState.selectedProvider;
      } else {
        const firstAvailable = providers.find(p => p.available);
        if (firstAvailable) selectedProvider = firstAvailable.id;
      }
    } catch {
      // If providers can't be fetched, show offline state.
    }
  });

  // ── Category selection (multi-select) ──────────────────────────────────────
  const CATEGORIES = [
    { id: 'technology', label: 'Technology', icon: '💻' },
    { id: 'business', label: 'Business', icon: '💼' },
    { id: 'creative', label: 'Creative/Art', icon: '🎨' },
    { id: 'gaming', label: 'Gaming', icon: '🎮' },
    { id: 'health', label: 'Health/Wellness', icon: '❤️' },
    { id: 'education', label: 'Education', icon: '🎓' },
    { id: 'finance', label: 'Finance', icon: '💰' },
    { id: 'entertainment', label: 'Entertainment', icon: '🎬' },
    { id: 'lifestyle', label: 'Lifestyle', icon: '☕' },
    { id: 'social', label: 'Social/Community', icon: '👥' },
    { id: 'science', label: 'Science/Research', icon: '🔬' },
    { id: 'other', label: 'Other', icon: '⋯' },
  ];

  let selectedCategories = $state<Set<string>>(new Set());

  function toggleCategory(id: string) {
    const next = new Set(selectedCategories);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    selectedCategories = next;
    notifyStateChange();
  }

  // ── Configuration ───────────────────────────────────────────────────────────
  let selectedPlatform = $state('any');
  let selectedOS = $state('any');
  let selectedInterface = $state('any');
  let selectedAudience = $state('general');
  let selectedTone = $state('professional');
  let selectedLength = $state('medium');
  let selectedLanguageStyle = $state('modern');

  // ── Provider & count ────────────────────────────────────────────────────────
  let selectedProvider = $state('');
  let selectedAccount = $state('');
  let nameCount = $state(12);

  // ── Generation state ────────────────────────────────────────────────────────
  type GeneratePhase = 'idle' | 'loading' | 'done' | 'error';
  let phase = $state<GeneratePhase>('idle');
  let generatedNames = $state<string[]>([]);
  let selectedName = $state('');
  let errorMessage = $state('');
  let pollTimer: ReturnType<typeof setInterval> | null = null;
  let showSpinner = $state(false);
  let spinnerTimer: ReturnType<typeof setTimeout> | null = null;

  function stopPolling() {
    if (pollTimer !== null) {
      clearInterval(pollTimer);
      pollTimer = null;
    }
  }

  onDestroy(() => {
    stopPolling();
    if (spinnerTimer) clearTimeout(spinnerTimer);
  });

  async function pollJob(jobId: string) {
    try {
      const job = await api.getGenerationJob(jobId);
      if (job.status === 'done') {
        stopPolling();
        showSpinner = false;
        generatedNames = job.names ?? [];
        phase = 'done';
        notifyStateChange();
      } else if (job.status === 'error') {
        stopPolling();
        showSpinner = false;
        errorMessage = job.error ?? 'Generation failed';
        phase = 'error';
        notifyStateChange();
      }
    } catch {
      stopPolling();
      showSpinner = false;
      errorMessage = 'Failed to poll job status';
      phase = 'error';
    }
  }

  async function handleGenerate(refinement?: RefinementInput) {
    if (!selectedProvider) return;

    phase = 'loading';
    showSpinner = false;
    errorMessage = '';

    // Show spinner after 240ms delay (avoids flash for fast responses)
    spinnerTimer = setTimeout(() => { showSpinner = true; }, 240);

    const params: GenerationParams = {
      categories: Array.from(selectedCategories),
      platform: selectedPlatform,
      os: selectedOS,
      interface: selectedInterface,
      audience: selectedAudience,
      tone: selectedTone,
      nameLength: selectedLength,
      languageStyle: selectedLanguageStyle,
    };

    try {
      const { jobId } = await api.generateNames({
        provider: selectedProvider,
        account: selectedAccount,
        params,
        count: nameCount,
        refinement: refinement ?? null,
      });

      pollTimer = setInterval(() => pollJob(jobId), 2000);
      await pollJob(jobId);
    } catch (err) {
      if (spinnerTimer) clearTimeout(spinnerTimer);
      showSpinner = false;
      errorMessage = err instanceof Error ? err.message : 'Generation failed';
      phase = 'error';
      notifyStateChange();
    }
  }

  function handleRefineExact() {
    if (!selectedName) return;
    handleGenerate({ mode: 'exact', likedNames: [selectedName] });
  }

  function handleRefineStyle() {
    if (!selectedName) return;
    handleGenerate({ mode: 'style', likedNames: [selectedName] });
  }

  function handleSelectName(name: string) {
    selectedName = name;
    onNameSelected(name);
    notifyStateChange();
  }

  const hasAvailableProviders = $derived(providers.some(p => p.available));
  const noProvidersInstalled = $derived(providers.length > 0 && !hasAvailableProviders);
  const isGenerating = $derived(phase === 'loading');
</script>

<div class="ai-panel">
  <!-- Section 1: Categories -->
  <div class="ai-panel__section">
    <p class="ai-panel__section-label">Category</p>
    <div class="ai-category-grid" role="group" aria-label="Project categories">
      {#each CATEGORIES as cat (cat.id)}
        {@const isSelected = selectedCategories.has(cat.id)}
        <button
          type="button"
          class="ai-category-chip"
          class:ai-category-chip--selected={isSelected}
          role="checkbox"
          aria-checked={isSelected}
          onclick={() => toggleCategory(cat.id)}
        >
          <span class="ai-category-chip__icon" aria-hidden="true">{cat.icon}</span>
          {cat.label}
        </button>
      {/each}
    </div>
  </div>

  <!-- Section 2: Configuration -->
  <div class="ai-panel__section ai-config-grid">
    <div class="ai-config-row">
      <label class="ai-config-label" for="ai-platform">Platform</label>
      <select id="ai-platform" class="ai-select" bind:value={selectedPlatform}>
        <option value="any">Any</option>
        <option value="mobile">Mobile (iOS/Android)</option>
        <option value="web">Web App</option>
        <option value="desktop">Desktop</option>
        <option value="cross-platform">Cross-platform</option>
      </select>
    </div>
    <div class="ai-config-row">
      <label class="ai-config-label" for="ai-audience">Audience</label>
      <select id="ai-audience" class="ai-select" bind:value={selectedAudience}>
        <option value="general">General</option>
        <option value="developers">Developers</option>
        <option value="professionals">Professionals</option>
        <option value="creatives">Creatives</option>
        <option value="children">Children/Teens</option>
      </select>
    </div>
    <div class="ai-config-row">
      <label class="ai-config-label" for="ai-os">Operating System</label>
      <select id="ai-os" class="ai-select" bind:value={selectedOS}>
        <option value="any">Any</option>
        <option value="macos">macOS</option>
        <option value="windows">Windows</option>
        <option value="linux">Linux</option>
        <option value="ios">iOS</option>
        <option value="android">Android</option>
      </select>
    </div>
    <div class="ai-config-row">
      <label class="ai-config-label" for="ai-tone">Tone</label>
      <select id="ai-tone" class="ai-select" bind:value={selectedTone}>
        <option value="professional">Professional</option>
        <option value="playful">Playful</option>
        <option value="serious">Serious</option>
        <option value="quirky">Quirky</option>
        <option value="minimal">Minimal</option>
      </select>
    </div>
    <div class="ai-config-row">
      <label class="ai-config-label" for="ai-interface">Interface Type</label>
      <select id="ai-interface" class="ai-select" bind:value={selectedInterface}>
        <option value="any">Any</option>
        <option value="gui">GUI App</option>
        <option value="cli">CLI Tool</option>
        <option value="web">Web Service</option>
        <option value="api">API / SDK</option>
        <option value="hybrid">Hybrid</option>
      </select>
    </div>
    <div class="ai-config-row">
      <label class="ai-config-label">Name Length</label>
      <div class="ai-length-group" role="radiogroup" aria-label="Name length preference">
        {#each [['short', 'Short'], ['medium', 'Medium'], ['descriptive', 'Descriptive']] as [val, lbl]}
          <label class="ai-length-option">
            <input type="radio" name="ai-length" value={val} bind:group={selectedLength} />
            {lbl}
          </label>
        {/each}
      </div>
    </div>
  </div>

  <!-- Section 3: Provider + count -->
  <div class="ai-panel__section ai-provider-row">
    <div class="ai-provider-col">
      <label class="ai-config-label" for="ai-provider">AI Provider</label>
      <select id="ai-provider" class="ai-select" bind:value={selectedProvider} disabled={!hasAvailableProviders}>
        {#if providers.length === 0}
          <option value="">Loading providers…</option>
        {:else if !hasAvailableProviders}
          <option value="">No CLI found</option>
        {:else}
          {#each providers as provider}
            <option value={provider.id} disabled={!provider.available}>
              {provider.name}{provider.available ? '' : ' (not installed)'}
            </option>
          {/each}
        {/if}
      </select>
    </div>

    {#if altergoAvailable && altergoAccounts.length > 0}
      <div class="ai-provider-col">
        <label class="ai-config-label" for="ai-account">Account</label>
        <select id="ai-account" class="ai-select" bind:value={selectedAccount}>
          <option value="">Direct CLI</option>
          {#each altergoAccounts as account}
            <option value={account.name}>{account.name}</option>
          {/each}
        </select>
      </div>
    {/if}

    <div class="ai-count-col">
      <label class="ai-config-label" for="ai-count">
        Names to generate: <span class="ai-count-value">{nameCount}</span>
      </label>
      <input
        type="range"
        id="ai-count"
        class="ai-count-slider"
        min="5"
        max="30"
        step="1"
        bind:value={nameCount}
      />
    </div>
  </div>

  {#if noProvidersInstalled}
    <div class="ai-panel__no-cli" role="alert">
      <p>No AI CLI found. Install <a href="https://claude.ai/claude-code" target="_blank" rel="noopener noreferrer">Claude Code</a>, <a href="https://gemini.google.com/cli" target="_blank" rel="noopener noreferrer">Gemini CLI</a>, or another supported provider to use AI name generation.</p>
      <p class="ai-panel__no-cli-note">Vedox itself makes no network calls — name generation runs through your locally installed CLI.</p>
    </div>
  {:else}
    <button
      type="button"
      class="ai-generate-btn"
      onclick={() => handleGenerate()}
      disabled={phase === 'loading' || !selectedProvider}
      aria-busy={phase === 'loading'}
    >
      {#if phase === 'loading' && showSpinner}
        <span class="ai-generate-btn__spinner" aria-hidden="true"></span>
        Generating…
      {:else}
        ✦ Generate Names
      {/if}
    </button>
    <p class="ai-panel__disclaimer">
      Names generated by your locally installed {providers.find(p => p.id === selectedProvider)?.name ?? 'AI CLI'}. Vedox makes no network calls.
    </p>
  {/if}

  <!-- Section 4: Results -->
  {#if phase === 'error'}
    <div class="ai-results-error" role="alert">{errorMessage}</div>
  {/if}

  {#if phase === 'done' && generatedNames.length > 0}
    <div class="ai-results-section">
      <p class="ai-panel__section-label">Results ({generatedNames.length})</p>
      <div class="ai-results" role="radiogroup" aria-label="Generated name suggestions">
        {#each generatedNames as name (name)}
          <label class="ai-result-card" class:ai-result-card--selected={selectedName === name}>
            <input
              type="radio"
              name="ai-result"
              value={name}
              class="ai-result-radio"
              checked={selectedName === name}
              onchange={() => handleSelectName(name)}
            />
            <span class="ai-result-card__name">{name}</span>
          </label>
        {/each}
      </div>
      <div role="status" aria-live="polite" class="sr-only">
        {generatedNames.length} name suggestions generated
      </div>

      <div class="ai-refine-row">
        <button
          type="button"
          class="ai-refine-btn"
          onclick={handleRefineExact}
          disabled={!selectedName || isGenerating}
          title={selectedName ? `Generate variations of "${selectedName}"` : 'Select a name first'}
        >
          Refine Exact
        </button>
        <button
          type="button"
          class="ai-refine-btn"
          onclick={handleRefineStyle}
          disabled={!selectedName || isGenerating}
          title={selectedName ? `Generate new names in the style of "${selectedName}"` : 'Select a name first'}
        >
          Refine Style
        </button>
      </div>
    </div>
  {/if}
</div>

<style>
  .ai-panel {
    display: flex;
    flex-direction: column;
    gap: var(--space-5);
    padding-top: var(--space-5);
  }

  .ai-panel__section {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .ai-panel__section-label {
    font-size: var(--text-caption, 11px);
    font-weight: 600;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--text-3);
  }

  /* Category chips */
  .ai-category-grid {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
  }

  .ai-category-chip {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    padding: var(--space-1) var(--space-3);
    border: 1px solid var(--border-default);
    border-radius: var(--radius-full);
    background: var(--surface-2);
    color: var(--text-2);
    font-size: var(--text-sm);
    font-weight: 500;
    font-family: var(--font-body);
    cursor: pointer;
    transition:
      border-color var(--duration-fast) var(--ease-out),
      background var(--duration-fast) var(--ease-out),
      color var(--duration-fast) var(--ease-out);
  }

  .ai-category-chip:hover {
    border-color: var(--border-strong);
    color: var(--text-1);
  }

  .ai-category-chip--selected {
    border-color: var(--accent-border);
    background: var(--accent-subtle);
    color: var(--accent-text);
  }

  .ai-category-chip:focus-visible {
    outline: 2px solid var(--accent-solid);
    outline-offset: 2px;
  }

  .ai-category-chip__icon {
    font-size: 14px;
    line-height: 1;
  }

  /* Config grid */
  .ai-config-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: var(--space-3) var(--space-5);
  }

  @media (max-width: 600px) {
    .ai-config-grid {
      grid-template-columns: 1fr;
    }
  }

  .ai-config-row {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .ai-config-label {
    font-size: var(--text-caption, 11px);
    font-weight: 600;
    color: var(--text-3);
    letter-spacing: 0.04em;
    text-transform: uppercase;
  }

  .ai-select {
    padding: var(--space-2) var(--space-3);
    background: var(--surface-2);
    border: 1px solid var(--border-default);
    border-radius: var(--radius-md);
    color: var(--text-1);
    font-size: var(--text-sm);
    font-family: var(--font-body);
    cursor: pointer;
    transition: border-color var(--duration-fast) var(--ease-out);
    appearance: none;
    background-image: url("data:image/svg+xml,%3Csvg width='12' height='12' viewBox='0 0 24 24' fill='none' stroke='%23999' stroke-width='2' xmlns='http://www.w3.org/2000/svg'%3E%3Cpolyline points='6 9 12 15 18 9'/%3E%3C/svg%3E");
    background-repeat: no-repeat;
    background-position: right var(--space-3) center;
    padding-right: var(--space-7);
  }

  .ai-select:focus {
    outline: none;
    border-color: var(--accent-solid);
    box-shadow: 0 0 0 3px var(--accent-subtle);
  }

  .ai-select:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  /* Length radio group */
  .ai-length-group {
    display: flex;
    gap: var(--space-3);
    align-items: center;
    flex-wrap: wrap;
    padding-top: var(--space-1);
  }

  .ai-length-option {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    font-size: var(--text-sm);
    color: var(--text-2);
    cursor: pointer;
  }

  .ai-length-option input[type="radio"] {
    accent-color: var(--accent-solid);
    cursor: pointer;
  }

  /* Provider row */
  .ai-provider-row {
    display: flex;
    flex-wrap: wrap;
    align-items: flex-end;
    gap: var(--space-4);
  }

  .ai-provider-col {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    min-width: 140px;
  }

  .ai-count-col {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    flex: 1;
    min-width: 180px;
  }

  .ai-count-value {
    font-family: var(--font-mono);
    font-variant-numeric: tabular-nums;
    color: var(--text-1);
  }

  .ai-count-slider {
    width: 100%;
    height: 4px;
    border-radius: var(--radius-full);
    background: var(--border-default);
    accent-color: var(--accent-solid);
    cursor: pointer;
    -webkit-appearance: none;
    appearance: none;
  }

  /* Generate button */
  .ai-generate-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-2);
    width: 100%;
    padding: var(--space-3) var(--space-6);
    background: var(--accent-solid);
    color: var(--accent-contrast);
    font-size: var(--text-ui, 13px);
    font-weight: 600;
    font-family: var(--font-body);
    letter-spacing: 0.04em;
    border: none;
    border-radius: var(--radius-lg);
    cursor: pointer;
    transition:
      background-color var(--duration-fast) var(--ease-out),
      transform var(--duration-fast) var(--ease-snap),
      box-shadow var(--duration-fast) var(--ease-out);
  }

  .ai-generate-btn:hover:not(:disabled) {
    background: var(--accent-solid-hover);
    box-shadow: 0 4px 16px var(--accent-subtle);
    transform: translateY(-1px);
  }

  .ai-generate-btn:active:not(:disabled) {
    transform: translateY(0);
  }

  .ai-generate-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .ai-generate-btn__spinner {
    width: 14px;
    height: 14px;
    border: 2px solid currentColor;
    border-top-color: transparent;
    border-radius: 50%;
    animation: spin 600ms linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  .ai-panel__disclaimer {
    font-size: var(--text-caption, 11px);
    color: var(--text-4);
    text-align: center;
  }

  .ai-panel__no-cli {
    padding: var(--space-4);
    background: var(--surface-2);
    border: 1px solid var(--border-default);
    border-radius: var(--radius-lg);
    font-size: var(--text-sm);
    color: var(--text-2);
    line-height: var(--leading-normal, 1.55);
  }

  .ai-panel__no-cli a {
    color: var(--accent-text);
    text-decoration: underline;
    text-underline-offset: 2px;
  }

  .ai-panel__no-cli-note {
    margin-top: var(--space-2);
    color: var(--text-3);
  }

  /* Results */
  .ai-results-section {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .ai-results {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
    gap: var(--space-2);
  }

  .ai-result-card {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-3) var(--space-4);
    border: 1px solid var(--border-default);
    border-radius: var(--radius-md);
    background: var(--surface-2);
    cursor: pointer;
    transition:
      border-color var(--duration-fast) var(--ease-out),
      background var(--duration-fast) var(--ease-out);
  }

  .ai-result-card:hover {
    border-color: var(--border-strong);
    background: var(--surface-3);
  }

  .ai-result-card--selected {
    border-color: var(--accent-solid);
    background: var(--accent-subtle);
  }

  .ai-result-radio {
    position: absolute;
    opacity: 0;
    width: 1px;
    height: 1px;
    pointer-events: none;
  }

  .ai-result-card__name {
    font-size: var(--text-base);
    font-weight: 500;
    color: var(--text-1);
  }

  .ai-result-card--selected .ai-result-card__name {
    color: var(--accent-text);
  }

  .ai-results-error {
    padding: var(--space-3) var(--space-4);
    background: oklch(70% 0.18 25 / 0.1);
    border: 1px solid oklch(70% 0.18 25 / 0.3);
    border-radius: var(--radius-md);
    color: var(--error);
    font-size: var(--text-sm);
  }

  /* Refine buttons */
  .ai-refine-row {
    display: flex;
    gap: var(--space-3);
  }

  .ai-refine-btn {
    flex: 1;
    padding: var(--space-2) var(--space-4);
    background: none;
    border: 1px solid var(--border-default);
    border-radius: var(--radius-md);
    color: var(--text-2);
    font-size: var(--text-sm);
    font-weight: 500;
    font-family: var(--font-body);
    cursor: pointer;
    transition:
      border-color var(--duration-fast) var(--ease-out),
      color var(--duration-fast) var(--ease-out);
  }

  .ai-refine-btn:hover:not(:disabled) {
    border-color: var(--accent-border);
    color: var(--accent-text);
  }

  .ai-refine-btn:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }

  .sr-only {
    position: absolute;
    width: 1px;
    height: 1px;
    padding: 0;
    margin: -1px;
    overflow: hidden;
    clip: rect(0, 0, 0, 0);
    white-space: nowrap;
    border-width: 0;
  }
</style>
