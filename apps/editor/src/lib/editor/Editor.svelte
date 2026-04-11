<!--
  Editor.svelte

  Root dual-mode editor component for Vedox.

  Modes:
    - "code"     — Raw Markdown via CodeMirror 6
    - "wysiwyg"  — WYSIWYG via Tiptap + prosemirror-markdown

  Both modes share the same canonical Markdown string as source of truth.
  Mode switches sync the string between modes; neither mode loses content.

  Frontmatter is always separated from the body:
    - Code Mode:    frontmatter is part of the raw string (visible + editable)
    - WYSIWYG Mode: frontmatter is shown in FrontmatterPanel above the editor

  This component is intentionally a pure UI component. It does NOT make API
  calls. All persistence is handled by the parent via the event callbacks.

  Props:
    initialContent: string    — full Markdown including frontmatter
    documentId: string        — used as localStorage key for mode preference

  Callbacks:
    onChange(content: string): void   — debounced 800ms after last keystroke
    onPublish(content: string, message: string): void  — explicit user action

  Accessibility:
    - Mode toggle button has aria-pressed and descriptive aria-label
    - "Unsaved changes" indicator has role="status" and aria-live="polite"
    - Keyboard focus order: Mode toggle → Frontmatter panel → Editor body
-->

<script lang="ts">
  import { onDestroy } from 'svelte';
  import CodeMirrorEditor from './CodeMirrorEditor.svelte';
  import TiptapEditor from './TiptapEditor.svelte';
  import FrontmatterPanel from './FrontmatterPanel.svelte';
  import MetadataSidecar from './MetadataSidecar.svelte';
  import StatusBar from './StatusBar.svelte';
  import Breadcrumbs from './Breadcrumbs.svelte';
  import ReviewQueue from '$lib/components/ReviewQueue.svelte';
  import { reviewQueueStore } from '$lib/stores/reviewQueue';
  import {
    parseDocument,
    serializeDocument,
    type FrontmatterFields
  } from './utils/frontmatter.js';
  import ReadingMeasureToggle from '$lib/components/ReadingMeasureToggle.svelte';

  // ---------------------------------------------------------------------------
  // Types
  // ---------------------------------------------------------------------------

  type EditorMode = 'code' | 'wysiwyg';

  // ---------------------------------------------------------------------------
  // Props
  // ---------------------------------------------------------------------------

  interface Props {
    initialContent: string;
    documentId: string;
    projectId?: string;
    docPath?: string;
    onChange?: (content: string) => void;
    onPublish?: (content: string, message: string) => void;
  }

  let {
    initialContent,
    documentId,
    projectId = '',
    docPath = '',
    onChange,
    onPublish
  }: Props = $props();

  // ---------------------------------------------------------------------------
  // State — Mode
  // ---------------------------------------------------------------------------

  function getStoredMode(docId: string): EditorMode {
    if (typeof localStorage === 'undefined') return 'wysiwyg';
    const stored = localStorage.getItem(`vedox-editor-mode-${docId}`);
    return stored === 'code' ? 'code' : 'wysiwyg';
  }

  function storeMode(docId: string, mode: EditorMode): void {
    try {
      localStorage.setItem(`vedox-editor-mode-${docId}`, mode);
    } catch {
      // localStorage unavailable — non-fatal.
    }
  }

  let mode: EditorMode = $state(getStoredMode(documentId));

  // ---------------------------------------------------------------------------
  // State — Content
  // ---------------------------------------------------------------------------

  // The full canonical Markdown string (frontmatter + body).
  let canonicalContent = $state(initialContent);

  // Parsed frontmatter fields (WYSIWYG mode displays these in the panel).
  let frontmatter: FrontmatterFields = $state(
    parseDocument(initialContent).frontmatter
  );

  // Body-only text (passed to Tiptap). Frontmatter is stripped.
  let tiptapBody: string = $state(parseDocument(initialContent).body.trimStart());

  // ---------------------------------------------------------------------------
  // State — Dirty / Auto-save
  // ---------------------------------------------------------------------------

  let isDirty = $state(false);
  let autoSaveTimer: ReturnType<typeof setTimeout> | null = null;
  const AUTOSAVE_DEBOUNCE_MS = 800;

  // ---------------------------------------------------------------------------
  // State — Publish dialog
  // ---------------------------------------------------------------------------

  let showPublishDialog = $state(false);
  let publishMessage = $state('');
  let publishDialogInputEl: HTMLInputElement | undefined = $state(undefined);

  // ---------------------------------------------------------------------------
  // State — AI Review Queue drawer
  // ---------------------------------------------------------------------------

  let showReviewQueue = $state(false);
  const reviewPendingCount = reviewQueueStore.pendingCount;

  // ---------------------------------------------------------------------------
  // State — Component refs
  // ---------------------------------------------------------------------------

  let codeMirrorRef: CodeMirrorEditor | undefined = $state(undefined);
  let tiptapRef: TiptapEditor | undefined = $state(undefined);

  // ---------------------------------------------------------------------------
  // Mode toggle
  // ---------------------------------------------------------------------------

  function switchMode(newMode: EditorMode): void {
    if (newMode === mode) return;

    // Before switching FROM code mode: re-parse the raw string so WYSIWYG
    // picks up any edits the user made in the raw editor.
    if (mode === 'code') {
      const parsed = parseDocument(canonicalContent);
      frontmatter = parsed.frontmatter;
      tiptapBody = parsed.body.trimStart();
    }

    // Before switching FROM wysiwyg mode: serialize to canonical Markdown
    // so Code Mode shows the current state (including frontmatter edits).
    if (mode === 'wysiwyg') {
      canonicalContent = serializeDocument(frontmatter, tiptapBody);
    }

    mode = newMode;
    storeMode(documentId, newMode);

    // Focus the new editor after the DOM has updated.
    // We use a microtask so Svelte has re-rendered.
    Promise.resolve().then(() => {
      if (newMode === 'code') {
        codeMirrorRef?.focus();
      } else {
        tiptapRef?.focus();
      }
    });
  }

  // ---------------------------------------------------------------------------
  // Content change handlers (called by child editors)
  // ---------------------------------------------------------------------------

  /**
   * Called by CodeMirrorEditor on every keystroke.
   * Content is the full raw Markdown string (frontmatter included).
   */
  function handleCodeChange(content: string): void {
    canonicalContent = content;
    scheduleAutoSave();
  }

  /**
   * Called by TiptapEditor on every transaction.
   * Body is the body-only Markdown (frontmatter excluded).
   */
  function handleWysiwygChange(body: string): void {
    tiptapBody = body;
    canonicalContent = serializeDocument(frontmatter, body);
    scheduleAutoSave();
  }

  /**
   * Called by FrontmatterPanel on any field blur/change.
   */
  function handleFrontmatterChange(fm: FrontmatterFields): void {
    frontmatter = fm;
    canonicalContent = serializeDocument(fm, tiptapBody);
    scheduleAutoSave();
  }

  // ---------------------------------------------------------------------------
  // Auto-save debounce
  // ---------------------------------------------------------------------------

  function scheduleAutoSave(): void {
    isDirty = true;
    if (autoSaveTimer !== null) {
      clearTimeout(autoSaveTimer);
    }
    autoSaveTimer = setTimeout(() => {
      onChange?.(canonicalContent);
      autoSaveTimer = null;
      // Note: isDirty stays true until the user explicitly publishes.
      // The parent is responsible for persisting draft state.
    }, AUTOSAVE_DEBOUNCE_MS);
  }

  // ---------------------------------------------------------------------------
  // Publish
  // ---------------------------------------------------------------------------

  async function openPublishDialog(): Promise<void> {
    // Flush any pending auto-save timer so content is up to date.
    if (autoSaveTimer !== null) {
      clearTimeout(autoSaveTimer);
      autoSaveTimer = null;
    }

    // Ensure canonical content is current.
    if (mode === 'wysiwyg') {
      canonicalContent = serializeDocument(frontmatter, tiptapBody);
    }

    publishMessage = `docs: update ${frontmatter.title || 'document'}`;
    showPublishDialog = true;
    await Promise.resolve(); // let Svelte render
    publishDialogInputEl?.focus();
    publishDialogInputEl?.select();
  }

  function confirmPublish(): void {
    if (!publishMessage.trim()) return;
    onPublish?.(canonicalContent, publishMessage.trim());
    isDirty = false;
    showPublishDialog = false;
    publishMessage = '';
  }

  function cancelPublish(): void {
    showPublishDialog = false;
    publishMessage = '';
  }

  function handlePublishKeydown(e: KeyboardEvent): void {
    if (e.key === 'Enter') confirmPublish();
    if (e.key === 'Escape') cancelPublish();
  }

  // ---------------------------------------------------------------------------
  // Cleanup
  // ---------------------------------------------------------------------------

  onDestroy(() => {
    if (autoSaveTimer !== null) {
      clearTimeout(autoSaveTimer);
    }
  });
