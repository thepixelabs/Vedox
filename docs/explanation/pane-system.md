---
title: "Pane System"
type: explanation
status: published
date: 2026-04-10
project: "vedox"
tags: ["panes", "layout", "multi-document", "editor"]
author: "tech-writer"
---

# Pane System

Vedox supports viewing multiple documents side-by-side in a horizontal pane layout. This document describes the pane architecture, its capacity limits, keyboard shortcuts, and current constraints.

Source: `apps/editor/src/lib/stores/panes.ts`

---

## Concept

A **pane** is a single document view. Multiple panes are arranged in a flat horizontal array -- no nested splits, no vertical stacking. This is v1; recursive splits are deferred.

Each pane tracks:

| Field | Type | Description |
|---|---|---|
| `id` | `string` | Random 7-character identifier |
| `docPath` | `string \| null` | Path to the open document. `null` means the pane is empty (picker state). |
| `mode` | `'rich' \| 'source'` | Rich = Tiptap WYSIWYG. Source = CodeMirror. |
| `scrollTop` | `number` | Preserved scroll position |
| `readingMeasure` | `'narrow' \| 'default' \| 'wide'` | Per-pane reading width preference |

Document content is **not** stored in the pane. The pane store is a coordination layer only; actual content lives in the Editor instance.

---

## Layout

Panes render in a CSS grid with dynamic column templates. Each pane occupies one column. A divider handle between columns allows drag-resizing with a 320px minimum width per pane.

The layout is implemented by the `PaneGroup` component (CSS grid container) and individual `PaneView` components (grid items).

---

## Viewport-aware capacity

The maximum number of simultaneous panes is determined by the viewport width. Each pane needs roughly 600px minimum to be usable.

| Viewport width | Max panes |
|---|---|
| < 1440px | 1 |
| 1440 -- 2559px | 2 |
| 2560 -- 3839px | 3 |
| 3840px+ (4K) | 4 |

These thresholds are evaluated at runtime via `window.innerWidth`. If the window is resized below the threshold for the current pane count, existing panes remain open but no new panes can be added.

---

## Pane operations

### Open a document

`panesStore.open(docPath, mode?)` places a document in a pane using this priority:

1. If the active pane is empty (no document), fill it.
2. If at capacity, replace the active pane's document in-place.
3. Otherwise, create a new pane.

The method returns the ID of the pane that received the document.

### Split

`panesStore.split()` adds an empty pane next to the current one. If already at max capacity, the call is a no-op and returns the current active pane ID.

### Close

`panesStore.close(id)` removes a pane. If the closed pane was active, focus shifts to the last remaining pane.

### Focus

`panesStore.focus(id)` makes a pane the active target for keyboard shortcuts and new document opens.

### Reading measure

`panesStore.setReadingMeasure(id, 'narrow' | 'default' | 'wide')` sets the per-pane reading width. This maps to the `--measure-narrow` / `--measure-default` / `--measure-wide` tokens from the [design system](./design-system.md).

### Editor mode

`panesStore.setMode(id, 'rich' | 'source')` toggles between the Tiptap WYSIWYG editor and the CodeMirror source editor for a specific pane.

---

## Keyboard shortcuts

| Shortcut | Action |
|---|---|
| `Cmd+\` | Split pane (add an empty pane beside the active one) |
| `Cmd+W` | Close the active pane |
| `Cmd+click` on a sidebar item | Open the document in a new pane (instead of replacing the active one) |

See the full [keyboard shortcuts reference](../how-to/keyboard-shortcuts.md).

---

## State persistence

Panes are **ephemeral** in v1. They do not survive a page reload. On reload, Vedox opens with a single pane showing the last-viewed document (tracked separately via the router).

Persistent pane layouts (saved workspaces) are planned for a later release.

---

## Derived stores

The pane store exposes two derived values for reactive UI:

- `panesStore.panes` -- read-only subscription to the full pane array
- `panesStore.activePane` -- derived store resolving to the currently focused pane (or `null`)
- `panesStore.activePaneId` -- read-only subscription to the active pane's ID
