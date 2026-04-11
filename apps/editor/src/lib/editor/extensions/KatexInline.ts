/**
 * KatexInline.ts
 *
 * Tiptap inline node for inline math: $x^2$
 *
 * Renders via KaTeX (lazy-loaded on first use). In the editor,
 * clicking the rendered math opens an inline editor for the LaTeX source.
 *
 * Round-trip: serialize(parse("$x^2$")) === "$x^2$"
 */

import { Node, mergeAttributes } from '@tiptap/core';
import type { Node as ProseMirrorNode } from '@tiptap/pm/model';
import type { NodeView, EditorView } from '@tiptap/pm/view';

// ---------------------------------------------------------------------------
// Lazy KaTeX loader
// ---------------------------------------------------------------------------

let katexModule: typeof import('katex') | null = null;
let katexCssLoaded = false;

async function loadKatex(): Promise<typeof import('katex')> {
  if (katexModule) return katexModule;
  katexModule = await import('katex');

  // Inject KaTeX CSS on first use
  if (!katexCssLoaded && typeof document !== 'undefined') {
    katexCssLoaded = true;
    const link = document.createElement('link');
    link.rel = 'stylesheet';
    link.href = 'https://cdn.jsdelivr.net/npm/katex@0.16.11/dist/katex.min.css';
    link.crossOrigin = 'anonymous';
    document.head.appendChild(link);
  }

  return katexModule;
}

// ---------------------------------------------------------------------------
// Node definition
// ---------------------------------------------------------------------------

declare module '@tiptap/core' {
  interface Commands<ReturnType> {
    katexInline: {
      insertInlineMath: (latex: string) => ReturnType;
    };
  }
}

export const KatexInline = Node.create({
  name: 'katexInline',

  group: 'inline',
  inline: true,
  atom: true,
  selectable: true,
  draggable: false,

  addAttributes() {
    return {
      latex: {
        default: '',
        parseHTML: (element: HTMLElement) =>
          element.getAttribute('data-latex') ?? '',
        renderHTML: (attributes: { latex: string }) => ({
          'data-latex': attributes.latex
        })
      }
    };
  },

  parseHTML() {
    return [{ tag: 'span[data-katex-inline]' }];
  },

  renderHTML({ HTMLAttributes }: { HTMLAttributes: Record<string, string> }) {
    return [
      'span',
      mergeAttributes(HTMLAttributes, { 'data-katex-inline': '' }),
      0
    ];
  },

  addStorage() {
    return {
      markdown: {
        serialize(
          state: { write: (s: string) => void },
          node: ProseMirrorNode
        ) {
          const latex = node.attrs.latex as string;
          state.write(`$${latex}$`);
        },
        parse: {}
      }
    };
  },

  addCommands() {
    return {
      insertInlineMath:
        (latex: string) =>
        ({ commands }: import('@tiptap/core').CommandProps) => {
          return commands.insertContent({
            type: this.name,
            attrs: { latex }
          });
        }
    };
  },

  addNodeView() {
    return ({
      node,
      view,
      getPos
    }: {
      node: ProseMirrorNode;
      view: EditorView;
      getPos: () => number | undefined;
    }): NodeView => {
      const span = document.createElement('span');
      span.className = 'katex-inline';
      span.setAttribute('data-katex-inline', '');
      span.setAttribute('contenteditable', 'false');

      let currentLatex = node.attrs.latex as string;
      let isEditing = false;

      async function renderMath(latex: string): Promise<void> {
        try {
          const katex = await loadKatex();
          span.innerHTML = '';
          const mathEl = document.createElement('span');
          katex.default.render(latex, mathEl, {
            throwOnError: false,
            displayMode: false
          });
          span.appendChild(mathEl);
        } catch {
          span.textContent = `$${latex}$`;
        }
      }

      renderMath(currentLatex);

      // Click to edit
      span.addEventListener('click', (e) => {
        e.stopPropagation();
        if (isEditing) return;
        isEditing = true;

        const input = document.createElement('input');
        input.type = 'text';
        input.className = 'katex-inline-editor';
        input.value = currentLatex;
        input.style.cssText =
          'font-family: var(--font-mono); font-size: 13px; padding: 2px 6px; border: 1px solid var(--accent-solid, #3b82f6); border-radius: 4px; background: var(--surface-2, #1a1a1a); color: var(--text-1, #fff); outline: none;';

        span.innerHTML = '';
        span.appendChild(input);
        input.focus();
        input.select();

        function commit(): void {
          const newLatex = input.value.trim();
          if (newLatex && newLatex !== currentLatex) {
            const pos = getPos();
            if (pos !== undefined) {
              view.dispatch(
                view.state.tr.setNodeMarkup(pos, undefined, {
                  latex: newLatex
                })
              );
            }
          }
          isEditing = false;
          renderMath(newLatex || currentLatex);
        }

        input.addEventListener('blur', commit);
        input.addEventListener('keydown', (ev) => {
          if (ev.key === 'Enter') {
            ev.preventDefault();
            input.blur();
          }
          if (ev.key === 'Escape') {
            isEditing = false;
            renderMath(currentLatex);
          }
        });
      });

      return {
        dom: span,

        update(updatedNode: ProseMirrorNode): boolean {
          if (updatedNode.type.name !== 'katexInline') return false;
          const newLatex = updatedNode.attrs.latex as string;
          if (newLatex !== currentLatex && !isEditing) {
            currentLatex = newLatex;
            renderMath(newLatex);
          }
          return true;
        },

        stopEvent(): boolean {
          return isEditing;
        },

        ignoreMutation(): boolean {
          return true;
        }
      };
    };
  }
});

/**
 * Returns tiptap-markdown config for inline math.
 * Intercepts inline code that matches $..$ pattern.
 */
export function getKatexInlineMarkdownConfig(): {
  name: string;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  fromMarkdown: any[];
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  toMarkdown: Record<string, any>;
} {
  return {
    name: 'katexInline',
    fromMarkdown: [
      {
        type: 'inlineMath',
        node: 'katexInline',
        getAttrs: () => ({}),
        getContent: (token: { value?: string }): { latex: string } => ({
          latex: token.value ?? ''
        })
      }
    ],
    toMarkdown: {
      katexInline: (
        state: { write: (s: string) => void },
        node: ProseMirrorNode
      ) => {
        state.write(`$${node.attrs.latex}$`);
      }
    }
  };
}
