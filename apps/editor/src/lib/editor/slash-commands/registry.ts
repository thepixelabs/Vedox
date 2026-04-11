/**
 * registry.ts
 *
 * Slash command registry for content-insertion commands.
 * Each command describes a block type that can be inserted via the "/" popover.
 */

import type { Editor } from '@tiptap/core';

export interface SlashCommand {
  /** Unique key */
  id: string;
  /** Display label */
  label: string;
  /** Search keywords (matched against user input after /) */
  keywords: string[];
  /** Optional group heading in the popover */
  group: string;
  /** Short description shown below the label */
  description: string;
  /** SVG icon HTML string */
  icon: string;
  /** Execute the insertion */
  action: (editor: Editor) => void;
}

// ---------------------------------------------------------------------------
// Icon constants (Lucide-style, 16x16)
// ---------------------------------------------------------------------------

const ICON = {
  heading:
    '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 12h8"/><path d="M4 18V6"/><path d="M12 18V6"/><path d="M21 18h-4V6h4"/></svg>',
  list:
    '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="8" x2="21" y1="6" y2="6"/><line x1="8" x2="21" y1="12" y2="12"/><line x1="8" x2="21" y1="18" y2="18"/><line x1="3" x2="3.01" y1="6" y2="6"/><line x1="3" x2="3.01" y1="12" y2="12"/><line x1="3" x2="3.01" y1="18" y2="18"/></svg>',
  orderedList:
    '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="10" x2="21" y1="6" y2="6"/><line x1="10" x2="21" y1="12" y2="12"/><line x1="10" x2="21" y1="18" y2="18"/><path d="M4 6h1v4"/><path d="M4 10h2"/><path d="M6 18H4c0-1 2-2 2-3s-1-1.5-2-1"/></svg>',
  code:
    '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="16 18 22 12 16 6"/><polyline points="8 6 2 12 8 18"/></svg>',
  quote:
    '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 21c3 0 7-1 7-8V5c0-1.25-.756-2.017-2-2H4c-1.25 0-2 .75-2 1.972V11c0 1.25.75 2 2 2 1 0 1 0 1 1v1c0 1-1 2-2 2s-1 .008-1 1.031V21z"/><path d="M15 21c3 0 7-1 7-8V5c0-1.25-.757-2.017-2-2h-4c-1.25 0-2 .75-2 1.972V11c0 1.25.75 2 2 2h.75c0 2.25.25 4-2.75 4v3z"/></svg>',
  divider:
    '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="2" x2="22" y1="12" y2="12"/></svg>',
  table:
    '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3v18"/><rect width="18" height="18" x="3" y="3" rx="2"/><path d="M3 9h18"/><path d="M3 15h18"/></svg>',
  mermaid:
    '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="5" r="3"/><line x1="12" x2="12" y1="8" y2="14"/><path d="M5 19l7-5 7 5"/></svg>',
  callout:
    '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><path d="M12 16v-4"/><path d="M12 8h.01"/></svg>',
  image:
    '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="18" height="18" x="3" y="3" rx="2" ry="2"/><circle cx="9" cy="9" r="2"/><path d="m21 15-3.086-3.086a2 2 0 0 0-2.828 0L6 21"/></svg>',
  math:
    '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 4h6l4 16h6"/><path d="M4 20h6"/><path d="M14 4h6"/></svg>'
};

// ---------------------------------------------------------------------------
// Command registry
// ---------------------------------------------------------------------------

