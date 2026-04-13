---
title: "How to read the status bar and breadcrumbs"
type: how-to
status: published
date: 2026-04-11
project: "vedox"
tags: ["flagship-ux", "status-bar", "breadcrumbs", "editor", "navigation"]
author: "Vedox Team"
difficulty: "beginner"
estimated_time_minutes: 3
prerequisites:
  - "A document open in a pane"
---

# How to read the status bar and breadcrumbs

This guide covers what the editor's breadcrumbs and status bar show and how to use them to navigate and monitor the document you are editing.

## Prerequisites

- A document is open in a pane

## Steps

1. **Locate the breadcrumbs.** The breadcrumbs sit directly above the editor content. Segments read `Project / Folder / Subfolder / Document`. The final segment is the document title with any `.md` extension stripped, and it is shown in the primary text color with no underline.

2. **Navigate by clicking a segment.** Any segment except the current one is a link. Click the project segment to jump to the project home; click a folder segment to jump to that folder's index. Clicking the current (last) segment is a no-op.

3. **Locate the status bar.** The status bar is a 24px strip pinned to the bottom of the editor. It uses JetBrains Mono at 11px with tabular figures so numbers stay aligned as they update.

4. **Read the left section: doc path.** The left section shows the document's project-relative path (for example `guides/getting-started.md`). If the path is wider than the available space it is truncated with an ellipsis and the full path is available via the element's tooltip.

5. **Read the center section: word count and reading time.** The center section shows two items separated by a `·`:

   - **Word count.** A live count of the words in the document. Frontmatter, fenced code blocks, and inline code are excluded from the count, and common markdown punctuation is stripped before counting so it matches what a reader would see.
   - **Reading time.** An estimate based on 200 words per minute, rounded to the nearest whole minute with a floor of 1 min.

6. **Read the right section: cursor position and git.** The right section shows the cursor position as `Ln <line> Col <column>`, followed by the current git branch (with a git-branch icon). An orange dot next to the branch name indicates uncommitted changes in the working tree. If the local branch is ahead of or behind the remote, arrows `↑N` and `↓N` appear next to the dot. If the project is not a git repository, the section shows `no git` in italics instead of the branch widget.

## Verification

- Typing in the document updates the word count and reading time without reloading the pane.
- Moving the cursor updates the Ln and Col values in real time.
- Making an uncommitted change in the working tree produces the orange dirty dot next to the branch name within 30 seconds — the status bar refetches git state every 30s to pick up changes made outside the editor.
- Clicking the project segment in the breadcrumbs navigates the active pane to the project home.

If the git widget never appears, check that the project directory is inside a git repository. If the word count looks low, confirm that the hidden content (frontmatter, code fences) you expect to be excluded is actually what is shown as excluded — the exclusion rules are documented in Step 5.

## Related

- [How to use split panes](./use-split-panes.md) — each pane has its own breadcrumbs and status bar
- [Keyboard Shortcuts](./keyboard-shortcuts.md) — shortcut reference for cursor movement commands whose effect you can see in the status bar
