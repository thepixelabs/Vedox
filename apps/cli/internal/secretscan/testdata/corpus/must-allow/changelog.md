# Changelog

## v2.0.0 — 2026-04-15

### Added
- Daemon mode with launchd/systemd integration
- Multi-repo registry with keychain token storage
- Doc Agent installation for Claude Code and Codex
- Secret detection pre-commit gate (secretscan package)
- Doc tree and reference graph
- Human-readable history timeline
- Personalization settings (7 categories)
- Analytics overview strip
- 5-step onboarding flow

### Security
- HMAC-SHA256 agent authentication (commit 3ff0ba7)
- secretscan Layer 1 deterministic rules (15 patterns)
- Rate limiting: 200 req/s burst per key
- Keychain ACL bound to code signature on macOS

### Breaking Changes
- Schema version 4 — v1 databases require migration
- `agent-keys.json` format updated (new `revoked` field)
