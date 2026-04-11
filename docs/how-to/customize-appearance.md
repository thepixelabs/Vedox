---
title: "Customize Appearance"
type: how-to
status: published
date: 2026-04-10
project: "vedox"
tags: ["themes", "density", "appearance", "settings", "customization"]
author: "tech-writer"
---

# Customize Appearance

Vedox ships five themes and three density modes. All preferences persist to `localStorage` and are restored before the first paint, so there is no flash of wrong theme on reload.

---

## Switch themes

Vedox has five curated themes. Three dark, two light.

| Theme | Family | Accent color |
|---|---|---|
| Graphite (default) | Dark | Indigo |
| Eclipse | Dark | Violet (OLED-black canvas) |
| Ember | Dark | Terracotta (warm) |
| Paper | Light | Cool indigo (warm off-white canvas) |
| Solar | Light | Amber (cream canvas) |

### Via the command palette

1. Press `Cmd+K` to open the command palette.
2. Type `>` to enter command mode.
3. Type `theme` to filter the theme commands.
4. Select the theme you want (e.g., "Theme: Eclipse") and press Enter.

The theme applies immediately.

### Via the Settings page

1. Navigate to Settings (`Cmd+K` then `> Open Settings`, or click the settings cog in the sidebar dock).
2. Select your preferred theme from the theme picker.

### Via the sidebar dock

The sidebar bottom dock includes a theme toggle button. Clicking it switches between your current theme and its dark/light partner:

- Graphite toggles to Paper (and back)
- Eclipse toggles to Solar (and back)
- Ember toggles to Paper (and back)

For the full theme picker with all five options, use Settings or the command palette.

---

## Change density

Density controls how tightly the UI is spaced. Font sizes stay the same; only padding, margins, and row heights change.

| Mode | Multiplier | Best for |
|---|---|---|
| Compact | 0.875x | Power users, small screens, seeing more content |
| Comfortable | 1.0x (default) | General use |
| Cozy | 1.125x | Relaxed reading, presentations, large displays |

### Via the command palette

1. Press `Cmd+K`.
2. Type `>density` to filter density commands.
3. Select the mode you want and press Enter.

### Via the Settings page

Navigate to Settings and choose your density preference.

---

## Change reading width

Each pane has an independent reading width that controls the maximum line length of prose content.

| Width | Token | Approximate width |
|---|---|---|
| Narrow | `--measure-narrow` | 64ch |
| Default | `--measure-default` | 68ch |
| Wide | `--measure-wide` | 80ch |

### Cycle with keyboard

Press `Cmd+Shift+L` to cycle through narrow, default, and wide in the active pane.

### Via the editor toolbar

The reading width toggle in the editor toolbar switches between the three widths. The active width is indicated visually.

---

## How preferences persist

All appearance preferences are stored in `localStorage`:

| Key | Values | Default |
|---|---|---|
| `vedox:theme` | `graphite`, `eclipse`, `ember`, `paper`, `solar` | `graphite` |
| `vedox:density` | `compact`, `comfortable`, `cozy` | `comfortable` |

Preferences are read and applied to the DOM **before the first paint** via a pre-hydration inline script in `app.html`. This prevents the flash of default theme that plagues many web applications.

Legacy values are handled gracefully. If `vedox:theme` contains `"dark"`, it is migrated to `"graphite"`. If it contains `"light"`, it is migrated to `"paper"`.

---

## Related

- [Design System](../explanation/design-system.md) -- token architecture, color space, and theme internals
- [Command Palette](../explanation/command-palette.md) -- full list of built-in commands
- [Keyboard Shortcuts](./keyboard-shortcuts.md) -- all shortcuts including appearance toggles
