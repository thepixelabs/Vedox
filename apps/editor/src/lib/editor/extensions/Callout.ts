/**
 * Callout.ts
 *
 * Custom Tiptap node extension for GitHub-style alert callouts.
 *
 * Markdown representation (round-trip canonical form):
 *   > [!NOTE]
 *   > Body text here
 *
 *   > [!TIP] Custom title
 *   > Body text here
 *
 * Five callout types: NOTE, TIP, WARNING, DANGER, INFO
 *
 * In WYSIWYG mode this renders via CalloutView.svelte as a styled box
 * with a 2px left border in the status color, 7% background wash,
 * and an icon + title header.
 *
 * Round-trip rule:
 *   serialize(parse(input)) === input
 */

import { Node, mergeAttributes } from '@tiptap/core';
import type { Node as ProseMirrorNode } from '@tiptap/pm/model';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type CalloutType = 'NOTE' | 'TIP' | 'WARNING' | 'DANGER' | 'INFO';

const CALLOUT_TYPES: CalloutType[] = ['NOTE', 'TIP', 'WARNING', 'DANGER', 'INFO'];

/**
 * Maps callout types to semantic status colors (CSS variable names).
 */
export const CALLOUT_COLORS: Record<CalloutType, string> = {
  NOTE: 'var(--info, #3b82f6)',
  TIP: 'var(--success, #22c55e)',
  WARNING: 'var(--warning, #f59e0b)',
  DANGER: 'var(--error, #ef4444)',
  INFO: 'var(--info, #3b82f6)'
};

/**
 * SVG icon paths for each callout type (Lucide-style).
 */
export const CALLOUT_ICONS: Record<CalloutType, string> = {
  NOTE: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><path d="M12 16v-4"/><path d="M12 8h.01"/></svg>',
  TIP: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 2v1"/><path d="M12 21v1"/><path d="m4.93 4.93.7.7"/><path d="m17.37 17.37.7.7"/><path d="M2 12h1"/><path d="M21 12h1"/><path d="m4.93 19.07.7-.7"/><path d="m17.37 6.63.7-.7"/><circle cx="12" cy="12" r="4"/></svg>',
  WARNING: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3Z"/><path d="M12 9v4"/><path d="M12 17h.01"/></svg>',
  DANGER: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0Z"/><line x1="12" x2="12" y1="9" y2="13"/><line x1="12" x2="12.01" y1="17" y2="17"/></svg>',
  INFO: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><path d="M12 16v-4"/><path d="M12 8h.01"/></svg>'
};

// ---------------------------------------------------------------------------
// Regex for parsing callout blocks from markdown
// ---------------------------------------------------------------------------

const CALLOUT_REGEX = /^\[!(NOTE|TIP|WARNING|DANGER|INFO)\](?:\s+(.+))?$/;

// ---------------------------------------------------------------------------
// Node schema
// ---------------------------------------------------------------------------

declare module '@tiptap/core' {
  interface Commands<ReturnType> {
    callout: {
      insertCallout: (type: CalloutType, title?: string) => ReturnType;
    };
  }
}

export const Callout = Node.create({
  name: 'callout',

  group: 'block',
  content: 'block+',
  defining: true,

  addAttributes() {
    return {
      calloutType: {
        default: 'NOTE' as CalloutType,
        parseHTML: (element: HTMLElement) =>
          (element.getAttribute('data-callout-type') as CalloutType) ?? 'NOTE',
        renderHTML: (attributes: { calloutType: CalloutType }) => ({
          'data-callout-type': attributes.calloutType
        })
      },
      title: {
        default: null as string | null,
        parseHTML: (element: HTMLElement) =>
          element.getAttribute('data-callout-title') || null,
        renderHTML: (attributes: { title: string | null }) => {
          if (!attributes.title) return {};
          return { 'data-callout-title': attributes.title };
        }
      }
    };
  },

  parseHTML() {
    return [
      {
        tag: 'div[data-callout]'
      }
    ];
  },

  renderHTML({ HTMLAttributes }: { HTMLAttributes: Record<string, string> }) {
    return [
      'div',
      mergeAttributes(HTMLAttributes, { 'data-callout': '' }),
      0
    ];
  },

  addStorage() {
    return {
      markdown: {
        serialize(
          state: {
            write: (s: string) => void;
            ensureNewLine: () => void;
            renderContent: (node: ProseMirrorNode) => void;
            out: string;
          },
          node: ProseMirrorNode
        ) {
          const calloutType = node.attrs.calloutType as CalloutType;
          const title = node.attrs.title as string | null;

          state.ensureNewLine();

          // First line: > [!TYPE] Optional title
          if (title) {
            state.write(`> [!${calloutType}] ${title}\n`);
          } else {
            state.write(`> [!${calloutType}]\n`);
          }

          // Render the body content. We need to prefix each line with "> "
          // Capture the inner content as markdown, then prefix each line
          const savedOut = state.out;
          state.out = '';
          state.renderContent(node);
          const innerContent = state.out;
          state.out = savedOut;

          // Prefix each line with "> "
          const lines = innerContent.split('\n');
          for (let i = 0; i < lines.length; i++) {
            const line = lines[i];
            if (i === lines.length - 1 && line === '') {
              // Skip trailing empty line from renderContent
              continue;
            }
            state.write(`> ${line}\n`);
          }

          state.ensureNewLine();
        },
        parse: {
          // Handled by the blockquote parser override below
        }
      }
    };
  },

  addCommands() {
    return {
      insertCallout:
        (type: CalloutType, title?: string) =>
        ({ commands }: import('@tiptap/core').CommandProps) => {
          return commands.insertContent({
            type: this.name,
            attrs: { calloutType: type, title: title ?? null },
            content: [
              {
                type: 'paragraph',
                content: [{ type: 'text', text: 'Callout content' }]
              }
            ]
          });
        }
    };
  },

  addNodeView() {
    return ({
      node,
      HTMLAttributes
    }: {
      node: ProseMirrorNode;
      HTMLAttributes: Record<string, string>;
    }) => {
      const calloutType = node.attrs.calloutType as CalloutType;
      const title = node.attrs.title as string | null;
      const color = CALLOUT_COLORS[calloutType];

      const wrapper = document.createElement('div');
      wrapper.className = 'callout';
      wrapper.setAttribute('data-callout', '');
      wrapper.setAttribute('data-callout-type', calloutType);
      Object.entries(HTMLAttributes).forEach(([key, val]) => {
        wrapper.setAttribute(key, val);
      });
      wrapper.style.setProperty('--callout-color', color);

      // Header with icon + type label
      const header = document.createElement('div');
      header.className = 'callout__header';

      const icon = document.createElement('span');
      icon.className = 'callout__icon';
      icon.innerHTML = CALLOUT_ICONS[calloutType];
      icon.style.color = color;
      header.appendChild(icon);

      const label = document.createElement('span');
      label.className = 'callout__label';
      label.textContent = title ?? calloutType.charAt(0) + calloutType.slice(1).toLowerCase();
      label.style.color = color;
      header.appendChild(label);

      wrapper.appendChild(header);

      // Content container for ProseMirror to render into
      const contentContainer = document.createElement('div');
      contentContainer.className = 'callout__body';
      wrapper.appendChild(contentContainer);

      return {
        dom: wrapper,
        contentDOM: contentContainer,
        update(updatedNode: ProseMirrorNode): boolean {
          if (updatedNode.type.name !== 'callout') return false;
          const newType = updatedNode.attrs.calloutType as CalloutType;
          const newTitle = updatedNode.attrs.title as string | null;
          const newColor = CALLOUT_COLORS[newType];

          wrapper.setAttribute('data-callout-type', newType);
          wrapper.style.setProperty('--callout-color', newColor);
          icon.innerHTML = CALLOUT_ICONS[newType];
          icon.style.color = newColor;
          label.textContent = newTitle ?? newType.charAt(0) + newType.slice(1).toLowerCase();
          label.style.color = newColor;

          return true;
        }
      };
    };
  }
});

