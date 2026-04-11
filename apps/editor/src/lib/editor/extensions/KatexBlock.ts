/**
 * KatexBlock.ts
 *
 * Tiptap block node for display math: $$...$$
 *
 * Renders via KaTeX (lazy-loaded on first use) in display mode.
 * Clicking the rendered math opens a textarea editor for the LaTeX source.
 *
 * Round-trip: serialize(parse("$$\n\\int_0^1 x dx\n$$")) === "$$\n\\int_0^1 x dx\n$$"
 */

import { Node, mergeAttributes } from '@tiptap/core';
import type { Node as ProseMirrorNode } from '@tiptap/pm/model';
import type { NodeView, EditorView } from '@tiptap/pm/view';

// ---------------------------------------------------------------------------
// Lazy KaTeX loader (shared with KatexInline)
// ---------------------------------------------------------------------------

let katexModule: typeof import('katex') | null = null;
let katexCssLoaded = false;

async function loadKatex(): Promise<typeof import('katex')> {
  if (katexModule) return katexModule;
  katexModule = await import('katex');

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
    katexBlock: {
      insertBlockMath: (latex: string) => ReturnType;
    };
  }
}

export const KatexBlock = Node.create({
  name: 'katexBlock',

  group: 'block',
  atom: true,
  selectable: true,
  draggable: true,
  defining: true,

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
    return [{ tag: 'div[data-katex-block]' }];
  },

  renderHTML({ HTMLAttributes }: { HTMLAttributes: Record<string, string> }) {
    return [
      'div',
      mergeAttributes(HTMLAttributes, { 'data-katex-block': '' })
    ];
  },

  addStorage() {
    return {
      markdown: {
        serialize(
          state: { write: (s: string) => void; ensureNewLine: () => void },
          node: ProseMirrorNode
        ) {
          const latex = node.attrs.latex as string;
          state.ensureNewLine();
          state.write('$$\n');
          state.write(latex);
          if (!latex.endsWith('\n')) state.write('\n');
          state.write('$$');
          state.ensureNewLine();
        },
        parse: {}
      }
    };
  },

  addCommands() {
    return {
      insertBlockMath:
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
      const wrapper = document.createElement('div');
      wrapper.className = 'katex-block';
      wrapper.setAttribute('data-katex-block', '');
      wrapper.setAttribute('tabindex', '0');
      wrapper.setAttribute('role', 'math');
      wrapper.setAttribute('contenteditable', 'false');

      const display = document.createElement('div');
      display.className = 'katex-block__display';
      wrapper.appendChild(display);

      let currentLatex = node.attrs.latex as string;
      let isEditing = false;

      async function renderMath(latex: string): Promise<void> {
        try {
          const katex = await loadKatex();
          display.innerHTML = '';
          const mathEl = document.createElement('div');
          katex.default.render(latex, mathEl, {
            throwOnError: false,
            displayMode: true
          });
          display.appendChild(mathEl);
        } catch {
          display.textContent = `$$${latex}$$`;
        }
      }

      renderMath(currentLatex);

      // Click to edit
      wrapper.addEventListener('click', (e) => {
        e.stopPropagation();
        if (isEditing) return;
        isEditing = true;

        const textarea = document.createElement('textarea');
        textarea.className = 'katex-block__editor';
        textarea.value = currentLatex;
        textarea.rows = Math.max(3, currentLatex.split('\n').length + 1);
        textarea.style.cssText =
          'width: 100%; font-family: var(--font-mono); font-size: 13px; padding: 12px 16px; border: 1px solid var(--accent-solid, #3b82f6); border-radius: 8px; background: var(--surface-2, #1a1a1a); color: var(--text-1, #fff); outline: none; resize: vertical;';

        wrapper.innerHTML = '';
        wrapper.appendChild(textarea);
        textarea.focus();

        function commit(): void {
          const newLatex = textarea.value.trim();
          if (newLatex !== currentLatex) {
            currentLatex = newLatex;
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
          wrapper.innerHTML = '';
          wrapper.appendChild(display);
          renderMath(currentLatex);
        }

        textarea.addEventListener('blur', commit);
        textarea.addEventListener('keydown', (ev) => {
          if (ev.key === 'Escape') {
            isEditing = false;
            wrapper.innerHTML = '';
            wrapper.appendChild(display);
            renderMath(currentLatex);
          }
        });
      });

      return {
        dom: wrapper,

        update(updatedNode: ProseMirrorNode): boolean {
          if (updatedNode.type.name !== 'katexBlock') return false;
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
        },

        destroy(): void {
          // Nothing to clean up
        }
      };
    };
  }
});

/**
 * Returns tiptap-markdown config for block math.
 */
export function getKatexBlockMarkdownConfig(): {
  name: string;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  fromMarkdown: any[];
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  toMarkdown: Record<string, any>;
} {
  return {
    name: 'katexBlock',
    fromMarkdown: [
      {
        type: 'math',
        node: 'katexBlock',
        getAttrs: () => ({}),
        getContent: (token: { value?: string }): { latex: string } => ({
          latex: token.value ?? ''
        })
      }
    ],
    toMarkdown: {
      katexBlock: (
        state: { write: (s: string) => void; ensureNewLine: () => void },
        node: ProseMirrorNode
      ) => {
        const latex = node.attrs.latex as string;
        state.ensureNewLine();
        state.write('$$\n');
        state.write(latex);
        if (!latex.endsWith('\n')) state.write('\n');
        state.write('$$');
        state.ensureNewLine();
      }
    }
  };
}
