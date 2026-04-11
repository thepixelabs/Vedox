---
title: "Vedox Design Framework"
type: doc
status: approved
date: 2026-04-07
project: "vedox"
tags: ["design-system", "ia", "components", "tokens", "a11y", "editor-ux", "framework"]
author: "Vedox Team"
owner: "creative-technologist"
---

> **What this is.** The single authoritative law for the visual, IA, component, and editor-UX side of every page in Vedox — the product UI **and** the documentation it serves to itself. It is the sibling of the editorial [WRITING_FRAMEWORK.md](./WRITING_FRAMEWORK.md), which owns frontmatter, file locations, voice, lifecycle, and content acceptance.
>
> **Boundary.** If a rule is about *what a file says* or *where a file lives in the repo*, it belongs to the WRITING_FRAMEWORK. If it is about *how that file is rendered, navigated, edited, or perceived*, it belongs here. Where the two touch, this document defers to WRITING_FRAMEWORK for the editorial half and links into it.
>
> **Status.** Approved. Changes to this document require a creative-technologist review and an ADR if they touch tokens, IA taxonomy, or the agent contract.

---

## 0. Audience and enforcement posture

This document is read by:

1. Humans building features in Vedox.
2. Agents (Claude, Codex, Copilot, Gemini, future) producing pages or components.
3. CI tooling — which will, over time, mechanically enforce as much of this document as possible (see § 9).

The framework is **prescriptive, not aspirational**. If a rule below conflicts with code already in `apps/editor`, the code is the bug, not the framework. Open an ADR before deviating.

---

## 1. Information architecture

### 1.1 Two surfaces, one taxonomy

Vedox renders two distinct surfaces with the **same** information architecture:

| Surface             | URL pattern                                  | Audience                           |
| ------------------- | -------------------------------------------- | ---------------------------------- |
| Workspace editor    | `/projects/[project]/docs/[...path]`         | Author working inside a workspace  |
| Published portal    | `/[project]/[category]/[slug]` (Phase 3, deferred) | Reader of a published doc site     |

The published portal surface is **deferred to Phase 3**. The URL pattern is fixed now so content authored in Phase 2 renders correctly when the reader surface ships.

Sub-workspace pattern: projects nested under a parent workspace live at `docs/projects/<project-slug>/` (canonical per CEO decision). Sub-workspace roots are siblings of `docs/platform/` and the other first-class top-level categories.

A page authored once must work in both surfaces with no rewriting. The IA below is therefore about *content classes*, not URLs.

### 1.2 Top-level taxonomy

A Vedox project's documentation is partitioned into **eight** top-level categories. Every page must belong to exactly one. The category is derived from the page's `type` frontmatter field (see § 1.5) — never from filename or directory alone.

| Category        | Purpose                                                            | Editorial home (per WRITING_FRAMEWORK)        |
| --------------- | ------------------------------------------------------------------ | --------------------------------------------- |
| **Overview**    | Project landing, README, getting-started                           | `README.md`, `docs/index.md`                  |
| **How-to**      | Task-oriented step recipes                                         | `docs/how-to/`                                |
| **Runbooks**    | Incident response, on-call, recovery procedures                    | `docs/runbooks/`                              |
| **ADRs**        | Architecture decision records                                      | `docs/adr/`                                   |
| **Reference**   | API references, schema, configuration, taxonomies                  | `docs/reference/`, `docs/api/`                |
| **Platform**    | Platform / infrastructure / network / logging / observability      | `docs/platform/`                              |
| **Issues**      | Project task backlog, agent review queue (ephemeral, app-rendered) | (database, surfaced via TaskBacklog)          |

If a content type emerges that does not fit one of these seven, the WRITING_FRAMEWORK is amended **first**, then this document, then code. Categories are not invented in the sidebar.

### 1.3 Navigation model

The reader's path through Vedox is fixed:

```
landing → category → page → adjacent page
   │          │        │            │
   │          │        │            └── prev/next within the same category
   │          │        └── breadcrumbs back up the tree
   │          └── category index lists pages sorted by status, then date desc
   └── single hero, single CTA, no marketing chrome
```

**Breadcrumbs** are mandatory on every page deeper than the project root and follow the pattern already in `routes/projects/[project]/docs/[...path]/+page.svelte`:

```
Projects / <project name> / <page title>
```

The current page is rendered with `aria-current="page"` and is **not** a link. Breadcrumb separators carry `aria-hidden="true"`. Truncation with `text-overflow: ellipsis` is allowed only on the project name segment.

**Search** is reachable from the sidebar (`SearchBar.svelte`) and from anywhere via the keyboard shortcut `Cmd/Ctrl + K`. Search results group by category before relevance — a runbook hit and a how-to hit must never be intermingled in the result list.

**Tags** are a flat secondary axis. Tags do not create new navigation pages; they filter existing category lists. Tag-only browsing is forbidden because it produces unscoped result sets.

### 1.4 Sidebar rules

The left sidebar is the **only** persistent navigation chrome. There is no top bar.

- The sidebar collapses to a 40px rail and persists collapsed state to `localStorage` under `vedox:sidebar-collapsed`.
- Sections, top-to-bottom: project switcher → search bar → category tree → bottom (settings, theme toggle).
- Section labels are uppercase 11px tracked at 0.06em — never larger.
- The sidebar background is `--color-surface-elevated`. Never `--color-surface-base`.
- Active page in the tree uses `--color-accent-subtle` background and `--color-text-primary` text. No bold weight. No left-border bar.
- The wordmark is monospace, lowercase, never an SVG logo. Vedox is a developer tool.

