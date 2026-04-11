---
title: Architecture Decision Record — Markdown as Source of Truth
type: adr
status: approved
date: 2026-04-07
tags:
  - architecture
  - markdown
  - storage
---

## Context

Vedox stores all documentation as Markdown files on disk. The SQLite database is a derived, rebuildable cache of frontmatter metadata and full-text search indexes. It is never the canonical source of truth.

## Decision

All document content lives in `.md` files within the workspace. The Go backend reads and writes these files atomically (write to temp + `fsync` + rename). The SQLite index is rebuilt on demand via `vedox reindex`.

## Consequences

- **Positive:** Documents survive database corruption. `vedox reindex` is the full recovery path.
- **Positive:** Git-native — every save is diffable, blame-able, and revertable.
- **Negative:** Full-text search requires an in-process SQLite query rather than a file scan; the index must be kept warm.
- **Neutral:** Frontmatter is the contract between the editor and the backend. Schema changes require a migration plan.
