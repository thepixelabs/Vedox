<script lang="ts">
  /**
   * ConfigureVoice.svelte — Onboarding step 4.
   *
   * Voice commands are coming soon (macOS-first, local/offline Whisper.cpp).
   * This step shows a placeholder with honest scope messaging and a
   * push-to-talk key picker skeleton so the UX slot is reserved.
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
  // State
  // ---------------------------------------------------------------------------

  let voiceEnabled = $state(false);
  let triggerKey = $state('');

  const TRIGGER_OPTIONS = [
    { value: '', label: 'none' },
    { value: 'F5', label: 'F5' },
    { value: 'F6', label: 'F6' },
    { value: 'ctrl+shift+v', label: 'Ctrl+Shift+V' },
    { value: 'cmd+shift+v', label: 'Cmd+Shift+V' },
  ];

  function handleNext() {
    onboardingStore.setVoiceConfigured(voiceEnabled);
    onNext();
  }
</script>

<div class="step-voice">
  <header class="step-voice__header">
    <h2 class="step-voice__title">configure voice</h2>
    <p class="step-voice__desc">
      say a trigger phrase to start the doc agent hands-free.
      uses local whisper.cpp — no cloud, no recording upload.
    </p>
  </header>

  <!-- ── Coming soon banner ───────────────────────────────────────────────── -->
  <div class="step-voice__soon" role="note" aria-label="Coming soon">
    <div class="step-voice__soon-badge">coming soon</div>
    <p class="step-voice__soon-text">
      voice commands are macOS-first and land in a near-term update.
      the toggle below is a preview — it does not activate anything yet.
    </p>
    <p class="step-voice__soon-text">
      on macOS, vedox uses <code>CoreAudio</code> for capture.
      linux support via <code>whisper.cpp</code> is planned.
      windows is best-effort.
    </p>
  </div>

  <!-- ── Controls (non-functional placeholder) ────────────────────────────── -->
  <fieldset class="step-voice__fieldset" disabled>
    <legend class="step-voice__legend">voice settings (preview)</legend>

    <label class="step-voice__toggle-row">
      <span class="step-voice__toggle-label">enable voice trigger</span>
      <input
        class="step-voice__checkbox"
        type="checkbox"
        bind:checked={voiceEnabled}
        disabled
        aria-label="Enable voice trigger (coming soon)"
      />
    </label>

    <label class="step-voice__field">
      <span class="step-voice__field-label">push-to-talk key</span>
      <select
        class="step-voice__select"
        bind:value={triggerKey}
        disabled
        aria-label="Push-to-talk key (coming soon)"
      >
        {#each TRIGGER_OPTIONS as opt (opt.value)}
          <option value={opt.value}>{opt.label}</option>
        {/each}
      </select>
    </label>

    <label class="step-voice__field">
      <span class="step-voice__field-label">trigger phrase</span>
      <input
        class="step-voice__input"
        type="text"
        value="vedox document everything"
        disabled
        aria-label="Voice trigger phrase (coming soon)"
      />
    </label>
  </fieldset>

  <!-- ── Actions ──────────────────────────────────────────────────────────── -->
  <footer class="step-voice__footer">
    <button
      class="step-btn step-btn--secondary"
      type="button"
      onclick={onBack}
    >
      ./back
    </button>
    <button
      class="step-btn step-btn--ghost"
      type="button"
      onclick={onSkip}
    >
      ./skip — set up later
    </button>
    <button
      class="step-btn step-btn--primary"
      type="button"
      onclick={handleNext}
    >
      ./continue
    </button>
  </footer>
</div>

<style>
  .step-voice {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
    min-height: 0;
  }

  /* ── Header ── */

  .step-voice__header {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .step-voice__title {
    margin: 0;
    font-size: var(--font-size-lg, 1.125rem);
    font-weight: 600;
    font-family: var(--font-mono);
    color: var(--color-text-primary);
  }

  .step-voice__desc {
    margin: 0;
    font-size: var(--font-size-sm);
    font-family: var(--font-mono);
    color: var(--color-text-secondary);
    line-height: 1.6;
  }

  /* ── Coming soon banner ── */

  .step-voice__soon {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    padding: var(--space-4);
    background-color: var(--color-surface-overlay);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    border-left: 3px solid var(--color-accent);
  }

  .step-voice__soon-badge {
    display: inline-flex;
    align-self: flex-start;
    padding: 2px var(--space-2);
    background-color: var(--color-accent-subtle);
    color: var(--color-accent);
    font-size: 10px;
    font-family: var(--font-mono);
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    border-radius: var(--radius-sm);
  }

  .step-voice__soon-text {
    margin: 0;
    font-size: var(--font-size-sm);
    font-family: var(--font-mono);
    color: var(--color-text-secondary);
    line-height: 1.6;
  }

  .step-voice__soon-text code {
    font-family: var(--font-mono);
    background-color: var(--color-surface-base);
    padding: 1px 4px;
    border-radius: var(--radius-sm);
    font-size: 0.9em;
  }

  /* ── Fieldset (disabled placeholder) ── */

  .step-voice__fieldset {
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: var(--space-4);
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    opacity: 0.45;
    cursor: not-allowed;
  }

  .step-voice__legend {
    font-size: 11px;
    font-family: var(--font-mono);
    font-weight: 600;
    color: var(--color-text-muted);
    text-transform: uppercase;
    letter-spacing: 0.06em;
    padding: 0 var(--space-1);
  }

  .step-voice__toggle-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
  }

  .step-voice__toggle-label {
    font-size: var(--font-size-sm);
    font-family: var(--font-mono);
    color: var(--color-text-secondary);
  }

  .step-voice__checkbox {
    width: 14px;
    height: 14px;
    accent-color: var(--color-accent);
    cursor: not-allowed;
  }

  .step-voice__field {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .step-voice__field-label {
    font-size: 11px;
    font-family: var(--font-mono);
    font-weight: 600;
    color: var(--color-text-muted);
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }

  .step-voice__select,
  .step-voice__input {
    width: 100%;
    padding: 7px var(--space-3);
    background-color: var(--color-surface-base);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    font-size: var(--font-size-sm);
    font-family: var(--font-mono);
    color: var(--color-text-primary);
    cursor: not-allowed;
    box-sizing: border-box;
  }

  /* ── Footer ── */

  .step-voice__footer {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    flex-wrap: wrap;
    flex-shrink: 0;
  }
</style>
