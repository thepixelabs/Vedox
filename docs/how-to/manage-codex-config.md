---
title: "How to Manage Codex Config from Vedox"
type: how-to
status: published
date: 2026-04-09
project: "vedox"
tags: ["codex", "provider-drawer", "config", "global-config", "placeholder"]
author: "Vedox Team"
difficulty: "intermediate"
estimated_time_minutes: 5
prerequisites:
  - "Codex adapter has shipped (Phase 3 — not yet released)"
  - "vedox dev is running and the UI is open at http://127.0.0.1:3001"
  - "You understand that Codex config is global, not per-project"
---

# How to Manage Codex Config from Vedox

Edit Codex's MCP server list and approval mode from the Vedox provider drawer.

> **This document is a placeholder.** The Codex adapter is shipping in Phase 3 of the provider drawer epic and is not yet merged. The shape below is the intended UX so this guide is ready to fill in when the adapter lands. Do not follow these steps yet — there is nothing to follow.

> **Read this before you use the Codex tab.** Codex stores its config in a single global file: `~/.codex/config.toml`. Codex has no per-project config file. When you edit the Codex tab from inside one Vedox project, you are editing the same file that every other Vedox project — and every shell you run `codex` in — reads from. The Vedox drawer shows the Codex tab inside a per-project drawer for layout reasons only; the underlying file is global. There is no way to scope Codex settings to one project from this UI. If you change the approval mode or add an MCP server here, the change is system-wide. Vedox will display a banner inside the Codex tab making this explicit.

## Prerequisites

- Codex adapter has shipped (Phase 3 — not yet released)
- `vedox dev` is running and the UI is open at http://127.0.0.1:3001
- You understand that Codex config is global, not per-project

## Steps

1. **Open the drawer.** Click the gear icon on the project page. Switch to the Codex tab. The "global config" banner appears at the top of the tab.

2. **Add an MCP server.** Click **Add server**. Enter a name and the server spec. Click **Save**. Vedox writes to `~/.codex/config.toml` under the `[mcp_servers]` table, preserving other tables and keys.

3. **Set the approval mode.** Pick `suggest`, `auto-edit`, or `full-auto` from the dropdown. Click **Save**. Vedox updates the `approval_mode` key at the top level of `~/.codex/config.toml`.

> The Codex sandbox configuration is out of scope for the initial Codex adapter. Edit the `[sandbox]` table by hand until a later phase covers it.

## Verification

```sh
cat ~/.codex/config.toml
```

The file matches what the drawer shows. Confirm by running `codex` from a separate project — the new MCP server is available there too, because the file is global.

## Troubleshooting

To be filled in when the adapter ships. Conflict handling on `~/.codex/config.toml` needs special care because two Vedox windows pointed at two different projects can both write to it; the etag check still applies, but the "Config changed outside Vedox" banner may fire because of edits made from another Vedox window.

## See Also

- [Provider drawer architecture](../explanation/provider-drawer-architecture.md)
- [How to Manage Claude Code Config from Vedox](manage-claude-code-config.md)
- [Provider drawer conflict](../runbooks/provider-drawer-conflict.md)
