<!--
  CodeMirrorEditor.svelte

  Code Mode: raw Markdown editing via CodeMirror 6.

  Features:
  - Markdown syntax highlighting (@codemirror/lang-markdown)
  - One Dark theme (@codemirror/theme-one-dark)
  - Line numbers (lineNumbers extension)
  - Monospace font (--font-mono design token)
  - Full-canvas, no toolbar
  - Preserves all whitespace, YAML frontmatter, raw Markdown exactly

  Props:
    content: string   — the full raw Markdown (including frontmatter)
    readonly?: boolean

  Events:
    onchange: (content: string) => void  — fired on every editor change
-->

<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { EditorView, lineNumbers, keymap, highlightActiveLine } from '@codemirror/view';
  import { EditorState, Compartment } from '@codemirror/state';
  import { markdown } from '@codemirror/lang-markdown';
  import { oneDark } from '@codemirror/theme-one-dark';
  import { defaultKeymap, history, historyKeymap } from '@codemirror/commands';
  import { syntaxHighlighting, defaultHighlightStyle } from '@codemirror/language';
  import {
    highlightActiveLineGutter,
    gutter
  } from '@codemirror/view';
  import { densityStore } from '$lib/theme/store';

  // ---------------------------------------------------------------------------
  // Props
  // ---------------------------------------------------------------------------

  interface Props {
    content: string;
    readonly?: boolean;
    onchange?: (content: string) => void;
  }

  let { content = $bindable(), readonly = false, onchange }: Props = $props();

  // ---------------------------------------------------------------------------
  // Refs
  // ---------------------------------------------------------------------------

  let containerEl: HTMLDivElement | undefined = $state(undefined);
  let view: EditorView | undefined;
  let unsubDensity: (() => void) | undefined;

  // Compartment lets us swap readonly state without rebuilding the editor.
  const readonlyCompartment = new Compartment();

  // ---------------------------------------------------------------------------
  // Build extensions list
  // ---------------------------------------------------------------------------

  function buildExtensions(ro: boolean) {
    return [
      history(),
      lineNumbers(),
      highlightActiveLine(),
      highlightActiveLineGutter(),
      gutter({ class: 'cm-breakpoint-gutter' }),
      markdown(),
      oneDark,
      syntaxHighlighting(defaultHighlightStyle, { fallback: true }),
      keymap.of([...defaultKeymap, ...historyKeymap]),
      readonlyCompartment.of(EditorState.readOnly.of(ro)),
      EditorView.updateListener.of((update) => {
        if (update.docChanged) {
          const newContent = update.state.doc.toString();
          content = newContent;
          onchange?.(newContent);
        }
      }),
      EditorView.theme({
        '&': {
          height: '100%',
          fontSize: '14px',
          fontFamily: 'var(--font-mono)'
        },
        '.cm-scroller': {
          fontFamily: 'inherit',
          lineHeight: '1.65'
        },
        '.cm-content': {
          padding: '16px 0',
          caretColor: 'var(--color-accent)'
        },
        '.cm-line': {
          padding: '0 20px'
        },
        '.cm-gutters': {
          background: 'var(--color-surface-code)',
          border: 'none',
          borderRight: '1px solid var(--color-border)',
          color: 'var(--color-text-subtle)',
          minWidth: '48px'
        },
        '.cm-activeLineGutter': {
          background: 'color-mix(in srgb, var(--color-accent) 10%, transparent)'
        },
        '.cm-activeLine': {
          background: 'color-mix(in srgb, var(--color-accent) 5%, transparent)'
        },
        '.cm-cursor': {
          borderLeftColor: 'var(--color-accent)'
        }
      })
    ];
  }

  // ---------------------------------------------------------------------------
  // Lifecycle
  // ---------------------------------------------------------------------------

  onMount(() => {
    if (!containerEl) return;

    const state = EditorState.create({
      doc: content,
      extensions: buildExtensions(readonly)
    });

    view = new EditorView({
      state,
      parent: containerEl
    });

    // Sync the --density CSS custom property on the CodeMirror root element
    // so that any density-aware styles inside the editor react to changes.
    unsubDensity = densityStore.subscribe(density => {
      const multiplier = density === 'compact' ? 0.875 : density === 'cozy' ? 1.125 : 1.0;
      if (view) {
        view.dom.style.setProperty('--density', String(multiplier));
      }
    });
  });

  onDestroy(() => {
    unsubDensity?.();
    view?.destroy();
    view = undefined;
  });

  // ---------------------------------------------------------------------------
  // Reactive: sync external content changes (e.g. mode switch)
  // ---------------------------------------------------------------------------

  $effect(() => {
    if (!view) return;
    const currentContent = view.state.doc.toString();
    if (content !== currentContent) {
      view.dispatch({
        changes: {
          from: 0,
          to: view.state.doc.length,
          insert: content
        }
      });
    }
  });

  // ---------------------------------------------------------------------------
  // Reactive: sync readonly state
  // ---------------------------------------------------------------------------

  $effect(() => {
    if (!view) return;
    view.dispatch({
      effects: readonlyCompartment.reconfigure(EditorState.readOnly.of(readonly))
    });
  });

  // ---------------------------------------------------------------------------
  // Public: focus
  // ---------------------------------------------------------------------------

  export function focus(): void {
    view?.focus();
  }
</script>

<div
  bind:this={containerEl}
  class="codemirror-container"
  aria-label="Markdown code editor"
></div>

<style>
  .codemirror-container {
    height: 100%;
    overflow: hidden;
    background: var(--color-surface-code);
  }

  /* Ensure CodeMirror fills the container */
  .codemirror-container :global(.cm-editor) {
    height: 100%;
    outline: none;
  }

  .codemirror-container :global(.cm-editor.cm-focused) {
    outline: none;
  }

  .codemirror-container :global(.cm-scroller) {
    overflow: auto;
    height: 100%;
  }
</style>
