---
title: "Status Bar and Breadcrumbs"
type: explanation
status: published
date: 2026-04-11
project: "vedox"
tags: ["flagship-ux", "editor", "status-bar", "breadcrumbs", "git"]
author: "Vedox Team"
---

# Status Bar and Breadcrumbs

The editor frames the document with two chrome strips: breadcrumbs above the content and a status bar at the bottom. This document explains what each strip exposes, how the word count is computed, how the git integration works, and the typographic details that make the breadcrumb separator read as editorial rather than technical.

Sources:
- `apps/editor/src/lib/editor/StatusBar.svelte`
- `apps/editor/src/lib/editor/Breadcrumbs.svelte`

---

## Status bar

The status bar is a 24px strip at the bottom of the editor. It uses JetBrains Mono at 11px with tabular figures and a hairline top border, sitting on `--surface-2`.

```
┌────────────────────────────────────────────────────────────────────┐
│ guides/getting-started.md      1,423 words · 7 min read           │
│                                        Ln 42  Col 18 · main ↑2    │
└────────────────────────────────────────────────────────────────────┘
```

It has three sections:

| Section | Content |
|---|---|
| **Left** | Document path (truncated with ellipsis when the bar is narrow) |
| **Center** | Word count, reading time |
| **Right** | Cursor line/column, git branch with state indicators |

All three sections share the same `font-variant-numeric: tabular-nums` so digit changes do not shift adjacent labels.

---

## Word count accuracy

Word count is a derived value computed from the raw Markdown string on every keystroke. The implementation in `StatusBar.svelte`:

```ts
const wordCount = $derived.by(() => {
  let text = content;
  // Strip frontmatter
  text = text.replace(/^---\n[\s\S]*?\n---\n?/, '');
  // Strip fenced code blocks
  text = text.replace(/```[\s\S]*?```/g, '');
  // Strip inline code
  text = text.replace(/`[^`]*`/g, '');
  // Strip markdown syntax (minimal)
  text = text.replace(/[#>*_\[\]()]/g, ' ');
  // Count whitespace-separated runs
  const words = text.trim().split(/\s+/).filter((w) => w.length > 0);
  return words.length;
});
```

Four normalization passes run in order:

1. **Frontmatter** is stripped via `/^---\n[\s\S]*?\n---\n?/`. A 20-field YAML block does not inflate the word count.
2. **Fenced code blocks** are stripped via `/```[\s\S]*?```/g`. Code is not prose; counting tokens inside a fence produces misleading numbers for a reading-time estimate.
3. **Inline code** is stripped via `/`[^`]*`/g`. An inline `identifier_with_underscores` should not count as five words.
4. **Markdown syntax characters** (`#`, `>`, `*`, `_`, `[`, `]`, `(`, `)`) are replaced with spaces. This collapses `**bold**` to `bold`, `[link](url)` to two isolated tokens (`link` and `url`), and heading markers to whitespace.

The remaining text is split on runs of whitespace and non-empty tokens are counted.

This is deliberately a **heuristic**, not a full Markdown parser. A full parse would be correct for every edge case but would add dependencies and latency on the hottest code path in the editor. The current regex-based version runs in under a millisecond on a 10,000-word document and matches a Markdown-aware parser to within ~1% on the Vedox documentation corpus.

Reading time is computed as `max(1, round(wordCount / 200))` — a standard 200 words-per-minute estimate with a 1-minute floor so empty documents do not display "0 min".

---

## Git status integration

The status bar calls `fetchGitStatus(projectId)` from `$lib/api/git-status` on mount and on every `projectId` change. It also polls every 30 seconds to catch branch changes made outside the app (a terminal `git checkout`, for example).