### 1.5 Mapping editorial content types to IA

The `type` field in a page's frontmatter is the single source of truth for IA placement. The current canonical `type` enum lives in `apps/editor/src/lib/editor/utils/frontmatter.ts` and is:

```
adr | api-reference | runbook | readme | how-to | doc
```

**This enum is incomplete and is being widened.** The framework declares the target enum below; the FrontmatterPanel and FrontmatterSchema must be widened to match in the next sprint (see § 11 conflicts).

Target `type` enum:

| `type` value      | IA category   | Page template (§ 4)         |
| ----------------- | ------------- | --------------------------- |
| `readme`          | Overview      | `OverviewTemplate`          |
| `doc`             | Overview      | `OverviewTemplate`          |
| `how-to`          | How-to        | `HowToTemplate`             |
| `runbook`         | Runbooks      | `RunbookTemplate`           |
| `adr`             | ADRs          | `AdrTemplate`               |
| `api-reference`   | Reference     | `ReferenceTemplate`         |
| `reference`       | Reference     | `ReferenceTemplate`         |
| `platform`        | Platform      | `PlatformTemplate`          |
| `issue`           | Issues        | `IssueTemplate` (app-only)  |

A page with no `type` falls back to `OverviewTemplate` and surfaces a warning chip in the editor. A page with a `type` value not in this enum is rejected at publish time.

---

## 2. Design tokens

The token system is the contract between every component and the visual language. **No component may emit a hard-coded color, px value, font size, radius, or shadow.** The token file is the single source of truth and the only thing the visual language depends on.

### 2.1 Where they live

- Authoritative file: `apps/editor/src/styles/tokens.css`
- Imported globally by: `apps/editor/src/app.css`
- Machine-readable mirror: § 10 of this document (JSON block)

The CSS file and the JSON block in § 10 must stay in lockstep. A future codegen step will emit `tokens.css` from § 10. Until then, both are edited by hand and a CI check (§ 9) verifies they match.

### 2.2 Semantic categories

The tokens are organised by **semantic role**, never by raw color name. There is no `--color-blue-500`. There is `--color-accent`.

| Category   | Tokens                                                                                                                              | Notes                                                          |
| ---------- | ----------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------- |
| Surface    | `--color-surface-base`, `--color-surface-elevated`, `--color-surface-overlay`                                                       | Three steps. No more.                                          |
| Text       | `--color-text-primary`, `--color-text-secondary`, `--color-text-muted`, `--color-text-subtle`, `--color-text-inverse`               | `--color-text-subtle` is for placeholders / gutter only.       |
| Accent     | `--color-accent`, `--color-accent-hover`, `--color-accent-subtle`                                                                   | Single accent family, indigo/violet.                           |
| Border     | `--color-border`, `--color-border-strong`                                                                                           | Two steps. Hover uses `--color-border-strong`.                 |
| Status     | `--color-success`, `--color-warning`, `--color-error`, `--color-info`                                                               | Used by chips, banners, validation messages.                   |
| Code       | `--color-surface-code`                                                                                                              | CodeMirror canvas only.                                        |
| Spacing    | `--space-1` … `--space-8`                                                                                                           | 4px base scale: 4, 8, 12, 16, 20, 24, 28, 32.                  |
| Typography | `--font-sans`, `--font-mono`, `--font-size-sm`, `--font-size-base`, `--font-size-lg`, `--font-size-xl`                              | Four sizes only: 12 / 14 / 16 / 20.                            |
| Radius     | `--radius-sm`, `--radius-md`, `--radius-lg`                                                                                         | 4 / 6 / 10. No `--radius-xl`. No `border-radius: 50%` outside avatars/spinners. |
| Shadow     | `--shadow-sm`, `--shadow-md`                                                                                                        | Two steps. `--shadow-lg` is forbidden — modals use overlay surface, not heavier shadow. |

### 2.3 Theme strategy

- **Dark is the first-class experience.** Light is the toggled alternative.
- Theme is stored in `data-theme="dark|light"` on `<html>` and persisted to `localStorage` under `vedox:theme`.
- **`prefers-color-scheme` is intentionally ignored** at the visual level — the user always controls theme explicitly. `prefers-color-scheme` *may* be used as the initial value on first load before any explicit choice is made; after that, the user choice wins.
- The `<html>` element transitions `background-color` and `color` over 150ms ease. Borders, shadows, and interactive states do **not** transition with the theme — only colors do.
- Per-component dark variants are forbidden. If a component looks wrong in dark mode, the token is wrong, not the component.

### 2.4 Motion

| Use case                        | Duration  | Easing           |
| ------------------------------- | --------- | ---------------- |
| Hover color/background change   | 80–120ms  | `ease`           |
| Focus ring appear               | instant   | none             |
| Sidebar collapse                | 200ms     | `ease`           |
| Theme transition                | 150ms     | `ease`           |
| Empty-state entrance            | 200ms     | `ease`           |
| Spinner rotation                | 600–700ms | `linear`         |
| Mode-toggle handoff (editor)    | 0ms       | none             |

**Rules:**
- No animation may exceed 250ms.
- No spring/bounce easing. We are a developer tool, not a marketing site.
- **`prefers-reduced-motion: reduce` disables every transition and animation in the system except spinners** (which fall back to a static "Loading…" label inside the spinner element).
- Animate to add clarity, never to celebrate.

