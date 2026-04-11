---
title: "How to Manage Claude Code Config from Vedox"
type: how-to
status: published
date: 2026-04-09
project: "vedox"
tags: ["claude-code", "provider-drawer", "config", "mcp", "agents"]
author: "Vedox Team"
difficulty: "beginner"
estimated_time_minutes: 10
prerequisites:
  - "vedox dev is running and the UI is open at http://127.0.0.1:3001"
  - "A project is registered in Vedox and selected"
  - "The project is a git repository"
  - "Either a .claude/ directory exists at the project root or you accept that Vedox will create one"
---

# How to Manage Claude Code Config from Vedox

Edit `CLAUDE.md`, permission rules, MCP servers, and subagents for one project from the Vedox UI instead of hand-editing files in `.claude/`.

## Prerequisites

- `vedox dev` is running and the UI is open at http://127.0.0.1:3001
- A project is registered in Vedox and selected
- The project is a git repository
- Either a `.claude/` directory exists at the project root or you accept that Vedox will create one

Use the drawer when you want a structured editor and conflict-safe writes. Edit the files by hand when you need comments in `.claude/settings.json` (see Troubleshooting) or when you are managing a config surface the drawer does not yet cover (hooks, skills, output styles).

## Steps

1. **Open the drawer.** On the project page, click the gear icon in the project header. A 520-pixel right-anchored drawer opens with four tabs: **Memory**, **Permissions**, **MCP**, **Agents**.

2. **Edit `CLAUDE.md` on the Memory tab.** The tab loads the file's contents into a CodeMirror editor. Type your changes. The tab title shows a dirty dot while there are unsaved edits. Click **Save**. The dot clears and the file is written atomically to `<project>/CLAUDE.md`.

3. **Add permission rules on the Permissions tab.** Click **Add allow rule** and enter a tool pattern (for example, `Bash(git status:*)`). Click **Add deny rule** and enter `Bash(rm -rf:*)`. Click **Save**. Vedox writes only the `permissions` key in `.claude/settings.json`; every other top-level key is preserved.

4. **Add an MCP server on the MCP tab.** Click **Add server**. Enter a name, then either a command and args (for a local stdio server) or a URL (for an HTTP server). Click **Save**. Vedox merges the entry into `.mcp.json` at the project root, leaving other keys untouched.

5. **Create a subagent on the Agents tab.** Click **New agent**. Enter a filename ending in `.md` (no slashes, no leading dot). A blank file is created at `.claude/agents/<filename>`. Open it from the list, paste a YAML frontmatter block (`name`, `description`, `tools`, `model`) followed by the system prompt body, and click **Save**.

## Verification

Open the files Vedox just touched from your terminal:

```sh
cat <project>/CLAUDE.md
cat <project>/.claude/settings.json
cat <project>/.mcp.json
ls <project>/.claude/agents/
```

Each file matches what the drawer shows. Run `git status` — the changed files appear as expected modifications.

## Troubleshooting

### Problem: Yellow banner "Config changed outside Vedox" appears when you click Save

**Cause:** Another process (a teammate, a script, or your own editor) wrote the file after Vedox loaded it. The sha256 etag Vedox sent no longer matches the file on disk and the server returned HTTP 409.

**Fix:** Click **Reload** to discard your edits and accept the on-disk version, or **Keep** to overwrite it. If you cannot decide, close the drawer and resolve the conflict in git. See [Provider drawer conflict](../runbooks/provider-drawer-conflict.md).

### Problem: "Could not load config" error on drawer open

**Cause:** Either the project name is invalid or the server cannot read the `.claude/` directory. Missing files are not an error — Vedox treats them as empty.

**Fix:** Check the `vedox dev` terminal for a `providers.handleGetClaudeConfig` log line with the failing path. Fix permissions on the path, then click **Retry** in the drawer.

### Problem: Comments in `.claude/settings.json` disappear after saving Permissions

**Cause:** Vedox parses settings.json with the standard JSON parser, which cannot round-trip `//` comments. A warning is logged when comments are detected (`providers: .claude/settings.json appears to contain comments`).

**Fix:** Until hujson support lands, do not put comments in `.claude/settings.json` if you also use the Permissions tab. Document the rules elsewhere.

### Problem: An agent file shows an empty name and description in the list

**Cause:** The YAML frontmatter at the top of the agent `.md` file is malformed. Vedox still lists the file (using the filename as the name) so you can fix it.

**Fix:** Open the agent in the drawer and correct the frontmatter block. The block must start with a `---` line, contain valid YAML, and end with a `---` line.

## See Also

- [Provider drawer architecture](../explanation/provider-drawer-architecture.md) — why the drawer is per-concern, per-provider
- [Provider drawer conflict](../runbooks/provider-drawer-conflict.md) — recovery when two writers race
