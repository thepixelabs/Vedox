---
title: "How the Dual-Mode Editor Works"
type: explanation
status: published
date: 2026-04-09
project: "vedox"
tags: ["editor", "tiptap", "codemirror", "architecture", "round-trip", "svelte"]
author: "Vedox Tech Writer"
slug: dual-mode-editor
---

# How the Dual-Mode Editor Works

Audience: contributors working on `apps/editor`, anyone who wants to understand how the Tiptap WYSIWYG and CodeMirror raw Markdown modes share state without losing content.

---

## The problem it solves

A documentation editor for engineers needs two things that usually conflict: a clean prose view (so non-technical writers can work without seeing `##` and `**`), and a raw Markdown view (so engineers can copy-paste code blocks and YAML frontmatter without fighting a rich-text toolbar). Most editors make you choose one. Vedox provides both and keeps them in sync.

---

## Architecture overview

The editor is three Svelte components working together:

```
Editor.svelte                   ← root; owns all state
├── CodeMirrorEditor.svelte     ← "Code" mode: raw Markdown via CodeMirror 6
├── TiptapEditor.svelte         ← "Overview" mode: rich text via Tiptap + ProseMirror
└── FrontmatterPanel.svelte     ← structured YAML fields (WYSIWYG mode only)
```

Both child editors receive their content from `Editor.svelte` and emit changes back via callbacks. Neither child calls the network directly — all persistence is handled by `onChange` and `onPublish` props passed by the page.

---

## The canonical Markdown string

The central invariant is: **one canonical Markdown string is always authoritative.** It is stored as `canonicalContent` in `Editor.svelte`. Both modes read from it on mount and write back to it on every change.

```
canonicalContent: string    ← frontmatter + body, full raw Markdown
```

The split between frontmatter and body is computed by `parseDocument()` from `utils/frontmatter.ts` (using `gray-matter`). When in WYSIWYG mode, the body is passed to Tiptap and the frontmatter is passed to `FrontmatterPanel`. When in Code mode, the full string (including frontmatter) is passed directly to CodeMirror.

---

## Mode switching

Mode is stored as `'code' | 'wysiwyg'` and persisted to `localStorage` per document:

```
localStorage key: vedox-editor-mode-${documentId}
```

When the user switches modes, `Editor.svelte`'s `switchMode()` function synchronises state in the direction of travel before updating the mode variable:

**Switching from Code to WYSIWYG:**
1. `parseDocument(canonicalContent)` re-parses the raw string the user just edited.
2. `frontmatter` and `tiptapBody` are updated from the parse result.
3. Tiptap receives the new body via a `$effect` that calls `editor.commands.setContent(body, false)`.

**Switching from WYSIWYG to Code:**
1. `serializeDocument(frontmatter, tiptapBody)` produces a fresh canonical Markdown string.
2. `canonicalContent` is updated with the result.
3. CodeMirror receives the new content via a `$effect` that dispatches a full-document replace transaction.

Neither mode loses content during a switch because the handoff happens synchronously before the DOM updates.

Both panels are kept in the DOM at all times but the inactive one is hidden with `opacity: 0; pointer-events: none` and marked `inert` so screen readers and keyboard navigation skip it. This avoids unmount/remount cost and preserves scroll position.

---

## Code mode: CodeMirror 6

`CodeMirrorEditor.svelte` wraps a CodeMirror 6 `EditorView`. The full raw Markdown string — including YAML frontmatter — is the document. There is no separation at this level.

Extensions in use:

| Extension | Purpose |
|---|---|
| `@codemirror/lang-markdown` | Markdown syntax highlighting |
| `@codemirror/theme-one-dark` | Dark theme |
| `lineNumbers()` | Gutter with line numbers |
| `history()` | Undo/redo |
| `highlightActiveLine()` | Active line highlight |
| `EditorState.readOnly` | Toggled via a `Compartment` — no rebuild needed |

On every change (`update.docChanged`), the view calls `onchange(newContent)` which propagates to `Editor.svelte`'s `handleCodeChange`.

External content updates (e.g. from a mode switch) are applied via `view.dispatch({ changes: { from: 0, to: doc.length, insert: newContent } })` inside a `$effect`. The effect guards against unnecessary dispatches by comparing the incoming string to the current document before dispatching.

---

## WYSIWYG mode: Tiptap + ProseMirror

`TiptapEditor.svelte` wraps a Tiptap `Editor`. The editor receives **only the body** — frontmatter is stripped by `Editor.svelte` before being passed as the `body` prop.

### Extensions

| Extension | Purpose |
|---|---|
| `StarterKit` | Bold, italic, code, headings H1–H4, lists, blockquote, horizontal rule, strike |
| `@tiptap/extension-link` | Hyperlink support with auto-link; `openOnClick: false` prevents accidental navigation |
| `tiptap-markdown` (Markdown extension) | Bidirectional Markdown ↔ ProseMirror serialization/parsing |
| `MermaidNode` | Custom extension for fenced `\`\`\`mermaid` blocks (see below) |

### Markdown serialization

Tiptap's `Markdown` extension (`tiptap-markdown`) provides `editor.storage.markdown.getMarkdown()` and handles `setContent(markdownString)`. On every ProseMirror transaction, `TiptapEditor` calls `getMarkdown()` and emits the body string to `Editor.svelte`.