### 2.5 Iconography

- **One icon set:** inline SVG sourced from Feather (`https://feathericons.com`). No icon font, no library import — copy the SVG into the component.
- **Sizing:** `14px` inside text rows, `20px` for header / banner contexts, `48px` only inside `EmptyState`. No other sizes.
- **Stroke:** `stroke-width="2"`, `stroke-linecap="round"`, `stroke-linejoin="round"`.
- **Color:** always `currentColor`. Icons inherit from text color. Never set a hex.
- **`aria-hidden="true"`** on every decorative SVG. If the icon carries meaning, the parent element gets an `aria-label`.

---

## 3. Component conventions

### 3.1 Existing component vocabulary

The components currently in `apps/editor/src/lib/components/` form the canonical vocabulary. Every new component must conform to the conventions they imply, not introduce parallel ones.

| Component             | Layer       | Purpose                                                                   | Use when…                                                              |
| --------------------- | ----------- | ------------------------------------------------------------------------- | ---------------------------------------------------------------------- |
| `Sidebar`             | Layout      | The single persistent navigation rail.                                    | Always — at most one per app shell.                                    |
| `ProjectSwitcher`     | Composite   | Search-first project picker.                                              | Top of sidebar, when there is at least one project.                    |
| `ProjectTree`         | Composite   | Hierarchical doc list for the active project.                             | Inside sidebar, scoped to current project.                             |
| `SearchBar`           | Composite   | FTS5-backed search input with inline result list.                         | Inside sidebar; future global `Cmd+K` palette wraps the same primitive.|
| `ThemeToggle`         | Primitive   | Icon button toggling `data-theme`.                                        | Bottom of sidebar, exactly one instance.                               |
| `EmptyState`          | Primitive   | Centered icon + heading + body + up-to-two actions.                       | Any zero-data, first-run, or "nothing here" surface.                   |
| `StatusList`          | Composite   | Accessible drag-reorderable list of status-tagged items.                  | Task backlog, agent review queue, any prioritised list.                |
| `StatusChip`          | Primitive   | Pill-shaped status label, five variants.                                  | Anywhere a status string would otherwise be plain text.                |
| `TaskBacklog`         | Page-level  | Per-project flat task list. Wraps `StatusList`.                           | Inside a project view; never standalone.                               |
| `ImportDialog`        | Composite   | Modal with two tabs (Import & Migrate, Link read-only).                   | Adding or linking external projects only.                              |
| `Editor`              | Page-level  | Dual-mode (CodeMirror ↔ Tiptap) document editor.                          | The canonical doc edit surface.                                        |
| `FrontmatterPanel`    | Composite   | Structured metadata form for the five canonical fields.                   | Above Tiptap body in WYSIWYG mode.                                     |
| `MermaidPopover`      | Composite   | Live preview popover for `mermaid` fenced blocks.                         | Inside Tiptap, on `MermaidNode` focus.                                 |

**Three layers, no others:**
1. **Primitives** — token-pure leaves with no business logic (`StatusChip`, `EmptyState`, `ThemeToggle`).
2. **Composites** — compose primitives, may hold local state, no API calls (`StatusList`, `FrontmatterPanel`, `SearchBar`).
3. **Page-level** — own data fetching, route awareness, and parent-passed callbacks (`Editor`, `TaskBacklog`, `Sidebar`).

A component that does not fit one of these layers is mis-designed. Ship the primitive first; lift state up later.

### 3.2 Naming

- **Component file name:** `PascalCase.svelte`.
- **CSS class name:** `kebab-case` BEM — `block`, `block__element`, `block--modifier`. The block name matches the kebab-case of the component (`StatusList` → `.status-list`). This convention is enforced by the existing codebase; do not introduce CSS modules, atomic CSS, or Tailwind.
- **Props:** `camelCase`. Boolean props are positive (`draggable`, not `notDraggable`).
- **Event callbacks:** `on<Verb>` — `onChange`, `onReorder`, `onPublish`. Never the bare verb.
- **Slot/snippet props:** the children snippet is named `children`. Named snippets are `header`, `footer`, `actions`, `icon`.
- **State variables (`$state`):** `camelCase`, no `is`/`has` prefix unless boolean.
- **Stores:** `<noun>Store` exported from `$lib/stores/<noun>.ts` — `themeStore`, `projectsStore`, `sidebarStore`.

### 3.3 File layout

A new component lives in **its own folder** if it has more than one file, otherwise as a single `.svelte` file at the root of `lib/components/`.

```
lib/components/
├── EmptyState.svelte                 ← single-file primitive
└── StatusList/                       ← multi-file composite
    ├── index.ts                      ← barrel + canonical type exports
    ├── StatusList.svelte
    ├── StatusChip.svelte
    └── StatusList.test.ts            ← optional, colocated
```

Rules:

- A folder must have an `index.ts` that exports the public surface.
- Tests live next to the component, never in a separate `__tests__` directory at the package root (the existing `lib/editor/__tests__/` is a Phase 1 holdover and will move).
- A component never imports from a sibling component's internal file path — only from its `index.ts`.
- A component never imports from `routes/` — data flows down via props, not up via imports.

### 3.4 Accessibility contract (per component)

Every interactive component **must** declare its accessibility contract in a top-of-file JSDoc comment. The contract must answer four questions:

