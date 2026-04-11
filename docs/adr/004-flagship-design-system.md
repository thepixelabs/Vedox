---
title: "ADR-004: Flagship Design System — OKLCH Tokens, Named Themes, Variable Fonts"
type: adr
status: accepted
date: 2026-04-10
project: "vedox"
tags: ["design-system", "theming", "oklch", "typography", "density", "css"]
author: "Vedox Team"
superseded_by: ""
---

## Context

Vedox's original styling used ad-hoc CSS variables (`--color-surface-base`, `--color-text-primary`) with hardcoded hex values. The system supported only a binary dark/light toggle. As the product matured, this caused:

1. **Inconsistent contrast ratios across surfaces.** Some text-on-surface pairings were barely readable because hex values were chosen by eye without a perceptual model.
2. **Theme sprawl.** Adding a new theme meant finding and updating 30+ scattered variables. There was no separation between raw colour values and semantic usage.
3. **No density system.** Spacing was hardcoded per-component. Making the UI denser or more spacious required touching every component individually.
4. **Font fallback chains were wrong.** The Phase 1 font stacks caused measurable layout shift on slow connections because the fallback metrics did not match the primary face.

## Decision

Adopt a 4-layer OKLCH token architecture, five curated themes, a density multiplier, and variable fonts with metric-override fallbacks.

### Layer 1 — Raw ramps (per-theme OKLCH values)

Each theme defines a surface ladder (`surface-0` through `surface-4` plus `surface-code`), a text ladder (`text-1` through `text-4` plus `text-inverse`), accent tokens (`accent-solid`, `accent-solid-hover`, `accent-text`, `accent-subtle`, `accent-border`, `accent-contrast`), and border tokens (`border-hairline`, `border-default`, `border-strong`). All values are authored in OKLCH.

The default theme (Graphite) lives in `:root` in `tokens.css`. The four alternatives override only the tokens that change, via `[data-theme="..."]` selectors in `themes.css`.

### Layer 2 — Semantic tokens

Components reference semantic names (`--surface-2`, `--text-1`, `--accent-solid`) rather than raw colour values. This indirection means a theme swap is a single attribute change on `<html>` — no JavaScript rerenders, no class toggles on individual components.

### Layer 3 — Component tokens (density, typography, spacing)

A single `--density` CSS custom property multiplies all density-aware spacing. Three modes:

| Mode | Multiplier | Selector |
|---|---|---|
| Compact | 0.875 | `[data-density="compact"]` |
| Comfortable | 1.000 (default) | `[data-density="comfortable"]` |
| Cozy | 1.125 | `[data-density="cozy"]` |

Typography uses a minor-third scale (1.200 ratio). Fixed UI text (`--text-2xs` through `--text-base`) stays at stable pixel values. Display steps (`--text-lg` through `--text-5xl`) use `clamp()` for viewport-responsive scaling between 375px and 1280px.

Spacing follows a 4px base with 13 steps (`--space-0` through `--space-12`).

### Layer 4 — Code syntax tokens

Code syntax colours are derived from the active accent. Keywords and types re-hue when the theme changes. Strings (`oklch(74% 0.14 145)`) and numbers (`oklch(78% 0.13 65)`) use fixed OKLCH values so the "string is green, number is gold" mental model survives across themes.

### Five curated themes

Three dark-first, two light. Each has a deliberate personality and a hand-picked accent.

| # | Name | Family | Accent | Canvas |
|---|---|---|---|---|
| 1 | **Graphite** (default) | Dark | Indigo `oklch(62% 0.18 265)` | Cool charcoal |
| 2 | **Eclipse** | Dark | Violet `oklch(68% 0.20 320)` | True-black (OLED) |
| 3 | **Ember** | Dark | Terracotta `oklch(70% 0.17 45)` | Warm near-black |
| 4 | **Paper** | Light | Cool indigo | Warm off-white |
| 5 | **Solar** | Light | Amber | Solarized-inspired cream |

Theme selection is explicit and user-driven. The system never honours `prefers-color-scheme` automatically — users choose a named theme via Settings or the command palette.

### Variable fonts

