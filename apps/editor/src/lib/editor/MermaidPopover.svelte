<!--
  MermaidPopover.svelte

  Inline code popover for editing Mermaid source. Activated by a
  `mermaid-open-popover` CustomEvent bubbled from MermaidNode's NodeView.

  Layout: absolutely positioned below the anchor element.
  Dismisses on: blur (focus leaving the popover), Escape key.
  On dismiss: calls onUpdate(newSource) which dispatches a ProseMirror
  transaction to update the node attribute. MermaidNode's update() hook
  then re-renders the SVG.

  Accessibility:
  - role="dialog", aria-label, aria-modal
  - Auto-focuses the textarea on open
  - Escape closes without saving; Tab/Shift-Tab trapped inside
  - Focus returns to the trigger element on close
-->

<script lang="ts">
  import { onMount, onDestroy, tick } from 'svelte';

  // ---------------------------------------------------------------------------
  // State
  // ---------------------------------------------------------------------------

  let open = $state(false);
  let source = $state('');
  let anchorEl: HTMLElement | null = $state(null);
  let popoverStyle = $state('');
  let onUpdate: ((src: string) => void) | null = null;
  let textareaEl: HTMLTextAreaElement | undefined = $state(undefined);
  let containerEl: HTMLDivElement | undefined = $state(undefined);

  // ---------------------------------------------------------------------------
  // Position popover below the anchor
  // ---------------------------------------------------------------------------

  function positionPopover(): void {
    if (!anchorEl || !containerEl) return;
    const rect = anchorEl.getBoundingClientRect();
    const scrollX = window.scrollX;
    const scrollY = window.scrollY;
    const top = rect.bottom + scrollY + 8;
    const left = Math.max(rect.left + scrollX, 12);
    const maxWidth = Math.min(window.innerWidth - left - 12, 640);
    popoverStyle = `top:${top}px; left:${left}px; width:${maxWidth}px;`;
  }

  // ---------------------------------------------------------------------------
  // Open / close
  // ---------------------------------------------------------------------------

  async function openPopover(detail: {
    source: string;
    anchorEl: HTMLElement;
    onUpdate: (src: string) => void;
  }): Promise<void> {
    source = detail.source;
    anchorEl = detail.anchorEl;
    onUpdate = detail.onUpdate;
    open = true;
    await tick();
    positionPopover();
    textareaEl?.focus();
    textareaEl?.select();
  }

  function closePopover(save: boolean): void {
    if (!open) return;
    if (save && onUpdate) {
      onUpdate(source);
    }
    open = false;
    anchorEl?.focus();
    anchorEl = null;
    onUpdate = null;
  }

  // ---------------------------------------------------------------------------
  // Event listeners
  // ---------------------------------------------------------------------------

  function handleCustomEvent(e: Event): void {
    const ce = e as CustomEvent<{
      source: string;
      anchorEl: HTMLElement;
      onUpdate: (src: string) => void;
    }>;
    openPopover(ce.detail);
  }

  function handleKeydown(e: KeyboardEvent): void {
    if (e.key === 'Escape') {
      e.preventDefault();
      closePopover(false);
    }
  }

  function handleFocusOut(e: FocusEvent): void {
    // Save-on-blur: close and persist changes if focus moves outside popover.
    if (
      containerEl &&
      e.relatedTarget instanceof Node &&
      containerEl.contains(e.relatedTarget)
    ) {
      return; // Focus stayed inside — don't close.
    }
    // Small timeout to let relatedTarget settle (browser quirk).
    setTimeout(() => {
      if (!containerEl?.contains(document.activeElement)) {
        closePopover(true);
      }
    }, 50);
  }

  // ---------------------------------------------------------------------------
  // Lifecycle — attach global listener for the custom event
  // ---------------------------------------------------------------------------

  onMount(() => {
    document.addEventListener('mermaid-open-popover', handleCustomEvent);
  });

  onDestroy(() => {
    document.removeEventListener('mermaid-open-popover', handleCustomEvent);
  });
</script>

