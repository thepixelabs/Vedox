---
title: "Quickstart"
type: how-to
status: published
date: 2026-04-17
project: "vedox"
tags: ["quickstart", "onboarding", "install", "agent"]
author: "tech-writer"
audience: "new-users"
summary: "Get Vedox running and write your first doc in under 10 minutes."
---

# quickstart

**time to complete:** under 10 minutes  
**prerequisite:** Vedox installed (`vedox doctor` exits cleanly — see [INSTALL.md](../INSTALL.md))

---

## 1. run onboarding

```sh
vedox init
```

this starts a 5-step guided setup:

1. **detect projects** — Vedox scans for Git repos under your home directory. select which projects you want to document.
2. **register doc repos** — create a new GitHub repo via `gh` CLI or register an existing one. tag it as public-facing or private. you can register more repos later in Settings.
3. **install the doc agent** — choose which AI provider to instrument: Claude Code (MCP), GitHub Copilot, OpenAI Codex, or Google Gemini CLI. Vedox installs the agent config automatically.
4. **configure voice** (optional) — enable "vedox document everything" as a voice trigger on macOS. skip this step on Linux or to set it up later.
5. **first doc suggestion** — Vedox picks an undocumented project and suggests a doc to write. you can accept, skip, or choose your own.

re-trigger onboarding at any time from Settings > General > Run setup again.

---

## 2. start the daemon

```sh
vedox server start
```

the daemon runs on `http://127.0.0.1:5150`. it watches your registered doc repos for file changes, keeps the SQLite index current, and serves the API that the editor reads.

to start the daemon automatically on login:

```sh
vedox server enable
```

check daemon status at any time:

```sh
vedox server status
```

---

## 3. open the editor

```sh
open http://127.0.0.1:5151
```

the editor loads your doc tree in the left panel — organized by project and document type, not a flat list. the right panel shows the document you're editing. use Cmd+K to open the command palette.

if the editor shows "daemon not reachable", confirm `vedox server status` is running and that port 5150 is not blocked.

---

## 4. install the doc agent into Claude Code

if you chose Claude Code during onboarding, the MCP server is already registered. confirm it with:

```sh
vedox agent status claude-code
```

if you skipped it or want to add it now:

```sh
vedox agent install claude-code
```

this writes an MCP server entry to your Claude Code config (`~/.claude/claude_desktop_config.json`) with HMAC auth pre-configured.

in any Claude Code session, say:

```
vedox document everything
```

the agent scans the current working directory, infers the project, writes one or more docs to the correct repo (public or private based on your routing rules), and commits them. you see the new files appear in the editor tree within a few seconds.

---

## 5. write your first doc manually

in the editor, press Cmd+K, type `new`, and select **New document**. pick a document type — `how-to`, `explanation`, `adr`, or `runbook` — and fill in the frontmatter fields the template provides.

the editor saves automatically every 800ms. when you're ready to commit:

- click **Publish** in the toolbar, or
- press Cmd+Shift+P and select **Publish document**

Vedox commits the file to the registered doc repo using your Git identity. the history timeline on the right side of the editor updates immediately.

---

## what's next

| task | where to go |
|---|---|
| add another doc repo | Settings > Repositories > Register repo |
| install the agent in Codex or Gemini | [docs/how-to/manage-codex-config.md](how-to/manage-codex-config.md) / [docs/how-to/manage-gemini-config.md](how-to/manage-gemini-config.md) |
| customize theme, fonts, density | [docs/how-to/customize-appearance.md](how-to/customize-appearance.md) |
| use the reference graph | Cmd+K > Graph view |
| keyboard shortcuts | [docs/how-to/keyboard-shortcuts.md](how-to/keyboard-shortcuts.md) |
| understand how the daemon works | [docs/explanation/editor-architecture.md](explanation/editor-architecture.md) |
| write a doc for this project | [docs/WRITING_FRAMEWORK.md](WRITING_FRAMEWORK.md) |
