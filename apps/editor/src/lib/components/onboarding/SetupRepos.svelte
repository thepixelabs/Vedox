<script lang="ts">
  /**
   * SetupRepos.svelte — Onboarding step 2.
   *
   * Two modes:
   *   a) Create a new bare-local doc repo (mkdir + git init via /api/repos/create)
   *   b) Register an existing folder as a doc repo (via /api/repos/register)
   *
   * Per founder override OQ-E: push/set-origin is deferred. Bare-local
   * is the default repo type. If the user skips, the inbox fallback at
   * ~/.vedox/inbox/ is used automatically by the daemon.
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
  // Local state
  // ---------------------------------------------------------------------------

  type Mode = 'choose' | 'create' | 'register';
  type ActionStatus = 'idle' | 'loading' | 'done' | 'error';

  let mode = $state<Mode>('choose');
  let actionStatus = $state<ActionStatus>('idle');
  let actionError = $state<string | null>(null);

  // Create mode
  let newRepoName = $state('');
  let newRepoParent = $state('~');

  // Register mode
  let existingPath = $state('');

  const newRepoPath = $derived(
    `${newRepoParent.replace(/\/+$/, '')}/${newRepoName.trim() || 'docs'}`
  );

  // ---------------------------------------------------------------------------
  // API calls
  // ---------------------------------------------------------------------------

  async function createRepo() {
    if (!newRepoName.trim()) return;
    actionStatus = 'loading';
    actionError = null;

    try {
      const res = await fetch('/api/repos/create', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: newRepoName.trim(),
          parent: newRepoParent,
          type: 'bare-local',
        }),
      });
      if (!res.ok) {
        const body = (await res.json().catch(() => ({}))) as Record<string, unknown>;
        throw new Error((body['message'] as string | undefined) ?? `${res.status}`);
      }
      const data = (await res.json()) as { path: string };
      onboardingStore.addRegisteredRepo(data.path);
      actionStatus = 'done';
      setTimeout(onNext, 600);
    } catch (err) {
      actionStatus = 'error';
      actionError = err instanceof Error ? err.message : String(err);
    }
  }

  async function registerRepo() {
    if (!existingPath.trim()) return;
    actionStatus = 'loading';
    actionError = null;

    try {
      const res = await fetch('/api/repos/register', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          path: existingPath.trim(),
          type: 'bare-local',
        }),
      });
      if (!res.ok) {
        const body = (await res.json().catch(() => ({}))) as Record<string, unknown>;
        throw new Error((body['message'] as string | undefined) ?? `${res.status}`);
      }
      const data = (await res.json()) as { path: string };
      onboardingStore.addRegisteredRepo(data.path);
      actionStatus = 'done';
      setTimeout(onNext, 600);
    } catch (err) {
      actionStatus = 'error';
      actionError = err instanceof Error ? err.message : String(err);
    }
  }

  function handleSubmit() {
    if (mode === 'create') {
      void createRepo();
    } else if (mode === 'register') {
      void registerRepo();
    }
  }

  const canSubmit = $derived(
    actionStatus !== 'loading' &&
    ((mode === 'create' && newRepoName.trim().length > 0) ||
     (mode === 'register' && existingPath.trim().length > 0))
  );

  const submitLabel = $derived(
    actionStatus === 'loading' ? 'working...' :
    actionStatus === 'done' ? 'done.' :
    mode === 'create' ? './create repo' : './register folder'
  );
</script>

<div class="step-repos">
  <header class="step-repos__header">
    <h2 class="step-repos__title">create or register a doc repo</h2>
    <p class="step-repos__desc">
      vedox keeps docs in a dedicated git repo — separate from your source code.
      create a new one or point at an existing folder.
    </p>
    <p class="step-repos__skip-hint">
      skip this step and docs land in <code>~/.vedox/inbox/</code> until you set one up.
    </p>
  </header>

  <!-- ── Mode chooser ─────────────────────────────────────────────────────── -->
  {#if mode === 'choose'}
    <div class="step-repos__choices">
      <button
        class="step-repos__choice"
        type="button"
        onclick={() => { mode = 'create'; }}
      >
        <span class="step-repos__choice-label">./create new repo</span>
        <span class="step-repos__choice-desc">
          pick a name and location — vedox runs git init for you
        </span>
      </button>
      <button
        class="step-repos__choice"
        type="button"
        onclick={() => { mode = 'register'; }}
      >
        <span class="step-repos__choice-label">./register existing folder</span>
        <span class="step-repos__choice-desc">
          paste the path to a folder already tracked by git
        </span>
      </button>
    </div>

    <footer class="step-repos__footer">
      <button class="step-btn step-btn--secondary" type="button" onclick={onBack}>
        ./back
      </button>
      <button class="step-btn step-btn--ghost" type="button" onclick={onSkip}>
        ./skip — use inbox
      </button>
    </footer>

  <!-- ── Create mode ──────────────────────────────────────────────────────── -->
  {:else if mode === 'create'}
    <div class="step-repos__form">
      <label class="step-repos__field">
        <span class="step-repos__field-label">repo name</span>
        <input
          class="step-repos__input"
          type="text"
          placeholder="my-docs"
          bind:value={newRepoName}
          disabled={actionStatus === 'loading' || actionStatus === 'done'}
          autocomplete="off"
          spellcheck={false}
          aria-label="New repo name"
        />
      </label>

      <label class="step-repos__field">
        <span class="step-repos__field-label">parent folder</span>
        <input
          class="step-repos__input"
          type="text"
          placeholder="~"
          bind:value={newRepoParent}
          disabled={actionStatus === 'loading' || actionStatus === 'done'}
          autocomplete="off"
          spellcheck={false}
          aria-label="Parent folder path"
        />
      </label>

      {#if newRepoName.trim()}
        <p class="step-repos__preview">will create: <code>{newRepoPath}</code></p>
      {/if}

      {#if actionStatus === 'error' && actionError}
        <p class="step-repos__error" role="alert">{actionError}</p>
      {/if}

      {#if actionStatus === 'done'}
        <p class="step-repos__success" role="status">repo created.</p>
      {/if}
    </div>

    <footer class="step-repos__footer">
      <button
        class="step-btn step-btn--primary"
        type="button"
        disabled={!canSubmit}
        onclick={handleSubmit}
      >
        {submitLabel}
      </button>
      <button
        class="step-btn step-btn--secondary"
        type="button"
        disabled={actionStatus === 'loading'}
        onclick={() => { mode = 'choose'; actionStatus = 'idle'; actionError = null; }}
      >
        ./back
      </button>
      <button
        class="step-btn step-btn--ghost"
        type="button"
        disabled={actionStatus === 'loading'}
        onclick={onSkip}
      >
        ./skip
      </button>
    </footer>

  <!-- ── Register mode ────────────────────────────────────────────────────── -->
  {:else if mode === 'register'}
    <div class="step-repos__form">
      <label class="step-repos__field">
        <span class="step-repos__field-label">folder path</span>
        <input
          class="step-repos__input"
          type="text"
          placeholder="/Users/me/my-existing-docs"
          bind:value={existingPath}
          disabled={actionStatus === 'loading' || actionStatus === 'done'}
          autocomplete="off"
          spellcheck={false}
          aria-label="Existing folder path"
        />
      </label>

      {#if actionStatus === 'error' && actionError}
        <p class="step-repos__error" role="alert">{actionError}</p>
      {/if}

      {#if actionStatus === 'done'}
        <p class="step-repos__success" role="status">folder registered.</p>
      {/if}
    </div>

    <footer class="step-repos__footer">
      <button
        class="step-btn step-btn--primary"
        type="button"
        disabled={!canSubmit}
        onclick={handleSubmit}
      >
        {submitLabel}
      </button>
      <button
        class="step-btn step-btn--secondary"
        type="button"
        disabled={actionStatus === 'loading'}
        onclick={() => { mode = 'choose'; actionStatus = 'idle'; actionError = null; }}
      >
        ./back
      </button>
      <button
        class="step-btn step-btn--ghost"
        type="button"
        disabled={actionStatus === 'loading'}
        onclick={onSkip}
      >
        ./skip
      </button>
    </footer>
  {/if}
</div>

<style>
  .step-repos {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
    min-height: 0;
  }

  /* ── Header ── */

  .step-repos__header {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .step-repos__title {
    margin: 0;
    font-size: var(--font-size-lg, 1.125rem);
    font-weight: 600;
    font-family: var(--font-mono);
    color: var(--color-text-primary);
  }

  .step-repos__desc,
  .step-repos__skip-hint {
    margin: 0;
    font-size: var(--font-size-sm);
    color: var(--color-text-secondary);
    line-height: 1.6;
    font-family: var(--font-mono);
  }

  .step-repos__skip-hint {
    color: var(--color-text-muted);
    font-size: 11px;
  }

  .step-repos__skip-hint code {
    font-family: var(--font-mono);
    background-color: var(--color-surface-overlay);
    padding: 1px 4px;
    border-radius: var(--radius-sm);
  }

  /* ── Choices ── */

  .step-repos__choices {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .step-repos__choice {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    padding: var(--space-4);
    background-color: var(--color-surface-overlay);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    text-align: left;
    cursor: pointer;
    transition: border-color 80ms var(--ease-out), background-color 80ms var(--ease-out);
  }

  .step-repos__choice:hover {
    border-color: var(--color-accent);
    background-color: var(--color-accent-subtle);
  }

  .step-repos__choice:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
  }

  .step-repos__choice-label {
    font-size: var(--font-size-sm);
    font-family: var(--font-mono);
    font-weight: 600;
    color: var(--color-accent);
  }

  .step-repos__choice-desc {
    font-size: 11px;
    font-family: var(--font-mono);
    color: var(--color-text-muted);
    line-height: 1.5;
  }

  /* ── Form ── */

  .step-repos__form {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .step-repos__field {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .step-repos__field-label {
    font-size: 11px;
    font-family: var(--font-mono);
    font-weight: 600;
    color: var(--color-text-muted);
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }

  .step-repos__input {
    width: 100%;
    padding: 7px var(--space-3);
    background-color: var(--color-surface-base);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    font-size: var(--font-size-sm);
    font-family: var(--font-mono);
    color: var(--color-text-primary);
    outline: none;
    transition: border-color 80ms var(--ease-out);
    box-sizing: border-box;
  }

  .step-repos__input:focus {
    border-color: var(--color-accent);
  }

  .step-repos__input:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  .step-repos__input::placeholder {
    color: var(--color-text-muted);
  }

  .step-repos__preview {
    margin: 0;
    font-size: 11px;
    font-family: var(--font-mono);
    color: var(--color-text-muted);
  }

  .step-repos__preview code {
    font-family: var(--font-mono);
    color: var(--color-accent);
  }

  .step-repos__error {
    margin: 0;
    font-size: var(--font-size-sm);
    font-family: var(--font-mono);
    color: var(--color-error, #e53e3e);
  }

  .step-repos__success {
    margin: 0;
    font-size: var(--font-size-sm);
    font-family: var(--font-mono);
    color: var(--color-success, #38a169);
  }

  /* ── Footer ── */

  .step-repos__footer {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    flex-wrap: wrap;
    flex-shrink: 0;
  }

  /* ── Reduced motion ── */

  @media (prefers-reduced-motion: reduce) {
    .step-repos__choice,
    .step-repos__input {
      transition: none;
    }
  }
</style>