1. **Keyboard:** which keys do what?
2. **Focus:** where does focus land on mount, on open, on close?
3. **ARIA:** which roles, labels, live regions, expanded states?
4. **Contrast:** does every text/background combination meet WCAG 2.2 AA (4.5:1 for body text, 3:1 for large text and UI affordances)?

`StatusList`, `EmptyState`, and `ImportDialog` already do this. New components without it are not merged.

### 3.5 State, loading, error, empty — the four-state rule

Every component that displays remote data must explicitly handle **all four** of:

| State    | Visual                                                          |
| -------- | --------------------------------------------------------------- |
| Loading  | Inline spinner with `aria-live="polite"` and "Loading…" text    |
| Empty    | Either inline text (`status-list__empty`) or `EmptyState` card  |
| Error    | Inline `role="alert"` with dismiss button, never a toast        |
| Data     | The happy path                                                  |

A component that handles only "data" is unfinished. CI snapshot tests exist (or will exist — § 9) to enforce this.

---

## 4. Page templates

A **template** is a layout contract. Every published doc page renders inside exactly one template, chosen by the `type` field. Templates are real Svelte components living in `apps/editor/src/lib/templates/<Name>Template.svelte`.

### 4.1 Shared template anatomy

All templates follow the same five-region structure already established by the doc editor route:

```
┌─────────────────────────────────────────┐
│  Header                                 │  ← breadcrumb + page meta + status chip
├─────────────────────────────────────────┤
│  Meta strip (optional)                  │  ← author, date, tags, severity, etc.
├─────────────────────────────────────────┤
│                                         │
│  Body                                   │  ← rendered Markdown / editable surface
│                                         │
├─────────────────────────────────────────┤
│  Related (optional)                     │  ← prev/next, related ADRs, linked runbooks
├─────────────────────────────────────────┤
│  Footer                                 │  ← edit-on-disk path, last-modified, source link
└─────────────────────────────────────────┘
```

**Required regions:** header, body, footer. Meta strip and related are optional but, when present, always appear in this order. No template may invent a sixth region.

**Layout:**
- Max body width: `760px` for prose templates (How-To, ADR, Runbook, Overview).
- Max body width: `none` for Reference and Platform — they often contain wide tables and code blocks.
- Horizontal padding: `var(--space-8)` on viewports ≥ 768px, `var(--space-4)` below.
- Vertical rhythm between regions: `var(--space-6)`.

**Responsive behavior:**
- Below 768px the sidebar collapses to a hamburger that opens an overlay, not a slide-in. Existing collapse logic in `Sidebar.svelte` is the source of truth.
- The breadcrumb truncates middle segments with an ellipsis, never the current segment.
- Body content remains left-aligned. Center alignment is forbidden outside `EmptyState`.

**Print behavior:**
- The sidebar and theme toggle are `display: none` in `@media print`.
- Body uses `--color-text-primary` on white regardless of theme.
- Page breaks: `break-inside: avoid` on every `h2`, code block, and table.
- Links print their URL after the anchor text, except inside code blocks.

### 4.2 Per-template specs

#### `OverviewTemplate` (`type: readme | doc`)
- Header: title, single-line tagline (from frontmatter `description`), no chip.
- Meta strip: hidden.
- Body: prose with optional hero image at top (max 320px tall).
- Related: "Browse all documents" link to project index.
- Footer: edit path, last-modified.

#### `HowToTemplate` (`type: how-to`)
- Header: title, `StatusChip` reflecting `status`.
- Meta strip: estimated time, prerequisites count.
- Body: must contain an `## Prerequisites` section and an ordered `## Steps` list. Lint warning if missing.
- Related: "Other how-tos in this project" — same category.
- Footer: edit path, last-modified, "Was this helpful?" placeholder (Phase 3, no telemetry).

#### `RunbookTemplate` (`type: runbook`)
- Header: title, severity badge (P0/P1/P2/P3) sourced from frontmatter `on_call_severity`. Severity badge uses error/warning/info status colors per § 5.2.
- Meta strip: `last_tested` date with a "stale" warning chip if older than 90 days.
- Body: must contain `## Symptoms`, `## Diagnosis`, `## Mitigation`, `## Recovery` sections (lint warning if missing).
- Related: linked ADRs and other runbooks tagged with the same `tags`.
- Footer: edit path, last-tested, on-call contact link.

#### `AdrTemplate` (`type: adr`)
- Header: title (e.g. "ADR-001: Markdown as Source of Truth"), `StatusChip` mapped from ADR statuses (`proposed | accepted | superseded | rejected | deprecated`).
- Meta strip: date, author, `superseded_by` link if present.
- Body: must contain `## Context`, `## Decision`, `## Consequences` sections.
- Related: superseded-by chain rendered as a horizontal timeline; sibling ADRs sorted by ID.
- Footer: edit path, ADR ID anchor.

#### `ReferenceTemplate` (`type: api-reference | reference`)
- Header: title, version chip if present.
- Meta strip: hidden.
- Body: full-width, no max-width. Tables and code blocks may be wide.
- Related: "Other references in this project".
- Footer: edit path, last-modified.

#### `PlatformTemplate` (`type: platform`)
- Header: title, environment chip (dev / staging / prod).
- Meta strip: owner team, last reviewed date.
- Body: full-width.
- Related: linked runbooks for this surface.
- Footer: edit path, last-modified, oncall.