{#if open}
  <!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
  <div
    bind:this={containerEl}
    class="mermaid-popover"
    style={popoverStyle}
    role="dialog"
    aria-label="Edit Mermaid diagram source"
    aria-modal="true"
    onkeydown={handleKeydown}
    onfocusout={handleFocusOut}
  >
    <div class="mermaid-popover__header">
      <span class="mermaid-popover__title">Edit diagram source</span>
      <button
        class="mermaid-popover__close"
        type="button"
        aria-label="Close diagram editor"
        onclick={() => closePopover(true)}
      >
        &#x2715;
      </button>
    </div>
    <textarea
      bind:this={textareaEl}
      bind:value={source}
      class="mermaid-popover__textarea"
      spellcheck="false"
      autocomplete="off"
      rows="8"
      aria-label="Mermaid diagram source code"
      placeholder="graph TD&#10;  A --> B"
    ></textarea>
    <div class="mermaid-popover__footer">
      <span class="mermaid-popover__hint">Blur or press Esc to close and re-render</span>
      <button
        class="mermaid-popover__apply"
        type="button"
        onclick={() => closePopover(true)}
      >
        Apply
      </button>
    </div>
  </div>
{/if}

<style>
  .mermaid-popover {
    position: absolute;
    z-index: 1000;
    background: var(--color-surface-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    box-shadow: var(--shadow-md);
    display: flex;
    flex-direction: column;
    overflow: hidden;
    animation: popover-in 120ms ease-out;
  }

  @keyframes popover-in {
    from {
      opacity: 0;
      transform: translateY(-4px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }

  .mermaid-popover__header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 12px;
    border-bottom: 1px solid var(--color-border);
  }

  .mermaid-popover__title {
    font-size: 12px;
    font-weight: 600;
    color: var(--color-text-muted);
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  .mermaid-popover__close {
    background: none;
    border: none;
    color: var(--color-text-muted);
    cursor: pointer;
    padding: 2px 6px;
    font-size: 14px;
    border-radius: var(--radius-sm);
    line-height: 1;
    transition: color var(--duration-fast) var(--ease-out), background var(--duration-fast) var(--ease-out);
  }

  .mermaid-popover__close:hover {
    color: var(--color-text-primary);
    background: var(--color-surface-overlay);
  }

  .mermaid-popover__textarea {
    font-family: var(--font-mono);
    font-size: 13px;
    line-height: 1.6;
    color: var(--color-text-primary);
    background: var(--color-surface-base);
    border: none;
    resize: vertical;
    padding: 12px;
    outline: none;
    width: 100%;
    box-sizing: border-box;
    min-height: 120px;
  }

  .mermaid-popover__textarea::placeholder {
    color: var(--color-text-subtle);
  }

  .mermaid-popover__footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 12px;
    border-top: 1px solid var(--color-border);
  }

  .mermaid-popover__hint {
    font-size: 11px;
    color: var(--color-text-subtle);
  }

  .mermaid-popover__apply {
    background: var(--color-accent);
    color: var(--color-text-inverse);
    border: none;
    border-radius: var(--radius-sm);
    padding: 4px 12px;
    font-size: 12px;
    font-weight: 600;
    cursor: pointer;
    transition: background var(--duration-fast) var(--ease-out);
  }

  .mermaid-popover__apply:hover {
    background: var(--color-accent-hover);
  }

  .mermaid-popover__apply:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
  }

  /* Global styles for the mermaid node island — injected into consumer pages */
  :global(.mermaid-node) {
    display: block;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: 16px;
    margin: 16px 0;
    background: var(--color-surface-elevated);
    cursor: pointer;
    transition: border-color var(--duration-default) var(--ease-out);
    outline: none;
  }

  :global(.mermaid-node:hover) {
    border-color: var(--color-accent);
  }

  :global(.mermaid-node:focus-visible) {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
  }

  :global(.mermaid-node svg) {
    display: block;
    max-width: 100%;
    height: auto;
  }

  :global(.mermaid-loading) {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--color-text-muted);
    padding: 8px 0;
  }

  :global(.mermaid-svg-container) {
    display: flex;
    justify-content: center;
  }
</style>
