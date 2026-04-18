<script lang="ts">
  /**
   * ScanProjects.svelte — Onboarding step 1.
   *
   * Attempts to call GET /api/scan to detect local git repos with docs.
   * If the daemon is not running, falls back to a manual folder picker.
   * Detected repos are shown in a scrollable list; user can select which
   * ones to bring into Vedox.
   *
   * Calls `onNext` when the user is ready to proceed.
   * Calls `onSkip` when the user skips.
   */

  import { onMount } from 'svelte';
  import { onboardingStore, type DetectedProject } from '$lib/stores/onboarding.svelte';

  interface Props {
    onNext: () => void;
    onSkip: () => void;
  }

  const { onNext, onSkip }: Props = $props();

  // ---------------------------------------------------------------------------
  // Scan state
  // ---------------------------------------------------------------------------

  type ScanStatus = 'idle' | 'scanning' | 'done' | 'error' | 'offline';

  let scanStatus = $state<ScanStatus>('idle');
  let scanError = $state<string | null>(null);
  let projects = $state<DetectedProject[]>([...onboardingStore.detectedProjects]);
  let selectedPaths = $state<Set<string>>(new Set());

  // ---------------------------------------------------------------------------
  // Auto-scan on mount
  // ---------------------------------------------------------------------------

  onMount(() => {
    runScan();
  });

  async function runScan() {
    scanStatus = 'scanning';
    scanError = null;

    try {
      const res = await fetch('/api/scan', { signal: AbortSignal.timeout(8000) });
      if (!res.ok) throw new Error(`${res.status}`);
      const data = (await res.json()) as { projects: DetectedProject[] };
      projects = data.projects ?? [];
      onboardingStore.setDetectedProjects(projects);
      // Pre-select all detected projects.
      selectedPaths = new Set(projects.map((p) => p.path));
      scanStatus = 'done';
    } catch (err) {
      // Daemon not running — show offline state with manual picker.
      const msg = err instanceof Error ? err.message : String(err);
      if (msg.includes('fetch') || msg.includes('Failed') || msg.includes('timeout')) {
        scanStatus = 'offline';
      } else {
        scanStatus = 'error';
        scanError = msg;
      }
    }
  }

  function toggleProject(path: string) {
    const next = new Set(selectedPaths);
    if (next.has(path)) {
      next.delete(path);
    } else {
      next.add(path);
    }
    selectedPaths = next;
  }

  function handleNext() {
    // Persist selection — only selected projects matter downstream.
    const selected = projects.filter((p) => selectedPaths.has(p.path));
    onboardingStore.setDetectedProjects(selected);
    onNext();
  }

  const hasSelection = $derived(selectedPaths.size > 0);
</script>

