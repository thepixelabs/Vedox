---
title: "Callouts"
type: explanation
status: published
date: 2026-04-11
project: "vedox"
tags: ["flagship-ux", "editor", "callouts", "markdown"]
author: "Vedox Team"
---

# Callouts

Vedox renders admonition-style boxes (note, tip, warning, danger, info) using GitHub's alert syntax. This document explains why that syntax was chosen, how the five types map to status colors, how callouts render in the WYSIWYG editor, and the one known limitation on the parse side.

Source: `apps/editor/src/lib/editor/extensions/Callout.ts`

---

## Why GitHub alert syntax

A callout is a blockquote that starts with `[!TYPE]`:

```markdown
> [!NOTE]
> Body text here.

> [!TIP] Optional custom title
> Body text here.
```

Vedox uses this form verbatim. It did not invent a new syntax. Three other options were considered and rejected:

- **MDX / JSX components** (`<Callout type="note">...</Callout>`) require a compiler, break plain-Markdown tooling, and make a `.md` file unreadable outside of MDX-aware viewers.
- **Custom fenced blocks** (` ```note `) overload the meaning of code fences and lose syntax highlighting in every other Markdown renderer.
- **Admonition directives** (`::: note`) are a CommonMark proposal, not a standard. Support across renderers is inconsistent.

GitHub alert syntax is a blockquote. Any Markdown renderer that has never heard of alerts still displays the content as a quoted paragraph with a `[!NOTE]` prefix. A renderer that understands alerts displays a styled box. Neither loses content. The round-trip canonical form is enforced by a golden-file test: `serialize(parse(input)) === input`.

---

## The five types

| Type | Status token | Intended use |
|---|---|---|
| `NOTE` | `--info` | Neutral context or clarification |
| `TIP` | `--success` | Suggestion that improves the outcome |
| `WARNING` | `--warning` | Something that can go wrong but is recoverable |
| `DANGER` | `--error` | Destructive or irreversible action |
| `INFO` | `--info` | Same color as NOTE; semantic variant |

The mapping lives in `CALLOUT_COLORS` in `Callout.ts`. Each value is a CSS `var()` reference — callouts automatically re-tint with the active theme via the [design system](./design-system.md) status tokens. There are no hard-coded hex values in the rendered output.

`NOTE` and `INFO` share `--info` deliberately. Writers use `NOTE` for a neutral aside and `INFO` when the content is more like a tooltip or notice; the distinction is semantic, not visual.

---

## How a callout renders

In the WYSIWYG editor, `Callout.addNodeView()` creates a DOM structure like:

```html
<div class="callout" data-callout data-callout-type="WARNING">
  <div class="callout__header">
    <span class="callout__icon"><!-- Lucide SVG --></span>
    <span class="callout__label">Warning</span>
  </div>
  <div class="callout__body">
    <!-- ProseMirror renders body content here -->
  </div>
</div>
```

The visual specification (from `TiptapEditor.svelte`):

- **2px left border** in the type's status color.
- **7% background wash** computed with `color-mix(in srgb, var(--callout-color) 7%, transparent)`. The wash re-mixes at paint time against the active theme, so no per-theme overrides exist.
- **Icon + label header** above the body. Icon is a 16x16 Lucide SVG inlined from `CALLOUT_ICONS`. Label defaults to the title-cased type (`NOTE` -> `Note`) or the custom title if one was provided after the `[!TYPE]` marker.
- **Rounded right edge** (`border-radius: 0 var(--radius-md) var(--radius-md) 0`) — the left edge stays flat so the 2px accent reads as a continuous vertical rule.

The body is a ProseMirror content slot (`contentDOM`). Any block content the schema allows — paragraphs, lists, code blocks, even nested callouts — can live inside.

---

## Round-trip serialization

`Callout.addStorage().markdown.serialize()` emits:

```
> [!TYPE] optional title
> first body line
> second body line
```

with a trailing blank line. It captures the inner content into a temporary buffer, splits on newlines, and prefixes every non-empty line with `> `. A trailing empty line from `renderContent` is dropped so the serializer does not double-quote empty trailing paragraphs.

This is the only form the serializer emits. There is no `<div data-callout>` HTML output in saved Markdown files — that HTML exists only inside the ProseMirror DOM tree at runtime.

---

## Inserting a callout

Three entry points insert a callout in WYSIWYG mode:

1. **Slash command.** Type `/callout` (or `/note`, `/tip`, `/warning`, etc.) and press Enter. The default type is `NOTE`; see [slash commands](./slash-commands.md).
2. **Command API.** `editor.commands.insertCallout('WARNING', 'Breaking change')` from the extension's declared command.
3. **Paste Markdown.** Paste a blockquote that starts with `[!TYPE]` and the GFM parser lifts it to a callout node (see limitation below).

---

## Known limitation: fromMarkdown parse-side

`Callout.ts` exports `getCalloutMarkdownConfig()` returning a `fromMarkdown` transformer that intercepts `blockquote` tokens, tests the first paragraph against `/^\[!(NOTE|TIP|WARNING|DANGER|INFO)\](?:\s+(.+))?$/`, and lifts matches into `callout` nodes.

This transformer is **not yet wired into the editor.** `TiptapEditor.svelte` currently configures `tiptap-markdown` without passing callout parse overrides, which means the parser uses remark's built-in blockquote handler. As a result:

- **Writing a callout in the editor** (slash command or API) works end-to-end: the node renders correctly and serializes back to `> [!TYPE]` form.
- **Opening a Markdown file** that already contains `> [!NOTE]` syntax renders it as a plain blockquote, not a styled callout.

The fix requires wiring `remark-gfm` (which understands GFM alerts at the AST level) into the `tiptap-markdown` configuration and passing `getCalloutMarkdownConfig()` as a markdown extension. Until that lands, round-trip fidelity for callouts requires the callout to have been created inside the editor at least once.

The `serialize(parse(input)) === input` invariant still holds for callouts created in-editor because the out-bound serializer is independent of the in-bound parser.

---

## File reference

| File | Role |
|---|---|
| `apps/editor/src/lib/editor/extensions/Callout.ts` | Node schema, node view, serializer, parse config |
| `apps/editor/src/lib/editor/TiptapEditor.svelte` | Wires `Callout` into the editor; owns callout CSS |
| `apps/editor/src/lib/editor/__tests__/callout.test.ts` | Unit tests for the callout node |
| `apps/editor/src/lib/editor/slash-commands/registry.ts` | `/callout` slash command entry |
