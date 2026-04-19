# Changelog

All notable changes to Vedox are documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Vedox uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
Pre-1.0 releases carry the `-alpha.N` suffix; breaking changes may occur
between alpha tags.

---

## [0.1.0-alpha.1] — 2026-04-17

### Added

**Core daemon and CLI** (`feat(v2)` — PR #4)
- `vedox server start|stop|status|restart|logs|install|uninstall` lifecycle
  commands with launchd (macOS) and systemd (Linux) supervisor support.
- PID-file management, `SIGHUP` live-reload, `--no-supervisor` flag for
  containerised or manual process management.
- `--deploy-mode=container` for Docker/OCI environments.
- `/healthz` endpoint for liveness probes.
- Bootstrap token printed to stderr on first start; used to gate all
  write operations until an agent key is provisioned.
- `vedox init` — system and per-project initialisation (idempotent).
- `vedox doctor` — 12 health checks covering git, gh, daemon, registry,
  disk, keychain WAL, and more.
- `vedox version` — prints version, commit hash, and build date.
- `vedox completion bash|zsh|fish` via cobra.

**Multi-repo registry**
- `FileRegistry` backed by `~/.vedox/repos.json` with an advisory file
  lock to prevent concurrent mutation.
- Three repo types: private, project-public, bare-local.
- Orphan detection and `SIGHUP` live-reload.

**Provider adapters** (`feat(providers)` — PR #1 predecessor, PR #4)
- Four provider adapters: Claude Code (MCP), Codex (TOML), Copilot
  (degraded / prose), Gemini (extensions).
- `ProviderInstaller` 6-verb interface: install, uninstall, enable,
  disable, status, repair.
- Atomic file writes, HMAC key bootstrap, install receipts, drift
  detection + repair.
- Per-project config drawer in the editor UI.

**Agent authentication** (`feat(agentauth)` — VDX-P3-AUTH)
- HMAC-SHA256 agent authentication middleware.
- `RequireAgent` wired into daemon startup; degrades to `RejectAllAuth`
  (503 on every agent-protected route) if keystore fails to load —
  fail-closed, never fail-open.

**Secret scanning** (`feat(secretscan)` — PR #5)
- Pre-commit gate via `GatePreCommit`.
- **262 detection rules** powered by `betterleaks` (MIT, by the original
  gitleaks author) — covering 1Password, Azure AD, Cloudflare, Discord,
  Databricks, Datadog, Figma, 100+ additional services.
- 15 hand-rolled deterministic rules retained as fallback when
  `betterleaks` cannot initialise; zero public-API change.
- 30-file red-team corpus for rule regression testing.
- Scan serialised with `sync.Mutex` — safe under concurrent callers.

**Secrets backends**
- Three backends: OS keyring, age-encrypted file, environment variable.
- `AutoDetect` fallback chain; 500 ms D-Bus timeout for headless Linux.
- `*_FILE` environment variable convention for container deployments.

**Document graph**
- goldmark AST link extractor — four link types: `md-link`, `wikilink`,
  `frontmatter`, `vedox://`.
- SQLite adjacency store, backlinks, broken-link detection.
- Cytoscape.js force-directed graph in the editor UI (6 node types,
  5 edge styles, hover-dim, click-navigate, filter chips).

**History and diffs**
- Paragraph-level Myers prose-diff.
- `git log --follow` integration.
- Deterministic change summaries.
- Vertical timeline UI with expandable block-level diffs.

**Code preview**
- `vedox://` scheme resolver with a 7-layer security sandbox and 500 KB cap.
- Shiki-powered hover card; LRU cache; auto theme sync.

**Analytics**
- 256-entry event collector with 5-second flush.
- SQLite aggregator with 60-second roll-up window.
- `GlobalDB` with three tables; 10 `subject.verb` event constants.
- Dashboard: stat cards, per-project bar chart, pipeline status, shimmer
  loading states.

**Voice (push-to-talk)**
- Intent parser: 7 commands, Levenshtein fuzzy matching, 8 STT mishearing
  variants.
- Audio pipeline with `Transcriber` / `AudioSource` interfaces.
- Push-to-talk orchestrator; hotkey abstraction.
- Daemon integration via `/api/voice/*` endpoints.

**Editor UI** (SvelteKit 2 + Svelte 5)
- Hierarchical Diataxis-grouped document tree with as-you-type filter,
  collapsible groups, count badges, ARIA tree pattern, localStorage
  persistence.
- 7-tab settings panel: 35+ settings, live preview, keyboard remap with
  conflict detection, cross-setting search.
- 5-step onboarding flow (scan / repos / agent / voice / done); all steps
  skippable; graceful degradation when daemon is offline.
- Sidebar navigation for `/graph`, `/analytics`, `/settings`.

**Distribution**
- goreleaser cross-compile matrix: universal macOS binary,
  `//go:build release` tag guard for SvelteKit embed, SLSA L3 provenance.
- Homebrew formula + tap design (brews block).
- `install.sh` curl-pipe installer for macOS + Linux (4 arch combos).
- Dockerfile (distroless-static, ~27 MB), `docker-compose.yaml` with
  named volume, tmpfs PID dir, Docker secrets for age passphrase,
  loopback-only port binding.
- GHCR multi-arch images (linux/amd64 + linux/arm64).

**Website**
- Brand identity rollout: 10 logos, landing page redesign, fossil-record
  favicon, hero mock editor.

**Docs and ADRs**
- WRITING_FRAMEWORK compliance backfilled across all how-tos and ADRs.
- CSP `unsafe-inline` / `unsafe-eval` deviation documented (VDX-P1-005).

### Changed

- Landing page repositioned: comparison table removed; FAQ reframed to
  product-only positioning.
- Brass registry entry updated to straddle FAQ/roadmap boundary.
- GitIgnore tightened; AlterGo install URL corrected.

### Fixed

- `isSecretFile` basename stripping stopped at the first dot, allowing
  files like `.env.draft.md` to bypass the secret-file blocklist.
  `stripDraftSuffixes` now iterates all known draft/backup extensions
  before applying the block check.
- `SetGraphStore` was never called in daemon wiring — `/api/graph` returned
  503. Fixed and guarded by regression test.
- `SetGlobalDB` call missing in daemon wiring — `/api/analytics/summary`
  returned 503. Fixed and guarded.
- `PassthroughAuth` used in daemon startup instead of real HMAC auth
  middleware (H-01).
- Bootstrap token printed to stdout, risking capture in supervisor logs
  (H-02). Now prints to stderr.
- `ruleNameIndex` package-level map caused data race on concurrent
  `secretscan.New()` calls (M-01). Moved into per-scanner struct.
- `betterleaks` `Match` field accepted as `Secret` fallback — `Match` is
  wider than the extracted capture and could leak context into `Redact()`.
  Now always uses the extracted secret value.
- Keychain tests touched the real macOS keychain; now use an ephemeral
  in-memory stub.
- `doctor` degrades gracefully when the keychain is unavailable rather
  than hard-failing the health check.
- Missing editor dependencies: tiptap table, image, and KaTeX extensions.
- Stale `#compare` nav anchor removed from website.
- CI: permissions, Node version mismatch, `cyclonedx-gomod` pin,
  `go.work` enforcement, `go mod tidy`.
- CI: `AutoDetect` tier test skipped when the environment selects a
  different secrets tier (flaky on Linux runners).
- CI: untracked secret files added; git config sandboxed in emit test.

### Security

- Secret scanning upgraded from 15 hand-rolled rules to 262 rules via
  `betterleaks` (PR #5). Covers all major cloud providers and SaaS APIs.
- `betterleaks` scan serialised with `sync.Mutex` to prevent concurrency
  regressions from leaking partial matches across goroutines.
- HMAC-SHA256 agent authentication replaces the placeholder
  `PassthroughAuth` no-op in daemon startup (H-01 fix).
- Bootstrap token moved to stderr to prevent accidental capture in
  supervisor log aggregation pipelines (H-02 fix).
- `POST /api/repos/create` and `POST /api/repos/register` now require a
  valid bootstrap token; previously accepted tokenless requests silently.
- Fail-closed `RejectAllAuth` (503) used when keystore fails to load;
  never fails open.
- Multi-suffix secret files (`.env.draft.md`, `.env.bak.txt`, etc.)
  now correctly blocked at the store and importer layers.

---

## [0.0.1] — 2026-03-01

Initial public release. Placeholder tag marking project creation.

[0.1.0-alpha.1]: https://github.com/vedox/vedox/compare/v0.0.1...v0.1.0-alpha.1
[0.0.1]: https://github.com/vedox/vedox/releases/tag/v0.0.1