<div class="step-scan">
  <header class="step-scan__header">
    <h2 class="step-scan__title">detect projects</h2>
    <p class="step-scan__desc">
      vedox scans your filesystem for git repos that contain markdown docs.
      pick the ones you want to track.
    </p>
  </header>

  <!-- ── Scan body ────────────────────────────────────────────────────────── -->
  <div class="step-scan__body" aria-live="polite">
    {#if scanStatus === 'idle' || scanStatus === 'scanning'}
      <div class="step-scan__loading">
        <span class="step-scan__spinner" aria-hidden="true"></span>
        <span>scanning filesystem...</span>
      </div>

    {:else if scanStatus === 'error'}
      <div class="step-scan__alert" role="alert">
        <span class="step-scan__alert-label">scan failed</span>
        <span>{scanError}</span>
        <button class="step-scan__retry" type="button" onclick={runScan}>
          ./retry
        </button>
      </div>

    {:else if scanStatus === 'offline'}
      <div class="step-scan__offline">
        <p class="step-scan__offline-note">
          the vedox daemon is not running. start it with{' '}
          <code>vedox server</code> to enable auto-scan, or add a project folder
          manually in the next step.
        </p>
        <button class="step-scan__retry" type="button" onclick={runScan}>
          ./retry scan
        </button>
      </div>

    {:else if scanStatus === 'done' && projects.length === 0}
      <div class="step-scan__empty">
        <p>no git repos with docs found nearby.</p>
        <p class="step-scan__empty-hint">
          you can register a folder manually in the next step.
        </p>
      </div>

    {:else if scanStatus === 'done'}
      <ul class="step-scan__list" role="list" aria-label="Detected projects">
        {#each projects as project (project.path)}
          {@const checked = selectedPaths.has(project.path)}
          <li class="step-scan__project">
            <label class="step-scan__project-label">
              <input
                class="step-scan__checkbox"
                type="checkbox"
                checked={checked}
                onchange={() => toggleProject(project.path)}
                aria-label="Include {project.name}"
              />
              <span class="step-scan__project-info">
                <span class="step-scan__project-name">{project.name}</span>
                <span class="step-scan__project-path">{project.path}</span>
              </span>
              <span class="step-scan__project-meta">
                {project.docCount} doc{project.docCount === 1 ? '' : 's'}
                {#if project.hasGit}
                  <span class="step-scan__git-dot" title="git repo" aria-label="git repo"></span>
                {/if}
              </span>
            </label>
          </li>
        {/each}
      </ul>

      {#if projects.length > 0}
        <p class="step-scan__selection-hint">
          {selectedPaths.size} of {projects.length} selected
        </p>
      {/if}
    {/if}
  </div>

  <!-- ── Actions ──────────────────────────────────────────────────────────── -->
  <footer class="step-scan__footer">
    <button
      class="step-btn step-btn--primary"
      type="button"
      disabled={scanStatus === 'scanning'}
      onclick={handleNext}
    >
      {hasSelection ? './use selected' : './continue'}
    </button>
    <button
      class="step-btn step-btn--ghost"
      type="button"
      onclick={onSkip}
    >
      ./skip
    </button>
  </footer>
</div>

<style>
  .step-scan {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
    min-height: 0;
  }

  /* ── Header ── */

  .step-scan__header {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .step-scan__title {
    margin: 0;
    font-size: var(--font-size-lg, 1.125rem);
    font-weight: 600;
    font-family: var(--font-mono);
    color: var(--color-text-primary);
  }

  .step-scan__desc {
    margin: 0;
    font-size: var(--font-size-sm);
    color: var(--color-text-secondary);
    line-height: 1.6;
    font-family: var(--font-mono);
  }

  /* ── Body ── */

  .step-scan__body {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
  }

  /* ── Loading ── */

  .step-scan__loading {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-4) 0;
    font-size: var(--font-size-sm);
    font-family: var(--font-mono);
    color: var(--color-text-muted);
  }

  .step-scan__spinner {
    display: inline-block;
    width: 14px;
    height: 14px;
    border: 2px solid var(--color-border);
    border-top-color: var(--color-accent);
    border-radius: 50%;
    animation: spin 600ms linear infinite;
    flex-shrink: 0;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  /* ── Alert / offline ── */

  .step-scan__alert,
  .step-scan__offline {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    padding: var(--space-4);
    background-color: color-mix(in srgb, var(--color-error, #e53e3e) 8%, transparent);
    border: 1px solid color-mix(in srgb, var(--color-error, #e53e3e) 25%, transparent);
    border-radius: var(--radius-md);
    font-size: var(--font-size-sm);
    font-family: var(--font-mono);
    color: var(--color-text-secondary);
  }

  .step-scan__offline {
    background-color: var(--color-surface-overlay);
    border-color: var(--color-border);
  }

  .step-scan__alert-label {
    font-weight: 600;
    color: var(--color-error, #e53e3e);
  }

  .step-scan__offline-note {
    margin: 0;
    line-height: 1.6;
  }

  .step-scan__offline-note code {
    font-family: var(--font-mono);
    background-color: var(--color-surface-overlay);
    padding: 1px 5px;
    border-radius: var(--radius-sm);
    font-size: 0.9em;
  }

  .step-scan__retry {
    align-self: flex-start;
    background: none;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    padding: 4px var(--space-3);
    font-size: var(--font-size-sm);
    font-family: var(--font-mono);
    color: var(--color-accent);
    cursor: pointer;
    transition: background-color 80ms var(--ease-out);
  }

  .step-scan__retry:hover {
    background-color: var(--color-accent-subtle);
  }

  .step-scan__retry:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
  }

  /* ── Empty ── */

  .step-scan__empty {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    font-size: var(--font-size-sm);
    font-family: var(--font-mono);
    color: var(--color-text-muted);
  }

  .step-scan__empty p {
    margin: 0;
  }

  .step-scan__empty-hint {
    color: var(--color-text-muted);
    font-size: 0.9em;
  }

  /* ── Project list ── */

  .step-scan__list {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .step-scan__project {
    border-radius: var(--radius-sm);
  }

  .step-scan__project-label {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-2) var(--space-3);
    border-radius: var(--radius-sm);
    cursor: pointer;
    transition: background-color 80ms var(--ease-out);
  }

  .step-scan__project-label:hover {
    background-color: var(--color-surface-overlay);
  }

  .step-scan__checkbox {
    accent-color: var(--color-accent);
    width: 14px;
    height: 14px;
    flex-shrink: 0;
    cursor: pointer;
  }

  .step-scan__project-info {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }

  .step-scan__project-name {
    font-size: var(--font-size-sm);
    font-family: var(--font-mono);
    font-weight: 500;
    color: var(--color-text-primary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .step-scan__project-path {
    font-size: 11px;
    font-family: var(--font-mono);
    color: var(--color-text-muted);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .step-scan__project-meta {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    font-size: 11px;
    font-family: var(--font-mono);
    color: var(--color-text-muted);
    flex-shrink: 0;
  }

  .step-scan__git-dot {
    display: inline-block;
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background-color: var(--color-accent);
  }

  .step-scan__selection-hint {
    margin: var(--space-3) 0 0;
    font-size: 11px;
    font-family: var(--font-mono);
    color: var(--color-text-muted);
  }

  /* ── Footer ── */

  .step-scan__footer {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    flex-shrink: 0;
  }

  /* ── Shared button tokens (used by all step components) ── */

  :global(.step-btn) {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    padding: 6px var(--space-4);
    border-radius: var(--radius-sm);
    font-size: var(--font-size-sm);
    font-family: var(--font-mono);
    font-weight: 500;
    cursor: pointer;
    border: 1px solid transparent;
    transition:
      background-color 80ms var(--ease-out),
      color 80ms var(--ease-out),
      border-color 80ms var(--ease-out);
    text-decoration: none;
  }

  :global(.step-btn:focus-visible) {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
  }

  :global(.step-btn:disabled) {
    opacity: 0.5;
    cursor: not-allowed;
  }

  :global(.step-btn--primary) {
    background-color: var(--color-accent);
    color: var(--color-accent-contrast, #fff);
    border-color: var(--color-accent);
  }

  :global(.step-btn--primary:hover:not(:disabled)) {
    background-color: var(--color-accent-hover, var(--color-accent));
  }

  :global(.step-btn--secondary) {
    background-color: var(--color-surface-overlay);
    color: var(--color-text-primary);
    border-color: var(--color-border);
  }

  :global(.step-btn--secondary:hover:not(:disabled)) {
    background-color: color-mix(in srgb, var(--color-surface-overlay) 80%, var(--color-border));
  }

  :global(.step-btn--ghost) {
    background: none;
    color: var(--color-text-muted);
    border-color: transparent;
  }

  :global(.step-btn--ghost:hover:not(:disabled)) {
    color: var(--color-text-secondary);
    background-color: var(--color-surface-overlay);
  }

  /* ── Reduced motion ── */

  @media (prefers-reduced-motion: reduce) {
    .step-scan__spinner {
      animation: none;
      opacity: 0.5;
    }
    .step-scan__project-label,
    .step-scan__retry {
      transition: none;
    }
  }
</style>
