---
title: "How to use the command palette"
type: how-to
status: published
date: 2026-04-11
project: "vedox"
tags: ["flagship-ux", "command-palette", "search", "navigation", "shortcuts"]
author: "Vedox Team"
difficulty: "beginner"
estimated_time_minutes: 5
prerequisites:
  - "vedox dev running with a project open"
---

# How to use the command palette

This guide covers the four modes of the Vedox command palette, the prefixes that select them, the keys that drive selection, and the built-in commands available out of the box.

## Prerequisites

- `vedox dev` is running and a project is open
- The command palette searches within a project scope. If no project is active, search returns an empty state with the hint "Open a project to search its docs."

## Steps

1. **Open the palette.** Press `Cmd+K` (or `Ctrl+K` on Windows and Linux). The palette opens centered on screen with the cursor in the input.

2. **Choose a mode by prefix.** The first character of the query selects one of four modes:

   | First character | Mode | What it searches |
   |---|---|---|
   | (none) | `search` | Full-text search across the project's documents |
   | `>` | `command` | The built-in command registry (see Step 5) |
   | `#` | `tag` | Tag-prefix filter (passed to the search backend) |
   | `/` | `path` | Path-based jump to a document by its file path |

   The mode updates live as you type, so deleting the prefix flips you back to full-text search without reopening the palette.

3. **Jump straight to path mode.** Press `Cmd+P` (or `Ctrl+P`). This sets the query to `/` and opens the palette in path mode in one keystroke — useful when you already know the path of the file you want.

4. **Navigate results.** Use the arrow keys:

   - `Arrow Down` moves the selection down; `Arrow Up` moves it up. Both wrap at the ends of the list.
   - `Enter` activates the selected result: a document hit navigates to the doc, a command hit runs its handler.
   - `Escape` closes the palette without taking action.

5. **Use the built-in commands.** Type `>` to enter command mode. With an empty filter you see all 12 commands; typing filters by title and description (case-insensitive substring match).

   | Command | Effect |
   |---|---|
   | Toggle sidebar | Show or hide the sidebar panel |
   | Theme: Graphite | Switch to the Graphite (dark neutral) theme |
   | Theme: Eclipse | Switch to the Eclipse (OLED-black, violet accent) theme |
   | Theme: Ember | Switch to the Ember (warm near-black) theme |
   | Theme: Paper | Switch to the Paper (warm off-white) theme |
   | Theme: Solar | Switch to the Solar (cream and amber) theme |
   | Density: Compact | Apply the compact density (0.875x spacing) |
   | Density: Comfortable | Apply the comfortable density (1.0x, default) |
   | Density: Cozy | Apply the cozy density (1.125x spacing) |
   | Split pane | Open a new empty pane beside the current one |
   | Open Settings | Navigate to the settings page |
   | Reload document index | Re-scan the workspace and rebuild the search index |

6. **Pick a command.** With the selection on the command you want, press `Enter`. The command runs, the palette closes, and the effect applies immediately (theme switches, density changes, and pane splits are visible without reload).

## Verification

- After Step 1, the palette is visible and receives keystrokes.
- After typing `>`, the placeholder and result list show command mode; after typing `>theme`, only the six theme commands remain.
- After Step 3, the input shows `/` and the palette is in path mode (no results are fetched until you type a path).
- After running `Theme: Eclipse` from command mode, the workspace is on the Eclipse theme and the palette is closed.

If results stay empty in search/tag/path modes, confirm a project is active — without a project scope the palette shows "Open a project to search its docs." instead of hitting the search backend. If search results arrive slowly, note that the palette debounces network queries by 120ms; this is deliberate and prevents a network request on every keystroke.

## Related

- [Keyboard Shortcuts](./keyboard-shortcuts.md) — full shortcut reference, including `Cmd+K` and `Cmd+P`
- [Command Palette](../explanation/command-palette.md) — background on the palette architecture and the command registry
- [How to customize appearance](./customize-appearance.md) — the theme and density commands listed in Step 5 are also available in Settings
