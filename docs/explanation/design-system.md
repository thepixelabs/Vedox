---
title: "Design System"
type: explanation
status: published
date: 2026-04-10
project: "vedox"
tags: ["design-system", "tokens", "color", "typography", "motion", "density"]
author: "tech-writer"
---

# Design System

Vedox uses a token-based design system authored in the OKLCH color space. Every visual decision -- color, spacing, type size, motion timing -- is expressed as a CSS custom property (token) that components consume. No component ever hard-codes a hex color, pixel value, or duration.

This document describes the system as implemented in `apps/editor/src/styles/`. It covers the token architecture, color system, typography stack, density modes, motion primitives, spacing scale, and z-index layers.

---

## Token architecture

The token system has four layers. Each layer references only the layer above it, never skipping. This makes theme switches a single-layer swap.

| Layer | Name | What it holds | Example |
|---|---|---|---|
| 1 | Raw ramps | Per-theme OKLCH values for surfaces, text, accents, borders | `--surface-0: oklch(14% 0.01 265)` |
| 2 | Semantic tokens | Named roles that components reference | `--accent-solid`, `--text-2`, `--border-default` |
| 3 | Component tokens | Scoped aliases inside specific components | Toolbar background, sidebar width |
| 4 | Code syntax tokens | Syntax highlighting colors derived from the active accent | `--code-keyword`, `--code-string` |

When a user switches themes, only Layer 1 values change. Layers 2-4 are indirections (`var(--surface-0)`, `var(--accent-solid)`) that resolve to the new Layer 1 values automatically. This is why theme switching is a single `data-theme` attribute swap on the root element with no JavaScript recalculation.

Token definitions live in:
- `apps/editor/src/styles/tokens.css` -- spacing, typography, motion, z-index, and the default Graphite theme
- `apps/editor/src/styles/themes.css` -- the four alternative themes (Eclipse, Ember, Paper, Solar)

---

## Color space: OKLCH

All colors are authored in **OKLCH** (`oklch(L C H / A)`), a perceptually uniform color space. The three axes are:

- **L** (Lightness) -- 0% is black, 100% is white. Unlike HSL, a 5% lightness increase *looks* like a 5% increase regardless of hue.
- **C** (Chroma) -- saturation intensity. 0 is fully desaturated (gray). Higher values are more vivid.
- **H** (Hue) -- the hue angle in degrees. 0 = red, 90 = yellow, 145 = green, 265 = indigo, 305 = fuchsia.
- **A** (Alpha) -- optional opacity, written after a `/` separator.

Perceptual uniformity matters because it means hover states (`+4% L`), disabled states (`-10% C`), and surface ladders produce visually consistent results across every hue. In HSL, "5% lighter blue" and "5% lighter yellow" look like different amounts of change. In OKLCH, they look the same.

### Surface ladder

Five surface levels define elevation in the UI. In dark themes, higher surfaces are lighter. In light themes, the relationship inverts subtly.

| Token | Role | Graphite (dark) | Paper (light) |
|---|---|---|---|
| `--surface-0` | Page canvas (deepest) | `oklch(14% 0.01 265)` | `oklch(98.5% 0.005 85)` |
| `--surface-1` | Editor well / sunken | `oklch(11% 0.01 265)` | `oklch(96% 0.008 85)` |
| `--surface-2` | Default panels | `oklch(16% 0.01 265)` | `oklch(100% 0 0)` |
| `--surface-3` | Sidebar / toolbar | `oklch(19% 0.012 265)` | `oklch(98% 0.004 85)` |
| `--surface-4` | Modals / popovers | `oklch(22% 0.014 265)` | `oklch(100% 0 0)` |
| `--surface-code` | Code block background | `oklch(11% 0.01 265)` | `oklch(96% 0.008 85)` |

The dark canvas is not black. It is a slightly blue-cool charcoal. Pure black is jarring on OLED displays and leaves no room for a sunken level below it.

### Text ladder

Five text levels create typographic hierarchy without relying on font weight alone.

