---
title: "Why five themes?"
type: explanation
status: published
date: 2026-04-11
project: "vedox"
tags: ["design-system", "themes", "color", "flagship-ux"]
author: "Vedox Team"
---

# Why five themes?

Vedox ships five curated themes: Graphite, Eclipse, Ember, Paper, and
Solar. Three dark, two light. It does not auto-switch based on your
system preference. This document explains the reasoning.

If you just want to pick one, jump to [How to pick one](#how-to-pick-one).

---

## Why not auto dark/light?

The obvious choice for a modern editor is to honor the
`prefers-color-scheme` media query and flip between a dark theme and a
light theme when the OS tells it to. Vedox does not do this. The
decision is deliberate.

`prefers-color-scheme` assumes two things that are not true for a
writing tool:

1. **That every user wants exactly one dark theme and one light theme.**
   In practice, a writer who uses Vedox at 09:00 on a bright morning
   and again at 23:00 in a dark room does not want "dark" as a single
   opinion. They want a specific dark — a warm one for late nights, a
   cool one for focus work, a true-black one on an OLED display. OS
   dark mode collapses three useful choices into one.
2. **That the OS preference is the user's current intent.** It isn't.
   Mac defaults to syncing with sunrise/sunset. Linux DEs scatter the
   preference across three or four config layers. On a browser tab the
   preference is further mediated by the browser. When a writer flips
   their laptop open at 14:00, the "dark or light" answer is a
   coincidence, not a decision.

There is a second, harder reason. Three of Vedox's themes (Graphite,
Eclipse, Ember) are all dark, and a user who picks Ember for its warm
terracotta accent has made a choice the OS cannot represent. If Vedox
honored `prefers-color-scheme` and the user then toggled their OS to
light, the app would have to guess: drop to Paper (cool) or drop to
Solar (warm)? Both are wrong for different reasons. The only correct
answer is: ask the user, remember the answer, never override it.

So Vedox picks Graphite on first run and persists whatever you choose
from there. Explicit beats clever.

---

## Why OKLCH?

Every color in Vedox is authored in OKLCH — a perceptually uniform
color space with three axes: Lightness (0-100%), Chroma (saturation),
and Hue (angle in degrees). The reason matters.

In HSL, "5% lighter blue" and "5% lighter yellow" do not look like the
same amount of change. Blue gets noticeably lighter; yellow barely
shifts. That means any hover state (`+4% L`), any disabled state
(`-10% C`), or any surface elevation ladder produces visually
inconsistent results depending on the hue you started from. A design
system built on HSL has to hand-tune every interaction for every
accent color.

In OKLCH, a 5% change in L looks like a 5% change in L — across red,
blue, yellow, green, everything. That means one rule covers all five
themes:

- Hover = `+6% L` on the accent
- Disabled = `-10% C`
- Surface 2 sits exactly one perceptual step above surface 1, in any
  theme

The contrast math also stays honest. Body text in Graphite
(`oklch(78% 0.008 265)` on `oklch(14% 0.01 265)`) hits 9.6:1 contrast.
Body text in Paper (`oklch(34% 0.012 265)` on `oklch(98.5% 0.005 85)`)
hits a similarly safe ratio. The ladder is built once, not re-verified
per theme.

This is why switching themes in Vedox is literally a single attribute
swap on `<html>`. No JavaScript recalculation. No per-theme shims. The
Layer 1 token values change; every higher layer resolves correctly on
its own.

---

## What each theme is for

The five themes are not cosmetic skins. Each one is tuned for a
specific use case.

### Graphite — the default

Dark, cool, neutral charcoal with an indigo accent
(`oklch(62% 0.18 265)`). The canvas is not pure black; it is a
slightly blue-cool dark gray. This leaves headroom for a sunken
surface below it (`--surface-1` is darker than `--surface-0`) and
avoids the OLED burn-in bloom that pure black creates around bright
text.

Pick Graphite when: you are not sure. It is the default for a reason.
It reads well at most indoor lighting levels, its accent is cool
enough to stay out of the way, and the contrast ladder is the most
conservative of the three dark themes.

### Eclipse — OLED dark

True-black canvas (`oklch(4% 0.003 290)`) with a violet accent
(`oklch(68% 0.20 290)`). The blacks are actually black, which means
pixels are fully off on an OLED display. Every other UI surface is
layered just above black and feels elevated.

Pick Eclipse when: you are on an OLED laptop or external display and
you want the battery and contrast advantage of pure black. The violet
accent is brighter than Graphite's indigo, which compensates for the
deeper canvas — subtle accents would disappear against true black.

### Ember — late night

Warm near-black with a terracotta accent (`oklch(70% 0.17 45)`). The
surface ladder is hue-shifted toward warm (hue angle 40 degrees), so
the entire UI has a subtle amber undertone instead of Graphite's blue
undertone.

Pick Ember when: you are writing after 22:00, you find cool-toned dark
UIs sterile, or your OS is already set to a warm display profile
(f.lux, Night Shift, Redshift). Warm-on-warm feels coherent; cool
Graphite on a warmed display looks flat and slightly sick.

### Paper — editorial reading

Warm off-white canvas (`oklch(98.5% 0.005 85)`) with a cool indigo
accent (`oklch(52% 0.16 265)`). The canvas has a tiny hint of cream
(hue 85), not stark white. Borders are hairline gray. Text is dark
indigo, not pure black.

Pick Paper when: you are reading long-form content, you are writing in
a bright room, or you want the Vedox editor to feel like a page rather
than an app. Paper is the only theme tuned for extended reading
sessions over extended writing sessions — the contrast is deliberately
softer than pure black-on-white.

### Solar — warm light

Cream canvas (`oklch(97% 0.020 90)`) with an amber accent
(`oklch(60% 0.17 75)`). Solarized-inspired. The light themes are
mirrored: Paper is cool on warm cream; Solar is warm on warmer cream.

Pick Solar when: you are outdoors in daylight, you find Paper too
contrasty, or you have a blue-light-filter workflow already set up
system-wide. Solar's lower contrast is easier on eyes adapted to a
warm screen.

---

## How to pick one

A quick decision tree.

1. **Do you know you want dark or light?** If no, stop. Use Graphite.
2. **If dark:**
   - On an OLED display? → **Eclipse.**
   - Writing after dark or warm display profile on? → **Ember.**
   - Otherwise → **Graphite.**
3. **If light:**
   - Reading long prose? → **Paper.**
   - Writing in daylight or with a warm color profile? → **Solar.**

The theme picker lives in Settings or the command palette. Type
`Cmd+K`, then `>theme` to see all five options. Switching is instant
and reversible.

See [Customize Appearance](../how-to/customize-appearance.md) for the
concrete steps, or [Design System](./design-system.md) for the token
architecture that makes five themes cost one theme's worth of
complexity.
