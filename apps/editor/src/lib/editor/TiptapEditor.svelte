<!--
  TiptapEditor.svelte

  WYSIWYG Mode: rich-text editing via Tiptap + @tiptap/extension-markdown.

  The Tiptap editor receives the document body (frontmatter stripped by the
  parent Editor.svelte). Frontmatter is handled separately in FrontmatterPanel.

  Extensions enabled:
    - StarterKit (Bold, Italic, Code, CodeBlock, Heading H1-H4, BulletList,
      OrderedList, Blockquote, HorizontalRule, Strike, Text, Paragraph, Doc)
    - Link (with auto-link detection)
    - Markdown (prosemirror-markdown serializer / deserializer)
    - MermaidNode (custom extension — fenced ```mermaid blocks)

  Round-trip invariant:
    serialize(parse(body)) === body
  Enforced by golden-file test suite. The Go backend is authoritative per the
  Phase 1 CTO ruling.

  Props:
    body: string        — Markdown body WITHOUT frontmatter
    readonly?: boolean

  Events:
    onchange: (body: string) => void  — fires on every editor transaction
-->

<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { Editor } from '@tiptap/core';
  import StarterKit from '@tiptap/starter-kit';
  import Link from '@tiptap/extension-link';
  import { Markdown } from 'tiptap-markdown';
  import { MermaidNode } from './extensions/MermaidNode.js';
  import { Callout } from './extensions/Callout.js';
  import { KatexInline } from './extensions/KatexInline.js';
  import { KatexBlock } from './extensions/KatexBlock.js';
  import { FootnoteRef } from './extensions/Footnotes.js';
  import { SlashCommand } from './extensions/SlashCommand.js';
  import Table from '@tiptap/extension-table';
  import TableRow from '@tiptap/extension-table-row';
  import TableHeader from '@tiptap/extension-table-header';
  import TableCell from '@tiptap/extension-table-cell';
  import Image from '@tiptap/extension-image';
  import { CodeBlockShiki } from './codeblock/CodeBlockShiki.svelte.js';
  import { warmHighlighter } from './codeblock/highlight.js';
  import MermaidPopover from './MermaidPopover.svelte';
  import BubbleToolbar from './BubbleToolbar.svelte';
  import SlashCommandPopover from './SlashCommandPopover.svelte';
  import { readingStore } from '$lib/stores/reading';
  import VedoxLinkHandler from '$lib/components/preview/VedoxLinkHandler.svelte';

  // ---------------------------------------------------------------------------
  // Props
  // ---------------------------------------------------------------------------

  interface Props {
    body: string;
    readonly?: boolean;
    onchange?: (body: string) => void;
  }

  let { body = $bindable(), readonly = false, onchange }: Props = $props();

  // ---------------------------------------------------------------------------
  // Refs
  // ---------------------------------------------------------------------------

  let editorEl: HTMLDivElement | undefined = $state(undefined);
  let editor: Editor | undefined;

  // ---------------------------------------------------------------------------
  // Bubble toolbar state
  // ---------------------------------------------------------------------------

  let bubbleVisible = $state(false);
  let bubbleX = $state(0);
  let bubbleY = $state(0);
  let bubbleEl: HTMLDivElement | undefined = $state(undefined);

  function updateBubblePosition(): void {
    if (!editor || editor.isDestroyed || !editorEl) {
      bubbleVisible = false;
      return;
    }

    const { state } = editor.view;
    const { from, to, empty } = state.selection;

    // Hide when selection is collapsed (no text selected)
    if (empty) {
      bubbleVisible = false;
      return;
    }

    // Hide if the selection spans across node types that aren't inline
    // (e.g. selecting across code blocks)
    const isTextSelection = state.selection.constructor.name === 'TextSelection';
    if (!isTextSelection) {
      bubbleVisible = false;
      return;
    }

    // Get the coordinates of the selection midpoint
    const fromCoords = editor.view.coordsAtPos(from);
    const toCoords = editor.view.coordsAtPos(to);

    // Position the bubble above the selection in viewport (fixed) coordinates.
    // Using fixed positioning so the bubble is never clipped by overflow:hidden
    // on ancestor elements.
    const midX = (fromCoords.left + toCoords.right) / 2;
    const topY = Math.min(fromCoords.top, toCoords.top);

    bubbleX = midX;
    bubbleY = topY - 8; // 8px gap above selection per spec
    bubbleVisible = true;
  }

  // ---------------------------------------------------------------------------
  // Lifecycle
  // ---------------------------------------------------------------------------

  onMount(() => {
    if (!editorEl) return;

    // Warm the Shiki highlighter so the first code block paints with colour
    // instead of flashing plaintext. Fire-and-forget; failures degrade to
    // plaintext, which is acceptable.
    warmHighlighter();

    editor = new Editor({
      element: editorEl,
      editable: !readonly,
      extensions: [
        StarterKit.configure({
          // We supply a Shiki-backed replacement below; disable the default.
          codeBlock: false,
          heading: {
            levels: [1, 2, 3, 4]
          },
          // Disable history — we manage undo via the editor's built-in
          history: {}
        }),
        CodeBlockShiki,
        Link.configure({
          openOnClick: false, // Prevent accidental navigation while editing
          autolink: true,
          HTMLAttributes: {
            rel: 'noopener noreferrer',
            class: 'editor-link'
          }
        }),
        Markdown.configure({
          html: false, // Security: no raw HTML from user input
          tightLists: true,
          tightListClass: 'tight',
          bulletListMarker: '-',
          linkify: false,
          breaks: false,
          transformPastedText: true,
          transformCopiedText: false
        }),
        MermaidNode,
        Callout,
        KatexInline,
        KatexBlock,
        FootnoteRef,
        Table.configure({
          resizable: true,
          HTMLAttributes: { class: 'vedox-table' }
        }),
        TableRow,
        TableHeader,
        TableCell,
        Image.configure({
          inline: false,
          allowBase64: false,
          HTMLAttributes: { class: 'vedox-image' }
        }),
        SlashCommand
      ],
      content: body,
      onUpdate: ({ editor: e }) => {
        // Serialize back to Markdown on every transaction.
        const newBody = e.storage.markdown.getMarkdown();
        body = newBody;
        onchange?.(newBody);
      },
      onSelectionUpdate: () => {
        updateBubblePosition();
      },
      onBlur: () => {
        // Small delay so clicking a bubble button doesn't dismiss instantly
        setTimeout(() => {
          if (!editor?.isFocused) {
            bubbleVisible = false;
          }
        }, 150);
      }
    });
  });

  // Ensure reading measure CSS var stays in sync on mount.
  const unsubReading = readingStore.subscribe(() => {});

  onDestroy(() => {
    editor?.destroy();
    editor = undefined;
    unsubReading();
  });

  // ---------------------------------------------------------------------------
  // Reactive: sync external body changes (e.g. mode switch)
  // ---------------------------------------------------------------------------

  $effect(() => {
    if (!editor || editor.isDestroyed) return;
    // Only update if the serialized content differs — avoids cursor jumps.
    const current = editor.storage.markdown.getMarkdown();
    if (body !== current) {
      editor.commands.setContent(body, false);
    }
  });

  // ---------------------------------------------------------------------------
  // Reactive: readonly toggle
  // ---------------------------------------------------------------------------

  $effect(() => {
    editor?.setEditable(!readonly);
  });

  // ---------------------------------------------------------------------------
  // Public: focus
  // ---------------------------------------------------------------------------

  export function focus(): void {
    editor?.commands.focus();
  }
</script>

<div class="tiptap-wrapper">
  <!-- VedoxLinkHandler intercepts vedox:// anchor clicks/hovers from ProseMirror output -->
  <VedoxLinkHandler>
    <!-- editorEl is the ProseMirror mount point -->
    <div
      bind:this={editorEl}
      class="tiptap-editor"
      class:tiptap-editor--readonly={readonly}
      aria-label="Rich text document editor"
    ></div>
  </VedoxLinkHandler>

  <!-- Floating bubble toolbar — appears above text selections -->
  {#if bubbleVisible && editor}
    <div
      class="bubble-toolbar-anchor"
      bind:this={bubbleEl}
      style:left="{bubbleX}px"
      style:top="{bubbleY}px"
    >
      <BubbleToolbar {editor} />
    </div>
  {/if}

  <!-- Mermaid popover floats above everything; listens for custom events -->
  <MermaidPopover />

  <!-- Slash command popover; listens for window events from SlashCommand extension -->
  <SlashCommandPopover />
</div>

<style>
  .tiptap-wrapper {
    height: 100%;
    display: flex;
    flex-direction: column;
    position: relative;
    overflow: hidden;
  }

  .tiptap-editor {
    flex: 1;
    overflow-y: auto;
    padding: 24px 32px;
    background: var(--color-surface-base);
    color: var(--color-text-primary);
    outline: none;
  }

  .tiptap-editor--readonly {
    cursor: default;
  }

  /* ---- Bubble toolbar anchor ---- */
  .bubble-toolbar-anchor {
    position: fixed;
    z-index: var(--z-popover, 40);
    /* Center the toolbar horizontally on the anchor point,
       and shift it fully above the anchor so it sits above the selection */
    transform: translateX(-50%) translateY(-100%);
    pointer-events: auto;
  }

  /* ---- ProseMirror content styles ---- */

  .tiptap-editor :global(.ProseMirror) {
    outline: none;
    max-width: var(--reading-measure, var(--measure-default, 68ch));
    margin: 0 auto;
    font-size: 15px;
    line-height: 1.75;
    font-family: var(--font-sans, system-ui, sans-serif);
  }

  .tiptap-editor :global(.ProseMirror > * + *) {
    margin-top: 0.75em;
  }

  /* Headings */
  .tiptap-editor :global(h1) {
    font-size: 2em;
    font-weight: 700;
    line-height: 1.2;
    margin-top: 1.5em;
    margin-bottom: 0.5em;
    color: var(--color-text-primary);
    border-bottom: 1px solid var(--color-border);
    padding-bottom: 0.3em;
  }

  .tiptap-editor :global(h2) {
    font-size: 1.5em;
    font-weight: 600;
    line-height: 1.3;
    margin-top: 1.4em;
    margin-bottom: 0.4em;
    color: var(--color-text-primary);
  }

  .tiptap-editor :global(h3) {
    font-size: 1.25em;
    font-weight: 600;
    margin-top: 1.2em;
    margin-bottom: 0.35em;
  }

  .tiptap-editor :global(h4) {
    font-size: 1.05em;
    font-weight: 600;
    margin-top: 1em;
    margin-bottom: 0.3em;
    color: var(--color-text-muted);
  }

  /* Paragraph */
  .tiptap-editor :global(p) {
    margin: 0;
  }

  /* Links */
  .tiptap-editor :global(.editor-link) {
    color: var(--color-accent);
    text-decoration: underline;
    text-underline-offset: 2px;
    text-decoration-color: color-mix(in srgb, var(--color-accent) 40%, transparent);
    cursor: pointer;
  }

  .tiptap-editor :global(.editor-link:hover) {
    color: var(--color-accent-hover);
    text-decoration-color: var(--color-accent-hover);
  }

  /* Bold / Italic */
  .tiptap-editor :global(strong) {
    font-weight: 700;
    color: var(--color-text-primary);
  }

  .tiptap-editor :global(em) {
    font-style: italic;
    color: var(--color-text-primary);
  }

  /* Inline code */
  .tiptap-editor :global(code:not(pre code)) {
    font-family: var(--font-mono);
    font-size: 0.875em;
    background: var(--color-surface-overlay);
    color: var(--color-accent);
    padding: 0.15em 0.4em;
    border-radius: var(--radius-sm);
    border: 1px solid var(--color-border);
  }

  /* Code block */
  .tiptap-editor :global(pre.code-block) {
    background: var(--color-surface-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: 16px 20px;
    overflow-x: auto;
  }

  .tiptap-editor :global(pre.code-block code) {
    font-family: var(--font-mono);
    font-size: 13px;
    background: none;
    color: var(--color-text-primary);
    border: none;
    padding: 0;
    border-radius: 0;
  }

  /* Blockquote */
  .tiptap-editor :global(blockquote) {
    border-left: 3px solid var(--color-accent);
    padding-left: 16px;
    margin: 1em 0;
    color: var(--color-text-muted);
    font-style: italic;
  }

  /* Lists */
  .tiptap-editor :global(ul),
  .tiptap-editor :global(ol) {
    padding-left: 1.5em;
  }

  .tiptap-editor :global(li + li) {
    margin-top: 0.25em;
  }

  .tiptap-editor :global(li p) {
    margin: 0;
  }

  /* HR */
  .tiptap-editor :global(hr) {
    border: none;
    border-top: 1px solid var(--color-border);
    margin: 2em 0;
  }

  /* ProseMirror placeholder */
  .tiptap-editor :global(.ProseMirror p.is-editor-empty:first-child::before) {
    content: attr(data-placeholder);
    float: left;
    color: var(--color-text-subtle);
    pointer-events: none;
    height: 0;
  }

  /* ProseMirror selection */
  .tiptap-editor :global(.ProseMirror-selectednode) {
    outline: 2px solid var(--color-accent);
    border-radius: var(--radius-sm);
  }

  /* ============================================================
     Callouts (GitHub alert syntax)
     ============================================================ */
  .tiptap-editor :global(.callout) {
    --callout-color: var(--info, #3b82f6);
    display: block;
    margin: 1.25em 0;
    padding: 12px 16px 12px 18px;
    border-left: 2px solid var(--callout-color);
    background: color-mix(in srgb, var(--callout-color) 7%, transparent);
    border-radius: 0 var(--radius-md, 8px) var(--radius-md, 8px) 0;
  }

  .tiptap-editor :global(.callout__header) {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 6px;
    font-size: 13px;
    font-weight: 600;
  }

  .tiptap-editor :global(.callout__icon) {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 16px;
    height: 16px;
    flex-shrink: 0;
  }

  .tiptap-editor :global(.callout__label) {
    font-weight: 600;
  }

  .tiptap-editor :global(.callout__body) {
    color: var(--color-text-primary);
  }

  .tiptap-editor :global(.callout__body > * + *) {
    margin-top: 0.5em;
  }

  .tiptap-editor :global(.callout__body > p:first-child) {
    margin-top: 0;
  }

  .tiptap-editor :global(.callout__body > p:last-child) {
    margin-bottom: 0;
  }

  /* ============================================================
     KaTeX math
     ============================================================ */
  .tiptap-editor :global(.katex-inline) {
    display: inline-block;
    cursor: pointer;
    padding: 0 2px;
    border-radius: var(--radius-sm, 4px);
    transition: background-color 120ms ease;
  }

  .tiptap-editor :global(.katex-inline:hover) {
    background: var(--surface-4, rgba(255, 255, 255, 0.06));
  }

  .tiptap-editor :global(.katex-block) {
    display: block;
    margin: 1.5em 0;
    padding: 16px;
    background: var(--surface-2, rgba(255, 255, 255, 0.02));
    border: 1px solid var(--border-hairline, rgba(255, 255, 255, 0.06));
    border-radius: var(--radius-md, 8px);
    cursor: pointer;
    text-align: center;
    overflow-x: auto;
    transition: border-color 120ms ease;
  }

  .tiptap-editor :global(.katex-block:hover) {
    border-color: var(--border-default, rgba(255, 255, 255, 0.12));
  }

  .tiptap-editor :global(.katex-block__display) {
    font-size: 1.05em;
  }

  /* ============================================================
     Footnote references
     ============================================================ */
  .tiptap-editor :global(.footnote-ref) {
    font-size: 0.75em;
    line-height: 0;
    vertical-align: super;
    margin: 0 1px;
  }

  .tiptap-editor :global(.footnote-ref__link) {
    color: var(--color-accent);
    text-decoration: none;
    font-weight: 600;
    font-variant-numeric: tabular-nums;
  }

  .tiptap-editor :global(.footnote-ref__link:hover) {
    text-decoration: underline;
    text-underline-offset: 2px;
  }

  /* ============================================================
     Tables (GFM round-trip)
     ============================================================ */
  .tiptap-editor :global(table.vedox-table) {
    width: 100%;
    border-collapse: collapse;
    margin: 1.25em 0;
    font-size: 13px;
    font-variant-numeric: tabular-nums;
    table-layout: fixed;
  }

  .tiptap-editor :global(table.vedox-table th),
  .tiptap-editor :global(table.vedox-table td) {
    padding: 8px 12px;
    border: 1px solid var(--border-hairline, rgba(255, 255, 255, 0.08));
    text-align: left;
    vertical-align: top;
    min-width: 60px;
    position: relative;
  }

  .tiptap-editor :global(table.vedox-table th) {
    background: var(--surface-2, rgba(255, 255, 255, 0.03));
    font-weight: 600;
    color: var(--color-text-primary);
    position: sticky;
    top: 0;
  }

  .tiptap-editor :global(table.vedox-table td) {
    color: var(--color-text-primary);
  }

  .tiptap-editor :global(.tableWrapper) {
    overflow-x: auto;
    margin: 1.25em 0;
  }

  .tiptap-editor :global(table.vedox-table .selectedCell) {
    background: var(--accent-subtle, rgba(59, 130, 246, 0.1));
  }

  .tiptap-editor :global(table.vedox-table .column-resize-handle) {
    position: absolute;
    right: -2px;
    top: 0;
    bottom: 0;
    width: 4px;
    background: var(--accent-solid, #3b82f6);
    cursor: col-resize;
    opacity: 0;
    transition: opacity 120ms ease;
  }

  .tiptap-editor :global(table.vedox-table:hover .column-resize-handle) {
    opacity: 0.3;
  }

  .tiptap-editor :global(table.vedox-table .column-resize-handle:hover) {
    opacity: 1;
  }

  /* ============================================================
     Images with captions
     ============================================================ */
  .tiptap-editor :global(img.vedox-image) {
    max-width: 100%;
    height: auto;
    display: block;
    margin: 1.5em auto 0.25em;
    border-radius: var(--radius-md, 8px);
  }

  /* Image caption: italic paragraph immediately following an image */
  .tiptap-editor :global(img.vedox-image + p),
  .tiptap-editor :global(p:has(> img.vedox-image) + p) {
    font-style: italic;
    color: var(--color-text-muted);
    text-align: center;
    font-size: 0.9em;
    margin-top: 0.25em;
    margin-bottom: 1.5em;
  }
</style>
