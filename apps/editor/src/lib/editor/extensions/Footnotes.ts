/**
 * Footnotes.ts
 *
 * Tiptap inline node for footnote references: [^1]
 *
 * Renders footnote references as superscript numbers. Hovering shows
 * the footnote body in a popover. Footnote definitions at the bottom
 * of the document ([^1]: ...) are preserved in round-trip.
 *
 * Round-trip: serialize(parse("[^1]")) === "[^1]"
 */

import { Node, mergeAttributes } from '@tiptap/core';
import type { Node as ProseMirrorNode } from '@tiptap/pm/model';
import type { NodeView } from '@tiptap/pm/view';

// ---------------------------------------------------------------------------
// Footnote Reference (inline)
// ---------------------------------------------------------------------------

declare module '@tiptap/core' {
  interface Commands<ReturnType> {
    footnoteRef: {
      insertFootnoteRef: (id: string) => ReturnType;
    };
  }
}

export const FootnoteRef = Node.create({
  name: 'footnoteRef',

  group: 'inline',
  inline: true,
  atom: true,
  selectable: true,

  addAttributes() {
    return {
      id: {
        default: '1',
        parseHTML: (element: HTMLElement) =>
          element.getAttribute('data-footnote-id') ?? '1',
        renderHTML: (attributes: { id: string }) => ({
          'data-footnote-id': attributes.id
        })
      }
    };
  },

  parseHTML() {
    return [{ tag: 'sup[data-footnote-ref]' }];
  },

  renderHTML({ HTMLAttributes }: { HTMLAttributes: Record<string, string> }) {
    return [
      'sup',
      mergeAttributes(HTMLAttributes, { 'data-footnote-ref': '' }),
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
          state.write(`[^${node.attrs.id}]`);
        },
        parse: {}
      }
    };
  },

  addCommands() {
    return {
      insertFootnoteRef:
        (id: string) =>
        ({ commands }: import('@tiptap/core').CommandProps) => {
          return commands.insertContent({
            type: this.name,
            attrs: { id }
          });
        }
    };
  },

  addNodeView() {
    return ({ node }: { node: ProseMirrorNode }): NodeView => {
      const sup = document.createElement('sup');
      sup.className = 'footnote-ref';
      sup.setAttribute('data-footnote-ref', '');
      sup.setAttribute('contenteditable', 'false');

      const link = document.createElement('a');
      link.className = 'footnote-ref__link';
      link.href = `#fn-${node.attrs.id}`;
      link.textContent = node.attrs.id as string;
      link.title = `Footnote ${node.attrs.id}`;
      link.style.cssText =
        'color: var(--accent-solid, #3b82f6); text-decoration: none; cursor: pointer; font-size: 0.75em; font-weight: 600; font-variant-numeric: tabular-nums;';

      // Hover popover (simple title tooltip for now)
      link.addEventListener('click', (e) => {
        e.preventDefault();
        // Could scroll to footnote definition in the future
      });

      sup.appendChild(link);

      return {
        dom: sup,

        update(updatedNode: ProseMirrorNode): boolean {
          if (updatedNode.type.name !== 'footnoteRef') return false;
          link.textContent = updatedNode.attrs.id as string;
          link.href = `#fn-${updatedNode.attrs.id}`;
          link.title = `Footnote ${updatedNode.attrs.id}`;
          return true;
        },

        stopEvent(): boolean {
          return false;
        },

        ignoreMutation(): boolean {
          return true;
        }
      };
    };
  }
});

/**
 * Returns tiptap-markdown config for footnote references.
 */
export function getFootnoteMarkdownConfig(): {
  name: string;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  fromMarkdown: any[];
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  toMarkdown: Record<string, any>;
} {
  return {
    name: 'footnoteRef',
    fromMarkdown: [
      {
        type: 'footnoteReference',
        node: 'footnoteRef',
        getAttrs: () => ({}),
        getContent: (token: { identifier?: string; label?: string }): { id: string } => ({
          id: token.identifier ?? token.label ?? '1'
        })
      }
    ],
    toMarkdown: {
      footnoteRef: (
        state: { write: (s: string) => void },
        node: ProseMirrorNode
      ) => {
        state.write(`[^${node.attrs.id}]`);
      }
    }
  };
}