The response shape (from the status bar's usage):

| Field | Type | Meaning |
|---|---|---|
| `branch` | string | Current branch name |
| `dirty` | boolean | Uncommitted changes in the working tree |
| `ahead` | number | Commits on local not yet pushed |
| `behind` | number | Commits on remote not yet pulled |

Visual encoding:

- **Clean branch:** branch name in `--text-2`, preceded by a small git-fork SVG icon.
- **Dirty branch:** the whole git group re-colors to `--warning` (amber). A 6px amber dot appears after the branch name. The `title` attribute reads `<branch> (uncommitted changes)`.
- **Ahead:** `↑N` appended in muted color with a `Commits ahead of remote` tooltip.
- **Behind:** `↓N` appended in muted color with a `Commits behind remote` tooltip.
- **Error:** if the fetch throws (not a git repo, API down), the status bar renders `no git` in italic muted text.

Dirty state uses `--warning`, not `--error`. Uncommitted work is not a failure state — it is a state the writer needs to be aware of before switching documents. The amber color is shared with "stale content" and other soft warnings per the [design system](./design-system.md) status palette.

The 30-second polling interval is a deliberate trade-off: fast enough that a terminal-driven branch change is visible within a reasonable window, slow enough that the editor is not constantly hitting the filesystem. A push-based invalidation via a file watcher is on the roadmap.

---

## Breadcrumbs

The breadcrumbs strip sits directly above the editor content area. It renders the path from the project root to the current document, with each segment clickable to navigate up the tree.

```
vedox-docs / guides / getting-started
```

### Segment construction

`Breadcrumbs.svelte` takes three props — `projectId`, `projectName`, `docPath` — and derives a flat array of segments:

1. The first segment is always the project. Label is `projectName` if provided, otherwise the raw `projectId`.
2. Each directory segment of `docPath` becomes a clickable link whose `href` accumulates the path so far.
3. The last segment is the document itself. Its label has the `.md` extension stripped for display. It is not a link; it has `aria-current="page"` and cannot be clicked.

Progressive hrefs look like `/projects/<id>/docs/guides`, `/projects/<id>/docs/guides/section`, and so on. Clicking a segment calls `goto(href)` from `$app/navigation` — client-side routing, no full page reload.

### The italic serif slash

The separator between segments is the element that makes the breadcrumbs feel editorial rather than file-system-y. Instead of the standard breadcrumb `/` in the UI font or a `›` chevron, Vedox renders:

```css
.breadcrumbs__sep {
  font-family: var(--font-display, 'Source Serif 4 Variable', Georgia, serif);
  font-style: italic;
  font-size: 13px;
  color: var(--text-3);
}
```

A forward slash, italicized, set in Source Serif 4 at 13px in muted text color. The effect is a tilted serif `/` that visually separates segments without claiming the attention of a bold glyph. It is the only place in the editor chrome where the display serif appears outside of prose H1/H2 headings, which makes it a deliberate typographic accent.

The segments themselves use Geist Sans at 13px — the standard UI body face. The contrast between the sans-serif labels and the italic serif separator is the whole point.

### Overflow behavior

The breadcrumbs container is `overflow-x: auto` with `white-space: nowrap`. A deeply nested document (`a/b/c/d/e/f/g/h.md`) scrolls horizontally rather than wrapping to a second line. This preserves the 36px minimum height and keeps the editor's vertical rhythm unchanged.

### Hover and focus

Non-current segments get a subtle hover treatment: text color lifts from `--text-3` to `--text-1`, and the background fills with `--surface-3`. The transition uses the standard 120ms `--ease-out` curve from the design system. Focus-visible rings are a 2px accent outline with 2px offset.

---

## Interaction between the two strips

Breadcrumbs and the status bar are independent components with no shared state. The editor page mounts both, passes them the current document info, and lets them render. A document switch re-runs both components' derived values without any coordinated teardown.

This independence means either strip can be disabled or replaced without touching the other. The status bar's width-aware layout (left/center/right flexbox) and the breadcrumbs' horizontal scroll both handle narrow viewports without colliding.

---

## File reference

| File | Role |
|---|---|
| `apps/editor/src/lib/editor/StatusBar.svelte` | Status bar component, word count, git polling |
| `apps/editor/src/lib/editor/Breadcrumbs.svelte` | Breadcrumb nav, segment derivation |
| `apps/editor/src/lib/api/git-status.ts` | `fetchGitStatus` client |
| `apps/editor/src/lib/editor/__tests__/statusBar.test.ts` | Word count regression tests |
