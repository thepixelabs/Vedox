---
title: "Keyboard Shortcuts"
type: how-to
status: published
date: 2026-04-10
project: "vedox"
tags: ["keyboard", "shortcuts", "navigation", "editor", "panes"]
author: "tech-writer"
---

# Keyboard Shortcuts

Complete reference of keyboard shortcuts in Vedox. On Mac, `Cmd` is the modifier. On Windows and Linux, substitute `Ctrl` for `Cmd`.

Source: `apps/editor/src/lib/data/shortcuts-data.ts`

---

## Navigation

| Shortcut | Description |
|---|---|
| `Cmd+K` | Open the [command palette](../explanation/command-palette.md) |
| `Cmd+P` | Quick file open (opens palette in path mode with `/` prefix) |

---

## Editor

| Shortcut | Description |
|---|---|
| `Cmd+B` | Bold |
| `Cmd+I` | Italic |
| `Cmd+\`` | Inline code |
| `Cmd+/` | Toggle comment |
| `Cmd+Shift+M` | Toggle editor mode (rich WYSIWYG ↔ source CodeMirror) |

---

## View

| Shortcut | Description |
|---|---|
| `Cmd+Shift+L` | Cycle reading width (narrow → default → wide) |
| `Cmd+[` | Decrease sidebar width |
| `Cmd+]` | Increase sidebar width |

---

## Panes

| Shortcut | Description |
|---|---|
| `Cmd+\` | Split pane (open an empty pane beside the active one) |
| `Cmd+W` | Close the active pane |

See the [Pane System](../explanation/pane-system.md) for details on multi-pane behavior and viewport capacity limits.

---

## Command palette shortcuts

These shortcuts work while the command palette is open.

| Key | Action |
|---|---|
| Arrow Up / Down | Move selection through results |
| Enter | Activate the selected result |
| Escape | Close the palette |

Type `>` as the first character to enter command mode. See the [Command Palette](../explanation/command-palette.md) for all modes and built-in commands.

---

## Shortcut registry

Shortcuts are registered centrally via the `registerShortcut()` API in `$lib/keyboard/shortcuts.ts`. The dispatcher is bound to `<svelte:window onkeydown>` in the root layout.

Matching is exact: if a shortcut requires `meta`, the event must have `meta` pressed. The first matching shortcut fires; there is no bubbling within the shortcut system.

Components register shortcuts in `onMount` and unregister in the returned cleanup function:

```ts
import { registerShortcut } from '$lib/keyboard/shortcuts';
import { onMount } from 'svelte';

onMount(() => {
  const unregister = registerShortcut({
    key: 'k',
    meta: true,
    description: 'Open command palette',
    handler: (event) => { /* ... */ },
  });
  return unregister;
});
```