#### `IssueTemplate` (`type: issue`)
- App-only — never published. Rendered inside `TaskBacklog`.
- Single row of `StatusList` plus inline edit affordances.
- No header / footer regions.

### 4.3 Template registration

Templates are registered in `apps/editor/src/lib/templates/index.ts`:

```ts
import OverviewTemplate from './OverviewTemplate.svelte';
import HowToTemplate from './HowToTemplate.svelte';
// ...
export const templates = {
  readme: OverviewTemplate,
  doc: OverviewTemplate,
  'how-to': HowToTemplate,
  // ...
} as const;
```

Adding a new template requires (a) an entry in this map, (b) a section in § 4.2 of this document, (c) a `type` enum entry in § 1.5 and the FrontmatterSchema. All three or none.

---

## 5. Editor UX (Tiptap / dual-mode)

### 5.1 The dual-mode contract

Vedox's editor has two modes that share one canonical Markdown string:

- **Code mode** — CodeMirror 6, raw Markdown including frontmatter.
- **WYSIWYG mode** — Tiptap, frontmatter rendered separately in `FrontmatterPanel`.

**Round-trip invariant (load-bearing — do not weaken):**

```
serialize(parse(body)) === body
```

A user switching from WYSIWYG → Code → WYSIWYG must see byte-identical Markdown. Any extension or custom node that breaks this is rejected. The Go backend (`goldmark`) is the authoritative parser; the frontend must produce output `goldmark` accepts unchanged.

The mode preference is persisted per-document under `vedox-editor-mode-${documentId}` in `localStorage`. The default for new documents is `wysiwyg`.

### 5.2 Toolbar order (Tiptap mode)

The toolbar is intentionally minimal. Order is fixed and may not be re-arranged:

```
┌──────────────────────────────────────────────────────────────────────────┐
│  H2  H3  │  B  I  S  `  │  •  1.  >  │  ⛓  ─  │  ⌃ Mode  │   Save  Publish │
└──────────────────────────────────────────────────────────────────────────┘
```

1. Block-level: H2, H3 (H1 is reserved for the page title).
2. Inline marks: Bold, Italic, Strike, Code.
3. Lists & blocks: bullet list, ordered list, blockquote.
4. Insert: link, horizontal rule.
5. Mode toggle (right-aligned).
6. Save / Publish buttons (right-aligned).

There is no font picker, color picker, alignment, or table button in the toolbar. Tables are inserted via slash menu or raw Markdown.

### 5.3 Slash menu items

The slash menu is the **only** way to insert exotic blocks. Order:

1. Heading 2
2. Heading 3
3. Bullet list
4. Ordered list
5. Quote
6. Code block
7. Mermaid diagram
8. Horizontal rule
9. Image (from clipboard / file picker — never URL)
10. Callout (info / warn / danger / success — see § 5.5)

Slash menu items must have an icon, a one-line description, and a keyboard shortcut hint where applicable. The menu is keyboard-only first: arrow keys navigate, Enter inserts, Escape closes.

### 5.4 Custom Tiptap nodes / marks

- **Naming:** `<Concept>Node.ts` for block nodes, `<Concept>Mark.ts` for inline marks. Files live in `apps/editor/src/lib/editor/extensions/`.
- Each extension exports a single `Node.create({...})` or `Mark.create({...})` and **must** declare `parseMarkdown` and `toMarkdown` so round-trip holds.
- Each extension owns its own keyboard shortcuts namespace. Global shortcuts (Cmd+B, Cmd+I, Cmd+K) are claimed by StarterKit and Link; do not collide.
- Custom nodes must not introduce dependencies that would force a network call at edit time. Mermaid renders locally via the existing `mermaid` package and `mermaidCache.ts`.

### 5.5 Callouts

Callouts are an inline block type with four variants — `info`, `warn`, `danger`, `success` — mapped to the four status tokens. Markdown form:

```markdown
> [!INFO] Optional title
> Body of the callout. Multiple lines allowed.
```

(GitHub-style alert syntax. Goldmark parses it; the frontend renders it; round-trip holds.)

Callout colors **must** come from `--color-info`, `--color-warning`, `--color-error`, `--color-success` via `color-mix(in srgb, ... 12%, transparent)` for the background, the same trick `StatusChip` uses. No new tokens.

### 5.6 Frontmatter panel

- The five canonical fields (`title`, `type`, `status`, `date`, `tags`) are rendered as form controls — never as raw YAML.
- Validation runs on blur via `FrontmatterSchema` (warn-only — never blocks save).
- Unknown frontmatter keys are preserved opaque on save and never displayed in the panel. They survive round-trip via `serializeDocument`.
- The panel is collapsed by default for documents older than 7 days (the metadata is stable; the body is what's being edited).

---

## 6. Accessibility baseline (non-negotiable)

| Requirement                                                       | Standard                                 |
| ----------------------------------------------------------------- | ---------------------------------------- |
| Color contrast — body text                                        | WCAG 2.2 AA — 4.5:1                      |
| Color contrast — large text (≥ 18pt or 14pt bold)                 | WCAG 2.2 AA — 3:1                        |
| Color contrast — UI affordances (icons, focus rings, borders)     | WCAG 2.2 AA non-text — 3:1               |
| Keyboard reachability                                             | 100% — every interactive element         |
| Focus visible                                                     | Always — `:focus-visible` outline 2px accent, 2px offset |
| Skip link                                                         | "Skip to main content" first focusable   |
| Landmark roles                                                    | `main`, `nav`, `aside`, `complementary`  |
| Live regions                                                      | All async status: `aria-live="polite"`   |
| Form labels                                                       | Visible label or `aria-label`            |
| Dialog focus trap                                                 | Trap on open, restore on close           |
| `prefers-reduced-motion`                                          | Disables every transition / animation    |
| Screen reader announcements for drag-reorder                      | Required (see `StatusList` precedent)    |

**Hard rules:**

- `<div>` is not a button. `<button type="button">` is.
- `<a>` is for navigation; `<button>` is for actions. Anchors styled as buttons must have `role="button"` (the `EmptyState` CTA shows the pattern).
- Color is never the sole carrier of meaning — pair with text and/or icon.
- An element with `aria-hidden="true"` cannot contain a focusable child.
- Heading levels never skip. The page title is `<h1>`; section headings start at `<h2>`.
- Images must have an `alt`. Decorative images get `alt=""`. SVGs get `aria-hidden="true"` unless they convey meaning, in which case the parent gets `aria-label`.

A component that fails any of these is rejected at PR review and, where mechanically detectable, by CI (§ 9).

---

## 7. Visual asset rules

### 7.1 Where assets live

- Per-project assets: `<workspace>/assets/<page-slug>/<filename>` — colocated next to the docs that reference them.
- Shared workspace assets: `<workspace>/assets/_shared/`.
- Editor app assets (logo, illustrations): `apps/editor/static/`.

The workspace asset paths are stable so they survive a copy of the workspace to another machine.

### 7.2 Naming convention

- Lowercase, kebab-case, no spaces.
- Suffix with a `@2x` for retina density assets, never higher.
- Diagrams: prefer source format alongside the export — `diagram.mmd` next to `diagram.svg`.

### 7.3 Format priority

```
SVG  >  PNG  >  JPG  >  (anything else)
```

- **SVG** for logos, icons, diagrams. Optimised with `svgo`. Inline `<svg>` is allowed up to 4KB; larger goes to `assets/`.
- **PNG** for screenshots. Compressed with `oxipng -o3`. Max width 1600px.
- **JPG** only for photographs. Quality 80. Max width 1600px.
- **WebP / AVIF / GIF / video** are forbidden in docs. We are local-first; we don't embed binaries we can't lint.

Animated diagrams are not animated images — they are `mermaid` blocks (see § 7.5).

### 7.4 Alt text obligation

- Every `![](...)` and every meaningful `<img>` has a descriptive `alt`.
- Decorative images get `alt=""`.
- A doc with a meaningful image and missing alt fails CI lint (§ 9).

### 7.5 Diagrams

**One diagram tool: Mermaid.** Embedded as fenced blocks:

```` markdown
```mermaid
flowchart LR
  A --> B
