---
title: "Slash Commands"
type: explanation
status: published
date: 2026-04-11
project: "vedox"
tags: ["flagship-ux", "editor", "slash-commands", "prosemirror"]
author: "Vedox Team"
---

# Slash Commands

Typing `/` on an empty line in the WYSIWYG editor opens a searchable popover of block-insertion commands: headings, lists, code blocks, callouts, tables, Mermaid diagrams, math, images. This document explains why the feature uses a hand-rolled ProseMirror plugin instead of Tiptap's `@tiptap/suggestion`, how the registry is structured, and how to add a new command.

Sources:
- `apps/editor/src/lib/editor/extensions/SlashCommand.ts`
- `apps/editor/src/lib/editor/slash-commands/registry.ts`

---

## Why a custom ProseMirror plugin

Tiptap ships a canonical solution for `/`-triggered menus: [`@tiptap/suggestion`](https://tiptap.dev/api/utilities/suggestion). Vedox does not use it. The reason is version compatibility.

Vedox is on **Tiptap v2**. The current release line of `@tiptap/suggestion` targets **Tiptap v3** and depends on ProseMirror plugin APIs that v2's packaged `@tiptap/pm` does not expose. Mixing a v3 suggestion plugin into a v2 editor produces type errors at build time and plugin-key collisions at runtime.

Two alternatives were considered:

- **Upgrade the editor to Tiptap v3.** Rejected for v1 of the flagship editor: the v3 migration touches every extension's lifecycle hooks and the round-trip golden files would need to be re-validated. Scheduled for a later release.
- **Pin an older `@tiptap/suggestion` compatible with v2.** Rejected because the older versions lack query parsing and keyboard handling that the UX needs, and would have to be forked to fix them.

The third option — write a minimal ProseMirror plugin — is about 100 lines of code, has zero external dependencies, and gives us exactly the behavior we want. It is `SlashCommand.ts`.

---

## How the plugin works

`SlashCommand` is a Tiptap `Extension` that registers one ProseMirror plugin. The plugin owns a small state machine and dispatches DOM events to drive the UI.

### State

```ts
interface SlashState {
  active: boolean;
  query: string;
  from: number;
  to: number;
}
```

The state is kept in the plugin's `state` field and updated on every transaction:

- **Inactive** on boot. No popover is shown.
- **Activated** when the user types `/` at the start of an empty paragraph. The `handleKeyDown` prop checks three conditions before activating:
  1. The selection is empty (not a range).
  2. The cursor is at `parentOffset === 0` (start of the block).
  3. The parent block is a paragraph (not a heading, list, code block, etc.).
- **Updated** on every subsequent transaction. The plugin reads the text between `from` and the current cursor. If it does not start with `/`, or contains whitespace, the state resets to inactive. Otherwise, `query` becomes the text after the `/`.

This means typing `/hea` sets `query: 'hea'`. Typing a space closes the popover. Backspacing past the `/` closes the popover.

### Events

The plugin communicates with the UI via window-level CustomEvents, not direct method calls:

| Event | Payload | Fired when |
|---|---|---|
| `vedox-slash-open` | `{ query, items, coords, onSelect, onClose }` | `/` typed at a valid position |
| `vedox-slash-update` | `{ query, items }` | Query text changes |
| `vedox-slash-close` | `—` | Query invalidated or popover dismissed |
| `vedox-slash-nav` | `{ key, accept() }` | ArrowDown / ArrowUp / Enter / Escape while active |

`SlashCommandPopover.svelte` listens for these events and renders the UI. Event-driven coupling keeps the plugin free of Svelte imports — the plugin is portable, the popover is swappable.

The nav event is bidirectional: the plugin dispatches the key and the listener calls `accept()` to signal "I handled this." The plugin then returns `true` from `handleKeyDown` to suppress ProseMirror's default handling. This is how arrow keys navigate the menu instead of moving the cursor.

### Commit

When the user selects a command:

1. The popover calls `onSelect(cmd)` passed in the open event.
2. `onSelect` dispatches a transaction that deletes the range `[from, to]` (the `/` and any query text) and resets the plugin state to inactive.
3. `cmd.action(editor)` runs. Most commands call `editor.chain().focus().toggleX().run()`.
4. The popover closes.

The order matters: delete the `/` text first, then run the action. If the action runs first, the `/` is still in the document and shows up in the inserted block.

---

## The registry pattern

All slash commands live in a single array in `registry.ts`:

```ts
export const slashCommands: SlashCommand[] = [
  { id: 'heading1', label: 'Heading 1', keywords: [...], group: 'Headings', ... },
  { id: 'heading2', ... },
  // ...
];
```

Each entry is a `SlashCommand`:

```ts
export interface SlashCommand {
  id: string;            // Unique identifier
  label: string;         // Display name
  keywords: string[];    // Matched against user query (prefix search)
  group: string;         // Section heading in the popover
  description: string;   // Subtitle shown below label
  icon: string;          // Inline SVG HTML string
  action: (editor: Editor) => void;  // What to run on select
}
```

The registry pattern has three properties worth naming:

1. **Single source of truth.** There is one array. Adding a command means adding one object. There are no decorator-based registrations or plugin manifests.
2. **No circular imports.** Commands reference the Tiptap `Editor` type but not any extension classes. Adding a command for a new node type does not require importing the extension into the registry file.
3. **Static list order.** The popover renders commands in array order, grouped by the `group` field. Reordering the array reorders the menu. There is no sort by usage frequency or alphabetization — intentional.

Current groups: `Headings`, `Lists`, `Blocks`, `Rich blocks`.

---

## Filter matching rules

`filterCommands(query)` in `registry.ts` is the matcher:

```ts
export function filterCommands(query: string): SlashCommand[] {
  if (!query) return slashCommands;
  const q = query.toLowerCase();
  return slashCommands.filter(
    (cmd) =>
      cmd.label.toLowerCase().includes(q) ||
      cmd.keywords.some((kw) => kw.includes(q))
  );
}
```

Rules:

- **Empty query** returns the full list.
- **Match is case-insensitive substring**, not fuzzy or Levenshtein. `hea` matches `Heading 1` (in `label`) and `Heading 2`. `h1` matches `Heading 1` (in `keywords`).
- **Match is against label OR any keyword.** Keywords exist to cover aliases and abbreviations (`h1`, `ul`, `ol`, `fence`, `admonition`).
- **Order is preserved.** Filtered results keep their registry order; no re-ranking.

This is intentionally simple. A writer typing `/not` sees every command whose label or keywords contain `not` — in this case, the Callout (keyword `note`). A fuzzy matcher would find accidental matches and waste screen space.

---

## Register a new slash command

To add a command, edit `apps/editor/src/lib/editor/slash-commands/registry.ts` and append an entry to `slashCommands`. Example — adding a "Task list" command that inserts a checkbox list:

```ts
{
  id: 'taskList',
  label: 'Task list',
  keywords: ['task', 'todo', 'checkbox', 'checklist'],
  group: 'Lists',
  description: 'Checkbox todo list',
  icon: ICON.list, // reuse existing icon or add one to the ICON const
  action: (editor) => editor.chain().focus().toggleTaskList().run()
}
```

Rules for new entries:

1. **`id` must be unique** across the registry. Used as a React-style key in the popover.
2. **`keywords` should include obvious synonyms and abbreviations.** A writer will not type the exact label.
3. **`group` should be one of the existing groups** unless you are deliberately adding a new section. New groups render in the order they first appear in the array.
4. **`action` must be idempotent.** Selecting the command twice in a row should not produce broken state. Use `toggleX` (which leaves state consistent) rather than `insertX` where both exist.
5. **`action` runs AFTER the `/query` text is deleted.** You do not need to clean up the prompt yourself.

If the new command targets a custom node type, the node's Tiptap extension must be registered in `TiptapEditor.svelte`'s extension list and expose the command via `addCommands()`. The slash registry only dispatches — it does not install nodes.

---

## Keyboard shortcuts inside the popover

While the popover is open:

| Key | Action |
|---|---|
| `ArrowDown` / `ArrowUp` | Move selection |
| `Enter` | Execute selected command |
| `Escape` | Close popover, restore cursor |

The plugin only fires `vedox-slash-nav` for these four keys. All other keys fall through to ProseMirror, which means typing continues to update the query naturally.

---

## File reference

| File | Role |
|---|---|
| `apps/editor/src/lib/editor/extensions/SlashCommand.ts` | ProseMirror plugin, state machine, event dispatch |
| `apps/editor/src/lib/editor/slash-commands/registry.ts` | Command list, `filterCommands()` |
| `apps/editor/src/lib/editor/SlashCommandPopover.svelte` | Popover UI, event listener |
| `apps/editor/src/lib/editor/__tests__/slashCommand.test.ts` | Plugin state-machine tests |
