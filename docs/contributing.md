---
title: "Contributing to Vedox"
type: how-to
status: published
date: 2026-04-09
project: "vedox"
tags: ["contributing", "dev-setup", "onboarding", "testing", "conventions"]
author: "Vedox Tech Writer"
slug: contributing
---

# Contributing to Vedox

Audience: developers who want to build, test, or submit changes to Vedox.

---

## Prerequisites

| Tool | Minimum version | Notes |
|---|---|---|
| Go | 1.23 | `go.work` at root; CLI lives in `apps/cli` |
| Node.js | 20 | LTS; CI runs on 22 |
| pnpm | 9 | `packageManager: pnpm@9.15.0` pinned in `package.json` |
| Turborepo | 2 | Installed automatically via `pnpm install` |
| Git | any | Must have `user.name` and `user.email` configured — the CLI checks this at startup |

Verify your Go version:

```sh
go version
# go version go1.23.x ...
```

Verify pnpm:

```sh
pnpm --version
# 9.x.x
```

If your Git identity is not set, the `vedox dev` command exits with `[VDX-003]`. Fix it before starting:

```sh
git config --global user.name "Your Name"
git config --global user.email "you@example.com"
```

---

## Clone and install

```sh
git clone https://github.com/thepixelabs/vedox.git
cd vedox
pnpm install
```

`pnpm install` installs all Node dependencies for `apps/editor`, `apps/www`, and both packages. It does not install Go dependencies — those are fetched automatically by `go build`.

To fetch and tidy Go dependencies explicitly:

```sh
cd apps/cli
go mod tidy
```

---

## Repository layout

```
vedox/
├── apps/
│   ├── cli/           Go 1.23 backend — HTTP API + file-watcher daemon
│   │   ├── cmd/       Cobra command definitions (dev, build, lint, reindex, version)
│   │   └── internal/  All internal packages (store, db, api, scanner, …)
│   ├── editor/        SvelteKit 5 WYSIWYG editor
│   │   └── src/lib/editor/  Dual-mode editor (Tiptap + CodeMirror)
│   └── www/           SvelteKit static marketing site
├── packages/
│   ├── markdown-core/ Phase 2 WASM placeholder (not yet implemented)
│   └── templates/     Zod frontmatter schemas shared by the editor and CLI
├── docs/              Vedox's own documentation (dogfooded)
├── go.work            Go workspace — includes apps/cli
├── turbo.json         Turborepo pipeline
└── vedox.config.toml  Workspace config (you create this; see below)
```

---

## Create a workspace config

The CLI requires `vedox.config.toml` in the directory where you run it. Create a minimal one at the repository root:

```toml
# vedox.config.toml
port      = 5150
workspace = "."
profile   = "dev"
```

`workspace` is resolved relative to the config file. `port` defaults to `5150`; the SvelteKit Vite dev server runs on `5151` and proxies `/api/*` to the Go server.

---

## Running the apps

### All apps together (recommended)

```sh
pnpm dev
```

Turborepo starts all persistent dev tasks in parallel:
- Go CLI server on `http://127.0.0.1:5150`
- SvelteKit editor on `http://127.0.0.1:5151`
- SvelteKit www on its own port

### CLI only

```sh
cd apps/cli
go run . dev
# or, with a custom config:
go run . dev --config /path/to/vedox.config.toml
```

Use `--debug` for verbose logging and full error cause chains:

```sh
go run . dev --debug
```

### Editor only

```sh
cd apps/editor
pnpm dev
```

### CLI binary build

```sh
cd apps/cli
make build
# produces apps/cli/bin/vedox
./bin/vedox version
```

The `Makefile` injects `version`, `commit`, and `buildDate` via `-ldflags`.

---

## Running tests

### All tests

```sh
pnpm test
```

Turborepo runs Go tests and Node tests in dependency order.

### Go tests only

```sh
cd apps/cli
go test -race -count=1 ./...
```

The `-race` flag is required — all Go test runs must use it. The `Makefile` target `make test` includes it automatically.

### Editor tests only (vitest)

```sh
cd apps/editor
pnpm test
```

The editor test suite runs in a jsdom environment. The most important tests are the **golden-file round-trip tests** in `src/lib/editor/__tests__/roundtrip/`. These are CI blockers — they assert that every fixture file round-trips through the Tiptap editor byte-for-byte.

If you add a new Markdown construct to the editor, add a corresponding golden file in that directory numbered sequentially (`16-your-feature.md`).

### Template package tests

```sh
cd packages/templates
pnpm test
```

---

## Linting

### All linters

```sh
pnpm lint
```

### Go vet and staticcheck

```sh
cd apps/cli
go vet ./...
make lint   # requires: go install honnef.co/go/tools/cmd/staticcheck@latest
```

### Frontmatter linter

The CLI includes a frontmatter linter that enforces the WRITING_FRAMEWORK contract on all `docs/` Markdown files:

```sh
# Lint the default docs/ directory
go run ./apps/cli lint

# Lint a specific file or directory
go run ./apps/cli lint docs/adr/

# JSON output (useful for tooling)
go run ./apps/cli lint --format json
```