Configuration:
- `html: false` — raw HTML from user input is never parsed or emitted. This is a security requirement.
- `bulletListMarker: '-'` — canonical list marker for round-trip consistency.
- `tightLists: true` — no blank lines between tight list items.
- `breaks: false` — soft line breaks are not converted to `<br>`.

### Frontmatter panel

In WYSIWYG mode, frontmatter fields are surfaced as a structured form (`FrontmatterPanel.svelte`) above the prose body. The panel edits the `frontmatter` object in `Editor.svelte` directly. On any field change, `handleFrontmatterChange` calls `serializeDocument(frontmatter, tiptapBody)` to rebuild `canonicalContent`.

The `FrontmatterSchema` in `utils/frontmatter.ts` (Zod) validates fields warn-only — a validation error does not block saving. Valid `type` values are `adr`, `how-to`, `runbook`, `readme`, `api-reference`, `explanation`, `issue`, `platform`, `infrastructure`, `network`, `logging`. Valid `status` values are `draft`, `review`, `published`, `deprecated`, `superseded`.

---

## Mermaid diagrams

The `MermaidNode` extension (`apps/editor/src/lib/editor/extensions/MermaidNode.ts`) intercepts fenced code blocks with language `mermaid` and converts them to a custom ProseMirror node instead of a `CodeBlock`.

In WYSIWYG mode, `MermaidNode` renders an SVG preview island using `mermaid.render()` from `mermaidCache.ts`. The SVG is sanitized with DOMPurify before DOM insertion (`USE_PROFILES: { svg: true, svgFilters: true }`). Mermaid is initialized with `securityLevel: 'strict'` — no click handlers are permitted in SVG output.

Rendered SVGs are cached in `localStorage` keyed by a djb2 hash of the source string (`vedox-mermaid-${hash}`). The cache holds up to 50 entries; older entries are pruned on overflow. This avoids re-rendering identical diagrams on every keystroke.

Clicking a Mermaid island dispatches a `mermaid-open-popover` CustomEvent. `MermaidPopover.svelte` listens for this event and opens an inline code editor. When the popover closes, it fires a `mermaid-update` event with the new source; `MermaidNode`'s node view handles it via `view.dispatch(tr.setNodeMarkup(...))`.

In Code mode, Mermaid blocks are plain text fences. The CodeMirror Markdown highlighter treats them as code blocks.

**Round-trip canonical form:**

```markdown
```mermaid
graph LR
  A --> B
```
```

The serializer in `MermaidNode` writes exactly this form: ` ```mermaid\n<source>\n``` ` with a trailing newline after the source if one is not already present. This is enforced by golden file `06-mermaid-block.md`.

---

## Auto-save and publish

`Editor.svelte` debounces `onChange` calls by 800 ms (`AUTOSAVE_DEBOUNCE_MS`). Every keystroke in either mode resets the timer. When the timer fires, `onChange(canonicalContent)` is called — the parent page is responsible for persisting the draft.

The "Publish" button opens a modal that prompts for a Git commit message (defaulting to `docs: update <title>`). Confirming calls `onPublish(canonicalContent, commitMessage)`. The parent page sends this to the Go API, which writes the file and creates a Git commit.

`isDirty` is set to `true` on any change and cleared only when the user explicitly publishes. The "Unsaved changes" indicator (a dot + text, `role="status" aria-live="polite"`) reflects this state.

---

## Round-trip fidelity guarantee

The round-trip invariant is:

```
serialize(parse(markdown)) === markdown
```

where `parse` is `editor.commands.setContent(markdown)` and `serialize` is `editor.storage.markdown.getMarkdown()`.

This is enforced by a golden-file test suite in `apps/editor/src/lib/editor/__tests__/roundtrip/`. There are 15 fixture files covering headings, lists, ordered lists, code blocks, links, Mermaid blocks, frontmatter, tables, blockquotes, inline formatting, empty documents, frontmatter-only documents, Unicode, long documents, and horizontal rules.

The test runner (`runner.test.ts`) loads each fixture, feeds it through a headless Tiptap editor in jsdom, and asserts byte-for-byte equality of the serialized output (after LF normalization). These tests are CI blockers — they cannot be skipped.

Any acceptable departure from exact byte equality (e.g. trailing newline normalization) must be recorded in the `KNOWN_NORMALIZATIONS` map in `runner.test.ts` with a written description. Adding an entry there is a conscious, reviewed decision, not a workaround.

The Go backend's parser (goldmark + goldmark-frontmatter) is declared authoritative per a Phase 1 CTO ruling. The TypeScript serializer must produce Markdown that the Go parser accepts without modification.

---

## File locations

| File | Role |
|---|---|
| `apps/editor/src/lib/editor/Editor.svelte` | Root component; owns all state |
| `apps/editor/src/lib/editor/CodeMirrorEditor.svelte` | Code mode |
| `apps/editor/src/lib/editor/TiptapEditor.svelte` | WYSIWYG mode |
| `apps/editor/src/lib/editor/FrontmatterPanel.svelte` | Structured frontmatter form |
| `apps/editor/src/lib/editor/MermaidPopover.svelte` | Mermaid inline edit popover |
| `apps/editor/src/lib/editor/utils/frontmatter.ts` | `parseDocument` / `serializeDocument` |
| `apps/editor/src/lib/editor/utils/mermaidCache.ts` | Mermaid render + localStorage cache |
| `apps/editor/src/lib/editor/extensions/MermaidNode.ts` | Custom Tiptap node for Mermaid |
| `apps/editor/src/lib/editor/__tests__/roundtrip/` | Golden-file fixtures + test runner |
