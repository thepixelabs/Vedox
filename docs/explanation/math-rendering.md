---
title: "Math Rendering"
type: explanation
status: published
date: 2026-04-11
project: "vedox"
tags: ["flagship-ux", "editor", "math", "katex"]
author: "Vedox Team"
---

# Math Rendering

Vedox renders LaTeX math with KaTeX, inline and block, with a click-to-edit UX in the WYSIWYG editor. This document explains the choice of KaTeX, the two syntaxes, the lazy-loading strategy, the edit flow, and the current parse-side limitation.

Sources:
- `apps/editor/src/lib/editor/extensions/KatexInline.ts`
- `apps/editor/src/lib/editor/extensions/KatexBlock.ts`

---

## Why KaTeX, not MathJax

KaTeX and MathJax are the two mature web math renderers. Vedox uses KaTeX for three reasons:

1. **Bundle size.** KaTeX renders a defined subset of LaTeX and ships as a single library plus a CSS file (~280 KB gzipped for the ESM build, plus 23 KB CSS). MathJax v3 loads a component graph at runtime and, depending on configuration, pulls 500 KB to 1 MB of JavaScript before the first equation paints.
2. **Synchronous rendering.** KaTeX's `render(latex, element)` is synchronous. An equation is painted on the same frame it is requested. MathJax is asynchronous and its queue-based rendering model does not compose cleanly with ProseMirror's node-view lifecycle.
3. **No-surprise output.** KaTeX throws or falls back on unsupported constructs (with `throwOnError: false` it renders the raw LaTeX in a red wrapper). MathJax's broader AMS support is valuable for academic papers but not for developer documentation.

A documentation tool needs fast, predictable math. Every equation in the Vedox docs set renders in under 3 ms on a mid-range laptop.

---

## Two syntaxes

| Form | Markdown | Node type | Renders as |
|---|---|---|---|
| Inline | `$x^2 + y^2$` | `katexInline` | Inline span, flows with surrounding text |
| Block | `$$\n\int_0^1 x\\,dx\n$$` | `katexBlock` | Centered block, standalone line |

The node types are separate because their ProseMirror schema positions differ: `katexInline` is `group: 'inline'`, `katexBlock` is `group: 'block'`. Both are `atom: true` — they have no text content the editor can traverse into; the LaTeX source lives in the `latex` attribute.

Round-trip canonical forms:

```
serialize(parse("$x^2$"))                    === "$x^2$"
serialize(parse("$$\n\\int_0^1 x dx\n$$"))   === "$$\n\\int_0^1 x dx\n$$"
```

The block serializer ensures a trailing newline after the LaTeX body before the closing `$$` so fenced math parses consistently regardless of how the user typed it.

---

## Lazy loading

KaTeX is not imported at editor boot. Both `KatexInline.ts` and `KatexBlock.ts` share the same loader pattern:

```ts
let katexModule: typeof import('katex') | null = null;
let katexCssLoaded = false;

async function loadKatex(): Promise<typeof import('katex')> {
  if (katexModule) return katexModule;
  katexModule = await import('katex');

  if (!katexCssLoaded && typeof document !== 'undefined') {
    katexCssLoaded = true;
    const link = document.createElement('link');
    link.rel = 'stylesheet';
    link.href = 'https://cdn.jsdelivr.net/npm/katex@0.16.11/dist/katex.min.css';
    link.crossOrigin = 'anonymous';
    document.head.appendChild(link);
  }

  return katexModule;
}
```

The first time a math node renders, the dynamic `import('katex')` triggers a code-split chunk fetch. The CSS stylesheet is appended to `<head>` on the same call. Both are cached module-level — every subsequent math node on the page uses the already-loaded module with no network round-trip.

This keeps the cold-load budget (see [performance budgets](./performance-budgets.md)) unaffected on documents that contain no math. A document with a single inline equation pays the KaTeX cost exactly once, at first paint, not at editor construction.

The CSS is loaded from `cdn.jsdelivr.net`. This is the only runtime CDN dependency in the editor and is a known trade-off: self-hosting the font files is on the roadmap to make the editor fully offline-capable.

---

## Click-to-edit

Both inline and block math nodes attach a click handler in their node view. Clicking a rendered equation swaps the KaTeX output for a plain editor:

- **Inline:** single-line `<input type="text">`, focused and pre-selected, styled with the mono font and accent border. Enter commits, Escape reverts.
- **Block:** multi-line `<textarea>` sized to the LaTeX source (`rows = max(3, lines + 1)`), resizable vertically. Blur commits, Escape reverts.

Commit flow:

1. Read the new LaTeX from the input/textarea.
2. If changed, dispatch a `setNodeMarkup` transaction at the node's position with the new `latex` attribute.
3. Destroy the input, re-render the math with the updated source.

The node view sets `contenteditable="false"` on its wrapper and returns `true` from `ignoreMutation()`. ProseMirror never tries to step inside a math node — the LaTeX source is never part of the document text. This is essential for round-trip fidelity: the canonical string is the `latex` attribute, not a DOM-derived text read-out.

While a math node is in edit mode, `stopEvent()` returns `true`, which prevents ProseMirror from hijacking the input's keyboard events. The user gets a normal text field.

---

## Known limitation: remark-math parse-side

Both extensions export `getKatexInlineMarkdownConfig()` and `getKatexBlockMarkdownConfig()` containing `fromMarkdown` transformers. Each transformer is keyed on mdast token types (`inlineMath`, `math`) that are only emitted when `remark-math` is registered with the Markdown parser.

`TiptapEditor.svelte` does not currently wire `remark-math` into `tiptap-markdown`. As a result:

- **Inserting math via the slash command** (`/math`) or API (`editor.commands.insertInlineMath('E = mc^2')`) works end-to-end.
- **Opening a Markdown file** that already contains `$x^2$` or a `$$...$$` block renders the source as literal text surrounded by dollar signs, not as math.

The fix is to register `remark-math` as a `tiptap-markdown` extension and pass the two `fromMarkdown` configs. The out-bound serializer is independent and already emits canonical forms, so files created in-editor round-trip correctly today.

---

## Escape hatches

If KaTeX fails to render (unsupported macro, syntax error), `throwOnError: false` tells KaTeX to emit its own error styling inline instead of throwing. The node view wraps the call in a `try/catch` as a second line of defense that falls back to literal `$...$` text. Neither path crashes the editor.

---

## File reference

| File | Role |
|---|---|
| `apps/editor/src/lib/editor/extensions/KatexInline.ts` | Inline math node, loader, node view |
| `apps/editor/src/lib/editor/extensions/KatexBlock.ts` | Block math node, loader, node view |
| `apps/editor/src/lib/editor/TiptapEditor.svelte` | Wires both extensions into the editor; owns math CSS |
| `apps/editor/src/lib/editor/slash-commands/registry.ts` | `/math` slash command entry |