/**
 * Returns the markdown parsing/serialization config for tiptap-markdown.
 * Intercepts blockquotes that start with [!TYPE] and converts them to
 * Callout nodes instead of blockquotes.
 */
export function getCalloutMarkdownConfig(): {
  name: string;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  fromMarkdown: any[];
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  toMarkdown: Record<string, any>;
} {
  return {
    name: 'callout',
    fromMarkdown: [
      {
        type: 'blockquote',
        getAttrs: (token: { children?: Array<{ type: string; children?: Array<{ type: string; value?: string }> }> }) => {
          // Check if the first child paragraph starts with [!TYPE]
          const firstChild = token.children?.[0];
          if (firstChild?.type !== 'paragraph') return false;
          const firstText = firstChild.children?.[0];
          if (firstText?.type !== 'text' || !firstText.value) return false;
          const match = firstText.value.match(CALLOUT_REGEX);
          if (!match) return false;
          return {
            calloutType: match[1],
            title: match[2] || null
          };
        },
        node: 'callout',
        getContent: (
          token: { children?: Array<{ type: string; children?: Array<{ type: string; value?: string }> }> },
          schema: import('@tiptap/pm/model').Schema
        ) => {
          // Strip the [!TYPE] line from the content, keep the rest
          const children = [...(token.children ?? [])];
          if (children.length > 0 && children[0].type === 'paragraph') {
            const firstPara = { ...children[0], children: [...(children[0].children ?? [])] };
            if (firstPara.children.length > 0 && firstPara.children[0].type === 'text') {
              const text = firstPara.children[0].value ?? '';
              const match = text.match(CALLOUT_REGEX);
              if (match) {
                // Remove the [!TYPE] line from the first paragraph
                const remaining = text.slice(match[0].length).replace(/^\n/, '');
                if (remaining) {
                  firstPara.children[0] = { ...firstPara.children[0], value: remaining };
                  children[0] = firstPara;
                } else if (firstPara.children.length > 1) {
                  firstPara.children = firstPara.children.slice(1);
                  children[0] = firstPara;
                } else {
                  // The entire first paragraph was just the [!TYPE] line
                  children.shift();
                }
              }
            }
          }
          // Return the remaining children as content
          if (children.length === 0) {
            return [schema.nodes.paragraph.create()];
          }
          return children;
        }
      }
    ],
    toMarkdown: {
      callout: (
        state: {
          write: (s: string) => void;
          ensureNewLine: () => void;
          renderContent: (node: ProseMirrorNode) => void;
          out: string;
        },
        node: ProseMirrorNode
      ) => {
        const calloutType = node.attrs.calloutType as CalloutType;
        const title = node.attrs.title as string | null;

        state.ensureNewLine();

        // Capture inner content
        const savedOut = state.out;
        state.out = '';
        state.renderContent(node);
        const innerContent = state.out;
        state.out = savedOut;

        // First line: > [!TYPE] Optional title
        if (title) {
          state.write(`> [!${calloutType}] ${title}\n`);
        } else {
          state.write(`> [!${calloutType}]\n`);
        }

        // Prefix remaining lines with "> "
        const lines = innerContent.split('\n');
        for (let i = 0; i < lines.length; i++) {
          const line = lines[i];
          if (i === lines.length - 1 && line === '') continue;
          state.write(`> ${line}\n`);
        }

        state.ensureNewLine();
      }
    }
  };
}