Lint rules are numbered `LINT-001` through `LINT-016`. Rule violations are printed as `WARN` or `ERROR`. In Phase 2 the exit code is always `0`; `--strict` is wired but not yet enforced (scheduled for Phase 3).

---

## Code conventions

### Go

- All packages in `apps/cli/internal/` are unexported to the outside world — do not add packages to `apps/cli/` root unless they are `cmd`.
- User-facing errors must use the VDX error taxonomy defined in `apps/cli/internal/errors/errors.go`. Never return raw `fmt.Errorf` strings to the user; wrap them in a `VedoxError` with the appropriate code.
- Error codes: VDX-001–VDX-099 are Phase 1 runtime; VDX-300+ are Phase 3 agent auth.
- The CLI makes **zero outbound network calls** by design. This is a hard policy stated in `apps/cli/main.go`. Any PR that adds outbound HTTP requires a config flag and an explicit review.
- Structured logging uses `log/slog`. Never use `fmt.Print*` for log output; use `slog.Info`, `slog.Warn`, `slog.Error`, `slog.Debug`. Raw Go stack traces are never shown to users.
- DocStore implementations must satisfy the `DocStore` interface in `apps/cli/internal/store/docstore.go` and enforce: path-traversal protection (VDX-005), secret file blocklist (VDX-006), atomic temp-file writes with fsync.

### TypeScript / Svelte

- SvelteKit 5 runes syntax (`$state`, `$effect`, `$props`) — not the legacy store/reactive-statement syntax.
- The `Editor.svelte` component is a pure UI component: it makes no API calls. All persistence flows through the `onChange` and `onPublish` callbacks passed by the parent.
- Frontmatter parsing and serialization go through `apps/editor/src/lib/editor/utils/frontmatter.ts` (`parseDocument` / `serializeDocument`). Do not manipulate frontmatter strings by hand elsewhere.
- Security: the Tiptap Markdown extension is configured with `html: false`. Do not change this. Mermaid SVG output is sanitized with DOMPurify before DOM insertion.
- Mode preference per document is persisted to `localStorage` under key `vedox-editor-mode-${documentId}`.

### Frontmatter and documents

Before writing any documentation into `docs/`, read `docs/WRITING_FRAMEWORK.md` and `docs/DESIGN_FRAMEWORK.md`. The linter enforces the schema mechanically. The eleven canonical content types, their required fields, and the naming conventions are defined there.

---

## CLI command reference

| Command | Description |
|---|---|
| `vedox dev` | Start the development server on `127.0.0.1:5150` |
| `vedox build` | Build static output (Phase 2) |
| `vedox lint [path...]` | Validate frontmatter against WRITING_FRAMEWORK |
| `vedox reindex` | Rebuild the SQLite full-text search index |
| `vedox version` | Print version, commit, and build date |

Global flags (all commands):

| Flag | Default | Description |
|---|---|---|
| `--config` | `./vedox.config.toml` | Path to workspace config |
| `--debug` | `false` | Verbose logging + full error cause chain |

---

## Dev server startup sequence

When you run `vedox dev`, the server performs these steps in order, failing fast at each:

1. Load `vedox.config.toml` — exits `[VDX-002]` if missing
2. Verify Git identity (`user.name` and `user.email`) — exits `[VDX-003]` if unset
3. Bind-test `127.0.0.1:<port>` — exits `[VDX-001]` if port is in use
4. Open the `LocalAdapter` DocStore and SQLite index
5. Start the background file indexer (watches workspace for `.md` changes, keeps FTS5 index in sync)
6. Restore linked projects from `.vedox/links.json`
7. Mount the HTTP API and start the server

---

## Submitting changes

1. Fork the repository or create a branch from `main`.
2. Run `pnpm test` and `go test -race -count=1 ./...` locally — CI runs both.
3. Run `go vet ./...` on any Go changes.
4. If you modified the editor's Markdown serialization, add or update golden files in `apps/editor/src/lib/editor/__tests__/roundtrip/`.
5. If you added a new VDX error code, add it to `apps/cli/internal/errors/errors.go` and document it in the appropriate error reference page.
6. Open a pull request against `main`. CI gates are: Go build + test + vet (ubuntu + macOS), Node build + test + lint (ubuntu + macOS).
7. Golden-file round-trip failures block merge — fix them, do not add them to `KNOWN_NORMALIZATIONS` without a written rationale in the PR.

---

## Troubleshooting

**`[VDX-001] Port 5150 is already in use`**
Run `lsof -i :5150` to find the process, or change `port` in `vedox.config.toml`.

**`[VDX-002] Config file not found`**
Create `vedox.config.toml` in the directory where you run `vedox dev`, or pass `--config /path/to/file`.

**`[VDX-003] Git identity not set`**
Run `git config --global user.name "..."` and `git config --global user.email "..."`.

**`go mod tidy` produces a diff in CI**
CI checks that `go.mod` and `go.sum` are committed and current. Run `go mod tidy` in `apps/cli/` and commit the result before pushing.
