---
title: "How to insert blocks with slash commands"
type: how-to
status: published
date: 2026-04-11
project: "vedox"
tags: ["flagship-ux", "slash-commands", "editor", "blocks"]
author: "Vedox Team"
difficulty: "beginner"
estimated_time_minutes: 3
prerequisites:
  - "A document open in a pane in rich (WYSIWYG) mode"
  - "Cursor in an empty paragraph"
---

# How to insert blocks with slash commands

This guide covers how to open the slash-command popover in the Vedox editor and use it to insert headings, lists, and rich blocks.

## Prerequisites

- A document is open in a pane in rich (WYSIWYG) mode
- The cursor is in an empty paragraph — slash commands only trigger at the start of an otherwise-empty paragraph block

## Steps

1. **Place the cursor on an empty line.** Press `Enter` to create a new paragraph, or click into an existing empty paragraph. The cursor must be at character offset 0 of a paragraph node; the popover will not open inside a heading, list item, quote, or code block.

2. **Type `/`.** The slash is inserted into the document and the popover appears below the cursor, listing all available commands grouped by section.

3. **Filter by typing.** Keep typing after the `/` to filter. The filter matches the command label and its keyword list (case-insensitive substring). For example, `/h1` narrows to Heading 1, `/quo` narrows to Blockquote, `/call` narrows to Callout. Typing a whitespace character closes the popover.

4. **Select a command.** Use `Arrow Down` and `Arrow Up` to move the selection. Press `Enter` to insert. The typed `/` and filter text are removed from the document and the selected block is inserted in their place.

5. **Cancel without inserting.** Press `Escape` (or move the cursor elsewhere). The `/` and any filter text remain as plain text in the paragraph; delete them if you do not want them.

6. **Know the available commands.** Thirteen commands ship in four groups:

   | Group | Command | Inserts |
   |---|---|---|
   | Headings | Heading 1 / 2 / 3 | A level-1, -2, or -3 heading |
   | Lists | Bullet list | An unordered list |
   | Lists | Numbered list | An ordered list |
   | Blocks | Code block | A fenced code block |
   | Blocks | Blockquote | A blockquote block |
   | Blocks | Divider | A horizontal rule |
   | Rich blocks | Table | A 3x3 GFM-compatible table with a header row |
   | Rich blocks | Mermaid diagram | A Mermaid block seeded with a TD flow |
   | Rich blocks | Callout | A `NOTE` callout (see the [callouts guide](./use-callouts.md) to change the type) |
   | Rich blocks | Math block | A KaTeX display block seeded with `E = mc^2` |
   | Rich blocks | Image | Prompts for an image URL and inserts the image |

## Verification

- After Step 2, a popover is visible with command rows and a group header above each group.
- After typing `/heading` and pressing `Enter` on the highlighted row, the current paragraph is converted to a heading of the chosen level.
- After pressing `Escape` in Step 5, the popover disappears and no block is inserted.

If the popover does not open after typing `/`, confirm the cursor is at the very start of an empty paragraph. Inside a heading, list item, or code block the slash is treated as a plain character.

## Related

- [How to use the command palette](./use-command-palette.md) — `Cmd+K` runs workspace-level commands; the slash menu inserts content blocks
- [How to add callouts](./use-callouts.md) — details on the callout block inserted by the `/callout` command
- [How to write math](./write-math.md) — details on the math block inserted by the `/math` command
