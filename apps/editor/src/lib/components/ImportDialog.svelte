<script lang="ts">
  /**
   * ImportDialog.svelte — modal dialog for importing or linking external projects.
   *
   * Two tabs:
   *   1. "Import & Migrate" (VDX-P2-D) — copies Markdown files into the Vedox
   *      workspace. Docs become fully editable inside Vedox.
   *   2. "Link (read-only)" (VDX-P2-F) — registers an external directory as a
   *      SymlinkAdapter. Docs stay in their original location and are rendered
   *      read-only in Vedox. Use Import to gain editing access.
   *
   * Accessibility:
   *   - role="dialog", aria-modal="true", aria-labelledby pointing at the heading.
   *   - Focus is trapped inside the dialog while it is open (Tab / Shift+Tab).
   *   - Escape closes the dialog.
   *   - Focus returns to the trigger element on close.
   *
   * Parent contract:
   *   - Bind `open` (boolean) to show/hide.
   *   - Pass `onImported` callback to refresh the project list after an import.
   *   - Pass `onLinked` callback to refresh the project list after a link.
   *   - Optionally pre-set `initialSrcPath` to pre-populate the path field.
   */

  import { onMount, onDestroy } from 'svelte';
  import { api, ApiError, type ImportResult, type LinkResult } from '$lib/api/client';
  import FolderPicker from './FolderPicker.svelte';

  // ---------------------------------------------------------------------------
  // Props
  // ---------------------------------------------------------------------------

  interface Props {
    /** Controls dialog visibility. */
    open: boolean;
    /**
     * Optional pre-populated source path, e.g. from a scanner result.
     * When set the path field is pre-filled and the name field is derived
     * from the last path segment automatically.
     */
    initialSrcPath?: string;
    /**
     * Called when the user clicks "Done" after a successful import.
     * The parent should use this to refresh the project list.
     */
    onImported?: (result: ImportResult) => void;
    /**
     * Called when the user clicks "Done" after a successful link.
     * The parent should use this to refresh the project list.
     */
    onLinked?: (result: LinkResult) => void;
  }

  let { open = $bindable(false), initialSrcPath = '', onImported, onLinked }: Props = $props();

  // ---------------------------------------------------------------------------
  // State — Picker
  // ---------------------------------------------------------------------------

  let pickerOpen: boolean = $state(false);
  let pickerTarget: 'import' | 'link' = $state('import');

  function openPicker(target: 'import' | 'link') {
    pickerTarget = target;
    pickerOpen = true;
  }

  function handlePathSelected(path: string) {
    srcPath = path;
    if (!projectNameTouched) {
      projectName = lastSegment(path);
    }
    pickerOpen = false;
  }

  // ---------------------------------------------------------------------------
  // State — tab
  // ---------------------------------------------------------------------------

  type Tab = 'import' | 'link';
  let activeTab: Tab = $state('import');

  // ---------------------------------------------------------------------------
  // State — import tab
  // ---------------------------------------------------------------------------

  type Phase = 'idle' | 'importing' | 'success' | 'error';

  let phase: Phase = $state('idle');
  let srcPath: string = $state('');
  let projectName: string = $state('');
  let importResult = $state<ImportResult | null>(null);
  let errorMessage: string = $state('');

  // ---------------------------------------------------------------------------
  // State — link tab
  // ---------------------------------------------------------------------------

  type LinkPhase = 'idle' | 'linking' | 'success' | 'error';

  let linkPhase: LinkPhase = $state('idle');
  let linkResult: LinkResult | null = $state(null);
  let linkErrorMessage: string = $state('');

  // DOM references for focus management.
  let dialogEl: HTMLElement | null = $state(null);
  let triggerEl: Element | null = null; // the element that opened the dialog

  // ---------------------------------------------------------------------------
  // Derived helpers
  // ---------------------------------------------------------------------------

  /** Derive the last path segment from srcPath for auto-populating projectName. */
  function lastSegment(p: string): string {
    const trimmed = p.replace(/[/\\]+$/, '');
    const parts = trimmed.split(/[/\\]/);
    return parts[parts.length - 1] ?? '';
  }

  // ---------------------------------------------------------------------------
  // Reactive: sync initialSrcPath → local fields when dialog opens
  // ---------------------------------------------------------------------------

  $effect(() => {
    if (open) {
      // Capture the currently focused element so we can restore focus on close.
      triggerEl = document.activeElement;

      // Pre-populate from initialSrcPath if provided.
      if (initialSrcPath && srcPath === '') {
        srcPath = initialSrcPath;
        projectName = lastSegment(initialSrcPath);
      }

      // Move focus into the dialog on the next tick.
      requestAnimationFrame(() => {
        const first = firstFocusable();
        first?.focus();
      });
    } else {
      // Restore focus to the trigger element on close.
      if (triggerEl instanceof HTMLElement) {
        triggerEl.focus();
      }
      // Reset state when dialog is dismissed.
      reset();
    }
  });

  // Auto-derive projectName from the typed path — but only when the user
  // has not manually edited projectName.
  let projectNameTouched = false;

  function handleSrcPathInput(e: Event) {
    srcPath = (e.target as HTMLInputElement).value;
    if (!projectNameTouched) {
      projectName = lastSegment(srcPath);
    }
  }

  function handleProjectNameInput(e: Event) {
    projectNameTouched = true;
    projectName = (e.target as HTMLInputElement).value;
  }

  // ---------------------------------------------------------------------------
  // Actions
  // ---------------------------------------------------------------------------

  async function handleSubmit(e: Event) {
    e.preventDefault();
    if (phase === 'importing') return;

    const trimSrc = srcPath.trim();
    const trimName = projectName.trim();
    if (!trimSrc || !trimName) return;

    phase = 'importing';
    errorMessage = '';

    try {
      const result = await api.importProject(trimSrc, trimName);
      importResult = result;
      phase = 'success';
    } catch (err) {
      phase = 'error';
      if (err instanceof ApiError) {
        errorMessage = `[${err.code}] ${err.message}`;
      } else if (err instanceof Error) {
        errorMessage = err.message;
      } else {
        errorMessage = 'An unexpected error occurred.';
      }
    }
  }

  function handleDone() {
    if (importResult && onImported) {
      onImported(importResult);
    }
    open = false;
  }

  function handleClose() {
    open = false;
  }

  function reset() {
    // Reset both tabs so the dialog is clean when it re-opens.
    activeTab = 'import';

    phase = 'idle';
    srcPath = '';
    projectName = '';
    projectNameTouched = false;
    importResult = null;
    errorMessage = '';

    linkPhase = 'idle';
    linkResult = null;
    linkErrorMessage = '';
  }

  // ---------------------------------------------------------------------------
  // Link tab handlers
  // ---------------------------------------------------------------------------

  function handleLinkExternalRootInput(e: Event) {
    srcPath = (e.target as HTMLInputElement).value;
    if (!projectNameTouched) {
      projectName = lastSegment(srcPath);
    }
  }

  function handleLinkProjectNameInput(e: Event) {
    projectNameTouched = true;
    projectName = (e.target as HTMLInputElement).value;
  }

  async function handleLinkSubmit(e: Event) {
    e.preventDefault();
    if (linkPhase === 'linking') return;

    const trimRoot = srcPath.trim();
    const trimName = projectName.trim();
    if (!trimRoot || !trimName) return;

    linkPhase = 'linking';
    linkErrorMessage = '';

    try {
      const result = await api.linkProject(trimRoot, trimName);
      linkResult = result;
      linkPhase = 'success';
    } catch (err) {
      linkPhase = 'error';
      if (err instanceof ApiError) {
        linkErrorMessage = `[${err.code}] ${err.message}`;
      } else if (err instanceof Error) {
        linkErrorMessage = err.message;
      } else {
        linkErrorMessage = 'An unexpected error occurred.';
      }
    }
  }

  function handleLinkDone() {
    if (linkResult && onLinked) {
      onLinked(linkResult);
    }
    open = false;
  }

  // ---------------------------------------------------------------------------
  // Keyboard + focus trap
  // ---------------------------------------------------------------------------

  function firstFocusable(): HTMLElement | null {
    if (!dialogEl) return null;
    const sel = 'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])';
    return dialogEl.querySelector<HTMLElement>(sel);
  }

  function lastFocusable(): HTMLElement | null {
    if (!dialogEl) return null;
    const sel = 'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])';
    const all = dialogEl.querySelectorAll<HTMLElement>(sel);
    return all[all.length - 1] ?? null;
  }

  function handleKeydown(e: KeyboardEvent) {
    if (!open) return;

    if (e.key === 'Escape') {
      e.preventDefault();
      handleClose();
      return;
    }

    // Focus trap: intercept Tab to keep focus inside the dialog.
    if (e.key === 'Tab') {
      const first = firstFocusable();
      const last = lastFocusable();
      if (!first || !last) return;

      if (e.shiftKey) {
        if (document.activeElement === first) {
          e.preventDefault();
          last.focus();
        }
      } else {
        if (document.activeElement === last) {
          e.preventDefault();
          first.focus();
        }
      }
    }
  }

  onMount(() => {
    document.addEventListener('keydown', handleKeydown);
  });

  onDestroy(() => {
    document.removeEventListener('keydown', handleKeydown);
  });

  // ---------------------------------------------------------------------------
  // Helpers for the template
  // ---------------------------------------------------------------------------

  const gitRemovalWarning = $derived(
    importResult?.warnings.find((w) => w.startsWith('Remember to commit')) ?? null
  );

  const indexingWarnings = $derived(
    importResult?.warnings.filter((w) => !w.startsWith('Remember to commit')) ?? []
  );
