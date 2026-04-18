<script lang="ts">
  /**
   * KeyboardSettings — Category 4
   *
   * Displays all remappable shortcuts grouped by category. Shows the active
   * binding (default or user override). Conflict detection: if two shortcuts
   * share the same key, both are flagged amber.
   *
   * Remapping UI: click a shortcut row's key badge → enter key capture mode →
   * press the desired key combination → Escape to cancel, Enter/click-away to
   * confirm. Conflicts are flagged but not blocked (user decides).
   */

  import { userPrefs, updatePrefs } from '$lib/stores/preferences';
  import { shortcuts, shortcutCategories, type ShortcutEntry } from '$lib/data/shortcuts-data';
  import { browser } from '$app/environment';

  interface Props {
    searchQuery?: string;
  }

  let { searchQuery = '' }: Props = $props();

  // Which row is currently in key-capture mode (null = none).
  let capturingKey: string | null = $state(null);
  // Pending key string being typed during capture.
  let pendingKey = $state('');

  const overrides = $derived($userPrefs.keyboard.overrides);

  function effectiveKey(s: ShortcutEntry): string {
    return overrides[s.key] ?? s.key;
  }

  /** Build a map of key → [action ids] for conflict detection. */
  const conflictMap = $derived(() => {
    const map = new Map<string, string[]>();
    for (const s of shortcuts) {
      const key = effectiveKey(s);
      const existing = map.get(key) ?? [];
      existing.push(s.key);
      map.set(key, existing);
    }
    return map;
  });

  function isConflict(s: ShortcutEntry): boolean {
    const key = effectiveKey(s);
    return (conflictMap().get(key)?.length ?? 0) > 1;
  }

  function startCapture(actionKey: string) {
    capturingKey = actionKey;
    pendingKey = '';
  }

  function cancelCapture() {
    capturingKey = null;
    pendingKey = '';
  }

  function confirmCapture(actionKey: string) {
    if (!pendingKey || !browser) return;
    updatePrefs('keyboard', {
      overrides: { ...overrides, [actionKey]: pendingKey },
    });
    capturingKey = null;
    pendingKey = '';
  }

  function resetShortcut(actionKey: string) {
    const next = { ...overrides };
    delete next[actionKey];
    updatePrefs('keyboard', { overrides: next });
  }

  function handleKeydown(e: KeyboardEvent, actionKey: string) {
    if (e.key === 'Escape') {
      e.preventDefault();
      cancelCapture();
      return;
    }
    if (e.key === 'Enter') {
      e.preventDefault();
      confirmCapture(actionKey);
      return;
    }
    e.preventDefault();
    // Build a human-readable key combo string.
    const mods: string[] = [];
    if (e.metaKey) mods.push('⌘');
    if (e.ctrlKey) mods.push('Ctrl');
    if (e.altKey) mods.push('⌥');
    if (e.shiftKey) mods.push('Shift');
    const key = e.key.length === 1 ? e.key.toUpperCase() : e.key;
    if (['Meta', 'Control', 'Alt', 'Shift'].includes(key)) return; // modifier-only
    pendingKey = [...mods, key].join('+');
  }

  function matches(text: string): boolean {
    if (!searchQuery) return true;
    return text.toLowerCase().includes(searchQuery.toLowerCase());
  }

  const filteredShortcuts = $derived(
    searchQuery
      ? shortcuts.filter(
          (s) =>
            s.description.toLowerCase().includes(searchQuery.toLowerCase()) ||
            s.category.toLowerCase().includes(searchQuery.toLowerCase()) ||
            effectiveKey(s).toLowerCase().includes(searchQuery.toLowerCase()),
        )
      : shortcuts,
  );

  const filteredCategories = $derived(
    searchQuery
      ? ([...new Set(filteredShortcuts.map((s) => s.category))] as typeof shortcutCategories)
      : shortcutCategories,
  );
</script>

