<script lang="ts">
  /**
   * VoiceSettings — Category 5
   *
   * Trigger phrase, mic toggle, push-to-talk key.
   * Voice wiring is deferred to a later workstream; this surface is
   * persisted in userPrefs and will be wired to the daemon once the
   * voice ingress feature ships (macOS-first, local Whisper.cpp).
   */

  import { userPrefs, updatePrefs } from '$lib/stores/preferences';

  interface Props {
    searchQuery?: string;
  }

  let { searchQuery = '' }: Props = $props();

  // Push-to-talk key capture state
  let capturingPttKey = $state(false);
  let pendingPttKey = $state('');

  const prefs = $derived($userPrefs.voice);

  function handleTriggerInput(e: Event) {
    const value = (e.target as HTMLInputElement).value;
    updatePrefs('voice', { triggerPhrase: value });
  }

  function startPttCapture() {
    capturingPttKey = true;
    pendingPttKey = '';
  }

  function cancelPttCapture() {
    capturingPttKey = false;
    pendingPttKey = '';
  }

  function confirmPttCapture() {
    if (!pendingPttKey) return;
    updatePrefs('voice', { pushToTalkKey: pendingPttKey });
    capturingPttKey = false;
    pendingPttKey = '';
  }

  function handlePttKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      e.preventDefault();
      cancelPttCapture();
      return;
    }
    if (e.key === 'Enter') {
      e.preventDefault();
      confirmPttCapture();
      return;
    }
    e.preventDefault();
    const mods: string[] = [];
    if (e.metaKey) mods.push('⌘');
    if (e.ctrlKey) mods.push('Ctrl');
    if (e.altKey) mods.push('⌥');
    if (e.shiftKey) mods.push('Shift');
    const key = e.key.length === 1 ? e.key.toUpperCase() : e.key;
    if (['Meta', 'Control', 'Alt', 'Shift'].includes(key)) return;
    pendingPttKey = [...mods, key].join('+');
  }

  function matches(text: string): boolean {
    if (!searchQuery) return true;
    return text.toLowerCase().includes(searchQuery.toLowerCase());
  }
</script>

<div class="settings-category">

  <!-- Status notice -->
  <div class="feature-notice">
    <span class="feature-notice__label">Planned</span>
    <p class="feature-notice__text">
      Voice commands are in the roadmap. Settings saved here will take effect
      when voice ingress ships (macOS-first via local Whisper.cpp — no cloud STT).
    </p>
  </div>

  <!-- Mic enable -->
  {#if matches('mic') || matches('microphone') || matches('voice') || matches('enable')}
    <div class="setting-row">
      <div class="setting-row__label">
        <span class="setting-row__name">Enable voice commands</span>
        <span class="setting-row__desc">Allow Vedox to listen for the trigger phrase. Uses the system default microphone.</span>
      </div>
      <div class="setting-row__control">
        <button
          type="button"
          role="switch"
          class="toggle-switch"
          class:toggle-switch--on={prefs.micEnabled}
          aria-checked={prefs.micEnabled}
          onclick={() => updatePrefs('voice', { micEnabled: !prefs.micEnabled })}
          aria-label="Toggle voice commands"
        >
          <span class="toggle-switch__thumb" aria-hidden="true"></span>
        </button>
      </div>
    </div>
  {/if}

  <!-- Trigger phrase -->
  {#if matches('trigger') || matches('phrase') || matches('wake word') || matches('voice')}
    <div class="setting-row setting-row--block">
      <div class="setting-row__label">
        <span class="setting-row__name">Trigger phrase</span>
        <span class="setting-row__desc">
          Say this phrase to activate the Vedox Doc Agent. Keep it distinct and 3+ words.
        </span>
      </div>
      <div class="setting-row__input-wrap">
        <input
          type="text"
          class="text-input"
          value={prefs.triggerPhrase}
          oninput={handleTriggerInput}
          placeholder="vedox document everything"
          aria-label="Trigger phrase"
          spellcheck="false"
        />
      </div>
    </div>
  {/if}

  <!-- Push-to-talk key -->
  {#if matches('push to talk') || matches('push-to-talk') || matches('ptt') || matches('key') || matches('voice')}
    <div class="setting-row">
      <div class="setting-row__label">
        <span class="setting-row__name">Push-to-talk key</span>
        <span class="setting-row__desc">Hold this key to activate voice input instead of using the trigger phrase.</span>
      </div>
      <div class="setting-row__control">
        {#if capturingPttKey}
          <!-- svelte-ignore a11y_autofocus -->
          <input
            class="key-capture-input"
            type="text"
            value={pendingPttKey || 'Press key…'}
            readonly
            autofocus
            aria-label="Press key for push-to-talk"
            onkeydown={handlePttKeydown}
            onblur={confirmPttCapture}
          />
          <button
            type="button"
            class="key-action-btn"
            onclick={cancelPttCapture}
            aria-label="Cancel"
          >Cancel</button>
        {:else}
          <button
            type="button"
            class="key-badge"
            onclick={startPttCapture}
            aria-label={prefs.pushToTalkKey ? `Push-to-talk: ${prefs.pushToTalkKey}. Click to change.` : 'Set push-to-talk key'}
            title="Click to set"
          >
            {#if prefs.pushToTalkKey}
              <kbd>{prefs.pushToTalkKey}</kbd>
            {:else}
              <span class="key-badge--empty">Not set</span>
            {/if}
          </button>
          {#if prefs.pushToTalkKey}
            <button
              type="button"
              class="key-action-btn"
              onclick={() => updatePrefs('voice', { pushToTalkKey: '' })}
              aria-label="Clear push-to-talk key"
            >Clear</button>
          {/if}
        {/if}
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

  .feature-notice {
    display: flex;
    align-items: flex-start;
    gap: var(--space-3);
    padding: var(--space-3) var(--space-4);
    background-color: var(--color-surface-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    margin-bottom: var(--space-4);
  }

  .feature-notice__label {
    flex-shrink: 0;
    font-size: 11px;
    font-weight: 600;
    letter-spacing: 0.04em;
    text-transform: uppercase;
    padding: 2px 6px;
    border-radius: var(--radius-sm);
    background: var(--color-accent-subtle);
    color: var(--color-accent);
  }

  .feature-notice__text {
    font-size: var(--font-size-sm);
    color: var(--color-text-muted);
    line-height: 1.5;
    margin: 0;
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

  .key-badge {
    background: none;
    border: none;
    padding: 0;
    cursor: pointer;
    border-radius: var(--radius-sm);
    display: inline-flex;
    align-items: center;
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
  }

  .key-badge--empty {
    font-size: var(--font-size-sm);
    color: var(--color-text-muted);
    border: 1px dashed var(--color-border);
    border-radius: var(--radius-sm);
    padding: 2px 8px;
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
    color: var(--color-text-muted);
    cursor: pointer;
    font-family: var(--font-sans);
    transition: border-color 80ms ease, color 80ms ease;
  }

  .key-action-btn:hover {
    border-color: var(--color-text-muted);
    color: var(--color-text-primary);
  }

  .key-action-btn:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
  }
</style>
