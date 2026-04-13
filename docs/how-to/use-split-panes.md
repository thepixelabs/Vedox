---
title: "How to use split panes"
type: how-to
status: published
date: 2026-04-11
project: "vedox"
tags: ["flagship-ux", "panes", "editor", "navigation"]
author: "Vedox Team"
difficulty: "beginner"
estimated_time_minutes: 5
prerequisites:
  - "vedox dev running with a project open"
  - "Viewport at least 1440px wide"
---

# How to use split panes

This guide covers how to open, populate, resize, and close split panes in the Vedox editor.

## Prerequisites

- `vedox dev` is running and a project is open
- Your viewport is at least 1440px wide (below this width the editor is limited to a single pane; see the capacity table in Step 5)

## Steps

1. **Open an empty split.** Press `Cmd+\` (or `Ctrl+\` on Windows and Linux). A new empty pane appears to the right of the active pane and becomes the active target.

2. **Fill the empty split.** The empty pane shows a picker. Click any document in the sidebar — because the active pane has no doc, the click fills it in place rather than replacing the doc in a different pane.

3. **Open a sidebar doc directly into a new split.** Hold `Cmd` (or `Ctrl`) and click any document in the sidebar. A new pane opens and the clicked doc loads into it in one action, without an intermediate empty state.

4. **Resize a pane.** Hover the divider between two panes until the cursor changes, then drag horizontally. Release to commit the new split ratio. The ratio is preserved for the session.

5. **Know the viewport capacity.** The maximum pane count is derived from `window.innerWidth`:

   | Viewport width | Max panes |
   |---|---|
   | `< 1440px` | 1 |
   | `>= 1440px` | 2 |
   | `>= 2560px` | 3 |
   | `>= 3840px` | 4 |

   When you are already at capacity, `Cmd+\` is a no-op and opening a new doc from the sidebar replaces the doc in the active pane instead of creating a new one.

6. **Close the active pane.** Press `Cmd+W` (or `Ctrl+W`). The active pane is removed and focus shifts to the pane that now sits last in the row. Closing the final remaining pane leaves the workspace with no active pane until you open a new doc.

## Verification

- After Step 1, the editor shows two panes side by side and the new one is highlighted as active.
- After Step 3, the Cmd+clicked doc is loaded in a pane that did not exist before the click.
- After Step 6, the pane count in the editor drops by one and keyboard input is routed to the pane that gained focus.

If `Cmd+\` appears to do nothing, check the viewport width against the table in Step 5 — you are at the capacity limit for your current window size. Resize the window wider or close an existing pane.

## Related

- [Keyboard Shortcuts](./keyboard-shortcuts.md) — complete reference of Vedox shortcuts including the panes section
- [Pane System](../explanation/pane-system.md) — background on how the pane tree is modeled and how viewport capacity is computed
- [How to use the command palette](./use-command-palette.md) — the `Split pane` command is also available via `Cmd+K`
