/**
 * MermaidNode.ts
 *
 * Custom Tiptap node extension for fenced Mermaid code blocks.
 *
 * Markdown representation (round-trip canonical form):
 *   ```mermaid
 *   <source>
 *   ```
 *
 * In WYSIWYG mode this renders as a non-editable SVG preview island.
 * A single click on the island opens an inline code popover (handled
 * by MermaidPopover.svelte via a custom DOM event). The popover fires
 * a `mermaid-update` CustomEvent with the new source when blurred.
 *
 * Security:
 *   - SVG output from mermaid.render() is sanitized with DOMPurify
 *     before insertion into the shadow DOM of the node view.
 *   - The Mermaid securityLevel is 'strict' (no click handlers in SVG).
 *
 * Round-trip rule:
 *   serialize(parse("```mermaid\n<source>\n```")) === "```mermaid\n<source>\n```"
 *   Enforced by golden-file 06-mermaid-block.md.
 */

import { Node, mergeAttributes } from '@tiptap/core';
import type { Node as ProseMirrorNode } from '@tiptap/pm/model';
import type { NodeView, EditorView } from '@tiptap/pm/view';
import DOMPurify from 'dompurify';
import { renderMermaid } from '../utils/mermaidCache.js';

// ---------------------------------------------------------------------------
// Node schema
// ---------------------------------------------------------------------------

declare module '@tiptap/core' {
  interface Commands<ReturnType> {
    mermaidNode: {
      insertMermaid: (source: string) => ReturnType;
    };
  }
}

