---
title: "Provider Drawer Save Fails with Conflict"
type: runbook
status: published
date: 2026-04-09
project: "vedox"
tags: ["provider-drawer", "conflict", "etag", "claude-code", "incident-response"]
author: "Vedox Team"
on_call_severity: "P3"
last_tested: "2026-04-09"
target_time_to_mitigate_minutes: 5
related_error_codes: []
service: "vedox-cli"
---

# Provider Drawer Save Fails with Conflict

## Symptoms

- The user clicks **Save** in the provider drawer (Memory, Permissions, MCP, or Agents tab) and a yellow banner appears reading "Config changed outside Vedox" with **Reload** and **Keep** buttons.
- The browser network tab shows the `PUT /api/projects/:project/providers/claude/...` request returning HTTP 409 with a body containing `currentEtag` and (for memory and agent writes) `currentContent`.
- The dirty indicator on the tab does not clear.
- No data is lost: the user's pending edit is still in the editor; the on-disk file is also intact.

## Immediate Actions

1. Tell the user not to click anything yet. Both versions still exist.
2. Open a terminal in the project directory and run `git status` to see whether the on-disk file is uncommitted.
3. Run `git log -1 -p -- <path-to-conflicted-file>` to see the most recent committed change to that file.
4. Identify the other writer (a teammate, a script, a second Vedox window, your own shell). Ask them to stop writing to the file until this is resolved.
5. Decide which version to keep — see Resolution Steps.

## Root Cause Investigation

The drawer sends a sha256 etag with every write. The server (`apps/cli/internal/api/providers.go`) re-reads the file before writing and rejects the write with HTTP 409 if the on-disk etag does not match. A 409 means **the file on disk changed between the moment the drawer loaded it and the moment Save was clicked**. The realistic causes are:

- **Case A — A teammate edited and committed the file**, then the user pulled. The drawer's loaded copy is older than the working tree.
- **Case B — A second Vedox window** (same machine, different project that points at a shared file like `~/.codex/config.toml`, or same project in two browser tabs) wrote the file via its own drawer.
- **Case C — A shell or editor wrote the file** outside Vedox: the user opened `CLAUDE.md` in their editor, made a change, saved, then forgot about the drawer.
- **Case D — A script or hook** (a pre-commit hook, a generator, a sync tool) wrote the file in the background.

## Resolution Steps

### Case A — Teammate edited the file

1. In the drawer banner, click **Reload**. The drawer discards the user's pending edit and loads the on-disk version.
2. Re-apply the user's change on top of the new content.
3. Click **Save**. The new etag matches; the write succeeds.

### Case B — Second Vedox window

1. Find the other window. Decide which window is the source of truth.
2. In the loser window, click **Reload** to drop its edits.
3. In the winner window, click **Save**.
4. Close the loser window to prevent recurrence.

### Case C — Editor or shell wrote the file

1. If the editor's change is the keeper, click **Reload** in the drawer, then close the drawer.
2. If the drawer's change is the keeper, copy the drawer's content to the clipboard, click **Reload**, paste, click **Save**.
3. If both changes need to survive, click **Reload**, leave the drawer, and merge the two versions in your editor by hand. The drawer does not ship a three-way merge.

### Case D — Background script

1. Stop the script.
2. Click **Reload** in the drawer to load whatever the script left.
3. Decide whether the drawer's pending edit should be re-applied. If yes, re-type it and click **Save**.
4. File a follow-up to make the script and the drawer not race (move the script behind a lock, or have it leave the file alone when Vedox is running).

## Verification

1. Click **Save** in the drawer. The 409 banner does not reappear and the dirty dot clears.
2. In the terminal, `cat` the file. Its contents match what the drawer shows.
3. Run `git diff <path>`. The diff is the change you intended — nothing more, nothing less.

## Prevention

- **Commit provider config files to git.** `CLAUDE.md`, `.claude/settings.json`, `.mcp.json`, and `.claude/agents/*.md` are part of the project. Tracking them gives you `git log -p` for free during incident triage and makes Case A debuggable.
- **Pick one writer per file at a time.** If your team uses the drawer, do not also run scripts that mutate the same files.
- **Avoid two Vedox windows on the same Codex config.** Codex's config is global (`~/.codex/config.toml`); two project windows both pointed at it will race. See [How to Manage Codex Config from Vedox](../how-to/manage-codex-config.md).
- **For background on the etag protocol**, read [Provider drawer architecture](../explanation/provider-drawer-architecture.md).

## See Also

- [How to Manage Claude Code Config from Vedox](../how-to/manage-claude-code-config.md)
- [Provider drawer architecture](../explanation/provider-drawer-architecture.md)
