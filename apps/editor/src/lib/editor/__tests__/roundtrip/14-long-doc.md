---
title: Vedox Runbook — Development Environment Setup
type: runbook
status: approved
date: 2026-04-07
tags:
  - runbook
  - setup
  - development
---

## Overview

This runbook walks through setting up a complete Vedox development environment from scratch. It covers prerequisites, installation, workspace configuration, and common troubleshooting steps. Follow each section in order — sections depend on prior steps completing successfully.

**Estimated time:** 20–30 minutes on a clean machine.

**Supported platforms:** macOS (Apple Silicon and Intel), Linux (Ubuntu 22.04+, Fedora 38+). Windows via WSL2 only — see the known-issues section at the end.

---

## Prerequisites

### System requirements

- macOS 13+ or Ubuntu 22.04+ or Fedora 38+
- 4 GB RAM minimum (8 GB recommended for large workspaces)
- 2 GB free disk space
- Internet connection for initial dependency download

### Required tools

| Tool | Version | Install command |
| --- | --- | --- |
| Node.js | 20.x LTS or later | `nvm install 20` |
| pnpm | 9.x or later | `npm install -g pnpm@9` |
| Go | 1.22 or later | See https://go.dev/dl/ |
| Git | 2.38 or later | `brew install git` or `apt install git` |

### Optional but recommended

- **Homebrew** (macOS): for installing system dependencies cleanly
- **direnv**: for automatic environment variable loading per workspace
- **jq**: for pretty-printing the JSON logs from `~/.vedox/logs/`

---

## Step 1: Clone the repository

```bash
git clone https://github.com/vedox/vedox.git
cd vedox
```

Verify you are on the `main` branch:

```bash
git branch --show-current
# Should output: main
```

---

## Step 2: Install Node.js dependencies

From the repository root:

```bash
pnpm install
```

This installs all workspace dependencies across `apps/editor`, `apps/cli`, and `packages/markdown-core`. Expect 500–800 packages to be installed on a fresh machine.

Verify the installation:

```bash
pnpm run build --filter=@vedox/editor -- --dry-run
# Should output: build task ready (no errors)
```

---

## Step 3: Set up the Go workspace

```bash
go work sync
go build ./...
```

If `go build` fails with a missing module error, run:

```bash
cd apps/cli
go mod tidy
cd ../..
go work sync
```

---

## Step 4: Configure your Git identity

Vedox requires `user.name` and `user.email` to be set in your Git config before you can publish documents. If these are unset, the Publish action will fail with error `VDX-301`.

Check your current config:

```bash
git config --global user.name
git config --global user.email
```

If either is empty:

```bash
git config --global user.name "Your Name"
git config --global user.email "you@example.com"
```

---

## Step 5: Create a workspace

A Vedox workspace is any directory containing a `vedox.config.ts` file. For local development, use the repository root itself:

```bash
cat vedox.config.ts
```

You should see:

```typescript
import type { VedoxConfig } from './packages/markdown-core/src/config';

const config: VedoxConfig = {
  workspace: '.',
  port: 3001,
  logDir: '~/.vedox/logs',
  bind: '127.0.0.1'
};

export default config;
```

---

## Step 6: Start the development server

```bash
pnpm run dev
```

Turborepo will start both the Go CLI daemon and the SvelteKit editor in parallel. Expect output like:

```
vedox:cli   | Vedox v0.1.0 — listening on http://127.0.0.1:3001
vedox:editor | Local: http://127.0.0.1:3001
```

Open your browser to `http://127.0.0.1:3001`.

---

## Step 7: Verify the editor loads

On first run you should see the empty-state screen with a single "Add your first project" CTA. If you see a blank white screen or an error:

1. Check the terminal for Go CLI errors — they will have `VDX-` prefixed error codes
2. Check `~/.vedox/logs/vedox.log` for structured JSON logs
3. See the troubleshooting section below

---

## Step 8: Add the Vedox documentation as the first project

Click "Add your first project" and select the `docs/` directory in the repository root. Vedox will scan the directory, index all `.md` files, and populate the left sidebar.

You should see:

