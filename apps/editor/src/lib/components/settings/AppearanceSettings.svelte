<script lang="ts">
  /**
   * AppearanceSettings — Category 1
   *
   * Theme, accent color, font size, line height, reading measure, density,
   * tree grouping. Theme changes apply to the DOM immediately via the
   * flagship themeStore; font-size and other CSS vars are synced inline.
   */

  import { themeStore, densityStore } from '$lib/theme/store';
  import ThemePreviewCard from '$lib/components/ThemePreviewCard.svelte';
  import FontPicker from '$lib/components/FontPicker.svelte';
  import { readingStore } from '$lib/stores/reading';
  import { userPrefs, updatePrefs } from '$lib/stores/preferences';
  import { browser } from '$app/environment';

  interface Props {
    searchQuery?: string;
  }

  let { searchQuery = '' }: Props = $props();

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

  const fontSizes = [
    { value: '13px' as const, label: 'Small' },
    { value: '16px' as const, label: 'Default' },
    { value: '18px' as const, label: 'Large' },
  ];

  const lineHeights = [
    { value: 'tight' as const, label: 'Tight', description: '1.4' },
    { value: 'normal' as const, label: 'Normal', description: '1.6' },
    { value: 'relaxed' as const, label: 'Relaxed', description: '1.8' },
  ];

  const measures = [
    { value: 'narrow' as const, label: 'Narrow' },
    { value: 'default' as const, label: 'Default' },
    { value: 'wide' as const, label: 'Wide' },
  ];

  const treeGroupings = [
    { value: 'type-first' as const, label: 'Type-first', description: 'Group by ADR / how-to / runbook…' },
    { value: 'folder-first' as const, label: 'Folder-first', description: 'Mirror the filesystem hierarchy' },
    { value: 'flat' as const, label: 'Flat', description: 'Alphabetical, no grouping' },
  ];

  const wordmarkFonts = [
    { value: 'display' as const, label: 'Serif (Fraunces)', description: 'Default editorial display font' },
    { value: 'mono' as const, label: 'Mono (JetBrains)', description: 'Monospaced wordmark' },
  ];

  const FONT_SIZE_KEY = 'vedox:font-size';

  // Font selection state — reads from localStorage to stay in sync with
  // the existing FontPicker infrastructure.
  let fontBody = $state(
    browser ? (localStorage.getItem('vedox:font-body') || '"Geist Variable", "Geist", system-ui, sans-serif') : '"Geist Variable", "Geist", system-ui, sans-serif'
  );
  let fontDisplay = $state(
    browser ? (localStorage.getItem('vedox:font-display') || '"Fraunces Variable", "Fraunces", Georgia, serif') : '"Fraunces Variable", "Fraunces", Georgia, serif'
  );
  let fontMono = $state(
    browser ? (localStorage.getItem('vedox:font-mono') || '"JetBrains Mono Variable", "JetBrains Mono", monospace') : '"JetBrains Mono Variable", "JetBrains Mono", monospace'
  );

  function handleFontChange(category: 'body' | 'display' | 'mono', newFamily: string) {
    if (!browser) return;
    document.documentElement.style.setProperty(`--font-${category}`, newFamily);
    localStorage.setItem(`vedox:font-${category}`, newFamily);
    if (category === 'body') fontBody = newFamily;
    else if (category === 'display') fontDisplay = newFamily;
    else fontMono = newFamily;
  }

  function applyFontSize(size: '13px' | '16px' | '18px') {
    if (!browser) return;
    document.documentElement.style.setProperty('--font-size-override', size);
    try { localStorage.setItem(FONT_SIZE_KEY, size); } catch { /* quota */ }
    updatePrefs('appearance', { fontSize: size });
  }

  function applyLineHeight(lh: 'tight' | 'normal' | 'relaxed') {
    const map = { tight: '1.4', normal: '1.6', relaxed: '1.8' };
    if (browser) {
      document.documentElement.style.setProperty('--line-height-body', map[lh]);
    }
    updatePrefs('appearance', { lineHeight: lh });
  }

  function setTheme(t: 'graphite' | 'eclipse' | 'ember' | 'paper' | 'solar') {
    themeStore.setTheme(t);
    updatePrefs('appearance', { theme: t });
  }

  function setDensity(d: 'compact' | 'comfortable' | 'cozy') {
    densityStore.setDensity(d);
    updatePrefs('appearance', { density: d });
  }

  function setMeasure(m: 'narrow' | 'default' | 'wide') {
    readingStore.setMeasure(m);
    updatePrefs('appearance', { measure: m });
  }

  function setWordmarkFont(w: 'display' | 'mono') {
    if (browser) {
      const value = w === 'mono' ? 'var(--font-mono)' : 'var(--font-display)';
      document.documentElement.style.setProperty('--font-wordmark', value);
    }
    updatePrefs('appearance', { wordmarkFont: w });
  }

  function matches(text: string): boolean {
    if (!searchQuery) return true;
    return text.toLowerCase().includes(searchQuery.toLowerCase());
  }

  const prefs = $derived($userPrefs.appearance);
