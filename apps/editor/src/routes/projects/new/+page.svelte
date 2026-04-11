<script lang="ts">
  import { goto, beforeNavigate } from '$app/navigation';
  import { api, ApiError } from '$lib/api/client';
  import AiNamePanel from '$lib/components/AiNamePanel.svelte';
  import { draft, saveDraft, clearDraft, defaultAiPanelDraft, type WizardAiPanelDraft } from '$lib/stores/wizardDraft';

  // ── Form state ─────────────────────────────────────────────────────────────
  let projectName = $state('');
  let tagline = $state('');
  let description = $state('');
  let nameInput: HTMLInputElement | undefined = $state();

  // ── Submission state ────────────────────────────────────────────────────────
  let submitting = $state(false);
  let submitError = $state('');

  // ── AI panel open state ─────────────────────────────────────────────────────
  let aiPanelOpen = $state(false);

  // ── AI panel state (lifted for persistence) ─────────────────────────────────
  let aiPanelDraft = $state<WizardAiPanelDraft>(defaultAiPanelDraft());

  // ── Draft restoration tracking ──────────────────────────────────────────────
  let restoredFromDraft = $state(false);

  // Hydrate form from an existing draft when the user navigates back.
  $effect(() => {
    const d = $draft;
    if (d !== null) {
      projectName = d.projectName;
      tagline = d.tagline;
      description = d.description;
      aiPanelDraft = { ...d.aiPanel };
      aiPanelOpen = d.aiPanel.open;
      restoredFromDraft = true;
    }
    // Auto-focus the name input on mount regardless.
    if (nameInput) nameInput.focus();
  });

  // Persist state to the draft store before every navigation away from this page.
  beforeNavigate(({ to }) => {
    // Navigating to the same page (e.g. submit error keeps user here) — skip.
    if (to?.url.pathname === '/projects/new') return;
    // Only save if there's something meaningful to preserve.
    const hasContent =
      projectName.trim() ||
      tagline.trim() ||
      description.trim() ||
      aiPanelDraft.generatedNames.length > 0;
    if (hasContent) {
      saveDraft({
        projectName,
        tagline,
        description,
        aiPanel: { ...aiPanelDraft, open: aiPanelOpen },
      });
    }
  });

  async function handleSubmit() {
    const name = projectName.trim();
    if (!name) {
      submitError = 'Project name is required.';
      nameInput?.focus();
      return;
    }

    submitting = true;
    submitError = '';

    try {
      const result = await api.createProject(
        name,
        tagline.trim() || undefined,
        description.trim() || undefined,
      );
      // Clear the draft before navigating so beforeNavigate doesn't re-save.
      clearDraft();
      await goto(`/projects/${encodeURIComponent(result.name)}`);
    } catch (err) {
      submitting = false;
      if (err instanceof ApiError) {
        if (err.code === 'VDX-301') {
          submitError = `A project named "${name}" already exists.`;
        } else if (err.code === 'VDX-300') {
          submitError = 'Invalid project name. Use only letters, numbers, hyphens, and underscores.';
        } else {
          submitError = err.message;
        }
      } else {
        submitError = 'Failed to create project. Is vedox dev running?';
      }
    }
  }

  function handleAINameSelected(name: string) {
    projectName = name;
  }

  function handleAiPanelStateChange(state: WizardAiPanelDraft) {
    aiPanelDraft = state;
  }
</script>

<svelte:head>
  <title>New Project — Vedox</title>
</svelte:head>