- "How to add a project" guide
- Five template documents (ADR, API Reference, Runbook, README, How-To)
- The ADR for Markdown-as-source-of-truth

---

## Verifying the dual-mode editor

Navigate to any document and verify:

1. **WYSIWYG mode** (default): Rich text renders correctly. Mermaid diagrams show as SVG previews.
2. **Code mode** (click "Code" button top-right): Raw Markdown is visible with syntax highlighting.
3. **Round-trip**: Edit in Code mode, switch to WYSIWYG — content is preserved. Edit in WYSIWYG, switch to Code — Markdown is correct.
4. **Auto-save**: After typing, the "Unsaved changes" indicator appears within 800ms.
5. **Publish**: Click "Publish", enter a commit message, click "Commit & Publish". A Git commit is created.

---

## Running the test suite

```bash
pnpm run test --filter=@vedox/editor
```

The round-trip golden-file tests must all pass. If any fail, do not merge. Fix the parser or the golden file (with a PR comment explaining the acceptable normalization).

To run only the round-trip tests:

```bash
pnpm run test --filter=@vedox/editor -- --reporter=verbose roundtrip
```

---

## Troubleshooting

### VDX-001: Port already in use

**Symptom:** `Error VDX-001: port 3001 is already in use.`

**Fix:** Find and stop the conflicting process:

```bash
lsof -ti:3001 | xargs kill -9
```

Or configure a different port in `vedox.config.ts`:

```typescript
const config: VedoxConfig = { port: 3002 };
```

### VDX-301: Git identity not configured

**Symptom:** Clicking "Publish" shows `Error VDX-301: git user.email is not set.`

**Fix:** Run `git config --global user.email "you@example.com"` and retry.

### VDX-404: Document not found

**Symptom:** Opening a document URL returns `Error VDX-404: document not found.`

**Fix:** The document may have been deleted outside Vedox. Run `vedox reindex` to rebuild the index.

### Blank white screen on load

**Symptom:** The browser shows a blank page with no error messages.

**Fix:** Check the browser console (F12). If you see a CSP violation, ensure you are accessing via `http://127.0.0.1:3001` and not `http://localhost:3001` — the CSP is scoped to `'self'` which treats these as different origins.

### Mermaid diagrams not rendering

**Symptom:** Mermaid blocks show "Rendering diagram…" indefinitely.

**Fix:** Open the browser console. Mermaid render errors are caught and logged. Common causes:

1. Malformed Mermaid syntax — click the diagram to open the edit popover and fix the source.
2. Missing `mermaid` package — run `pnpm install` in `apps/editor/`.

### SQLite index out of sync

**Symptom:** Search returns stale results, or documents created outside Vedox are missing.

**Fix:**

```bash
rm .vedox/index.db
vedox reindex
```

This reconstructs the entire index from the Markdown files with zero data loss.

---

## WSL2 on Windows — Known Issues

- File watching via `inotify` inside WSL2 may miss events from Windows-native tools (VS Code, Explorer) modifying files. Use WSL2-native tools exclusively when editing through Vedox.
- Symlinks across the WSL2/Windows boundary are not supported — Vedox will reject them silently.
- Port forwarding: access the dev server via `http://localhost:3001` in the Windows browser (WSL2 auto-forwards); `127.0.0.1:3001` may not resolve from Windows.

Native Windows support is a Phase 2 time-boxed investigation (5 engineering days). Until then, WSL2 is the recommended path.

---

## Appendix: Log format

Vedox writes structured JSON logs to `~/.vedox/logs/vedox.log`:

```json
{"time":"2026-04-07T12:00:00Z","level":"info","msg":"server started","port":3001,"bind":"127.0.0.1"}
{"time":"2026-04-07T12:00:05Z","level":"info","msg":"document indexed","path":"docs/runbook.md","size":4096}
{"time":"2026-04-07T12:00:10Z","level":"error","msg":"publish failed","code":"VDX-301","error":"git user.email not set"}
```

File contents are **never** written to logs — only paths, sizes, and metadata. This is enforced by the logging wrapper.

Logs are rotated every 7 days. To tail the log:

```bash
tail -f ~/.vedox/logs/vedox.log | jq .
```
