---
title: "Editor Architecture"
type: explanation
status: published
date: 2026-04-11
project: "vedox"
tags: ["flagship-ux", "editor", "architecture", "tiptap", "codemirror"]
author: "Vedox Team"
---

# Editor Architecture

This document is a map of the Vedox editor. It names every flagship feature, points to the file that owns it, and describes the data flow from a Markdown file on disk to the editor view and back. For the deep dive on state management and mode switching, see [How the Dual-Mode Editor Works](./dual-mode-editor.md); this page is the roll-up that ties the individual feature explanations together.

---

## The two modes

The Vedox editor is two editors behind one component root. See [How the Dual-Mode Editor Works](./dual-mode-editor.md) for the full treatment of state ownership, mode switching, and the canonical Markdown invariant. In summary:

- **WYSIWYG mode** (`TiptapEditor.svelte`) uses Tiptap v2 on top of ProseMirror. It renders rich-text blocks and flagship features like callouts, math, and Mermaid diagrams as interactive node views.
- **Source mode** (`CodeMirrorEditor.svelte`) uses CodeMirror 6 with `@codemirror/lang-markdown`. It edits the raw Markdown string directly, including frontmatter.
- **`Editor.svelte`** is the root component. It owns the canonical Markdown string and hands the body to whichever child editor is active. Both child editors stay mounted (with the inactive one hidden and marked `inert`) so mode switches do not lose scroll position.

Both modes read from and write to the same canonical Markdown string, and a golden-file test suite enforces `serialize(parse(m)) === m` for every fixture in `apps/editor/src/lib/editor/__tests__/roundtrip/`.

---

## Extension composition in WYSIWYG mode

`TiptapEditor.svelte` configures a Tiptap `Editor` with the following extensions, in this order:

```
StarterKit (headings 1-4, lists, blockquote, HR, strike, bold, italic, ...)
    ├── codeBlock: false                 ← replaced by CodeBlockShiki below
    └── history: {}                      ← undo/redo via Tiptap defaults
CodeBlockShiki                           ← Shiki-powered syntax highlighting
Link                                     ← openOnClick: false, autolink: true
Markdown (tiptap-markdown)               ← bidirectional Markdown <-> ProseMirror
MermaidNode                              ← custom node for ```mermaid fences
Callout                                  ← GitHub alert syntax boxes
KatexInline                              ← $x^2$ inline math
KatexBlock                               ← $$ ... $$ display math
FootnoteRef                              ← [^1] style footnotes
Table / TableRow / TableHeader / TableCell   ← GFM tables with column resize
Image                                    ← <img> with caption affordance
SlashCommand                             ← "/" popover for block insertion
```

Each extension either augments the schema with new node/mark types or adds behavior (`SlashCommand` is behavior-only). The composition order matters for two things: shortcut priority and markdown parse/serialize registration. `StarterKit.configure({ codeBlock: false })` explicitly removes the stock code block so `CodeBlockShiki` owns the `codeBlock` node type without a collision.

### Where each flagship feature lives

| Feature | File | Deep dive |
|---|---|---|
| Dual-mode root + canonical content | `apps/editor/src/lib/editor/Editor.svelte` | [dual-mode editor](./dual-mode-editor.md) |
| WYSIWYG mode | `apps/editor/src/lib/editor/TiptapEditor.svelte` | [dual-mode editor](./dual-mode-editor.md) |
| Source mode | `apps/editor/src/lib/editor/CodeMirrorEditor.svelte` | [dual-mode editor](./dual-mode-editor.md) |
| Frontmatter form | `apps/editor/src/lib/editor/FrontmatterPanel.svelte` | [dual-mode editor](./dual-mode-editor.md) |
| Callouts | `apps/editor/src/lib/editor/extensions/Callout.ts` | [callouts](./callouts.md) |
| Inline math | `apps/editor/src/lib/editor/extensions/KatexInline.ts` | [math rendering](./math-rendering.md) |
| Block math | `apps/editor/src/lib/editor/extensions/KatexBlock.ts` | [math rendering](./math-rendering.md) |
| Slash command plugin | `apps/editor/src/lib/editor/extensions/SlashCommand.ts` | [slash commands](./slash-commands.md) |
| Slash command registry | `apps/editor/src/lib/editor/slash-commands/registry.ts` | [slash commands](./slash-commands.md) |
| Footnotes | `apps/editor/src/lib/editor/extensions/Footnotes.ts` | — |
| Mermaid diagrams | `apps/editor/src/lib/editor/extensions/MermaidNode.ts` | [dual-mode editor](./dual-mode-editor.md) |
| Shiki code blocks | `apps/editor/src/lib/editor/codeblock/CodeBlockShiki.svelte.ts` | — |
| Status bar | `apps/editor/src/lib/editor/StatusBar.svelte` | [status bar and breadcrumbs](./status-bar-and-breadcrumbs.md) |
| Breadcrumbs | `apps/editor/src/lib/editor/Breadcrumbs.svelte` | [status bar and breadcrumbs](./status-bar-and-breadcrumbs.md) |
| Bubble toolbar | `apps/editor/src/lib/editor/BubbleToolbar.svelte` | — |
| Mermaid edit popover | `apps/editor/src/lib/editor/MermaidPopover.svelte` | — |
| Slash command popover | `apps/editor/src/lib/editor/SlashCommandPopover.svelte` | [slash commands](./slash-commands.md) |

---

## Data flow: Markdown file -> editor view -> disk

```
                 ┌─────────────────────┐
                 │  Markdown file      │
                 │  (project workspace)│
                 └──────────┬──────────┘
                            │
                            │ HTTP GET via Go API
                            ▼
                 ┌─────────────────────┐
                 │  Editor page (+page.svelte)
                 │  fetches canonicalContent │
                 └──────────┬──────────┘
                            │
                            │ prop: canonicalContent
                            ▼
                 ┌─────────────────────┐
                 │  Editor.svelte      │
                 │  owns all state     │
                 └──────────┬──────────┘
                            │
              ┌─────────────┴─────────────┐
              │                           │
              ▼                           ▼
   ┌──────────────────┐         ┌──────────────────┐
   │ WYSIWYG mode     │         │ Source mode      │
   │                  │         │                  │
   │ parseDocument()  │         │ full canonical   │
   │   ├── frontmatter│         │ (frontmatter     │
   │   │   └─► Frontmatter │    │  + body)         │
   │   │       Panel       │    │   │              │
   │   └── body           │    │   ▼              │
   │       │              │    │ CodeMirror       │
   │       ▼              │    │ EditorView       │
   │   Tiptap Editor      │    │                  │
   │   (ProseMirror)      │    │                  │
   │     + extensions     │    │                  │
   └──────────┬───────────┘    └────────┬─────────┘
              │                          │
              │ onUpdate (every tx)      │ update.docChanged
              │ getMarkdown()            │
              ▼                          ▼
        ┌──────────────────────────────────────┐
        │  Editor.svelte handlers              │
        │  serializeDocument(fm, body)         │
        │  -> canonicalContent                 │
        └──────────────┬───────────────────────┘
                       │
                       │ 800 ms debounce
                       ▼
              ┌────────────────────┐
              │  onChange callback │
              │  (parent page)     │
              └──────────┬─────────┘
                         │
                         │ HTTP PUT via Go API
                         ▼
              ┌────────────────────┐
              │  Markdown file     │
              │  (workspace)       │
              └────────────────────┘