<div class="wizard-page" class:wizard-page--from-draft={restoredFromDraft}>
  <!-- Left column: context -->
  <div class="wizard-page__context">
    <div class="wizard-page__context-inner">
      <div class="wizard-context-mark" aria-hidden="true">
        <svg width="48" height="48" viewBox="0 0 48 48" fill="none" xmlns="http://www.w3.org/2000/svg">
          <circle cx="24" cy="24" r="22" stroke="currentColor" stroke-width="1.5" opacity="0.2"/>
          <path d="M16 24 L24 14 L32 24 L24 34 Z" stroke="currentColor" stroke-width="1.5" fill="none" opacity="0.6"/>
          <circle cx="24" cy="24" r="3" fill="currentColor"/>
        </svg>
      </div>
      <h1 class="wizard-context-headline">
        Every great project starts with a name.
      </h1>
      <p class="wizard-context-sub">
        Give your documentation project an identity. You can always change it later.
      </p>
    </div>
  </div>

  <!-- Right column: form -->
  <div class="wizard-page__form">
    <a href="/projects" class="wizard-back" aria-label="Back to projects">
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        <line x1="19" y1="12" x2="5" y2="12"/>
        <polyline points="12 19 5 12 12 5"/>
      </svg>
      All projects
    </a>

    <form class="wizard-form" onsubmit={(e) => { e.preventDefault(); handleSubmit(); }}>

      <!-- Hero: project name -->
      <div class="wizard-field wizard-field--hero">
        <label class="wizard-field__label" for="project-name">Project name *</label>
        <input
          id="project-name"
          bind:this={nameInput}
          type="text"
          class="wizard-field__input"
          placeholder="My Documentation"
          bind:value={projectName}
          aria-required="true"
          aria-describedby={submitError ? 'wizard-error' : undefined}
          autocomplete="off"
          spellcheck="false"
        />
      </div>

      <!-- Tagline -->
      <div class="wizard-field">
        <label class="wizard-field__label" for="project-tagline">Tagline <span class="wizard-field__optional">optional</span></label>
        <input
          id="project-tagline"
          type="text"
          class="wizard-field__input wizard-field__input--tagline"
          placeholder="A brief description of what this documents"
          bind:value={tagline}
          autocomplete="off"
        />
      </div>

      <!-- Description -->
      <div class="wizard-field">
        <label class="wizard-field__label" for="project-description">Description <span class="wizard-field__optional">optional</span></label>
        <textarea
          id="project-description"
          class="wizard-field__textarea"
          placeholder="What is this project about? Who is it for?"
          bind:value={description}
          rows="3"
        ></textarea>
      </div>

      <!-- AI Name Help: expandable disclosure -->
      <details class="wizard-ai-expand" bind:open={aiPanelOpen}>
        <summary class="wizard-ai-expand__trigger">
          <span class="wizard-ai-expand__icon" aria-hidden="true">✦</span>
          Get AI Help with Name
          <svg class="wizard-ai-expand__chevron" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <polyline points="6 9 12 15 18 9"/>
          </svg>
        </summary>
        <div class="wizard-ai-expand__panel">
          <AiNamePanel
            onNameSelected={handleAINameSelected}
            initialState={aiPanelDraft}
            onStateChange={handleAiPanelStateChange}
          />
        </div>
      </details>

      <!-- Error message -->
      {#if submitError}
        <p class="wizard-error" id="wizard-error" role="alert">{submitError}</p>
      {/if}

      <!-- Submit -->
      <button
        type="submit"
        class="wizard-submit"
        disabled={submitting || !projectName.trim()}
        aria-busy={submitting}
      >
        {#if submitting}
          <span class="wizard-submit__spinner" aria-hidden="true"></span>
          Creating…
        {:else}
          Create Project
        {/if}
      </button>

    </form>
  </div>
</div>

<style>
  .wizard-page {
    min-height: 100vh;
    display: grid;
    grid-template-columns: 1fr 1fr;
    max-width: 1120px;
    margin: 0 auto;
  }

  /* Restore animation: plays when user returns to wizard from the draft card */
  .wizard-page--from-draft {
    animation: wizard-restore 320ms var(--ease-spring, cubic-bezier(0.34, 1.56, 0.64, 1)) both;
  }

  @keyframes wizard-restore {
    from {
      opacity: 0;
      transform: scale(0.97) translateY(6px);
      transform-origin: top right;
    }
    to {
      opacity: 1;
      transform: none;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .wizard-page--from-draft {
      animation: none;
    }
  }

  @media (max-width: 767px) {
    .wizard-page {
      grid-template-columns: 1fr;
    }

    .wizard-page__context {
      display: none;
    }
  }

  /* ── Left context column ───────────────────────────────────────────────────── */

  .wizard-page__context {
    padding: var(--space-11) var(--space-9);
    display: flex;
    flex-direction: column;
    justify-content: center;
    border-right: 1px solid var(--border-hairline);
    color: var(--text-1);
  }

  .wizard-page__context-inner {
    display: flex;
    flex-direction: column;
    gap: var(--space-5);
    max-width: 360px;
  }

  .wizard-context-mark {
    color: var(--accent-solid);
  }

  .wizard-context-headline {
    font-family: var(--font-display);
    font-optical-sizing: auto;
    font-variation-settings: "opsz" 48, "wght" 700;
    font-size: var(--text-4xl);
    line-height: 1.05;
    letter-spacing: -0.025em;
    color: var(--text-1);
    max-width: 14ch;
    font-feature-settings: "kern" 1, "liga" 1, "dlig" 1;
  }

  .wizard-context-sub {
    font-size: var(--text-lg);
    color: var(--text-3);
    max-width: 30ch;
    line-height: var(--leading-normal);
  }

  /* ── Right form column ─────────────────────────────────────────────────────── */

  .wizard-page__form {
    padding: var(--space-9);
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
    overflow-y: auto;
  }

  .wizard-back {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    font-size: var(--text-sm);
    color: var(--text-3);
    text-decoration: none;
    transition: color var(--duration-fast) var(--ease-out);
    align-self: flex-start;
    margin-bottom: calc(-1 * var(--space-2));
  }

  .wizard-back:hover {
    color: var(--text-1);
  }

  /* ── Fields ───────────────────────────────────────────────────────────────── */

  .wizard-form {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
  }

  .wizard-field {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .wizard-field__label {
    font-size: 11px;
    font-weight: 600;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--text-3);
  }

  .wizard-field__optional {
    font-weight: 400;
    letter-spacing: 0;
    text-transform: none;
    font-size: 10px;
    color: var(--text-4);
  }

  /* Hero name input — underline only */
  .wizard-field--hero .wizard-field__input {
    font-size: clamp(26px, calc(26px + 0.882vw), 34px);
    font-weight: 500;
    font-family: var(--font-body);
    background: none;
    border: none;
    border-bottom: 1.5px solid var(--border-default);
    border-radius: 0;
    padding: var(--space-3) 0;
    color: var(--text-1);
    width: 100%;
    transition: border-color var(--duration-default) var(--ease-out);
    caret-color: var(--accent-solid);
  }

  .wizard-field--hero .wizard-field__input::placeholder {
    color: var(--text-4);
  }

  .wizard-field--hero .wizard-field__input:focus {
    outline: none;
    border-bottom-color: var(--accent-solid);
  }

  /* Tagline input — underline only, smaller */
  .wizard-field__input--tagline {
    font-size: var(--text-xl);
    font-weight: 400;
    font-family: var(--font-body);
    background: none;
    border: none;
    border-bottom: 1px solid var(--border-default);
    border-radius: 0;
    padding: var(--space-2) 0;
    color: var(--text-2);
    width: 100%;
    transition: border-color var(--duration-default) var(--ease-out);
    caret-color: var(--accent-solid);
  }

  .wizard-field__input--tagline::placeholder { color: var(--text-4); }
  .wizard-field__input--tagline:focus {
    outline: none;
    border-bottom-color: var(--accent-solid);
  }

  /* Description textarea — box style */
  .wizard-field__textarea {
    font-size: var(--text-base);
    font-family: var(--font-body);
    background: var(--surface-2);
    border: 1px solid var(--border-default);
    border-radius: var(--radius-md);
    padding: var(--space-3) var(--space-4);
    color: var(--text-1);
    resize: vertical;
    min-height: 88px;
    width: 100%;
    line-height: var(--leading-normal);
    transition: border-color var(--duration-default) var(--ease-out),
                box-shadow var(--duration-default) var(--ease-out);
  }

  .wizard-field__textarea::placeholder { color: var(--text-4); }
  .wizard-field__textarea:focus {
    outline: none;
    border-color: var(--accent-solid);
    box-shadow: 0 0 0 3px var(--accent-subtle);
  }

  /* ── AI expand ────────────────────────────────────────────────────────────── */

  .wizard-ai-expand {
    border: 1px solid var(--border-default);
    border-radius: var(--radius-lg);
    background: var(--surface-2);
    overflow: hidden;
    transition: border-color var(--duration-fast) var(--ease-out);
  }

  .wizard-ai-expand[open] {
    border-color: var(--accent-border);
  }

  .wizard-ai-expand__trigger {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-3) var(--space-4);
    cursor: pointer;
    font-size: var(--text-base);
    font-weight: 500;
    color: var(--text-2);
    list-style: none;
    user-select: none;
    transition: color var(--duration-fast) var(--ease-out);
  }

  .wizard-ai-expand__trigger::-webkit-details-marker { display: none; }

  .wizard-ai-expand__trigger:hover {
    color: var(--text-1);
  }

  .wizard-ai-expand[open] .wizard-ai-expand__trigger {
    color: var(--accent-text);
    border-bottom: 1px solid var(--border-hairline);
  }

  .wizard-ai-expand__icon {
    color: var(--accent-solid);
    font-size: 16px;
  }

  .wizard-ai-expand__chevron {
    margin-left: auto;
    transition: transform var(--duration-default) var(--ease-in-out);
  }

  .wizard-ai-expand[open] .wizard-ai-expand__chevron {
    transform: rotate(180deg);
  }

  .wizard-ai-expand__panel {
    padding: 0 var(--space-4) var(--space-4);
  }

  /* ── Error ────────────────────────────────────────────────────────────────── */

  .wizard-error {
    font-size: var(--text-sm);
    color: var(--error);
    padding: var(--space-3) var(--space-4);
    background: oklch(70% 0.18 25 / 0.1);
    border: 1px solid oklch(70% 0.18 25 / 0.25);
    border-radius: var(--radius-md);
  }

  /* ── Submit button ────────────────────────────────────────────────────────── */

  .wizard-submit {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-2);
    width: 100%;
    padding: var(--space-4) var(--space-8);
    background: var(--accent-solid);
    color: var(--accent-contrast);
    font-size: 13px;
    font-weight: 600;
    font-family: var(--font-body);
    letter-spacing: 0.04em;
    border: none;
    border-radius: var(--radius-lg);
    cursor: pointer;
    transition:
      background-color var(--duration-fast) var(--ease-out),
      transform var(--duration-fast) var(--ease-snap, cubic-bezier(0.85, 0, 0.15, 1)),
      box-shadow var(--duration-fast) var(--ease-out);
  }

  .wizard-submit:hover:not(:disabled) {
    background: var(--accent-solid-hover);
    box-shadow: 0 4px 16px var(--accent-subtle);
    transform: translateY(-1px);
  }

  .wizard-submit:active:not(:disabled) {
    transform: translateY(0);
  }

  .wizard-submit:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .wizard-submit__spinner {
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
</style>
