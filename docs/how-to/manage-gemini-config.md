---
title: "How to Manage Gemini Config from Vedox"
type: how-to
status: published
date: 2026-04-09
project: "vedox"
tags: ["gemini", "provider-drawer", "config", "mcp", "placeholder"]
author: "Vedox Team"
difficulty: "beginner"
estimated_time_minutes: 5
prerequisites:
  - "Gemini adapter has shipped (Phase 3 — not yet released)"
  - "vedox dev is running and the UI is open at http://127.0.0.1:3001"
  - "A project is registered in Vedox and selected"
---

# How to Manage Gemini Config from Vedox

Edit `GEMINI.md` and Gemini's MCP server list for one project from the Vedox UI.

> **This document is a placeholder.** The Gemini adapter is shipping in Phase 3 of the provider drawer epic and is not yet merged. The shape below is the intended UX so this guide is ready to fill in when the adapter lands. Do not follow these steps yet — there is nothing to follow.

## Prerequisites

- Gemini adapter has shipped (Phase 3 — not yet released)
- `vedox dev` is running and the UI is open at http://127.0.0.1:3001
- A project is registered in Vedox and selected

## Steps

1. **Open the drawer.** Click the gear icon on the project page. When the Gemini adapter is loaded, the drawer header will indicate Gemini alongside Claude Code, and a colored left-border accent will mark which tab belongs to which provider.

2. **Edit `GEMINI.md` on the Memory tab.** The tab loads `<project>/GEMINI.md` into the editor. Type your changes and click **Save**. The file is written atomically with the same etag conflict check used for Claude.

3. **Add an MCP server on the MCP tab.** Click **Add server**. Enter a name and the server spec (command + args or URL). Click **Save**. Vedox merges the entry into `<project>/.gemini/settings.json` under the `mcpServers` key, leaving other keys untouched.

> Permissions, hooks, sandbox, and extensions configuration in `.gemini/settings.json` are out of scope for the initial Gemini adapter. Edit those keys by hand until a later phase covers them.

## Verification

```sh
cat <project>/GEMINI.md
cat <project>/.gemini/settings.json
```

Both files match what the drawer shows.

## Troubleshooting

To be filled in when the adapter ships. The conflict UX, missing-directory handling, and "config changed outside Vedox" banner behave the same as the Claude Code drawer; see [How to Manage Claude Code Config from Vedox](manage-claude-code-config.md) for the patterns.

## See Also

- [Provider drawer architecture](../explanation/provider-drawer-architecture.md)
- [How to Manage Claude Code Config from Vedox](manage-claude-code-config.md)
- [Provider drawer conflict](../runbooks/provider-drawer-conflict.md)