export const slashCommands: SlashCommand[] = [
  // ---- Headings ----
  {
    id: 'heading1',
    label: 'Heading 1',
    keywords: ['h1', 'heading', 'title'],
    group: 'Headings',
    description: 'Large section heading',
    icon: ICON.heading,
    action: (editor) => editor.chain().focus().toggleHeading({ level: 1 }).run()
  },
  {
    id: 'heading2',
    label: 'Heading 2',
    keywords: ['h2', 'heading', 'subtitle'],
    group: 'Headings',
    description: 'Medium section heading',
    icon: ICON.heading,
    action: (editor) => editor.chain().focus().toggleHeading({ level: 2 }).run()
  },
  {
    id: 'heading3',
    label: 'Heading 3',
    keywords: ['h3', 'heading'],
    group: 'Headings',
    description: 'Small section heading',
    icon: ICON.heading,
    action: (editor) => editor.chain().focus().toggleHeading({ level: 3 }).run()
  },

  // ---- Lists ----
  {
    id: 'bulletList',
    label: 'Bullet list',
    keywords: ['ul', 'unordered', 'list', 'bullet'],
    group: 'Lists',
    description: 'Unordered bullet list',
    icon: ICON.list,
    action: (editor) => editor.chain().focus().toggleBulletList().run()
  },
  {
    id: 'orderedList',
    label: 'Numbered list',
    keywords: ['ol', 'ordered', 'list', 'number'],
    group: 'Lists',
    description: 'Ordered numbered list',
    icon: ICON.orderedList,
    action: (editor) => editor.chain().focus().toggleOrderedList().run()
  },

  // ---- Blocks ----
  {
    id: 'codeBlock',
    label: 'Code block',
    keywords: ['code', 'pre', 'fence', 'snippet'],
    group: 'Blocks',
    description: 'Syntax-highlighted code block',
    icon: ICON.code,
    action: (editor) => editor.chain().focus().toggleCodeBlock().run()
  },
  {
    id: 'blockquote',
    label: 'Blockquote',
    keywords: ['quote', 'blockquote', 'citation'],
    group: 'Blocks',
    description: 'Indented quote block',
    icon: ICON.quote,
    action: (editor) => editor.chain().focus().toggleBlockquote().run()
  },
  {
    id: 'divider',
    label: 'Divider',
    keywords: ['hr', 'horizontal', 'rule', 'divider', 'separator'],
    group: 'Blocks',
    description: 'Horizontal rule separator',
    icon: ICON.divider,
    action: (editor) => editor.chain().focus().setHorizontalRule().run()
  },

  // ---- Rich blocks ----
  {
    id: 'table',
    label: 'Table',
    keywords: ['table', 'grid', 'data'],
    group: 'Rich blocks',
    description: 'Insert a GFM-compatible table',
    icon: ICON.table,
    action: (editor) => {
      const commands = editor.commands as unknown as Record<
        string,
        (...args: unknown[]) => boolean
      >;
      if (typeof commands.insertTable === 'function') {
        commands.insertTable({ rows: 3, cols: 3, withHeaderRow: true });
      }
    }
  },
  {
    id: 'mermaid',
    label: 'Mermaid diagram',
    keywords: ['mermaid', 'diagram', 'chart', 'graph', 'flow'],
    group: 'Rich blocks',
    description: 'Mermaid diagram block',
    icon: ICON.mermaid,
    action: (editor) => {
      const commands = editor.commands as unknown as Record<
        string,
        (...args: unknown[]) => boolean
      >;
      if (typeof commands.insertMermaid === 'function') {
        commands.insertMermaid('graph TD\n    A[Start] --> B[End]');
      }
    }
  },
  {
    id: 'callout',
    label: 'Callout',
    keywords: ['callout', 'alert', 'note', 'tip', 'warning', 'admonition'],
    group: 'Rich blocks',
    description: 'Callout / alert block',
    icon: ICON.callout,
    action: (editor) => {
      const commands = editor.commands as unknown as Record<
        string,
        (...args: unknown[]) => boolean
      >;
      if (typeof commands.insertCallout === 'function') {
        commands.insertCallout('NOTE');
      }
    }
  },
  {
    id: 'math',
    label: 'Math block',
    keywords: ['math', 'latex', 'katex', 'equation', 'formula'],
    group: 'Rich blocks',
    description: 'LaTeX math block (KaTeX)',
    icon: ICON.math,
    action: (editor) => {
      const commands = editor.commands as unknown as Record<
        string,
        (...args: unknown[]) => boolean
      >;
      if (typeof commands.insertBlockMath === 'function') {
        commands.insertBlockMath('E = mc^2');
      }
    }
  },
  {
    id: 'image',
    label: 'Image',
    keywords: ['image', 'img', 'photo', 'picture'],
    group: 'Rich blocks',
    description: 'Insert an image',
    icon: ICON.image,
    action: (editor) => {
      const url = typeof prompt === 'function' ? prompt('Image URL:') : null;
      if (!url) return;
      const commands = editor.commands as unknown as Record<
        string,
        (...args: unknown[]) => boolean
      >;
      if (typeof commands.setImage === 'function') {
        commands.setImage({ src: url });
      }
    }
  }
];

/**
 * Filter slash commands by query string.
 * Matches against label and keywords (case-insensitive prefix match).
 */
export function filterCommands(query: string): SlashCommand[] {
  if (!query) return slashCommands;
  const q = query.toLowerCase();
  return slashCommands.filter(
    (cmd) =>
      cmd.label.toLowerCase().includes(q) ||
      cmd.keywords.some((kw) => kw.includes(q))
  );
}
