<script lang="ts">
  /**
   * /onboarding — 5-step guided first-run flow.
   *
   * Steps:
   *   1. ScanProjects    — detect local git repos with docs
   *   2. SetupRepos      — create or register a doc repo
   *   3. InstallAgent    — pick providers, run install
   *   4. ConfigureVoice  — push-to-talk (coming soon placeholder)
   *   5. AllDone         — summary + first-doc suggestion
   *
   * State is managed by onboardingStore ($state rune, persisted to localStorage).
   * All steps are skippable per founder override OQ-E.
   * Re-triggerable from /settings — just navigate to /onboarding to restart.
   *
   * Pixelabs voice: lowercase labels, ./unix CTAs, no emoji, no carousel.
   */

  import { goto } from '$app/navigation';
  import { onboardingStore, STEPS, STEP_COUNT } from '$lib/stores/onboarding.svelte';

  import ScanProjects from '$lib/components/onboarding/ScanProjects.svelte';
  import SetupRepos from '$lib/components/onboarding/SetupRepos.svelte';
  import InstallAgent from '$lib/components/onboarding/InstallAgent.svelte';
  import ConfigureVoice from '$lib/components/onboarding/ConfigureVoice.svelte';
  import AllDone from '$lib/components/onboarding/AllDone.svelte';

  // ---------------------------------------------------------------------------
  // Reactive step
  // ---------------------------------------------------------------------------

  const store = onboardingStore;

  function handleNext() {
    store.next();
  }

  function handleBack() {
    store.back();
  }

  function handleSkip() {
    store.skip();
  }

  function handleFinish() {
    void goto('/');
  }

  // ---------------------------------------------------------------------------
  // Progress bar width
  // ---------------------------------------------------------------------------

  const progressWidth = $derived(`${store.progressPercent}%`);
  const stepLabel = $derived(
    `step ${store.step} of ${STEP_COUNT} — ${store.currentStepDef.title}`
  );
</script>

<svelte:head>
  <title>setup — vedox</title>
</svelte:head>