```
````

The `MermaidNode` extension and `MermaidPopover` already handle live preview in the editor. Mermaid renders entirely client-side — no network call.

**Forbidden:**
- PlantUML (requires Java / network).
- Draw.io / diagrams.net XML (binary, not greppable).
- ASCII boxes-and-arrows for any diagram more complex than 4 boxes — use Mermaid.
- Embedded Lucidchart, Whimsical, or any SaaS iframe — Vedox is local-first.

A simple ASCII tree (`├──`, `└──`) is allowed for filesystem layouts. That is the only ASCII art exception.

---

## 8. The agent contract (visual / UX side)

This section is read by agents producing pages or components for Vedox. Read it before writing a single line.

> **You will be rejected** if you produce a page or component that violates any of the following.

1. **Use the tokens.** Every color, spacing, font size, radius, and shadow must be a `var(--*)` from `tokens.css`. A hard-coded `#1a1a1f`, `12px`, or `border-radius: 4px` in your output is grounds for rejection. The only exception is the existing `editor` and `tokens.css` files themselves.

2. **Use the templates.** Pages live inside one of the templates listed in § 4.2. You do not invent a new layout. If your content does not fit a template, the editorial framework needs an amendment first — stop and ask.

3. **Use the existing components.** Need a status pill? Use `StatusChip`. Need a zero-state? Use `EmptyState`. Need a list? Use `StatusList`. You do not write a parallel "TaskCard" or "ItemRow". A duplicate of an existing component is grounds for rejection regardless of how clean the code is.

4. **Meet the accessibility baseline (§ 6).** Every interactive element keyboard-reachable, every async surface with a live region, every dialog with focus trap. No exceptions for "this is just a prototype".

5. **Handle all four states (§ 3.5).** Loading, empty, error, data. A component that only renders the happy path is unfinished.

6. **Round-trip (editor work).** Custom Tiptap nodes must declare `parseMarkdown` and `toMarkdown` and pass the round-trip test. If you cannot guarantee `serialize(parse(x)) === x`, you do not ship the extension.

7. **No network calls.** Vedox is loopback-only. Your component does not import a CDN font, fetch a remote icon, ping an analytics endpoint, or load a Google webfont. Zero outbound calls is a product invariant.

8. **No new design language.** You do not introduce a new icon set, a new font, a new shadow, a new radius, or a new accent color. The system has one of each. Extend the tokens via PR if you genuinely need something — do not work around them inline.

9. **Defer to editorial.** If your work touches frontmatter, file paths, voice, or content structure, the WRITING_FRAMEWORK is authoritative. If the two frameworks disagree, raise an ADR; do not pick one silently.