export const MermaidNode = Node.create({
  name: 'mermaidNode',

  group: 'block',
  atom: true, // non-editable island — ProseMirror treats as single unit
  draggable: true,
  selectable: true,
  defining: true,

  addAttributes() {
    return {
      source: {
        default: '',
        parseHTML: (element: HTMLElement) =>
          element.getAttribute('data-mermaid-source') ?? '',
        renderHTML: (attributes: { source: string }) => ({
          'data-mermaid-source': attributes.source
        })
      }
    };
  },

  // ---------------------------------------------------------------------------
  // HTML parsing (for clipboard paste and initial load)
  // ---------------------------------------------------------------------------

  parseHTML() {
    return [
      {
        tag: 'div[data-mermaid-node]'
      }
    ];
  },

  renderHTML({ HTMLAttributes }: { HTMLAttributes: Record<string, string> }) {
    return ['div', mergeAttributes(HTMLAttributes, { 'data-mermaid-node': '' })];
  },

  // ---------------------------------------------------------------------------
  // Markdown parsing / serialization
  // ---------------------------------------------------------------------------

  addStorage() {
    return {
      markdown: {
        serialize(
          state: { write: (s: string) => void; ensureNewLine: () => void },
          node: ProseMirrorNode
        ) {
          state.ensureNewLine();
          state.write('```mermaid\n');
          state.write(node.attrs.source as string);
          if (!(node.attrs.source as string).endsWith('\n')) {
            state.write('\n');
          }
          state.write('```');
          state.ensureNewLine();
        },
        parse: {
          // Handled by the fenced code block parser — we intercept at the
          // TiptapEditor level via the @tiptap/extension-markdown inputRules.
          // The fromMarkdown transformer is registered below.
        }
      }
    };
  },

  // ---------------------------------------------------------------------------
  // Commands
  // ---------------------------------------------------------------------------

  addCommands() {
    return {
      insertMermaid:
        (source: string) =>
        ({ commands }: import('@tiptap/core').CommandProps) => {
          return commands.insertContent({
            type: this.name,
            attrs: { source }
          });
        }
    };
  },

  // ---------------------------------------------------------------------------
  // Node view (custom DOM rendering for WYSIWYG mode)
  // ---------------------------------------------------------------------------

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
      // Outer wrapper
      const wrapper = document.createElement('div');
      wrapper.className = 'mermaid-node';
      wrapper.setAttribute('data-mermaid-node', '');
      wrapper.setAttribute('tabindex', '0');
      wrapper.setAttribute('role', 'img');
      wrapper.setAttribute('aria-label', 'Mermaid diagram. Click to edit source.');

      // SVG container
      const svgContainer = document.createElement('div');
      svgContainer.className = 'mermaid-svg-container';
      wrapper.appendChild(svgContainer);

      // Loading indicator
      const loadingEl = document.createElement('div');
      loadingEl.className = 'mermaid-loading';
      loadingEl.textContent = 'Rendering diagram…';
      svgContainer.appendChild(loadingEl);

      // Render SVG asynchronously
      let currentSource = node.attrs.source as string;

      function isDarkMode(): boolean {
        return document.documentElement.getAttribute('data-theme-mode') === 'dark';
      }

      async function renderSvg(source: string): Promise<void> {
        const darkMode = isDarkMode();
        const rawSvg = await renderMermaid(source, darkMode);
        // Sanitize before DOM insertion — security requirement.
        const cleanSvg = DOMPurify.sanitize(rawSvg, {
          USE_PROFILES: { svg: true, svgFilters: true }
        });
        svgContainer.innerHTML = cleanSvg;
      }

      renderSvg(currentSource);

      // Re-render when the theme mode changes (dark ↔ light).
      // MutationObserver watches data-theme-mode on <html>.
      let lastMode = isDarkMode();
      const themeObserver = new MutationObserver(() => {
        const nowDark = isDarkMode();
        if (nowDark !== lastMode) {
          lastMode = nowDark;
          renderSvg(currentSource);
        }
      });
      themeObserver.observe(document.documentElement, {
        attributes: true,
        attributeFilter: ['data-theme-mode']
      });

      // Click handler — dispatch custom event for MermaidPopover.svelte
      function handleClick(e: MouseEvent): void {
        e.stopPropagation();
        wrapper.dispatchEvent(
          new CustomEvent('mermaid-open-popover', {
            bubbles: true,
            detail: {
              source: currentSource,
              anchorEl: wrapper,
              onUpdate: (newSource: string) => {
                const pos = getPos();
                if (pos === undefined) return;
                view.dispatch(
                  view.state.tr.setNodeMarkup(pos, undefined, {
                    source: newSource
                  })
                );
              }
            }
          })
        );
      }

      wrapper.addEventListener('click', handleClick);
      wrapper.addEventListener('keydown', (e: KeyboardEvent) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          handleClick(e as unknown as MouseEvent);
        }
      });

      return {
        dom: wrapper,

        update(updatedNode: ProseMirrorNode): boolean {
          if (updatedNode.type !== node.type) return false;
          const newSource = updatedNode.attrs.source as string;
          if (newSource !== currentSource) {
            currentSource = newSource;
            renderSvg(newSource);
          }
          return true;
        },

        destroy(): void {
          themeObserver.disconnect();
          wrapper.removeEventListener('click', handleClick);
        },

        stopEvent(): boolean {
          // Prevent ProseMirror from handling events inside the island.
          return true;
        },

        ignoreMutation(): boolean {
          return true;
        }
      };
    };
  }
});

// ---------------------------------------------------------------------------
// Markdown extension helper — registers the mermaid fenced-block transformer.
// Called from TiptapEditor.svelte when building the extension list.
// ---------------------------------------------------------------------------

/**
 * Returns the fromMarkdown configuration for @tiptap/extension-markdown.
 * Intercepts fenced code blocks with `language: mermaid` and converts them
 * to MermaidNode atoms instead of CodeBlock nodes.
 */
export function getMermaidMarkdownConfig(): {
  name: string;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  fromMarkdown: any[];
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  toMarkdown: Record<string, any>;
} {
  return {
    name: 'mermaidNode',
    fromMarkdown: [
      {
        type: 'code' as const,
        // mdast node type for fenced code
        getAttrs: (token: { lang?: string }) => {
          if (token.lang !== 'mermaid') return false;
          return {};
        },
        node: 'mermaidNode',
        getContent: (
          token: { value?: string },
          _schema: unknown
        ): { source: string } => ({
          source: token.value ?? ''
        })
      }
    ],
    toMarkdown: {
      mermaidNode: (
        state: { write: (s: string) => void; ensureNewLine: () => void },
        node: ProseMirrorNode
      ) => {
        state.ensureNewLine();
        const src = (node.attrs.source as string) ?? '';
        state.write('```mermaid\n');
        state.write(src);
        if (!src.endsWith('\n')) state.write('\n');
        state.write('```');
        state.ensureNewLine();
      }
    }
  };
}
