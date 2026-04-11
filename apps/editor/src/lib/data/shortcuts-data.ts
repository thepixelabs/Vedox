/**
 * shortcuts-data.ts — static registry of keyboard shortcuts for display in settings.
 */

export interface ShortcutEntry {
  key: string;
  description: string;
  category: 'Navigation' | 'Editor' | 'View' | 'Panes';
}

export const shortcuts: ShortcutEntry[] = [
  // Navigation
  { key: '⌘K', description: 'Open command palette', category: 'Navigation' },
  { key: '⌘P', description: 'Quick file open', category: 'Navigation' },
  // Editor
  { key: '⌘B', description: 'Bold', category: 'Editor' },
  { key: '⌘I', description: 'Italic', category: 'Editor' },
  { key: '⌘`', description: 'Inline code', category: 'Editor' },
  { key: '⌘/', description: 'Toggle comment', category: 'Editor' },
  { key: '⌘Shift+M', description: 'Toggle editor mode (rich ↔ source)', category: 'Editor' },
  // View
  { key: '⌘Shift+L', description: 'Cycle reading width', category: 'View' },
  { key: '⌘[', description: 'Decrease sidebar width', category: 'View' },
  { key: '⌘]', description: 'Increase sidebar width', category: 'View' },
  // Panes
  { key: '⌘\\', description: 'Split pane', category: 'Panes' },
  { key: '⌘W', description: 'Close active pane', category: 'Panes' },
];

/** Unique ordered categories for grouped rendering. */
export const shortcutCategories = [
  ...new Set(shortcuts.map((s) => s.category)),
] as const;