<div class="settings-category">
  {#if filteredShortcuts.length === 0}
    <p class="no-results">No shortcuts match "{searchQuery}".</p>
  {:else}
    {#each filteredCategories as category}
      <div class="shortcuts-group">
        <h3 class="shortcuts-group__title">{category}</h3>
        <dl class="shortcuts-list">
          {#each filteredShortcuts.filter((s) => s.category === category) as s (s.key)}
            {@const isCapturing = capturingKey === s.key}
            {@const conflict = isConflict(s)}
            {@const isOverridden = s.key in overrides}
            <div
              class="shortcut-row"
              class:shortcut-row--capturing={isCapturing}
              class:shortcut-row--conflict={conflict}
            >
              <dt class="shortcut-desc">{s.description}</dt>
              <dd class="shortcut-key-cell">
                {#if isCapturing}
                  <!-- svelte-ignore a11y_autofocus -->
                  <input
                    class="key-capture-input"
                    type="text"
                    value={pendingKey || 'Press keys…'}
                    readonly
                    autofocus
                    aria-label="Press key combination for {s.description}"
                    onkeydown={(e) => handleKeydown(e, s.key)}
                    onblur={() => confirmCapture(s.key)}
                  />
                  <button
                    type="button"
                    class="key-action-btn key-action-btn--cancel"
                    onclick={cancelCapture}
                    aria-label="Cancel key capture"
                  >Cancel</button>
                {:else}
                  <button
                    type="button"
                    class="key-badge"
                    class:key-badge--conflict={conflict}
                    class:key-badge--overridden={isOverridden}
                    onclick={() => startCapture(s.key)}
                    aria-label="Change shortcut for {s.description}. Currently {effectiveKey(s)}"
                    title="Click to remap"
                  >
                    <kbd>{effectiveKey(s)}</kbd>
                  </button>
                  {#if isOverridden}
                    <button
                      type="button"
                      class="key-action-btn key-action-btn--reset"
                      onclick={() => resetShortcut(s.key)}
                      aria-label="Reset to default ({s.key})"
                      title="Reset to default"
                    >Reset</button>
                  {/if}
                  {#if conflict}
                    <span class="conflict-label" aria-label="Shortcut conflict detected" title="Another shortcut uses this key">!</span>
                  {/if}
                {/if}
              </dd>
            </div>
          {/each}
        </dl>
      </div>
    {/each}
  {/if}

  <div class="shortcuts-reset-row">
    <button
      type="button"
      class="reset-all-btn"
      onclick={() => updatePrefs('keyboard', { overrides: {} })}
      disabled={Object.keys(overrides).length === 0}
    >Reset all shortcuts to defaults</button>
  </div>
</div>

<style>
  .settings-category {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
  }

  .no-results {
    font-size: var(--font-size-sm);
    color: var(--color-text-muted);
    padding: var(--space-4) 0;
  }

  .shortcuts-group {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .shortcuts-group__title {
    font-size: var(--font-size-xs);
    font-weight: 600;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--color-text-muted);
  }

  .shortcuts-list {
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
  }

  .shortcut-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-4);
    padding: var(--space-2) 0;
    border-bottom: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    transition: background-color 80ms ease;
  }

  .shortcut-row:last-child {
    border-bottom: none;
  }

  .shortcut-row--conflict {
    background-color: color-mix(in oklch, var(--color-warning, oklch(80% 0.18 75)) 8%, transparent);
  }

  .shortcut-desc {
    font-size: var(--font-size-sm);
    color: var(--color-text-primary);
    flex: 1;
    min-width: 0;
  }

  .shortcut-key-cell {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    margin: 0;
    flex-shrink: 0;
  }

  .key-badge {
    display: inline-flex;
    align-items: center;
    background: none;
    border: none;
    padding: 0;
    cursor: pointer;
    border-radius: var(--radius-sm);
  }

  .key-badge:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
  }

  .key-badge kbd {
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
    transition: border-color 80ms ease, color 80ms ease;
  }

  .key-badge:hover kbd {
    border-color: var(--color-accent);
    color: var(--color-text-primary);
  }

  .key-badge--overridden kbd {
    border-color: var(--color-accent);
    color: var(--color-accent);
  }

  .key-badge--conflict kbd {
    border-color: oklch(75% 0.18 75);
    color: oklch(75% 0.18 75);
  }

  .key-capture-input {
    padding: 2px 6px;
    border-radius: var(--radius-sm);
    border: 1px solid var(--color-accent);
    background: var(--color-surface-elevated);
    color: var(--color-text-primary);
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    line-height: 1.4;
    white-space: nowrap;
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
    caret-color: transparent;
    min-width: 80px;
  }

  .key-action-btn {
    font-size: var(--font-size-xs);
    padding: 2px var(--space-2);
    border-radius: var(--radius-sm);
    border: 1px solid var(--color-border);
    background: none;
    cursor: pointer;
    transition: border-color 80ms ease, color 80ms ease;
    font-family: var(--font-sans);
  }

  .key-action-btn:hover {
    border-color: var(--color-text-muted);
  }

  .key-action-btn--cancel {
    color: var(--color-text-muted);
  }

  .key-action-btn--reset {
    color: var(--color-text-muted);
  }

  .key-action-btn:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
  }

  .conflict-label {
    width: 16px;
    height: 16px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    font-size: 11px;
    font-weight: 700;
    border-radius: 50%;
    background: oklch(75% 0.18 75);
    color: #000;
    flex-shrink: 0;
  }

  .shortcuts-reset-row {
    padding-top: var(--space-4);
    border-top: 1px solid var(--color-border);
  }

  .reset-all-btn {
    font-size: var(--font-size-sm);
    color: var(--color-text-muted);
    background: none;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: var(--space-2) var(--space-4);
    cursor: pointer;
    font-family: var(--font-sans);
    transition: color 80ms ease, border-color 80ms ease;
  }

  .reset-all-btn:hover:not(:disabled) {
    color: var(--color-text-primary);
    border-color: var(--color-text-muted);
  }

  .reset-all-btn:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }

  .reset-all-btn:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
  }
</style>
