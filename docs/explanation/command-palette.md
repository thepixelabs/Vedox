---
title: "Command Palette"
type: explanation
status: published
date: 2026-04-10
project: "vedox"
tags: ["command-palette", "search", "commands", "navigation"]
author: "tech-writer"
---

# Command Palette

The command palette is a modal search and command interface opened with `Cmd+K` (Mac) or `Ctrl+K` (Windows/Linux). It provides four modes for finding documents, running commands, filtering by tag, and jumping to file paths.

Source: `apps/editor/src/lib/components/CommandPalette/store.ts`

---

## Modes

The palette mode is determined by the first character of the input. Typing a prefix character switches the mode live as you type.

| Prefix | Mode | Behavior |
|---|---|---|
| *(none)* | **Search** | Full-text document search via `/api/search` (FTS5). Results show title + snippet. |
| `>` | **Command** | Filters the built-in command registry. No network call. |
| `#` | **Tag** | Filter by tag (placeholder -- wired to FTS5 in the current implementation). |
| `/` | **Path** | Path-based file jump (wired to FTS5 in the current implementation). |

`Cmd+P` opens the palette pre-filled with `/`, putting it directly into path mode.

---

## Search mode (default)

When the user types without a prefix, the palette performs a debounced full-text search (120ms debounce) against the active project's FTS5 index via the Go backend.

- Results are ranked by score. Title matches score higher than body matches.
- Each result shows title, content type, status, and a highlighted snippet.
- Selecting a result navigates to that document.
- Search requires a project scope. If no project is open, the palette shows "Open a project to search its docs."

---

## Command mode

Typing `>` switches to command mode. All registered commands are shown immediately (no typing required). Further characters filter the list by title and description.

### Built-in commands

| ID | Title | Description |
|---|---|---|
| `sidebar.toggle` | Toggle sidebar | Show or hide the sidebar panel |
| `theme.graphite` | Theme: Graphite | Dark neutral, the default |
| `theme.eclipse` | Theme: Eclipse | OLED-black with violet accent |
| `theme.ember` | Theme: Ember | Warm near-black for late-night sessions |
| `theme.paper` | Theme: Paper | Warm off-white light mode |
| `theme.solar` | Theme: Solar | Cream and amber, soft light |
| `density.compact` | Density: Compact | Tighter spacing for power users |
| `density.comfortable` | Density: Comfortable | Balanced spacing (default) |
| `density.cozy` | Density: Cozy | Generous spacing for relaxed reading |
| `pane.split` | Split pane | Open a new empty pane beside the current one |
| `nav.settings` | Open Settings | Navigate to the settings page |
| `index.reload` | Reload document index | Re-scan the workspace and rebuild the search index |

Theme and density commands take effect immediately. See [Customize Appearance](../how-to/customize-appearance.md) for the full customization workflow.

---

## Animation

The palette opens with a 180ms `--ease-snap` animation: opacity fade combined with a `scale(0.97 -> 1)` transform. A 12px backdrop blur with 140% saturation (`backdrop-filter: blur(12px) saturate(140%)`) provides depth separation from the content behind it.

The palette sits at z-index `--z-cmdk` (70), above modals (50) and toasts (60). This ensures it is always accessible regardless of what other UI is open.

When `prefers-reduced-motion: reduce` is active, all motion is suppressed. The palette appears and disappears instantly.

---

## Keyboard interaction

| Key | Action |
|---|---|
| `Cmd+K` / `Ctrl+K` | Toggle palette open/closed |
| `Cmd+P` / `Ctrl+P` | Open palette in path mode (`/` prefix) |
| Arrow Up / Down | Move selection through results (wraps) |
| Enter | Activate the selected result (navigate or run command) |
| Escape | Close the palette |

---

## Architecture

The palette state is managed by five Svelte writable stores:

| Store | Type | Purpose |
|---|---|---|
| `openStore` | `boolean` | Modal visibility |
| `queryStore` | `string` | Current input value |
| `modeStore` | derived | Current mode (derived from query prefix) |
| `resultsStore` | `PaletteResult[]` | Search hits or command matches |
| `selectedIndexStore` | `number` | Highlighted row index |

The `Cmd+K` listener is installed once by `initPaletteShortcut()` during the `CommandPalette` component's `onMount`. It captures keydown events at the window level. The same function wires the reactive subscription that drives search results from query and project scope changes.

Project scope is set externally via `setScopeProject(projectId)`, called by the layout when the active project changes. The palette does not read `$page` directly.
