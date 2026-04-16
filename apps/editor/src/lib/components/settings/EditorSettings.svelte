<script lang="ts">
  /**
   * EditorSettings — Category 2
   *
   * Default view (split/preview/source), auto-save interval, spell-check toggle.
   */

  import { userPrefs, updatePrefs } from '$lib/stores/preferences';

  interface Props {
    searchQuery?: string;
  }

  let { searchQuery = '' }: Props = $props();

  const views = [
    { value: 'split' as const, label: 'Split', description: 'Editor and preview side-by-side' },
    { value: 'preview' as const, label: 'Preview', description: 'Rendered output only' },
    { value: 'source' as const, label: 'Source', description: 'Raw Markdown source' },
  ];

  const autoSaveOptions = [
    { value: 0, label: 'Off' },
    { value: 1000, label: '1 s' },
    { value: 3000, label: '3 s' },
    { value: 5000, label: '5 s' },
    { value: 10000, label: '10 s' },
    { value: 30000, label: '30 s' },
  ];

  function matches(text: string): boolean {
    if (!searchQuery) return true;
    return text.toLowerCase().includes(searchQuery.toLowerCase());
  }

  const prefs = $derived($userPrefs.editor);
</script>

<div class="settings-category">

  <!-- Default view -->
  {#if matches('view') || matches('editor') || matches('split') || matches('preview') || matches('source')}
    <div class="setting-row">
      <div class="setting-row__label">
        <span class="setting-row__name">Default view</span>
        <span class="setting-row__desc">Opening layout when a document is first opened.</span>
      </div>
      <div class="setting-row__control">
        <div class="seg-buttons" role="group" aria-label="Default view">
          {#each views as v (v.value)}
            <button
              type="button"
              class="seg-btn"
              class:seg-btn--active={prefs.defaultView === v.value}
              aria-pressed={prefs.defaultView === v.value}
              title={v.description}
              onclick={() => updatePrefs('editor', { defaultView: v.value })}
            >{v.label}</button>
          {/each}
        </div>
      </div>
    </div>
  {/if}

  <!-- Auto-save -->
  {#if matches('auto-save') || matches('autosave') || matches('save') || matches('interval')}
    <div class="setting-row">
      <div class="setting-row__label">
        <span class="setting-row__name">Auto-save interval</span>
        <span class="setting-row__desc">How often unsaved changes are written to disk. Set to Off to save manually.</span>
      </div>
      <div class="setting-row__control">
        <div class="seg-buttons" role="group" aria-label="Auto-save interval">
          {#each autoSaveOptions as opt (opt.value)}
            <button
              type="button"
              class="seg-btn"
              class:seg-btn--active={prefs.autoSaveInterval === opt.value}
              aria-pressed={prefs.autoSaveInterval === opt.value}
              onclick={() => updatePrefs('editor', { autoSaveInterval: opt.value })}
            >{opt.label}</button>
          {/each}
        </div>
      </div>
    </div>
  {/if}

  <!-- Spell-check -->
  {#if matches('spell') || matches('spellcheck') || matches('spell check')}
    <div class="setting-row">
      <div class="setting-row__label">
        <span class="setting-row__name">Spell check</span>
        <span class="setting-row__desc">Underline misspelled words in the rich text editor. Uses browser spell-check engine.</span>
      </div>
      <div class="setting-row__control">
        <button
          type="button"
          role="switch"
          class="toggle-switch"
          class:toggle-switch--on={prefs.spellCheck}
          aria-checked={prefs.spellCheck}
          onclick={() => updatePrefs('editor', { spellCheck: !prefs.spellCheck })}
          aria-label="Toggle spell check"
        >
          <span class="toggle-switch__thumb" aria-hidden="true"></span>
        </button>
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

  /* Toggle switch */
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
</style>