10. **Document the a11y contract.** Every new interactive component opens with a JSDoc comment listing keyboard, focus, ARIA, contrast obligations. This is the same precedent set by `StatusList`, `EmptyState`, and `ImportDialog`.

If you are unsure whether a rule applies, the answer is "yes, it applies". Vedox's design system is small on purpose. Constraints make it consistent.

---

## 9. Enforcement plan

### 9.1 Mechanically enforced (CI)

| Rule                                                | Tool                                          | Failure mode |
| --------------------------------------------------- | --------------------------------------------- | ------------ |
| No hard-coded hex / rgb in component CSS            | Custom stylelint rule (`vedox/no-raw-color`)  | error        |
| No hard-coded `px` outside `tokens.css`             | Custom stylelint rule (`vedox/no-raw-length`) | error (warn for `1px` borders) |
| Component file naming                               | ESLint                                        | error        |
| BEM class naming                                    | stylelint `selector-class-pattern`            | warn         |
| Missing alt text on images in docs                  | `markdownlint` + custom rule                  | error        |
| Heading-level skip                                  | `markdownlint` MD001                          | error        |
| Missing `aria-label` on icon-only button            | `eslint-plugin-svelte` + `axe-core` in vitest | error        |
| Color contrast on rendered components               | `axe-core` against Storybook snapshots        | error (Phase 3) |
| Round-trip `serialize(parse(x)) === x`              | Existing golden-file test in `lib/editor/__tests__` | error  |
| `tokens.css` and § 10 JSON in lockstep              | `pnpm test:tokens` (parses both, diffs)       | error        |
| Page `type` value is in the canonical enum          | `vedox lint` (CLI command, Phase 2 follow-up) | error        |
| Diagram fence is `mermaid` (not `plantuml`/etc.)    | `markdownlint` custom rule                    | error        |

### 9.2 Human-reviewed

The following cannot be mechanically enforced and require a creative-technologist review:

- Whether a new component justifies its existence (§ 3.1 vocabulary growth).
- Whether a new template justifies its existence (§ 4.2 list growth).
- Whether a transition adds clarity or just flair (§ 2.4).
- Visual hierarchy and information density on a new page.
- Microcopy tone in error and empty states (deferred to WRITING_FRAMEWORK in part).
- Whether a new editorial content type justifies a new IA category (§ 1.2 list growth).

A PR that touches `tokens.css`, `templates/`, or this document requires a creative-technologist sign-off before merge.

---

## 10. Machine-readable design tokens

The following JSON block is the **single source of truth** that a future codegen step will use to emit `tokens.css` and `tokens.ts`. Until codegen lands, this block and `apps/editor/src/styles/tokens.css` are kept in sync by hand and verified by CI (§ 9).

```json
{
  "$schema": "https://vedox.dev/schemas/design-tokens.v1.json",
  "version": "1.0.0",
  "themes": {
    "light": {
      "color": {
        "surface": {
          "base": "#ffffff",
          "elevated": "#f5f5f7",
          "overlay": "#ebebed",
          "code": "#f0f0f4"
        },
        "text": {
          "primary": "#1a1a1f",
          "secondary": "#4a4a56",
          "muted": "#8a8a98",
          "subtle": "#b4b4c4",
          "inverse": "#ffffff"
        },
        "accent": {
          "default": "#5b6af5",
          "hover": "#4754e8",
          "subtle": "#eef0fe"
        },
        "border": {
          "default": "#e2e2e8",
          "strong": "#c4c4ce"
        },
        "status": {
          "success": "#22874a",
          "warning": "#c47200",
          "error": "#c8282d",
          "info": "#0f6cbd"
        }
      },
      "shadow": {
        "sm": "0 1px 3px rgba(0, 0, 0, 0.08), 0 1px 2px rgba(0, 0, 0, 0.04)",
        "md": "0 4px 12px rgba(0, 0, 0, 0.1), 0 2px 4px rgba(0, 0, 0, 0.06)"
      }
    },
    "dark": {
      "color": {
        "surface": {
          "base": "#111114",
          "elevated": "#1c1c21",
          "overlay": "#26262d",
          "code": "#0d0d10"
        },
        "text": {
          "primary": "#f0f0f5",
          "secondary": "#b0b0c0",
          "muted": "#686878",
          "subtle": "#4a4a5a",
          "inverse": "#111114"
        },
        "accent": {
          "default": "#818cf8",
          "hover": "#a5b0fb",
          "subtle": "#1e2040"
        },
        "border": {
          "default": "#2e2e38",
          "strong": "#46464e"
        },
        "status": {
          "success": "#4ade80",
          "warning": "#fbbf24",
          "error": "#f87171",
          "info": "#60a5fa"
        }
      },
      "shadow": {
        "sm": "0 1px 3px rgba(0, 0, 0, 0.3), 0 1px 2px rgba(0, 0, 0, 0.2)",
        "md": "0 4px 12px rgba(0, 0, 0, 0.4), 0 2px 4px rgba(0, 0, 0, 0.25)"
      }
    }
  },
  "shared": {
    "space": {
      "1": "4px",
      "2": "8px",
      "3": "12px",
      "4": "16px",
      "5": "20px",
      "6": "24px",
      "7": "28px",
      "8": "32px"
    },
    "font": {
      "sans": "-apple-system, BlinkMacSystemFont, \"Inter\", \"Segoe UI\", Roboto, sans-serif",
      "mono": "\"JetBrains Mono\", \"Fira Code\", \"Cascadia Code\", ui-monospace, \"SF Mono\", monospace",
      "size": {
        "sm": "12px",
        "base": "14px",
        "lg": "16px",
        "xl": "20px"
      }
    },
    "radius": {
      "sm": "4px",
      "md": "6px",
      "lg": "10px"
    },
    "z": {
      "base": 0,
      "sticky": 100,
      "dropdown": 200,
      "overlay": 900,
      "modal": 1000,
      "toast": 1100,
      "tooltip": 1200
    },
    "motion": {
      "duration": {
        "instant": "0ms",
        "fast": "80ms",
        "default": "120ms",
        "theme": "150ms",
        "layout": "200ms"
      },
      "easing": {
        "default": "ease",
        "linear": "linear"
      }
    }
  },
  "ia": {
    "categories": [
      "overview",
      "how-to",
      "runbooks",
      "adrs",
      "reference",
      "platform",
      "issues"
    ],
    "type_to_template": {
      "readme": "OverviewTemplate",
      "doc": "OverviewTemplate",
      "how-to": "HowToTemplate",
      "runbook": "RunbookTemplate",
      "adr": "AdrTemplate",
      "api-reference": "ReferenceTemplate",
      "reference": "ReferenceTemplate",
      "platform": "PlatformTemplate",
      "issue": "IssueTemplate"
    }
  }
}
```