<div class="onboarding" role="main" aria-label="Vedox setup">
  <!-- ── Header ─────────────────────────────────────────────────────────────── -->
  <header class="onboarding__header" aria-label="Onboarding header">
    <a class="onboarding__wordmark" href="/" aria-label="Go to vedox home">
      vedox
    </a>
    <span class="onboarding__step-label" aria-live="polite" aria-atomic="true">
      {stepLabel}
    </span>
  </header>

  <!-- ── Progress bar ──────────────────────────────────────────────────────── -->
  <div
    class="onboarding__progress"
    role="progressbar"
    aria-valuemin={0}
    aria-valuemax={100}
    aria-valuenow={store.progressPercent}
    aria-label="Setup progress: {store.progressPercent}%"
  >
    <div
      class="onboarding__progress-fill"
      style:width={progressWidth}
    ></div>
  </div>

  <!-- ── Step stepper (visual only) ────────────────────────────────────────── -->
  <nav class="onboarding__stepper" aria-label="Setup steps">
    <ol class="onboarding__stepper-list" role="list">
      {#each STEPS as stepDef (stepDef.id)}
        {@const isDone = store.step > stepDef.index}
        {@const isCurrent = store.step === stepDef.index}
        {@const isSkipped = store.skippedSteps.includes(stepDef.index)}
        <li
          class="onboarding__stepper-item"
          class:onboarding__stepper-item--done={isDone && !isSkipped}
          class:onboarding__stepper-item--current={isCurrent}
          class:onboarding__stepper-item--skipped={isSkipped}
          aria-current={isCurrent ? 'step' : undefined}
        >
          <span class="onboarding__stepper-dot" aria-hidden="true">
            {#if isDone && !isSkipped}
              <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                <polyline points="20 6 9 17 4 12"/>
              </svg>
            {:else}
              {stepDef.index}
            {/if}
          </span>
          <span class="onboarding__stepper-label">{stepDef.title}</span>
        </li>
      {/each}
    </ol>
  </nav>

  <!-- ── Step content ──────────────────────────────────────────────────────── -->
  <main class="onboarding__content" id="onboarding-step-content" aria-live="polite">
    {#if store.step === 1}
      <ScanProjects onNext={handleNext} onSkip={handleSkip} />
    {:else if store.step === 2}
      <SetupRepos onBack={handleBack} onNext={handleNext} onSkip={handleSkip} />
    {:else if store.step === 3}
      <InstallAgent onBack={handleBack} onNext={handleNext} onSkip={handleSkip} />
    {:else if store.step === 4}
      <ConfigureVoice onBack={handleBack} onNext={handleNext} onSkip={handleSkip} />
    {:else if store.step === 5}
      <AllDone onBack={handleBack} onFinish={handleFinish} />
    {/if}
  </main>
</div>

<style>
  /* ── Page shell ─────────────────────────────────────────────────────────── */

  .onboarding {
    display: flex;
    flex-direction: column;
    min-height: 100vh;
    background-color: var(--color-surface-base);
    /* Center the card horizontally */
    align-items: center;
  }

  /* ── Header ── */

  .onboarding__header {
    width: 100%;
    max-width: 680px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--space-5) var(--space-6);
    flex-shrink: 0;
  }

  .onboarding__wordmark {
    font-size: var(--font-size-base);
    font-family: var(--font-mono);
    font-weight: 700;
    color: var(--color-text-primary);
    text-decoration: none;
    letter-spacing: -0.02em;
  }

  .onboarding__wordmark:hover {
    color: var(--color-accent);
  }

  .onboarding__wordmark:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
    border-radius: var(--radius-sm);
  }

  .onboarding__step-label {
    font-size: 11px;
    font-family: var(--font-mono);
    color: var(--color-text-muted);
    text-align: right;
  }

  /* ── Progress bar ── */

  .onboarding__progress {
    width: 100%;
    max-width: 680px;
    height: 2px;
    background-color: var(--color-border);
    flex-shrink: 0;
    overflow: hidden;
    /* Don't use border-radius — clean edge to edge */
  }

  .onboarding__progress-fill {
    height: 100%;
    background-color: var(--color-accent);
    transition: width 200ms var(--ease-out);
  }

  /* ── Stepper (step dots) ── */

  .onboarding__stepper {
    width: 100%;
    max-width: 680px;
    padding: var(--space-5) var(--space-6) var(--space-4);
    flex-shrink: 0;
  }

  .onboarding__stepper-list {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    align-items: flex-start;
    gap: 0;
    position: relative;
  }

  /* Connector line between dots */
  .onboarding__stepper-list::before {
    content: '';
    position: absolute;
    top: 10px;
    left: 10px;
    right: 10px;
    height: 1px;
    background-color: var(--color-border);
    z-index: 0;
  }

  .onboarding__stepper-item {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-2);
    flex: 1;
    position: relative;
    z-index: 1;
  }

  .onboarding__stepper-dot {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 20px;
    height: 20px;
    border-radius: 50%;
    border: 1.5px solid var(--color-border);
    background-color: var(--color-surface-base);
    font-size: 9px;
    font-family: var(--font-mono);
    font-weight: 600;
    color: var(--color-text-muted);
    transition:
      background-color 150ms var(--ease-out),
      border-color 150ms var(--ease-out),
      color 150ms var(--ease-out);
  }

  .onboarding__stepper-item--current .onboarding__stepper-dot {
    border-color: var(--color-accent);
    background-color: var(--color-accent);
    color: var(--color-accent-contrast, #fff);
  }

  .onboarding__stepper-item--done .onboarding__stepper-dot {
    border-color: var(--color-accent);
    background-color: var(--color-accent-subtle);
    color: var(--color-accent);
  }

  .onboarding__stepper-item--skipped .onboarding__stepper-dot {
    border-color: var(--color-border);
    background-color: var(--color-surface-overlay);
    color: var(--color-text-muted);
    opacity: 0.5;
  }

  .onboarding__stepper-label {
    font-size: 10px;
    font-family: var(--font-mono);
    color: var(--color-text-muted);
    text-align: center;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 80px;
    transition: color 150ms var(--ease-out);
  }

  .onboarding__stepper-item--current .onboarding__stepper-label {
    color: var(--color-accent);
    font-weight: 600;
  }

  .onboarding__stepper-item--done .onboarding__stepper-label {
    color: var(--color-text-secondary);
  }

  /* ── Step content ── */

  .onboarding__content {
    width: 100%;
    max-width: 680px;
    padding: var(--space-4) var(--space-6) var(--space-10);
    flex: 1;
    min-height: 0;
  }

  /* ── Responsive: narrow viewports ── */

  @media (max-width: 480px) {
    .onboarding__header,
    .onboarding__stepper,
    .onboarding__content {
      padding-left: var(--space-4);
      padding-right: var(--space-4);
    }

    .onboarding__stepper-label {
      display: none;
    }
  }

  /* ── Reduced motion ── */

  @media (prefers-reduced-motion: reduce) {
    .onboarding__progress-fill {
      transition: none;
    }
    .onboarding__stepper-dot,
    .onboarding__stepper-label {
      transition: none;
    }
  }
</style>
