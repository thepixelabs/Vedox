<script lang="ts">
  /**
   * AllDone.svelte — Onboarding step 5.
   *
   * Summary of what was set up, first-doc suggestion, link to docs.
   * Calls `onFinish` when the user clicks the final CTA.
   */

  import { onboardingStore } from '$lib/stores/onboarding.svelte';

  interface Props {
    onBack: () => void;
    onFinish: () => void;
  }

  const { onBack, onFinish }: Props = $props();

  // ---------------------------------------------------------------------------
  // Summary items
  // ---------------------------------------------------------------------------

  const repos = $derived(onboardingStore.registeredRepos);
  const providers = $derived(onboardingStore.selectedProviders);
  const detectedCount = $derived(onboardingStore.detectedProjects.length);
  const skippedSteps = $derived(onboardingStore.skippedSteps);

  const hasRepos = $derived(repos.length > 0);
  const hasProviders = $derived(providers.length > 0);

  const providerLabels: Record<string, string> = {
    'claude-code': 'Claude Code',
    'codex': 'OpenAI Codex',
    'gemini': 'Google Gemini',
    'copilot': 'GitHub Copilot',
  };

  function handleFinish() {
    onboardingStore.complete();
    onFinish();
  }
</script>

<div class="step-done">
  <header class="step-done__header">
    <h2 class="step-done__title">you're ready.</h2>
    <p class="step-done__desc">
      vedox is set up. here's what happened.
    </p>
  </header>

  <!-- ── Summary ──────────────────────────────────────────────────────────── -->
  <ul class="step-done__summary" role="list" aria-label="Setup summary">
    <!-- Detected projects -->
    <li class="step-done__item" class:step-done__item--skipped={skippedSteps.includes(1)}>
      <span class="step-done__item-dot" aria-hidden="true"></span>
      <span class="step-done__item-text">
        {#if skippedSteps.includes(1)}
          project scan — skipped
        {:else if detectedCount > 0}
          detected {detectedCount} project{detectedCount === 1 ? '' : 's'}
        {:else}
          project scan — no repos found nearby
        {/if}
      </span>
    </li>

    <!-- Repos -->
    <li class="step-done__item" class:step-done__item--skipped={skippedSteps.includes(2) && !hasRepos}>
      <span class="step-done__item-dot" aria-hidden="true"></span>
      <span class="step-done__item-text">
        {#if hasRepos}
          {repos.length} doc repo{repos.length === 1 ? '' : 's'} registered
          {#each repos as repo (repo)}
            <br /><code class="step-done__code">{repo}</code>
          {/each}
        {:else if skippedSteps.includes(2)}
          doc repo — skipped. docs land in <code class="step-done__code">~/.vedox/inbox/</code>
        {:else}
          doc repo — not registered
        {/if}
      </span>
    </li>

    <!-- Agent -->
    <li class="step-done__item" class:step-done__item--skipped={skippedSteps.includes(3) && !hasProviders}>
      <span class="step-done__item-dot" aria-hidden="true"></span>
      <span class="step-done__item-text">
        {#if hasProviders}
          doc agent installed in:
          {providers.map((id) => providerLabels[id] ?? id).join(', ')}
        {:else if skippedSteps.includes(3)}
          doc agent — skipped. install later from settings &rsaquo; agent
        {:else}
          doc agent — not installed
        {/if}
      </span>
    </li>

    <!-- Voice -->
    <li class="step-done__item step-done__item--skipped">
      <span class="step-done__item-dot" aria-hidden="true"></span>
      <span class="step-done__item-text">
        voice — coming soon
      </span>
    </li>
  </ul>

  <!-- ── First doc suggestion ─────────────────────────────────────────────── -->
  <div class="step-done__suggestion">
    <p class="step-done__suggestion-label">start here</p>
    <p class="step-done__suggestion-body">
      create your first doc with <kbd>Cmd+N</kbd>. pick a doc type (how-to, adr,
      runbook) and vedox places it in the right folder automatically.
    </p>
    {#if !hasProviders}
      <p class="step-done__suggestion-body">
        or run <code>vedox server</code> in a terminal and say "vedox document
        everything" in claude code to let the agent write the first doc for you.
      </p>
    {/if}
  </div>

  <!-- ── Links ─────────────────────────────────────────────────────────────── -->
  <div class="step-done__links">
    <a
      class="step-done__link"
      href="/settings"
      aria-label="Open settings"
    >
      settings
    </a>
    <span class="step-done__link-sep" aria-hidden="true">·</span>
    <a
      class="step-done__link"
      href="/onboarding"
      aria-label="Re-run onboarding"
      onclick={(e) => { e.preventDefault(); onboardingStore.reset(); window.location.reload(); }}
    >
      re-run onboarding
    </a>
    <span class="step-done__link-sep" aria-hidden="true">·</span>
    <a
      class="step-done__link"
      href="https://vedox.dev/docs"
      target="_blank"
      rel="noopener noreferrer"
      aria-label="Open documentation (new tab)"
    >
      docs
    </a>
  </div>

  <!-- ── Actions ──────────────────────────────────────────────────────────── -->
  <footer class="step-done__footer">
    <button
      class="step-btn step-btn--primary"
      type="button"
      onclick={handleFinish}
    >
      ./open vedox
    </button>
    <button
      class="step-btn step-btn--secondary"
      type="button"
      onclick={onBack}
    >
      ./back
    </button>
  </footer>
</div>

<style>
  .step-done {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
    min-height: 0;
  }

  /* ── Header ── */

  .step-done__header {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .step-done__title {
    margin: 0;
    font-size: var(--font-size-xl, 1.25rem);
    font-weight: 700;
    font-family: var(--font-mono);
    color: var(--color-text-primary);
  }

  .step-done__desc {
    margin: 0;
    font-size: var(--font-size-sm);
    font-family: var(--font-mono);
    color: var(--color-text-secondary);
  }

  /* ── Summary ── */

  .step-done__summary {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .step-done__item {
    display: flex;
    align-items: flex-start;
    gap: var(--space-3);
    font-size: var(--font-size-sm);
    font-family: var(--font-mono);
    color: var(--color-text-primary);
    line-height: 1.6;
  }

  .step-done__item--skipped {
    color: var(--color-text-muted);
  }

  .step-done__item-dot {
    display: inline-block;
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background-color: var(--color-accent);
    flex-shrink: 0;
    margin-top: 6px;
  }

  .step-done__item--skipped .step-done__item-dot {
    background-color: var(--color-border);
  }

  .step-done__item-text {
    flex: 1;
    min-width: 0;
  }

  .step-done__code {
    font-family: var(--font-mono);
    font-size: 0.9em;
    background-color: var(--color-surface-overlay);
    padding: 1px 4px;
    border-radius: var(--radius-sm);
    color: var(--color-accent);
  }

  /* ── Suggestion ── */

  .step-done__suggestion {
    padding: var(--space-4);
    background-color: var(--color-surface-overlay);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    border-left: 3px solid var(--color-accent);
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .step-done__suggestion-label {
    margin: 0;
    font-size: 10px;
    font-family: var(--font-mono);
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-accent);
  }

  .step-done__suggestion-body {
    margin: 0;
    font-size: var(--font-size-sm);
    font-family: var(--font-mono);
    color: var(--color-text-secondary);
    line-height: 1.6;
  }

  .step-done__suggestion-body code {
    font-family: var(--font-mono);
    background-color: var(--color-surface-base);
    padding: 1px 4px;
    border-radius: var(--radius-sm);
    font-size: 0.9em;
  }

  .step-done__suggestion-body kbd {
    font-family: var(--font-mono);
    font-size: 0.85em;
    padding: 1px 5px;
    background-color: var(--color-surface-base);
    border: 1px solid var(--color-border);
    border-bottom-width: 2px;
    border-radius: var(--radius-sm);
    color: var(--color-text-primary);
  }

  /* ── Links ── */

  .step-done__links {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font-size: 11px;
    font-family: var(--font-mono);
    flex-wrap: wrap;
  }

  .step-done__link {
    color: var(--color-accent);
    text-decoration: none;
    text-underline-offset: 2px;
  }

  .step-done__link:hover {
    text-decoration: underline;
  }

  .step-done__link:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
    border-radius: var(--radius-sm);
  }

  .step-done__link-sep {
    color: var(--color-text-muted);
    user-select: none;
  }

  /* ── Footer ── */

  .step-done__footer {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    flex-shrink: 0;
  }
</style>