---

## 11. Cross-references

- **Editorial sibling:** [WRITING_FRAMEWORK.md](./WRITING_FRAMEWORK.md) — frontmatter, naming, voice, lifecycle, content acceptance criteria. The two documents are co-equal; cite each other and never duplicate.
- **Token source file:** `apps/editor/src/styles/tokens.css`
- **Component vocabulary:** `apps/editor/src/lib/components/`
- **Editor implementation:** `apps/editor/src/lib/editor/`
- **Existing IA reference:** `apps/editor/src/routes/projects/[project]/docs/[...path]/+page.svelte`
- **Frontmatter schema (current):** `apps/editor/src/lib/editor/utils/frontmatter.ts`
- **ADR-001 (storage model — load-bearing):** `docs/adr/001-markdown-as-source-of-truth.md`

---

## 12. Open conflicts and follow-ups

These are the gaps surfaced while writing this document. They are listed for the next sprint, not silently absorbed.

1. **Frontmatter `type` enum — RESOLVED.** `frontmatter.ts` now carries the 11 canonical types from WRITING_FRAMEWORK: `adr | how-to | runbook | readme | api-reference | explanation | issue | platform | infrastructure | network | logging`. `FrontmatterPanel.svelte`'s `TYPE_OPTIONS` still needs alignment — follow-up for staff-engineer.

2. **Frontmatter `status` enum — RESOLVED.** Canonical lifecycle is `draft → review → published → deprecated → superseded` (last is ADR-only). `approved` and `archived` are removed. `accepted` is an ADR-only alias that the linter normalizes to `published`. Schema in `frontmatter.ts` has been updated. `FrontmatterPanel.svelte` still needs its `STATUS_OPTIONS` list aligned — follow-up for staff-engineer.

3. **Templates don't yet exist as components.** § 4 declares ten templates; only the doc-edit *route* exists today. Action: create `apps/editor/src/lib/templates/` and a `TemplateRouter.svelte` that picks the template by `type`. Owner: staff-engineer, blocks Phase 3.

4. **No `Cmd+K` global palette.** § 1.3 declares it; only the sidebar `SearchBar` exists today. Action: lift `SearchBar` into a `CommandPalette` primitive that wraps it. Owner: creative-technologist.

5. **Token-usage linter does not exist.** § 9 declares `vedox/no-raw-color` and `vedox/no-raw-length` stylelint rules. They have to be written. Owner: staff-engineer.

6. **`vedox lint` CLI command does not exist.** § 9 declares it. The rule set is small (frontmatter `type` valid, alt text present, headings linear, mermaid-only diagrams). Owner: staff-engineer.

7. **Skip link — RESOLVED.** A visually-hidden "Skip to main content" link has been added as the first focusable element in `+layout.svelte`, pointing at `#main-content`. WCAG 2.2 AA bypass-blocks requirement satisfied.

8. **Test colocation violation — MIGRATION TASK.** The directory `apps/editor/src/lib/editor/__tests__/` violates § 3.3 (tests must live next to the source they cover). This is a Phase 1 holdover. **Do not move these files as part of Phase 2 feature work** — the frontmatter golden-file tests are load-bearing for the serialize/parse contract and a move touches import paths across the editor package. Action: tracked as a standalone migration task; owner staff-engineer; not blocking any phase-gate.

9. **`prefers-reduced-motion` — RESOLVED.** `EmptyState.svelte` (entrance animation, spinner, CTA transitions) and `+layout.svelte` (empty-state CTA transitions) now carry `@media (prefers-reduced-motion: reduce)` guards. `StatusList.svelte` already had its guard in place. Any new component with a `transition` or `@keyframes` rule must include the guard before it ships.

10. **`SHADOW-LG` is forbidden by § 2.2 but is not asserted anywhere.** Action: add to `vedox/no-raw-shadow` stylelint rule once the rule exists.

---

*End of DESIGN_FRAMEWORK.md. Read it. Apply it. Ship within it.*