</script>

<!-- Backdrop -->
{#if open}
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="import-dialog__backdrop" onclick={handleClose}></div>

  <!-- Dialog -->
  <div
    bind:this={dialogEl}
    class="import-dialog"
    role="dialog"
    aria-modal="true"
    aria-labelledby="import-dialog-title"
  >
    <!-- Header -->
    <header class="import-dialog__header">
      <h2 class="import-dialog__title" id="import-dialog-title">Add Project</h2>
      <button
        class="import-dialog__close"
        type="button"
        aria-label="Close dialog"
        onclick={handleClose}
      >
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
          <line x1="18" y1="6" x2="6" y2="18"/>
          <line x1="6" y1="6" x2="18" y2="18"/>
        </svg>
      </button>
    </header>

    <!-- Tab switcher -->
    <div class="import-dialog__tabs" role="tablist" aria-label="Add project mode">
      <button
        class="import-dialog__tab"
        class:import-dialog__tab--active={activeTab === 'import'}
        role="tab"
        aria-selected={activeTab === 'import'}
        aria-controls="import-dialog-panel-import"
        type="button"
        onclick={() => (activeTab = 'import')}
      >
        Import &amp; Migrate
      </button>
      <button
        class="import-dialog__tab"
        class:import-dialog__tab--active={activeTab === 'link'}
        role="tab"
        aria-selected={activeTab === 'link'}
        aria-controls="import-dialog-panel-link"
        type="button"
        onclick={() => (activeTab = 'link')}
      >
        Link (read-only)
      </button>
    </div>

    <!-- ── Import & Migrate tab ─────────────────────────────────────────────── -->
    <div
      id="import-dialog-panel-import"
      role="tabpanel"
      aria-labelledby="tab-import"
      hidden={activeTab !== 'import'}
    >
    {#if phase === 'idle' || phase === 'importing' || phase === 'error'}
      <form class="import-dialog__form" onsubmit={handleSubmit} novalidate>
        <p class="import-dialog__description">
          Copies all Markdown files from a local project into your Vedox workspace
          and indexes them for search. The files become fully editable in Vedox.
        </p>

        <!-- Source path -->
        <div class="import-dialog__field">
          <label class="import-dialog__label" for="import-src-path">
            Project root path
          </label>
          <div class="import-dialog__input-group">
            <input
              id="import-src-path"
              class="import-dialog__input"
              type="text"
              placeholder="/Users/you/code/my-project"
              value={srcPath}
              oninput={handleSrcPathInput}
              disabled={phase === 'importing'}
              autocomplete="off"
              spellcheck={false}
              required
            />
            <button
              class="import-dialog__browse-btn"
              type="button"
              onclick={() => openPicker('import')}
              disabled={phase === 'importing'}
            >
              Browse...
            </button>
          </div>
          <p class="import-dialog__hint">Absolute path to the project directory on this machine.</p>
        </div>

        <!-- Project name -->
        <div class="import-dialog__field">
          <label class="import-dialog__label" for="import-project-name">
            Project name
          </label>
          <input
            id="import-project-name"
            class="import-dialog__input"
            type="text"
            placeholder="my-project"
            value={projectName}
            oninput={handleProjectNameInput}
            disabled={phase === 'importing'}
            autocomplete="off"
            spellcheck={false}
            required
          />
          <p class="import-dialog__hint">
            Sub-directory name inside the Vedox workspace. Auto-derived from the path above.
          </p>
        </div>

        <!-- Inline error -->
        {#if phase === 'error'}
          <p class="import-dialog__error" role="alert">{errorMessage}</p>
        {/if}

        <!-- Actions -->
        <div class="import-dialog__actions">
          <button
            class="import-dialog__submit"
            type="submit"
            disabled={phase === 'importing' || !srcPath.trim() || !projectName.trim()}
          >
            {#if phase === 'importing'}
              <span class="import-dialog__spinner" aria-hidden="true"></span>
              Importing...
            {:else}
              Import Documents
            {/if}
          </button>
          <button
            class="import-dialog__cancel"
            type="button"
            onclick={handleClose}
            disabled={phase === 'importing'}
          >
            Cancel
          </button>
        </div>
      </form>

    {:else if phase === 'success' && importResult}
      <!-- Success result -->
      <div class="import-dialog__result">
        <!-- Summary -->
        <p class="import-dialog__result-summary">
          <svg class="import-dialog__check" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <polyline points="20 6 9 17 4 12"/>
          </svg>
          Imported <strong>{importResult.imported.length}</strong>
          {importResult.imported.length === 1 ? 'document' : 'documents'} into
          <code>{projectName}</code>.
        </p>

        <!-- Skipped files -->
        {#if importResult.skipped.length > 0}
          <details class="import-dialog__details">
            <summary class="import-dialog__details-summary">
              {importResult.skipped.length} file{importResult.skipped.length === 1 ? '' : 's'} skipped
            </summary>
            <ul class="import-dialog__file-list">
              {#each importResult.skipped as f (f)}
                <li class="import-dialog__file-item">{f}</li>
              {/each}
            </ul>
          </details>
        {/if}

        <!-- Git removal warning — prominently styled -->
        {#if gitRemovalWarning}
          {@const hasMarkers = gitRemovalWarning.includes('TITLE:')}
          <div class="import-dialog__git-warning" role="note">
            {#if hasMarkers}
              {@const title = gitRemovalWarning.split('TITLE:')[1]?.split('\n')[0]?.trim()}
              {@const bodyAndCmd = gitRemovalWarning.split('BODY:')[1]}
              {@const body = bodyAndCmd?.split('COMMAND:')[0]?.trim()}
              {@const command = bodyAndCmd?.split('COMMAND:')[1]?.trim()}

              <p class="import-dialog__git-warning-heading">{title || 'Action required'}</p>
              {#if body}
                {#each body.split('\n\n') as para}
                  <p class="import-dialog__git-warning-body">{para}</p>
                {/each}
              {/if}
              {#if command}
                <div class="import-dialog__command-container">
                  <code class="import-dialog__command">{command}</code>
                  <button
                    class="import-dialog__copy-btn"
                    type="button"
                    title="Copy command"
                    onclick={() => navigator.clipboard.writeText(command)}
                  >
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                      <rect x="9" y="9" width="13" height="13" rx="2" ry="2"/>
                      <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>
                    </svg>
                  </button>
                </div>
              {/if}
            {:else}
              {@const parts = gitRemovalWarning.split('\nCOMMAND: ')}
              <p class="import-dialog__git-warning-heading">Action required</p>
              {#if parts.length === 2}
                <p class="import-dialog__git-warning-body">{parts[0]}</p>
                <div class="import-dialog__command-container">
                  <code class="import-dialog__command">{parts[1]}</code>
                  <button
                    class="import-dialog__copy-btn"
                    type="button"
                    title="Copy command"
                    onclick={() => navigator.clipboard.writeText(parts[1])}
                  >
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                      <rect x="9" y="9" width="13" height="13" rx="2" ry="2"/>
                      <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>
                    </svg>
                  </button>
                </div>
              {:else}
                <p class="import-dialog__git-warning-body">{gitRemovalWarning}</p>
              {/if}
            {/if}
          </div>
        {/if}

        <!-- Other indexing warnings -->
        {#each indexingWarnings as w (w)}
          {@const hasMarkers = w.includes('TITLE:')}
          <div class="import-dialog__warning-minor" role="note">
            {#if hasMarkers}
              {@const title = w.split('TITLE:')[1]?.split('\n')[0]?.trim()}
              {@const bodyAndCmd = w.split('BODY:')[1]}
              {@const body = bodyAndCmd?.split('COMMAND:')[0]?.trim()}
              {@const command = bodyAndCmd?.split('COMMAND:')[1]?.trim()}

              {#if title}<strong>{title}</strong>{/if}
              {#if body}
                {#each body.split('\n\n') as para}
                  <p>{para}</p>
                {/each}
              {/if}
              {#if command}
                <div class="import-dialog__command-container">
                  <code class="import-dialog__command">{command}</code>
                  <button
                    class="import-dialog__copy-btn"
                    type="button"
                    title="Copy command"
                    onclick={() => navigator.clipboard.writeText(command)}
                  >
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                      <rect x="9" y="9" width="13" height="13" rx="2" ry="2"/>
                      <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>
                    </svg>
                  </button>
                </div>
              {/if}
            {:else}
              {@const parts = w.split('\nCOMMAND: ')}
              {#if parts.length === 2}
                <p>{parts[0]}</p>
                <div class="import-dialog__command-container">
                  <code class="import-dialog__command">{parts[1]}</code>
                  <button
                    class="import-dialog__copy-btn"
                    type="button"
                    title="Copy command"
                    onclick={() => navigator.clipboard.writeText(parts[1])}
                  >
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                      <rect x="9" y="9" width="13" height="13" rx="2" ry="2"/>
                      <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>
                    </svg>
                  </button>
                </div>
              {:else}
                <p>{w}</p>
              {/if}
            {/if}
          </div>
        {/each}

        <!-- Done -->
        <div class="import-dialog__actions import-dialog__actions--result">
          <button class="import-dialog__submit" type="button" onclick={handleDone}>
            Done
          </button>
        </div>
      </div>
    {/if}
    </div>

    <!-- ── Link (read-only) tab ─────────────────────────────────────────────── -->
    <div
      id="import-dialog-panel-link"
      role="tabpanel"
      aria-labelledby="tab-link"
      hidden={activeTab !== 'link'}
    >
    {#if linkPhase === 'idle' || linkPhase === 'linking' || linkPhase === 'error'}
      <form class="import-dialog__form" onsubmit={handleLinkSubmit} novalidate>
        <div class="import-dialog__callout" role="note">
          <strong>Read-only.</strong> Docs stay in their original location. You can
          read and search them in Vedox, but editing requires
          <button
            type="button"
            class="import-dialog__callout-link"
            onclick={() => (activeTab = 'import')}
          >Import &amp; Migrate</button>.
        </div>

        <!-- External root path -->
        <div class="import-dialog__field">
          <label class="import-dialog__label" for="link-external-root">
            Project root path
          </label>
          <div class="import-dialog__input-group">
            <input
              id="link-external-root"
              class="import-dialog__input"
              type="text"
              placeholder="/Users/you/code/my-api"
              value={srcPath}
              oninput={handleLinkExternalRootInput}
              disabled={linkPhase === 'linking'}
              autocomplete="off"
              spellcheck={false}
              required
            />
            <button
              class="import-dialog__browse-btn"
              type="button"
              onclick={() => openPicker('link')}
              disabled={linkPhase === 'linking'}
            >
              Browse...
            </button>
          </div>
          <p class="import-dialog__hint">
            Absolute path to the external project. Must not be inside the Vedox workspace.
          </p>
        </div>

        <!-- Project name -->
        <div class="import-dialog__field">
          <label class="import-dialog__label" for="link-project-name">
            Project name
          </label>
          <input
            id="link-project-name"
            class="import-dialog__input"
            type="text"
            placeholder="my-api"
            value={projectName}
            oninput={handleLinkProjectNameInput}
            disabled={linkPhase === 'linking'}
            autocomplete="off"
            spellcheck={false}
            required
          />
          <p class="import-dialog__hint">
            How this project appears in the Vedox sidebar. Auto-derived from the path above.
          </p>
        </div>

        <!-- Inline error -->
        {#if linkPhase === 'error'}
          <p class="import-dialog__error" role="alert">{linkErrorMessage}</p>
        {/if}

        <!-- Actions -->
        <div class="import-dialog__actions">
          <button
            class="import-dialog__submit"
            type="submit"
            disabled={linkPhase === 'linking' || !srcPath.trim() || !projectName.trim()}
          >
            {#if linkPhase === 'linking'}
              <span class="import-dialog__spinner" aria-hidden="true"></span>
              Linking...
            {:else}
              Link Project
            {/if}
          </button>
          <button
            class="import-dialog__cancel"
            type="button"
            onclick={handleClose}
            disabled={linkPhase === 'linking'}
          >
            Cancel
          </button>
        </div>
      </form>

    {:else if linkPhase === 'success' && linkResult}
      <!-- Success result -->
      <div class="import-dialog__result">
        <p class="import-dialog__result-summary">
          <svg class="import-dialog__check" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <polyline points="20 6 9 17 4 12"/>
          </svg>
          Linked <strong>{linkResult.docCount}</strong>
          {linkResult.docCount === 1 ? 'document' : 'documents'} from
          <code>{linkResult.projectName}</code>
          {#if linkResult.framework !== 'unknown'}
            <span class="import-dialog__framework-badge">{linkResult.framework}</span>
          {/if}
        </p>
        <p class="import-dialog__description">
          This project is read-only in Vedox. To edit docs, switch to the
          <button
            type="button"
            class="import-dialog__callout-link"
            onclick={() => (activeTab = 'import')}
          >Import &amp; Migrate</button> tab.
        </p>
        <div class="import-dialog__actions import-dialog__actions--result">
          <button class="import-dialog__submit" type="button" onclick={handleLinkDone}>
            Done
          </button>
        </div>
      </div>
    {/if}
    </div>
  </div>

  {#if pickerOpen}
    <div class="import-dialog__picker-overlay">
      <div class="import-dialog__picker-card">
        <header class="import-dialog__header">
          <h2 class="import-dialog__title">Select Project Folder</h2>
          <button
            class="import-dialog__close"
            type="button"
            aria-label="Close picker"
            onclick={() => (pickerOpen = false)}
          >
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
              <line x1="18" y1="6" x2="6" y2="18"/>
              <line x1="6" y1="6" x2="18" y2="18"/>
            </svg>
          </button>
        </header>
        <FolderPicker
          initialPath={srcPath}
          onSelect={handlePathSelected}
          onCancel={() => (pickerOpen = false)}
        />
      </div>
    </div>
  {/if}
{/if}

<style>
  /* ── Backdrop ─────────────────────────────────────────────────────────────── */

  .import-dialog__backdrop {
    position: fixed;
    inset: 0;
    background-color: rgba(0, 0, 0, 0.45);
    z-index: 100;
    /* Fade-in */
    animation: backdrop-in 150ms ease both;
  }

  @keyframes backdrop-in {
    from { opacity: 0; }
    to   { opacity: 1; }
  }

  /* ── Dialog shell ─────────────────────────────────────────────────────────── */

  .import-dialog {
    position: fixed;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    z-index: 101;

    width: min(560px, calc(100vw - var(--space-8)));
    max-height: calc(100vh - var(--space-8) * 2);
    overflow-y: auto;

    background-color: var(--color-surface-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-xl, var(--radius-lg));
    box-shadow: var(--shadow-lg);

    /* Slide-up entrance */
    animation: dialog-in 180ms ease both;
  }

  @keyframes dialog-in {
    from {
      opacity: 0;
      transform: translate(-50%, calc(-50% + 12px));
    }
    to {
      opacity: 1;
      transform: translate(-50%, -50%);
    }
  }

  /* ── Header ───────────────────────────────────────────────────────────────── */

  .import-dialog__header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--space-5) var(--space-6);
    border-bottom: 1px solid var(--color-border);
    flex-shrink: 0;
  }

  .import-dialog__title {
    font-size: var(--font-size-lg);
    font-weight: 600;
    color: var(--color-text-primary);
    letter-spacing: -0.01em;
    margin: 0;
  }

  .import-dialog__close {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 28px;
    height: 28px;
    padding: 0;
    background: transparent;
    border: none;
    border-radius: var(--radius-sm);
    color: var(--color-text-muted);
    cursor: pointer;
    transition: color var(--duration-fast) var(--ease-out), background-color var(--duration-fast) var(--ease-out);
  }

  .import-dialog__close:hover {
    color: var(--color-text-primary);
    background-color: var(--color-surface-overlay);
  }

  .import-dialog__close:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
  }

  /* ── Tab switcher ─────────────────────────────────────────────────────────── */

  .import-dialog__tabs {
    display: flex;
    border-bottom: 1px solid var(--color-border);
    padding: 0 var(--space-6);
    gap: var(--space-1);
    flex-shrink: 0;
  }

  .import-dialog__tab {
    padding: var(--space-3) var(--space-1);
    font-size: var(--font-size-sm);
    font-weight: 500;
    font-family: var(--font-sans);
    color: var(--color-text-muted);
    background: transparent;
    border: none;
    border-bottom: 2px solid transparent;
    cursor: pointer;
    transition: color var(--duration-fast) var(--ease-out), border-color var(--duration-fast) var(--ease-out);
    white-space: nowrap;
    /* Offset the bottom border so it sits on the container border. */
    margin-bottom: -1px;
  }

  .import-dialog__tab:hover {
    color: var(--color-text-secondary);
  }

  .import-dialog__tab--active {
    color: var(--color-text-primary);
    border-bottom-color: var(--color-accent);
  }

  .import-dialog__tab:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
    border-radius: var(--radius-sm);
  }

  /* ── Read-only callout ────────────────────────────────────────────────────── */

  .import-dialog__callout {
    font-size: var(--font-size-sm);
    color: var(--color-text-secondary);
    background-color: color-mix(in srgb, var(--color-accent) 8%, transparent);
    border: 1px solid color-mix(in srgb, var(--color-accent) 20%, transparent);
    border-radius: var(--radius-md);
    padding: var(--space-3) var(--space-4);
    line-height: 1.55;
  }

  .import-dialog__callout strong {
    color: var(--color-text-primary);
  }

  .import-dialog__callout-link {
    background: none;
    border: none;
    padding: 0;
    font: inherit;
    color: var(--color-accent);
    cursor: pointer;
    text-decoration: underline;
    text-underline-offset: 2px;
  }

  .import-dialog__callout-link:hover {
    opacity: 0.8;
  }

  /* ── Framework badge ──────────────────────────────────────────────────────── */

  .import-dialog__framework-badge {
    display: inline-block;
    padding: 1px var(--space-2);
    font-size: var(--font-size-xs, 0.75rem);
    font-weight: 500;
    background-color: var(--color-surface-overlay);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    color: var(--color-text-secondary);
    margin-left: var(--space-1);
    vertical-align: middle;
  }

  /* ── Form ─────────────────────────────────────────────────────────────────── */

  .import-dialog__form,
  .import-dialog__result {
    padding: var(--space-6);
    display: flex;
    flex-direction: column;
    gap: var(--space-5);
  }

  .import-dialog__description {
    font-size: var(--font-size-sm);
    color: var(--color-text-secondary);
    line-height: 1.55;
    margin: 0;
  }

  .import-dialog__field {
    display: flex;
    flex-direction: column;
    gap: var(--space-1-5, var(--space-2));
  }

  .import-dialog__label {
    font-size: var(--font-size-sm);
    font-weight: 500;
    color: var(--color-text-primary);
  }

  .import-dialog__input {
    width: 100%;
    padding: var(--space-2) var(--space-3);
    font-size: var(--font-size-sm);
    font-family: var(--font-mono);
    color: var(--color-text-primary);
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    transition: border-color var(--duration-fast) var(--ease-out), box-shadow var(--duration-fast) var(--ease-out);
    box-sizing: border-box;
  }

  .import-dialog__input-group {
    display: flex;
    gap: var(--space-2);
  }

  .import-dialog__browse-btn {
    padding: var(--space-2) var(--space-4);
    font-size: var(--font-size-sm);
    font-weight: 500;
    color: var(--color-text-secondary);
    background-color: var(--color-surface-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    cursor: pointer;
    transition: background-color var(--duration-fast) var(--ease-out), color var(--duration-fast) var(--ease-out), border-color var(--duration-fast) var(--ease-out);
    white-space: nowrap;
  }

  .import-dialog__browse-btn:hover:not(:disabled) {
    border-color: var(--color-border-strong);
    color: var(--color-text-primary);
    background-color: var(--color-surface-overlay);
  }

  .import-dialog__browse-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .import-dialog__input:focus {
    outline: none;
    border-color: var(--color-accent);
    box-shadow: 0 0 0 3px color-mix(in srgb, var(--color-accent) 18%, transparent);
  }

  .import-dialog__input:disabled {
    opacity: 0.55;
    cursor: not-allowed;
  }

  .import-dialog__hint {
    font-size: var(--font-size-xs, 0.75rem);
    color: var(--color-text-muted);
    margin: 0;
    line-height: 1.4;
  }

  /* ── Inline error ─────────────────────────────────────────────────────────── */

  .import-dialog__error {
    font-size: var(--font-size-sm);
    color: var(--color-error);
    background-color: color-mix(in srgb, var(--color-error) 10%, transparent);
    border: 1px solid color-mix(in srgb, var(--color-error) 30%, transparent);
    border-radius: var(--radius-md);
    padding: var(--space-3) var(--space-4);
    margin: 0;
    line-height: 1.5;
  }

  /* ── Actions ──────────────────────────────────────────────────────────────── */

  .import-dialog__actions {
    display: flex;
    gap: var(--space-3);
    align-items: center;
    padding-top: var(--space-2);
  }

  .import-dialog__actions--result {
    padding-top: var(--space-1);
  }

  .import-dialog__submit {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-5);
    font-size: var(--font-size-sm);
    font-weight: 500;
    font-family: var(--font-sans);
    color: var(--color-text-inverse);
    background-color: var(--color-accent);
    border: 1px solid transparent;
    border-radius: var(--radius-md);
    cursor: pointer;
    transition: background-color var(--duration-fast) var(--ease-out);
    white-space: nowrap;
  }

  .import-dialog__submit:hover:not(:disabled) {
    background-color: var(--color-accent-hover);
  }

  .import-dialog__submit:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .import-dialog__submit:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 3px;
  }

  .import-dialog__cancel {
    padding: var(--space-2) var(--space-4);
    font-size: var(--font-size-sm);
    font-weight: 500;
    font-family: var(--font-sans);
    color: var(--color-text-secondary);
    background-color: transparent;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    cursor: pointer;
    transition: border-color var(--duration-fast) var(--ease-out), color var(--duration-fast) var(--ease-out);
  }

  .import-dialog__cancel:hover:not(:disabled) {
    border-color: var(--color-border-strong);
    color: var(--color-text-primary);
  }

  .import-dialog__cancel:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .import-dialog__cancel:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 3px;
  }

  /* ── Spinner (inside submit button) ──────────────────────────────────────── */

  .import-dialog__spinner {
    display: inline-block;
    width: 14px;
    height: 14px;
    border: 2px solid rgba(255, 255, 255, 0.35);
    border-top-color: #fff;
    border-radius: 50%;
    animation: import-spin 650ms linear infinite;
    flex-shrink: 0;
  }

  @keyframes import-spin {
    to { transform: rotate(360deg); }
  }

  /* ── Success result ───────────────────────────────────────────────────────── */

  .import-dialog__result-summary {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font-size: var(--font-size-base);
    color: var(--color-text-primary);
    margin: 0;
    line-height: 1.5;
  }

  .import-dialog__check {
    color: var(--color-success, #22c55e);
    flex-shrink: 0;
  }

  .import-dialog__result-summary code {
    font-family: var(--font-mono);
    font-size: 0.9em;
    background-color: var(--color-surface-overlay);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    padding: 0 var(--space-1);
  }

  /* ── Skipped files details ────────────────────────────────────────────────── */

  .import-dialog__details {
    font-size: var(--font-size-sm);
    color: var(--color-text-secondary);
  }

  .import-dialog__details-summary {
    cursor: pointer;
    color: var(--color-text-secondary);
    font-weight: 500;
    padding: var(--space-1) 0;
  }

  .import-dialog__details-summary:hover {
    color: var(--color-text-primary);
  }

  .import-dialog__file-list {
    list-style: none;
    padding: var(--space-2) var(--space-3);
    margin: var(--space-2) 0 0;
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    max-height: 160px;
    overflow-y: auto;
  }

  .import-dialog__file-item {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs, 0.75rem);
    color: var(--color-text-muted);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  /* ── Git removal warning ──────────────────────────────────────────────────── */

  .import-dialog__git-warning {
    background-color: color-mix(in srgb, var(--color-warning, #f59e0b) 8%, transparent);
    border: 1px solid color-mix(in srgb, var(--color-warning, #f59e0b) 25%, transparent);
    border-radius: var(--radius-lg);
    display: flex;
    flex-direction: column;
    overflow: hidden;
    margin-top: var(--space-2);
  }

  .import-dialog__git-warning-heading {
    font-size: var(--font-size-sm);
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--color-warning, #f59e0b);
    background-color: color-mix(in srgb, var(--color-warning, #f59e0b) 12%, transparent);
    padding: var(--space-2) var(--space-4);
    margin: 0;
    border-bottom: 1px solid color-mix(in srgb, var(--color-warning, #f59e0b) 20%, transparent);
  }

  .import-dialog__git-warning-body {
    font-size: var(--font-size-sm);
    color: var(--color-text-primary);
    margin: 0;
    line-height: 1.6;
    padding: var(--space-4) var(--space-4) var(--space-2);
  }

  .import-dialog__command-container {
    position: relative;
    margin: 0 var(--space-4) var(--space-4);
    display: flex;
    flex-direction: column;
  }

  .import-dialog__command {
    font-family: var(--font-mono);
    font-size: 12px;
    background-color: #000;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: var(--space-3);
    padding-right: var(--space-10);
    color: #e2e8f0; /* light gray/white for contrast on black */
    white-space: pre-wrap;
    word-break: break-all;
    display: block;
    line-height: 1.6;
    max-height: 160px;
    overflow-y: auto;
  }

  .import-dialog__copy-btn {
    position: absolute;
    top: var(--space-2);
    right: var(--space-2);
    width: 32px;
    height: 32px;
    display: flex;
    align-items: center;
    justify-content: center;
    background-color: rgba(255, 255, 255, 0.1);
    border: 1px solid rgba(255, 255, 255, 0.2);
    border-radius: var(--radius-sm);
    color: #cbd5e1;
    cursor: pointer;
    transition: color var(--duration-fast) var(--ease-out), background-color var(--duration-fast) var(--ease-out), border-color var(--duration-fast) var(--ease-out);
    z-index: 2;
  }

  .import-dialog__copy-btn:hover {
    color: #fff;
    background-color: rgba(255, 255, 255, 0.2);
    border-color: rgba(255, 255, 255, 0.4);
  }

  .import-dialog__copy-btn:active {
    transform: scale(0.95);
  }

  /* ── Minor indexing warnings ──────────────────────────────────────────────── */

  .import-dialog__warning-minor {
    font-size: var(--font-size-sm);
    color: var(--color-text-secondary);
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: var(--space-3) var(--space-4);
    margin: 0;
    line-height: 1.5;
  }

  /* ── Picker Overlay ───────────────────────────────────────────────────────── */

  .import-dialog__picker-overlay {
    position: fixed;
    inset: 0;
    background-color: rgba(0, 0, 0, 0.45);
    z-index: 120;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: var(--space-4);
    animation: backdrop-in 150ms ease both;
  }

  .import-dialog__picker-card {
    width: min(440px, calc(100vw - var(--space-8))); /* Narrower "hamburgery" shape */
    background-color: var(--color-surface-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-xl);
    box-shadow: var(--shadow-xl);
    display: flex;
    flex-direction: column;
    animation: dialog-in 180ms ease both;
  }
</style>
