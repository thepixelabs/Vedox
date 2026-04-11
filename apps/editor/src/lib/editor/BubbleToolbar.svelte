<!--
  BubbleToolbar.svelte

  Floating inline-mark toolbar that appears above text selections in the
  Tiptap WYSIWYG editor. Exposes bold, italic, inline code, and link marks.

  Positioning is handled by the parent TiptapEditor.svelte via Tiptap's
  `onSelectionUpdate` callback — this component only renders the buttons
  and dispatches mark toggles.

  Visual spec: .tasks/vedox-flagship-ux/design-system.md § 7.2
  Token system: apps/editor/src/styles/tokens.css

  Accessibility:
    - role="toolbar" with aria-label
    - Each button has a descriptive title for keyboard/screen-reader users
    - Active marks are conveyed via aria-pressed
-->

<script lang="ts">
  import type { Editor } from '@tiptap/core';

  // ---------------------------------------------------------------------------
  // Props
  // ---------------------------------------------------------------------------

  interface Props {
    editor: Editor | null;
  }

  let { editor }: Props = $props();

  // ---------------------------------------------------------------------------
  // Mark definitions
  // ---------------------------------------------------------------------------

  interface MarkDef {
    name: string;
    label: string;
    title: string;
    weight?: number;
    style?: string;
    decoration?: string;
    mono?: boolean;
  }

  const marks: MarkDef[] = [
    { name: 'bold',   label: 'B',          title: 'Bold (⌘B)',         weight: 700 },
    { name: 'italic', label: 'I',          title: 'Italic (⌘I)',       style: 'italic' },
    { name: 'strike', label: 'S',          title: 'Strikethrough',     decoration: 'line-through' },
    { name: 'code',   label: '\u2039\u203A', title: 'Inline code (⌘E)', mono: true },
    { name: 'link',   label: '\u2197',     title: 'Link (stub)' },
  ];

  // ---------------------------------------------------------------------------
  // Actions
  // ---------------------------------------------------------------------------

  function toggle(mark: string): void {
    if (!editor) return;
    if (mark === 'link') {
      // Link insertion is a future-phase feature. For now, stub.
      return;
    }
    editor.chain().focus().toggleMark(mark).run();
  }

  function isActive(mark: string): boolean {
    return editor?.isActive(mark) ?? false;
  }
</script>

<div class="bubble-toolbar" role="toolbar" aria-label="Text formatting">
  {#each marks as mark}
    <button
      class="bubble-btn"
      class:bubble-btn--active={isActive(mark.name)}
      type="button"
      title={mark.title}
      aria-pressed={isActive(mark.name)}
      style:font-weight={mark.weight ?? 500}
      style:font-style={mark.style ?? 'normal'}
      style:text-decoration={mark.decoration ?? 'none'}
      class:bubble-btn--mono={mark.mono}
      onclick={() => toggle(mark.name)}
    >
      {mark.label}
    </button>
  {/each}
</div>

<style>
  .bubble-toolbar {
    display: flex;
    align-items: center;
    gap: 2px;
    padding: 4px;
    background: var(--surface-4);
    border: 1px solid var(--border-default);
    border-radius: var(--radius-md);
    box-shadow: var(--shadow-overlay);
    /* Slide up 4px on appear, per § 7.2 */
    animation: bubble-in 140ms var(--ease-out) both;
  }

  @keyframes bubble-in {
    from {
      opacity: 0;
      transform: translateY(4px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }

  /* Respect user motion preferences */
  @media (prefers-reduced-motion: reduce) {
    .bubble-toolbar {
      animation: none;
    }
  }

  .bubble-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 28px;
    height: 28px;
    border: none;
    background: transparent;
    color: var(--text-2);
    border-radius: var(--radius-sm);
    cursor: pointer;
    font-size: var(--text-sm);
    font-family: var(--font-body);
    transition:
      background-color 80ms ease,
      color 80ms ease;
    /* Reset default button styles */
    padding: 0;
    line-height: 1;
  }

  .bubble-btn--mono {
    font-family: var(--font-mono);
    font-size: var(--text-xs);
  }

  .bubble-btn:hover {
    background: var(--surface-3);
    color: var(--text-1);
  }

  .bubble-btn--active {
    background: var(--accent-subtle);
    color: var(--accent-text);
  }

  .bubble-btn--active:hover {
    background: var(--accent-subtle);
    color: var(--accent-text);
  }

  .bubble-btn:focus-visible {
    outline: 2px solid var(--accent-solid);
    outline-offset: -1px;
  }
</style>