</script>

<div class="settings-category">
  <!-- Theme -->
  {#if matches('theme') || matches('color') || matches('graphite') || matches('eclipse') || matches('ember') || matches('paper') || matches('solar')}
    <div class="setting-group">
      <div class="setting-group__header">
        <span class="setting-group__name">Theme</span>
        <span class="setting-group__desc">Controls the visual color palette applied across the entire editor.</span>
      </div>
      <div class="theme-grid" role="radiogroup" aria-label="Theme selection">
        {#each themes as t (t.theme)}
          <button
            type="button"
            class="theme-card-wrapper"
            class:theme-card-wrapper--active={$themeStore === t.theme}
            aria-pressed={$themeStore === t.theme}
            onclick={() => setTheme(t.theme)}
          >
            <ThemePreviewCard theme={t.theme} label={t.label} description={t.description} />
          </button>
        {/each}
      </div>
    </div>
  {/if}

  <!-- Font size -->
  {#if matches('font size') || matches('text size') || matches('font')}
    <div class="setting-row">
      <div class="setting-row__label">
        <span class="setting-row__name">Font size</span>
        <span class="setting-row__desc">Base text size applied across the interface.</span>
      </div>
      <div class="setting-row__control">
        <div class="seg-buttons" role="group" aria-label="Font size">
          {#each fontSizes as fs (fs.value)}
            <button
              type="button"
              class="seg-btn"
              class:seg-btn--active={prefs.fontSize === fs.value}
              aria-pressed={prefs.fontSize === fs.value}
              onclick={() => applyFontSize(fs.value)}
            >{fs.label}</button>
          {/each}
        </div>
      </div>
    </div>
  {/if}

  <!-- Line height -->
  {#if matches('line height') || matches('leading') || matches('spacing')}
    <div class="setting-row">
      <div class="setting-row__label">
        <span class="setting-row__name">Line height</span>
        <span class="setting-row__desc">Vertical spacing between lines of text in the editor.</span>
      </div>
      <div class="setting-row__control">
        <div class="seg-buttons" role="group" aria-label="Line height">
          {#each lineHeights as lh (lh.value)}
            <button
              type="button"
              class="seg-btn"
              class:seg-btn--active={prefs.lineHeight === lh.value}
              aria-pressed={prefs.lineHeight === lh.value}
              title={lh.description}
              onclick={() => applyLineHeight(lh.value)}
            >{lh.label}</button>
          {/each}
        </div>
      </div>
    </div>
  {/if}

  <!-- Reading width / measure -->
  {#if matches('reading width') || matches('measure') || matches('line length')}
    <div class="setting-row">
      <div class="setting-row__label">
        <span class="setting-row__name">Reading width</span>
        <span class="setting-row__desc">Maximum line length in the document editor.</span>
      </div>
      <div class="setting-row__control">
        <div class="seg-buttons" role="group" aria-label="Reading width">
          {#each measures as m (m.value)}
            <button
              type="button"
              class="seg-btn"
              class:seg-btn--active={$readingStore === m.value}
              aria-pressed={$readingStore === m.value}
              onclick={() => setMeasure(m.value)}
            >{m.label}</button>
          {/each}
        </div>
      </div>
    </div>
  {/if}

  <!-- Information density -->
  {#if matches('density') || matches('spacing') || matches('compact') || matches('cozy')}
    <div class="setting-row">
      <div class="setting-row__label">
        <span class="setting-row__name">Information density</span>
        <span class="setting-row__desc">Spacing between UI elements.</span>
      </div>
      <div class="setting-row__control">
        <div class="seg-buttons" role="group" aria-label="Information density">
          {#each densities as d (d.value)}
            <button
              type="button"
              class="seg-btn"
              class:seg-btn--active={$densityStore === d.value}
              aria-pressed={$densityStore === d.value}
              title={d.description}
              onclick={() => setDensity(d.value)}
            >{d.label}</button>
          {/each}
        </div>
      </div>
    </div>
  {/if}

  <!-- Typography (fonts) -->
  {#if matches('font') || matches('typography') || matches('typeface')}
    <div class="setting-group">
      <div class="setting-group__header">
        <span class="setting-group__name">Typography</span>
        <span class="setting-group__desc">Font families for body, display, and code. Changes apply immediately.</span>
      </div>
      <div class="font-pickers">
        <FontPicker category="body" value={fontBody} onChange={(v) => handleFontChange('body', v)} />
        <FontPicker category="display" value={fontDisplay} onChange={(v) => handleFontChange('display', v)} />
        <FontPicker category="mono" value={fontMono} onChange={(v) => handleFontChange('mono', v)} />
      </div>
    </div>
  {/if}

  <!-- Tree grouping -->
  {#if matches('tree') || matches('grouping') || matches('sidebar') || matches('type-first') || matches('folder-first')}
    <div class="setting-row">
      <div class="setting-row__label">
        <span class="setting-row__name">Doc tree grouping</span>
        <span class="setting-row__desc">Primary grouping in the document tree sidebar.</span>
      </div>
      <div class="setting-row__control">
        <div class="seg-buttons" role="group" aria-label="Tree grouping">
          {#each treeGroupings as tg (tg.value)}
            <button
              type="button"
              class="seg-btn"
              class:seg-btn--active={prefs.treeGrouping === tg.value}
              aria-pressed={prefs.treeGrouping === tg.value}
              title={tg.description}
              onclick={() => updatePrefs('appearance', { treeGrouping: tg.value })}
            >{tg.label}</button>
          {/each}
        </div>
      </div>
    </div>
  {/if}

  <!-- Wordmark font -->
  {#if matches('wordmark') || matches('font') || matches('sidebar') || matches('logo')}
    <div class="setting-row">
      <div class="setting-row__label">
        <span class="setting-row__name">Wordmark font</span>
        <span class="setting-row__desc">Typeface used for the "vedox" logotype in the sidebar.</span>
      </div>
      <div class="setting-row__control">
        <div class="seg-buttons" role="group" aria-label="Wordmark font">
          {#each wordmarkFonts as wf (wf.value)}
            <button
              type="button"
              class="seg-btn"
              class:seg-btn--active={prefs.wordmarkFont === wf.value}
              aria-pressed={prefs.wordmarkFont === wf.value}
              title={wf.description}
              onclick={() => setWordmarkFont(wf.value)}
            >{wf.label}</button>
          {/each}
        </div>
      </div>
    </div>
  {/if}
</div>

<style>
  .settings-category {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .setting-group {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    padding: var(--space-4) 0;
    border-bottom: 1px solid var(--color-border);
  }

  .setting-group:last-child {
    border-bottom: none;
  }

  .setting-group__header {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .setting-group__name {
    font-size: var(--font-size-sm);
    font-weight: 500;
    color: var(--color-text-primary);
  }

  .setting-group__desc {
    font-size: var(--font-size-sm);
    color: var(--color-text-muted);
    line-height: 1.4;
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
  }

  .theme-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
    gap: var(--space-3);
  }

  .theme-card-wrapper {
    all: unset;
    cursor: pointer;
    border-radius: var(--radius-md);
    outline-offset: 2px;
    display: block;
  }

  .theme-card-wrapper:focus-visible {
    outline: 2px solid var(--color-accent);
  }

  .theme-card-wrapper--active {
    box-shadow: 0 0 0 2px var(--color-accent);
    border-radius: var(--radius-md);
  }

  .seg-buttons {
    display: flex;
    gap: 2px;
    background-color: var(--color-surface-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: 2px;
  }

  .seg-btn {
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
    line-height: 1.4;
  }

  .seg-btn:hover {
    color: var(--color-text-primary);
  }

  .seg-btn--active {
    background-color: var(--color-surface-base);
    color: var(--color-text-primary);
    font-weight: 500;
    box-shadow: var(--shadow-sm);
  }

  .seg-btn:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
  }

  .font-pickers {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
  }
</style>