| Token | Role | Graphite (dark) | Contrast on canvas |
|---|---|---|---|
| `--text-1` | Headings, emphasis | `oklch(96% 0.005 265)` | 17.4:1 (AAA) |
| `--text-2` | Body copy (default) | `oklch(78% 0.008 265)` | 9.6:1 (AAA) |
| `--text-3` | Captions, muted | `oklch(60% 0.010 265)` | 5.2:1 (AA) |
| `--text-4` | Placeholders, gutters | `oklch(45% 0.010 265)` | 3.1:1 (AA large) |
| `--text-inverse` | Text on accent fills | `oklch(15% 0.01 265)` | -- |

Body text uses `--text-2` (78% lightness), not `--text-1`. Pure white body text on dark backgrounds causes eye fatigue. Reserving `--text-1` for headings and focused inputs creates natural hierarchy.

### Border ladder

Three border levels. All hairline-thin (1px). Borders define edges without claiming attention.

| Token | Graphite (dark) |
|---|---|
| `--border-hairline` | `oklch(28% 0.010 265)` |
| `--border-default` | `oklch(34% 0.012 265)` |
| `--border-strong` | `oklch(46% 0.014 265)` |

`box-shadow` for elevation is forbidden outside overlay contexts. Borders only.

### Accent tokens

Each theme defines a complete accent family:

| Token | Purpose |
|---|---|
| `--accent-solid` | Primary accent (buttons, active states) |
| `--accent-solid-hover` | Hover state (+6% L typically) |
| `--accent-text` | Accent-colored text (links, labels) |
| `--accent-subtle` | Translucent wash (hover backgrounds, badges) |
| `--accent-border` | Accent-tinted borders (focus rings, active tabs) |
| `--accent-contrast` | Text on accent-solid fills |

### Status colors

| Token | OKLCH (dark) | Use |
|---|---|---|
| `--success` | `oklch(74% 0.16 162)` | Saved, published, passing |
| `--warning` | `oklch(80% 0.14 80)` | Stale content, lint warnings |
| `--error` | `oklch(70% 0.18 25)` | Failed operations, validation |
| `--info` | `oklch(72% 0.14 230)` | Notices, tips |

Status colors are deliberately desaturated compared to typical red/green palettes. They sit alongside the accent without competing for attention.

---

## Themes

Vedox ships five curated themes. Three are dark-first, two are light. Switching themes sets a `data-theme` attribute on the document root; only Layer 1 token values change.

| Theme | Family | Accent | Character |
|---|---|---|---|
| **Graphite** (default) | Dark | Indigo `oklch(62% 0.18 265)` | Cool charcoal. The flagship default. |
| **Eclipse** | Dark | Violet `oklch(68% 0.20 290)` | True-black OLED canvas. Deepest darks in the set. |
| **Ember** | Dark | Terracotta `oklch(70% 0.17 45)` | Warm near-black. Late-night writing. |
| **Paper** | Light | Cool indigo `oklch(52% 0.16 265)` | Warm off-white. Editorial / print feel. |
| **Solar** | Light | Amber `oklch(60% 0.17 75)` | Solarized-inspired cream canvas. |

### How theme switching works

1. The user picks a theme (Settings page or [command palette](../explanation/command-palette.md)).
2. The theme store writes the name to `localStorage` under key `vedox:theme`.
3. The store sets `document.documentElement.setAttribute("data-theme", name)`.
4. CSS selectors in `themes.css` (`[data-theme="eclipse"]`, etc.) override the Layer 1 tokens.
5. A brief `.theme-transition` class is added to enable a 180ms color crossfade, then removed on the next animation frame.

The default theme (Graphite) is declared at `:root` in `tokens.css`. Users with JavaScript disabled or no stored preference get Graphite automatically.

Vedox does **not** honor `prefers-color-scheme` automatically. Theme choice is always explicit.

### Code syntax tokens

Syntax highlighting colors are derived from the active accent. Fixed scaffold colors (strings, numbers, functions) stay stable across themes so your eye learns the mapping. Only `keyword` and `type` re-tint with the accent.