```

### Read path

1. The editor page fetches the document from the Go API and passes it to `Editor.svelte` as `canonicalContent` (full frontmatter + body).
2. `Editor.svelte` calls `parseDocument()` in `utils/frontmatter.ts` to split the string into a `frontmatter` object and a `body` string.
3. In WYSIWYG mode, `tiptapBody` is handed to `TiptapEditor.svelte`, which calls `editor.commands.setContent(body, false)` inside a `$effect`. Tiptap's `Markdown` extension parses the body into a ProseMirror document. The `frontmatter` object is handed to `FrontmatterPanel.svelte`.
4. In source mode, the full `canonicalContent` is handed to `CodeMirrorEditor.svelte`, which dispatches a full-document replace transaction.

### Write path

1. Every ProseMirror transaction or CodeMirror doc change fires `onchange` back to `Editor.svelte`.
2. In WYSIWYG mode, `TiptapEditor` calls `editor.storage.markdown.getMarkdown()` to serialize the ProseMirror document back to a Markdown body. `Editor.svelte` combines body + frontmatter via `serializeDocument()` to rebuild `canonicalContent`.
3. In source mode, `canonicalContent` is the document directly — no serialization step.
4. `Editor.svelte` debounces `onChange(canonicalContent)` calls by 800 ms. The debounce resets on every keystroke. When the timer fires, the parent page receives the new canonical string and persists it (typically via HTTP to the Go API, which writes the file and optionally creates a Git commit).

### Round-trip invariant

The round-trip test suite in `apps/editor/src/lib/editor/__tests__/roundtrip/` verifies that for each golden fixture, feeding the fixture through `parse` (Tiptap `setContent`) and then `serialize` (`getMarkdown`) produces the original bytes. The suite is a CI blocker. Any extension that changes the schema or the serializer must include a fixture that proves its round-trip.

---

## Related reading

- [Design system](./design-system.md) — the tokens and themes the editor styles against.
- [Pane system](./pane-system.md) — how multiple `Editor.svelte` instances are hosted side-by-side.
- [Command palette](./command-palette.md) — the other `/`-like interaction surface; separate from slash commands, separate keybinding, separate popover.
- [How the dual-mode editor works](./dual-mode-editor.md) — the state management deep dive that this page links to repeatedly.
- [Callouts](./callouts.md), [Math rendering](./math-rendering.md), [Slash commands](./slash-commands.md) — per-feature deep dives.
- [Status bar and breadcrumbs](./status-bar-and-breadcrumbs.md) — the chrome around the editor content.
- [Performance budgets](./performance-budgets.md) — the latency targets the editor is held to in CI.