</script>

<!-- ============================================================
     Editor root
     ============================================================ -->

<div class="editor-root">
  <!-- ---- Header bar ---- -->
  <header class="editor-header" role="banner">
    <!-- Left: status indicator -->
    <div class="editor-header__left">
      <span
        class="editor-dirty-indicator"
        class:editor-dirty-indicator--visible={isDirty}
        role="status"
        aria-live="polite"
        aria-label={isDirty ? 'Unsaved changes' : ''}
      >
        {#if isDirty}
          <span class="editor-dirty-dot" aria-hidden="true"></span>
          Unsaved changes
        {/if}
      </span>
    </div>

    <!-- Right: mode toggle + publish -->
    <div class="editor-header__right">
      <!-- Mode toggle -->
      <div class="mode-toggle" role="group" aria-label="Editor mode">
        <button
          class="mode-toggle__btn"
          class:mode-toggle__btn--active={mode === 'wysiwyg'}
          type="button"
          aria-pressed={mode === 'wysiwyg'}
          aria-label="Switch to Overview (WYSIWYG) mode"
          onclick={() => switchMode('wysiwyg')}
        >
          Overview
        </button>
        <button
          class="mode-toggle__btn"
          class:mode-toggle__btn--active={mode === 'code'}
          type="button"
          aria-pressed={mode === 'code'}
          aria-label="Switch to Code (raw Markdown) mode"
          onclick={() => switchMode('code')}
        >
          Code
        </button>
      </div>

      <!-- Publish button -->
      <button
        class="publish-btn"
        type="button"
        onclick={openPublishDialog}
        title="Publish"
        aria-label="Publish document — creates a Git commit"
      >
        Publish
      </button>

      <!-- Reading measure toggle -->
      <ReadingMeasureToggle />

      <!-- AI Review Queue toggle -->
      <button
        class="ai-review-btn"
        class:ai-review-btn--active={showReviewQueue}
        type="button"
        title="Writing review"
        aria-pressed={showReviewQueue}
        aria-label="Toggle AI writing review panel"
        onclick={() => showReviewQueue = !showReviewQueue}
      >
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
          <path d="M12 20h9"/><path d="M16.5 3.5a2.121 2.121 0 0 1 3 3L7 19l-4 1 1-4L16.5 3.5z"/>
        </svg>
        AI
        {#if $reviewPendingCount > 0}
          <span class="ai-review-btn__badge">{$reviewPendingCount}</span>
        {/if}
      </button>
    </div>
  </header>

  <!-- ---- Breadcrumbs (above editor content) ---- -->
  {#if projectId && docPath}
    <Breadcrumbs {projectId} {docPath} />
  {/if}

  <!-- ---- Editor body (three-zone: content + sidecar) ---- -->
  <div class="editor-body">
    <!-- Content area -->
    <div class="editor-content">
      <!-- Code Mode -->
      <div
        class="editor-panel editor-panel--code"
        class:editor-panel--active={mode === 'code'}
        aria-hidden={mode !== 'code'}
        inert={mode !== 'code' ? true : undefined}
      >
        <CodeMirrorEditor
          bind:this={codeMirrorRef}
          bind:content={canonicalContent}
          onchange={handleCodeChange}
        />
      </div>

      <!-- WYSIWYG Mode -->
      <div
        class="editor-panel editor-panel--wysiwyg"
        class:editor-panel--active={mode === 'wysiwyg'}
        aria-hidden={mode !== 'wysiwyg'}
        inert={mode !== 'wysiwyg' ? true : undefined}
      >
        <!-- Frontmatter panel above the prose body -->
        <FrontmatterPanel
          bind:frontmatter
          onchange={handleFrontmatterChange}
        />

        <!-- Tiptap WYSIWYG body -->
        <TiptapEditor
          bind:this={tiptapRef}
          bind:body={tiptapBody}
          onchange={handleWysiwygChange}
        />
      </div>
    </div>

    <!-- Metadata sidecar — visible at >= 1920px only -->
    <div class="editor-sidecar-wrapper">
      <MetadataSidecar {projectId} {docPath} />
    </div>

    <!-- AI Review Queue drawer — slides in from the right -->
    {#if showReviewQueue}
      <div class="review-drawer" role="complementary" aria-label="AI Review suggestions">
        <ReviewQueue />
      </div>
    {/if}
  </div>

  <!-- ---- Status bar (bottom 24px strip) ---- -->
  <StatusBar content={canonicalContent} {projectId} {docPath} />

  <!-- ---- Publish dialog ---- -->
  {#if showPublishDialog}
    <!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
    <div
      class="publish-dialog-backdrop"
      role="presentation"
      onclick={(e) => { if (e.target === e.currentTarget) cancelPublish(); }}
      onkeydown={(e) => { if (e.key === 'Escape') cancelPublish(); }}
    >
      <div
        class="publish-dialog"
        role="dialog"
        aria-modal="true"
        aria-label="Publish document"
        aria-describedby="publish-dialog-desc"
      >
        <h2 class="publish-dialog__title">Publish document</h2>
        <p id="publish-dialog-desc" class="publish-dialog__desc">
          Creates a Git commit with the current content. Add a short commit message.
        </p>
        <label class="publish-dialog__label" for="publish-message">
          Commit message
        </label>
        <input
          bind:this={publishDialogInputEl}
          id="publish-message"
          type="text"
          class="publish-dialog__input"
          bind:value={publishMessage}
          onkeydown={handlePublishKeydown}
          maxlength="200"
          placeholder="docs: describe your change"
          autocomplete="off"
        />
        <div class="publish-dialog__actions">
          <button
            class="publish-dialog__cancel"
            type="button"
            onclick={cancelPublish}
          >
            Cancel
          </button>
          <button
            class="publish-dialog__confirm"
            type="button"
            onclick={confirmPublish}
            disabled={!publishMessage.trim()}
          >
            Commit &amp; Publish
          </button>
        </div>
      </div>
    </div>
  {/if}
</div>

<style>
  /* ---- Root ---- */
  .editor-root {
    display: flex;
    flex-direction: column;
    height: 100%;
    background: var(--color-surface-base);
    color: var(--color-text-primary);
    overflow: hidden;
    position: relative;
  }

  /* ---- Header / Sticky toolbar ---- */
  .editor-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 var(--space-4);
    height: calc(40px * var(--density, 1));
    flex-shrink: 0;
    border-bottom: 1px solid var(--border-hairline);
    background: var(--surface-2);
    gap: var(--space-3);
  }

  .editor-header__left {
    flex: 1;
    min-width: 0;
  }

  .editor-header__right {
    display: flex;
    align-items: center;
    gap: 10px;
    flex-shrink: 0;
  }

  /* ---- Dirty indicator ---- */
  .editor-dirty-indicator {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-size: var(--text-xs);
    color: var(--text-3);
    font-family: var(--font-body);
    opacity: 0;
    transition: opacity var(--duration-default) var(--ease-out);
    white-space: nowrap;
  }

  .editor-dirty-indicator--visible {
    opacity: 1;
  }

  .editor-dirty-dot {
    width: 6px;
    height: 6px;
    border-radius: var(--radius-full);
    background: var(--warning);
    flex-shrink: 0;
  }

  /* ---- Mode toggle ---- */
  .mode-toggle {
    display: flex;
    align-items: center;
    gap: 2px;
    /* No background or border on the group — buttons float */
  }

  .mode-toggle__btn {
    display: flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    border: none;
    color: var(--text-3);
    font-size: var(--text-xs);
    font-weight: 500;
    font-family: var(--font-body);
    padding: 6px;
    min-width: 28px;
    height: 28px;
    border-radius: var(--radius-sm);
    cursor: pointer;
    transition:
      color var(--duration-fast) var(--ease-out),
      background-color var(--duration-fast) var(--ease-out);
    white-space: nowrap;
  }

  .mode-toggle__btn:hover:not(.mode-toggle__btn--active) {
    background: var(--surface-4);
    color: var(--text-1);
  }

  .mode-toggle__btn--active {
    background: var(--accent-subtle);
    color: var(--accent-text);
    font-weight: 600;
  }

  .mode-toggle__btn:focus-visible {
    outline: 2px solid var(--accent-solid);
    outline-offset: 1px;
    z-index: 1;
    position: relative;
  }

  /* ---- Toolbar group divider ---- */
  .editor-header__right::before {
    content: '';
    display: block;
    width: 1px;
    height: 16px;
    background: var(--border-hairline);
    margin-inline: 8px;
  }

  /* ---- Publish button (pill, per §5.4 right-cluster) ---- */
  .publish-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    border: 1px solid var(--border-hairline);
    border-radius: var(--radius-md);
    color: var(--text-2);
    font-size: var(--text-sm);
    font-weight: 600;
    font-family: var(--font-body);
    padding: 6px 14px;
    height: 28px;
    cursor: pointer;
    transition:
      background-color var(--duration-fast) var(--ease-out),
      border-color var(--duration-fast) var(--ease-out),
      color var(--duration-fast) var(--ease-out);
    white-space: nowrap;
  }

  .publish-btn:hover {
    background: var(--accent-solid);
    border-color: var(--accent-solid);
    color: var(--accent-contrast);
  }

  .publish-btn:focus-visible {
    outline: 2px solid var(--accent-solid);
    outline-offset: 2px;
  }

  /* ---- Editor body (three-zone layout) ---- */
  .editor-body {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: row;
    overflow: hidden;
  }

  .editor-content {
    flex: 1;
    min-width: 0;
    position: relative;
    overflow: hidden;
  }

  /* ---- Metadata sidecar wrapper ---- */
  .editor-sidecar-wrapper {
    display: none;
    height: 100%;
    flex-shrink: 0;
  }

  @media (min-width: 1920px) {
    .editor-sidecar-wrapper {
      display: block;
    }
  }

  /* ---- AI Review button ---- */
  .ai-review-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 4px;
    background: transparent;
    border: 1px solid var(--border-hairline);
    border-radius: var(--radius-md);
    color: var(--text-3);
    font-size: var(--text-xs);
    font-weight: 500;
    font-family: var(--font-body);
    padding: 6px 10px;
    height: 28px;
    cursor: pointer;
    transition:
      color var(--duration-fast) var(--ease-out),
      background-color var(--duration-fast) var(--ease-out),
      border-color var(--duration-fast) var(--ease-out);
    white-space: nowrap;
  }

  .ai-review-btn:hover {
    background: var(--surface-4);
    color: var(--text-1);
    border-color: var(--border-default);
  }

  .ai-review-btn--active {
    background: var(--accent-subtle);
    color: var(--accent-text);
    border-color: var(--accent-border);
  }

  .ai-review-btn:focus-visible {
    outline: 2px solid var(--accent-solid);
    outline-offset: 1px;
  }

  .ai-review-btn__badge {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-width: 16px;
    height: 16px;
    border-radius: var(--radius-full);
    background: var(--accent-solid);
    color: var(--accent-contrast);
    font-size: 10px;
    font-weight: 700;
    padding: 0 4px;
    line-height: 1;
  }

  /* ---- Review Queue drawer ---- */
  .review-drawer {
    width: 340px;
    flex-shrink: 0;
    height: 100%;
    border-left: 1px solid var(--border-hairline);
    background: var(--surface-2);
    overflow: hidden;
    animation: review-drawer-in 200ms var(--ease-out) both;
  }

  @keyframes review-drawer-in {
    from {
      opacity: 0;
      transform: translateX(16px);
    }
    to {
      opacity: 1;
      transform: translateX(0);
    }
  }

  /* ---- Editor panels (crossfade) ---- */
  .editor-panel {
    position: absolute;
    inset: 0;
    opacity: 0;
    pointer-events: none;
    transition: opacity var(--duration-default) var(--ease-out);
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .editor-panel--active {
    opacity: 1;
    pointer-events: auto;
  }

  /* ---- Publish dialog ---- */
  .publish-dialog-backdrop {
    position: fixed;
    inset: 0;
    background: oklch(0% 0 0 / 0.6);
    z-index: var(--z-modal);
    display: flex;
    align-items: center;
    justify-content: center;
    backdrop-filter: blur(4px);
    animation: backdrop-in var(--duration-fast) var(--ease-out);
  }

  @keyframes backdrop-in {
    from { opacity: 0; }
    to { opacity: 1; }
  }

  .publish-dialog {
    background: var(--surface-4);
    border: 1px solid var(--border-default);
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow-overlay);
    padding: var(--space-6);
    width: 480px;
    max-width: calc(100vw - var(--space-8));
    animation: dialog-in 200ms var(--ease-out);
  }

  @keyframes dialog-in {
    from {
      opacity: 0;
      transform: scale(0.96) translateY(8px);
    }
    to {
      opacity: 1;
      transform: scale(1) translateY(0);
    }
  }

  .publish-dialog__title {
    font-size: var(--text-lg);
    font-weight: 700;
    color: var(--text-1);
    margin: 0 0 var(--space-2);
  }

  .publish-dialog__desc {
    font-size: var(--text-sm);
    color: var(--text-3);
    margin: 0 0 var(--space-5);
    line-height: var(--leading-normal);
  }

  .publish-dialog__label {
    display: block;
    font-size: var(--text-2xs);
    font-weight: 600;
    color: var(--text-3);
    text-transform: uppercase;
    letter-spacing: 0.06em;
    margin-bottom: 6px;
  }

  .publish-dialog__input {
    width: 100%;
    background: var(--surface-2);
    border: 1px solid var(--border-default);
    border-radius: var(--radius-sm);
    color: var(--text-1);
    font-size: var(--text-base);
    font-family: var(--font-mono);
    padding: var(--space-2) var(--space-3);
    outline: none;
    box-sizing: border-box;
    transition: border-color var(--duration-fast) var(--ease-out);
  }

  .publish-dialog__input:focus {
    border-color: var(--accent-solid);
  }

  .publish-dialog__actions {
    display: flex;
    justify-content: flex-end;
    gap: var(--space-2);
    margin-top: var(--space-5);
  }

  .publish-dialog__cancel {
    background: none;
    border: 1px solid var(--border-hairline);
    border-radius: var(--radius-md);
    color: var(--text-3);
    font-size: var(--text-sm);
    padding: 7px var(--space-4);
    cursor: pointer;
    transition:
      border-color var(--duration-fast) var(--ease-out),
      color var(--duration-fast) var(--ease-out);
    font-family: var(--font-body);
  }

  .publish-dialog__cancel:hover {
    border-color: var(--border-strong);
    color: var(--text-1);
  }

  .publish-dialog__confirm {
    background: var(--accent-solid);
    border: none;
    border-radius: var(--radius-md);
    color: var(--accent-contrast);
    font-size: var(--text-sm);
    font-weight: 600;
    padding: 7px var(--space-4);
    cursor: pointer;
    transition: background-color var(--duration-fast) var(--ease-out);
    font-family: var(--font-body);
  }

  .publish-dialog__confirm:hover:not(:disabled) {
    background: var(--accent-solid-hover);
  }

  .publish-dialog__confirm:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .publish-dialog__confirm:focus-visible,
  .publish-dialog__cancel:focus-visible {
    outline: 2px solid var(--accent-solid);
    outline-offset: 2px;
  }
</style>