| Token | Binding | Stable across themes? |
|---|---|---|
| `--code-keyword` | `var(--accent-solid)` | No -- follows accent |
| `--code-type` | Complementary to accent hue | No -- shifts per theme |
| `--code-string` | `oklch(74% 0.14 145)` (green) | Yes |
| `--code-number` | `oklch(78% 0.13 65)` (warm yellow) | Yes |
| `--code-function` | `oklch(76% 0.13 220)` (cyan) | Yes |
| `--code-comment` | `var(--text-3)` | Theme-aware (muted) |
| `--code-variable` | `var(--text-1)` | Theme-aware (primary) |
| `--code-operator` | `var(--text-2)` | Theme-aware (secondary) |
| `--code-punct` | `var(--text-3)` | Theme-aware (muted) |

---

## Typography

Three typeface families, all open-source and self-hosted via fontsource packages. No external CDN requests.

| Role | Face | License | Package |
|---|---|---|---|
| Body / UI | Geist Variable | OFL | `@fontsource-variable/geist` |
| Display | Fraunces Variable | OFL | `@fontsource-variable/fraunces` |
| Monospace (primary) | JetBrains Mono Variable | OFL | `@fontsource-variable/jetbrains-mono` |
| Monospace (fallback) | Commit Mono | OFL | `@fontsource/commit-mono` |

**Display (Fraunces)** appears only on prose `h1` headings and `h2` subheadings. This rarity gives it editorial weight. Reference and non-prose templates use Geist for all headings.

**Body (Geist Sans)** is the workhorse. Buttons, sidebar items, body text, form labels -- everything that is not a display heading or code.

**Monospace (JetBrains Mono)** leads the `--font-mono` stack with Commit Mono as fallback. Ligatures are off in code blocks (`"liga" 0, "calt" 0`). Tabular figures and slashed zeros are on for all numeric surfaces.

### Type scale

The scale uses fluid `clamp()` for display sizes (responsive to viewport width) and fixed px for UI text.

| Token | Size | Use |
|---|---|---|
| `--text-2xs` | 11px | Sidebar labels (uppercase, tracked), tooltips |
| `--text-xs` | 12px | Status chips, breadcrumbs, table headers |
| `--text-sm` | 13px | Compact body, sidebar tree, palette items |
| `--text-base` | 16px | Default body text |
| `--text-lg` | clamp(16px -- 18px) | Cozy body, lead paragraphs, callouts |
| `--text-xl` | clamp(18px -- 22px) | Subheadings (h3), sidebar header |
| `--text-2xl` | clamp(22px -- 28px) | Section headings (h2) |
| `--text-3xl` | clamp(26px -- 34px) | Large headings |
| `--text-4xl` | clamp(32px -- 44px) | Page title (h1) -- Fraunces display |
| `--text-5xl` | clamp(44px -- 72px) | Hero / splash -- Fraunces display |

### Reading measure

Line length is controlled in `ch` units so it adapts to the rendered font.

| Token | Width | Use |
|---|---|---|
| `--measure-narrow` | 64ch | Narrow reading preference |
| `--measure-default` | 68ch | Default reading column |
| `--measure-wide` | 80ch | Wide reading preference, compact density |

### Letter spacing scale

| Token | Value | Use |
|---|---|---|
| `--tracking-tighter` | -0.025em | Display type (44px+) |
| `--tracking-tight` | -0.015em | h2/h3 headings |
| `--tracking-normal` | 0 | Body text |
| `--tracking-wide` | 0.04em | Uppercase section labels |
| `--tracking-wider` | 0.06em | Small caps, badges |
| `--tracking-widest` | 0.10em | Decorative labels |

---

## Density system

Three density modes scale spacing without changing font sizes. The mechanism is a single CSS variable `--density` that components multiply into their padding, margins, and row heights.

| Mode | Multiplier | Character |
|---|---|---|
| **Compact** | 0.875 | Tighter spacing. Power users, small screens. |
| **Comfortable** | 1.0 | The default. Balanced. |
| **Cozy** | 1.125 | Generous spacing. Relaxed reading. |

Density is applied via a `data-density` attribute on the document root:

```css
:root[data-density="compact"]     { --density: 0.875; }
:root[data-density="comfortable"] { --density: 1.000; }
:root[data-density="cozy"]        { --density: 1.125; }
```

Components that scale with density multiply their spacing tokens:

```css
.sidebar-item {
  padding-block: calc(var(--space-2) * var(--density));
}
```

Font sizes stay fixed. Size control is per-component, not global. Density affects whitespace only.

