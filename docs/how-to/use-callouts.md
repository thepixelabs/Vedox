---
title: "How to add callouts"
type: how-to
status: published
date: 2026-04-11
project: "vedox"
tags: ["flagship-ux", "callouts", "markdown", "editor"]
author: "Vedox Team"
difficulty: "beginner"
estimated_time_minutes: 3
prerequisites:
  - "A document open in either rich or source mode"
  - "Familiarity with blockquote syntax"
---

# How to add callouts

This guide covers the GitHub-style callout syntax Vedox supports, the five callout types, how to add a custom title, and how to write multi-paragraph bodies.

## Prerequisites

- A document is open in either rich or source mode
- You are comfortable with standard blockquote syntax (`>` at the start of each line)

## Steps

1. **Pick a callout type.** Vedox supports five types. Each one maps to a semantic status color and a distinct header icon in rich mode.

   | Type | Purpose |
   |---|---|
   | `NOTE` | Neutral aside or cross-reference |
   | `TIP` | Best practice or time-saving suggestion |
   | `WARNING` | Something that can go wrong if ignored |
   | `DANGER` | Data loss, security, or irreversible action |
   | `INFO` | Background context or additional detail |

2. **Write the opening line.** A callout is a blockquote whose first line is `[!TYPE]`. The type must be upper-case. For a NOTE with no custom title:

   ```markdown
   > [!NOTE]
   > The workspace reloads automatically when a watched file changes.
   ```

3. **Add an optional title.** Append the title text after the type, separated by a single space. The title replaces the default type label in the rendered header:

   ```markdown
   > [!TIP] Prefer `Cmd+P` for known paths
   > Path mode skips full-text ranking and jumps straight to the file.
   ```

4. **Write a multi-paragraph body.** Prefix every line of the body with `> `. Separate paragraphs with a `>` line (a blockquote line with no content after the marker):

   ```markdown
   > [!WARNING]
   > Running `vedox index --reset` deletes the FTS5 index on disk.
   >
   > The next search after a reset rebuilds the index from scratch.
   > Expect the first query to take a few seconds on large workspaces.
   ```

5. **Insert a callout from the slash menu.** In rich mode, type `/callout` on an empty line and press `Enter`. A `NOTE` callout with placeholder body text is inserted. Edit the body in place; to change the type, switch to source mode (`Cmd+Shift+M`) and edit `[!NOTE]` to another type.

6. **See one example per type.** Each of the five blocks below is valid Vedox markdown:

   ```markdown
   > [!NOTE]
   > Vedox stores all preferences in `localStorage`.

   > [!TIP]
   > Press `Cmd+\` to split the editor into two panes.

   > [!WARNING]
   > Closing the last pane leaves the workspace with no active document.

   > [!DANGER]
   > Force-pushing to `main` rewrites history for every collaborator.

   > [!INFO]
   > Reading time is estimated at 200 words per minute.
   ```

## Verification

- In rich mode, each callout renders as a boxed region with a 2px left border in the status color, a 7% background wash, and a header row containing an icon and the type label (or your custom title).
- In source mode, round-tripping a callout through save and reload leaves the markdown byte-for-byte identical — the serializer guarantees `serialize(parse(input)) === input` for the canonical `> [!TYPE]` form.
- The `[!TYPE]` line itself does not appear in the rendered body; it is consumed by the parser and replaced with the header.

If a callout renders as a plain blockquote, check that the type is upper-case and that the square brackets and exclamation mark are present exactly as `[!NOTE]`. Any deviation falls back to a standard blockquote.

## Related

- [How to insert blocks with slash commands](./use-slash-commands.md) — the `/callout` command inserts a callout from the slash menu
- [Callout extension source](../../apps/editor/src/lib/editor/extensions/Callout.ts) — canonical list of supported types and round-trip rules