| Role | Font | Format | Fallback strategy |
|---|---|---|---|
| Body | Geist Sans Variable | Self-hosted woff2 (~85KB) | System stack with metric overrides |
| Display | Fraunces Variable | Self-hosted woff2 | Source Serif 4 / system serif with metric overrides |
| Code | JetBrains Mono Variable | Self-hosted woff2 (~110KB) | Commit Mono / system mono with metric overrides |

Metric-override `@font-face` declarations ensure fallback faces match the ascent, descent, and line-gap of the primary — eliminating layout shift during font loading.

### Backwards compatibility

Legacy aliases (`--color-surface-base`, `--color-text-primary`, `--color-accent`, etc.) map to the new semantic tokens via `var()` references. They are declared at the bottom of `tokens.css` and will be removed once every call-site is migrated.

The theme store (`$lib/theme/store.ts`) accepts the legacy `"dark"` / `"light"` strings and normalises them to `"graphite"` / `"paper"`, so existing components that call `setTheme("dark")` continue to work.

## Consequences

**Positive:**

- New components reference semantic tokens only — never raw hex values. This is enforced by convention and lint.
- Adding a 6th theme requires only a new `[data-theme="name"]` block in `themes.css` and a new entry in the `ALL_THEMES` array in `store.ts`.
- Density changes cascade automatically through `calc(... * var(--density))` — no per-component overrides needed.
- OKLCH values produce perceptually uniform lightness steps. Swapping themes preserves contrast ratios without manual per-surface tuning.
- Variable fonts reduce HTTP requests from 8 (4 weights x 2 styles in static fonts) to 3 (one file per family).

**Negative:**

- Browser support floor rises to Safari 15.4+ / Chrome 111+ for OKLCH. Not a concern for Electron but blocks users on older Safari. Accepted because the target persona (staff engineer, 4K monitor, modern browser) is not on Safari 14.
- Token count grows from ~35 to ~140. Navigating the token file requires the section headers and layered organisation to stay clear. If the file becomes disorganised, the benefit of the system degrades.
- Legacy aliases add a maintenance burden during the migration period. Every legacy alias is a `var()` indirection that slows CSS debugging slightly.

**Neutral / follow-on work:**

- A lint rule or stylelint plugin should warn on raw colour values in component CSS to enforce the "semantic tokens only" contract.
- Phase 2 will migrate the spacing scale steps (`--space-7` and `--space-8`) from their legacy Phase 1 values to the flagship target values (32px and 40px respectively).
- The "Bring your own mono font" slot in appearance settings (for users who own a Berkeley Mono license) is a post-v1 feature.

## Alternatives Considered

### HSL over OKLCH

Use HSL for colour tokens instead of OKLCH.

Rejected because HSL's hue channel produces uneven perceived brightness. A surface ladder authored in HSL requires manual per-step lightness compensation to look uniform. OKLCH's perceptual lightness channel (`L`) eliminates this — equal `L` steps produce equal perceived brightness, which is the entire point of a systematic surface ladder.

### `prefers-color-scheme` auto-detection

Automatically select dark or light theme based on the OS setting.

Rejected because it forces a binary dark/light choice, losing Eclipse, Ember, and Solar entirely. The curated five-theme system is a deliberate product decision — users pick their theme explicitly. A `prefers-color-scheme` check could be added later to set the *initial* default (Graphite vs Paper) for first-time users, but it must never override an explicit choice.

### Static fonts over variable

Ship static woff2 files per weight (Regular, Medium, Semibold, Bold) instead of variable woff2.

Rejected because static fonts require 4 files per family (minimum), producing 8-12 HTTP requests for the full font stack. Variable fonts collapse each family to a single file, reducing requests to 3. The bundle size difference is marginal (variable files are ~10-15% larger per file), but the request count reduction matters for cold load performance.

### Tailwind utility classes

Replace the custom property token system with Tailwind's utility-first approach.

Rejected because the existing CSS architecture is token-based custom properties consumed by component-scoped stylesheets. Migrating to Tailwind would require rewriting every component's styles and would trade a coherent 4-layer token system for a flat utility namespace with no semantic layer. The token architecture is the design system; Tailwind is a different design system philosophy.
