<!--
  PaneView.svelte

  Wraps a single Editor instance inside the pane grid.
  Handles focus ring, close button, and empty-state rendering.

  Named PaneView (not Pane) to avoid shadowing the Pane type from the store.
-->

<script lang="ts" module>
  /** Shape of pre-loaded document data passed from the route. */
  export interface DocData {
    content: string;
    metadata?: Record<string, unknown>;
  }
</script>

<script lang="ts">
  import type { Pane } from '$lib/stores/panes';
  import Editor from '$lib/editor/Editor.svelte';
  import EmptyState from './EmptyState.svelte';

  interface Props {
    pane: Pane;
    isActive?: boolean;
    docData: DocData | null;
    projectId: string;
    onChange?: (content: string) => void;
    onPublish?: (content: string, message: string) => void;
    onFocus?: () => void;
    onClose?: () => void;
  }

  let {
    pane,
    isActive = false,
    docData,
    projectId,
    onChange,
    onPublish,
    onFocus,
    onClose,
  }: Props = $props();

  /**
   * Build documentId in the same format the route currently uses:
   * "{projectId}/{docPath}"
   */
  const documentId = $derived(
    pane.docPath ? `${projectId}/${pane.docPath}` : ''
  );
</script>

<!-- svelte-ignore a11y_click_events_have_key_events -->
<div
  class="pane"
  class:pane--active={isActive}
  onclick={onFocus}
  onfocusin={onFocus}
  role="region"
  aria-label={pane.docPath ?? 'Empty pane'}
>
  <div class="pane-header">
    <span class="pane-path">
      {pane.docPath ?? 'New document'}
    </span>
    <button
      class="pane-close"
      onclick={(e) => { e.stopPropagation(); onClose?.(); }}
      aria-label="Close pane"
    >
      &#x2715;
    </button>
  </div>

  <div class="pane-content">
    {#if pane.docPath && docData}
      <Editor
        initialContent={docData.content}
        {documentId}
        {onChange}
        {onPublish}
      />
    {:else if pane.docPath && !docData}
      <div class="pane-loading">
        <span class="pane-spinner" aria-hidden="true"></span>
        Loading document...
      </div>
    {:else}
      <EmptyState
        icon={`<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>`}
        heading="Nothing open"
        body="Click a file, or ⌘K to search."
      />
    {/if}
  </div>
</div>

<style>
  .pane {
    display: flex;
    flex-direction: column;
    height: 100%;
    background: var(--color-surface-base);
    min-width: 0;
  }

  .pane--active {
    box-shadow: inset 0 2px 0 0 var(--color-accent);
  }

  .pane-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 var(--space-3, 12px);
    height: 32px;
    border-bottom: 1px solid var(--color-border);
    background: var(--color-surface-elevated);
    font-size: var(--font-size-xs, 11px);
    color: var(--color-text-muted);
    flex-shrink: 0;
    gap: var(--space-2, 8px);
  }

  .pane-path {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    min-width: 0;
    font-family: var(--font-mono);
  }

  .pane-close {
    width: 20px;
    height: 20px;
    border: none;
    background: transparent;
    color: var(--color-text-muted);
    cursor: pointer;
    border-radius: var(--radius-sm, 4px);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 10px;
    flex-shrink: 0;
    transition: background 80ms var(--ease-out), color 80ms var(--ease-out);
  }

  .pane-close:hover {
    background: var(--color-surface-overlay);
    color: var(--color-text-primary);
  }

  .pane-close:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: -2px;
  }

  .pane-content {
    flex: 1;
    overflow: hidden;
    position: relative;
    min-height: 0;
  }

  .pane-empty,
  .pane-loading {
    display: flex;
    align-items: center;
    justify-content: center;
    height: 100%;
    color: var(--color-text-muted);
    font-size: var(--font-size-sm, 13px);
    gap: var(--space-2, 8px);
  }

  .pane-spinner {
    display: inline-block;
    width: 14px;
    height: 14px;
    border: 2px solid var(--color-border);
    border-top-color: var(--color-accent);
    border-radius: 50%;
    animation: pane-spin 600ms linear infinite;
    flex-shrink: 0;
  }

  @keyframes pane-spin {
    to {
      transform: rotate(360deg);
    }
  }
</style>
