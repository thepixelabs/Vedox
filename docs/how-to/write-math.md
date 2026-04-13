---
title: "How to write math"
type: how-to
status: published
date: 2026-04-11
project: "vedox"
tags: ["flagship-ux", "math", "katex", "latex", "editor"]
author: "Vedox Team"
difficulty: "intermediate"
estimated_time_minutes: 5
prerequisites:
  - "A document open in the editor"
  - "Familiarity with KaTeX-supported LaTeX syntax"
---

# How to write math

This guide covers inline and block math in Vedox, the KaTeX syntax subset the renderer accepts, and click-to-edit for rendered equations.

## Prerequisites

- A document is open in the editor
- You are writing LaTeX that KaTeX supports (see the syntax note in Step 4)

## Steps

1. **Write inline math.** Wrap a LaTeX expression in single dollar signs. Inline math flows with surrounding text and uses KaTeX's non-display mode:

   ```markdown
   Einstein's mass-energy relation is $E = mc^2$, which implies that mass is a form of stored energy.
   ```

2. **Write block (display) math.** Wrap the expression in double dollar signs on their own lines. Block math is centered on its own line and uses KaTeX's display mode, which gives operators their larger display-size glyphs:

   ```markdown
   The definite integral of a decaying exponential is:

   $$
   \int_0^\infty e^{-x} \, dx = 1
   $$
   ```

3. **Insert a block from the slash menu.** In rich mode, type `/math` on an empty line and press `Enter`. A display-math block seeded with `E = mc^2` is inserted. Click it to replace the placeholder with your own expression.

4. **Click rendered math to edit it.** In rich mode both inline and block math render via KaTeX and are read-only by default. Click a rendered equation to open an inline editor showing the raw LaTeX source. Press `Enter` to commit the edit, `Escape` to cancel and restore the previous source, or click away to commit on blur.

5. **Know the KaTeX syntax subset.** Vedox ships KaTeX 0.16.11. KaTeX implements a subset of LaTeX — most common math constructs work, but general LaTeX packages and document-level commands do not. Reliable features include:

   - Greek letters (`\alpha`, `\Omega`), operators (`\sum`, `\int`, `\prod`, `\lim`), and relations (`\le`, `\ge`, `\neq`, `\approx`)
   - Fractions (`\frac{a}{b}`), roots (`\sqrt{x}`, `\sqrt[3]{x}`), and powers/subscripts (`x^2`, `a_{ij}`)
   - Matrices (`\begin{pmatrix}...\end{pmatrix}`), aligned equations (`\begin{aligned}...\end{aligned}`), and cases (`\begin{cases}...\end{cases}`)
   - Text inside math (`\text{if } n > 0`) and spacing commands (`\,`, `\;`, `\quad`)

   Unsupported constructs render as the literal source with a KaTeX error class instead of throwing, because the renderer is invoked with `throwOnError: false`. If an expression is rendering as plain text, it is almost always an unsupported command rather than a syntax error on your side — consult the [KaTeX function support table](https://katex.org/docs/supported.html) to confirm.

## Verification

- Inline `$E = mc^2$` renders as a typeset expression that flows with its surrounding sentence.
- A block `$$...$$` expression renders centered on its own line with display-mode glyph sizing.
- Clicking a rendered equation opens a single-line input (inline) or a textarea (block) containing the exact LaTeX source, and committing the edit re-renders without leaving stray markup in the document.
- Round-tripping a document through save and reload preserves the original dollar-sign syntax byte-for-byte.

## Related

- [How to insert blocks with slash commands](./use-slash-commands.md) — the `/math` command inserts a block-math node
- [KaTeX supported functions](https://katex.org/docs/supported.html) — authoritative list of LaTeX commands KaTeX implements