The density preference persists to `localStorage` under key `vedox:density` and is restored before the first paint. See [Customize Appearance](../how-to/customize-appearance.md) for how to change it.

---

## Motion

Motion in Vedox exists to make causation visible. If the user did something and a thing changed, motion shows what was cause and what was effect. Then it stops.

### Durations

| Token | Duration | Use |
|---|---|---|
| `--duration-fast` | 120ms | Color hovers, tooltip fade-in, button press |
| `--duration-default` | 180ms | Theme crossfade, command palette open, sidebar slide |
| `--duration-slow` | 240ms | Toast slide-in, editor mode swap |

No animation exceeds 250ms.

### Easing curves

| Token | Bezier | Use |
|---|---|---|
| `--ease-out` | `cubic-bezier(0.16, 1.00, 0.30, 1.00)` | General hovers, default transitions |
| `--ease-in-out` | `cubic-bezier(0.65, 0.00, 0.35, 1.00)` | Sidebar collapse, modals |
| `--ease-snap` | `cubic-bezier(0.85, 0.00, 0.15, 1.00)` | Signature curve -- confident, deliberate |
| `--ease-spring` | `cubic-bezier(0.20, 0.00, 0.00, 1.00)` | Command palette, slash menu |

`--ease-snap` is the signature easing. It is used for every theme switch, mode toggle, and primary interaction.

No spring physics. Cubic bezier only.

### Reduced motion

The `prefers-reduced-motion: reduce` media query is a universal kill switch. It sets all animation durations to `0.01ms` and disables scroll-behavior animations. Opacity fades are preserved because they are not perceived as motion.

```css
@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after {
    animation-duration: 0.01ms !important;
    animation-iteration-count: 1 !important;
    transition-duration: 0.01ms !important;
    scroll-behavior: auto !important;
  }
}
```

### Interactive element pattern

Every clickable element follows a shared three-state grammar via the `.interactive` class:

- **Rest:** hairline border, surface-2 background, text-2 color
- **Hover:** strong border, 8% accent wash via `color-mix(in oklch)`, text-1 color
- **Focus-visible:** 2px accent outline, 3px offset
- **Active:** `scale(0.985)` press effect at 60ms

The `color-mix` hover ties the hover state to the active theme without per-theme overrides.

---

## Spacing scale

A 4px base unit with 13 steps. Layout uses 8px multiples. Dense components and type use 4px multiples.

| Token | Value |
|---|---|
| `--space-0` | 0 |
| `--space-px` | 1px |
| `--space-1` | 4px |
| `--space-2` | 8px |
| `--space-3` | 12px |
| `--space-4` | 16px |
| `--space-5` | 20px |
| `--space-6` | 24px |
| `--space-7` | 28px |
| `--space-8` | 32px |
| `--space-9` | 48px |
| `--space-10` | 64px |
| `--space-11` | 96px |
| `--space-12` | 128px |

Components compose from this scale only. There is no `padding: 14px` in the codebase.

> **Note:** `--space-7` and `--space-8` are at their Phase 1 legacy values (28px and 32px). The flagship target values (32px and 40px) will be adopted in Phase 2 when call-sites are migrated in lockstep with component updates.

---

## Z-index scale

Eight ordered layers prevent z-index conflicts.

| Token | Value | Use |
|---|---|---|
| `--z-base` | 0 | Default stacking |
| `--z-sticky` | 10 | Sticky toolbar, sticky table headers |
| `--z-dropdown` | 20 | Slash menu, frontmatter type picker |
| `--z-tooltip` | 30 | Tooltips |
| `--z-popover` | 40 | Mermaid preview, link preview |
| `--z-modal` | 50 | Import dialog, settings modal |
| `--z-toast` | 60 | Toast notifications |
| `--z-cmdk` | 70 | Command palette (sits above modals intentionally) |

---

## Border radii

| Token | Value |
|---|---|
| `--radius-sm` | 4px |
| `--radius-md` | 6px |
| `--radius-lg` | 8px |
| `--radius-xl` | 12px |
| `--radius-2xl` | 16px |
| `--radius-full` | 9999px |

The command palette uses `--radius-xl` (12px). It is the only place radius exceeds 10px. Most interactive elements use `--radius-md` (6px).
