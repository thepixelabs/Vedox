---
title: "ADR-001: Markdown as Source of Truth"
type: adr
status: accepted
date: 2026-04-07
project: "vedox"
tags: ["storage", "markdown", "sqlite", "portability", "architecture"]
author: "Vedox Team"
superseded_by: ""
---

## Context

Vedox needs a durable storage model for documentation. The core product proposition is local-first, Git-native documentation — meaning the storage model directly determines whether that proposition holds.

We evaluated storage options against three constraints:

1. **Portability.** A user's documentation corpus must be fully portable. Moving from Vedox to any other tool, or moving the workspace to a different machine, must not require an export step. The data must be readable by non-Vedox tooling at rest.

2. **Git-nativity.** Version history, authorship, blame, and diffing must work with standard Git tooling. A storage format that cannot be committed to a Git repository is incompatible with the product thesis.

3. **Localhost-first.** The system must run with zero external dependencies. No cloud service, no background daemon beyond the Vedox process itself, no network access required.

The team also identified a recovery requirement: if the metadata index is corrupted or deleted, the system must be fully recoverable from the on-disk document files with no data loss. This rules out any storage model where the authoritative state lives in a database that cannot be regenerated from a human-readable file tree.

## Decision

We will store all documents as Markdown files (`.md`) on the local filesystem. SQLite is used exclusively as a rebuildable cache of extracted frontmatter and full-text search indexes. Deleting the SQLite database loses no data. Running `vedox reindex` rebuilds the entire index from the Markdown file tree.

The implications of this model are architectural constraints, not implementation choices:

- Frontmatter YAML embedded in each `.md` file is the authoritative record of document metadata. The SQLite `documents` table is a projection of that frontmatter, not the other way around.
- The `vedox reindex` command is a first-class, supported recovery path — not a debugging utility. It must be tested as part of every release (DR test: `rm ~/.vedox/index.db && vedox reindex`).
- The SQLite file (`~/.vedox/index.db`) is excluded from the workspace Git repository. The `.md` files are committed. This separation makes the boundary between source of truth and cache explicit.
- Atomic file writes are required on all DocStore mutations: write to a temp file, `fsync`, then rename. The index is updated after the file write succeeds. Partial writes are not possible; partial index updates are recoverable by reindex.

## Consequences

**Positive:**

- The entire workspace is a Git repository. History, authorship, branching, and merging work with standard Git tools and any Git host. No Vedox-specific export is needed to back up, share, or migrate documentation.
- The workspace is portable to any machine that can run `git clone`. A developer switching laptops copies their documentation by cloning their workspace repository.
- The system works fully offline. No network access is required for any operation.
- No vendor lock-in. Every document is a plaintext file readable in any text editor. Leaving Vedox means opening the file tree in another tool — there is nothing to export.
- `vedox reindex` is a complete disaster recovery path. SQLite corruption is a recoverable non-event.
- Full-text search is available from day one via SQLite FTS5. No additional process or index server is required.

**Negative:**

- Binary attachments (images, PDFs, diagrams) cannot be stored in Markdown and must be referenced as files at a relative path. Large binary assets in the workspace will inflate the Git repository size over time.
- Full-text search is limited to text content. Handwriting in images, text in PDFs, or content in embedded binary formats is not indexed.
- Schema changes to frontmatter fields require a `vedox reindex` run to propagate to the SQLite cache. This is fast (seconds for typical workspace sizes) but is a user-visible operation rather than a background migration.
- The Markdown-on-disk format is the contract. Adding required frontmatter fields in a future version of Vedox means existing documents become schema-invalid until updated. Migration tooling is required.

**Neutral / follow-on work:**

- The `vedox reindex` command must be exercised in CI as an acceptance criterion for every release (Phase 1 Definition of Done).
- A SQLite migration framework (versioned, atomic) is required in Phase 1 to handle schema changes to the index without requiring a full user-initiated reindex on every upgrade.
- The `DocStore` interface must expose atomic write semantics to callers — the "write to temp, fsync, rename" pattern is an implementation detail of `LocalAdapter`, not something callers should reimplement.

## Alternatives Considered

### Option A: PostgreSQL as Primary Store

Use PostgreSQL as the primary document store and source of truth, with Markdown export as a secondary feature.

Rejected because PostgreSQL requires a running server process. This directly violates the localhost-first constraint — `vedox dev` would require `pg_ctl start`, which is a prerequisite Vedox cannot impose on users. Team deployments would require database provisioning, backups, and operational runbooks. The product persona (solo developers, small teams) has no appetite for this operational surface area.

### Option B: SQLite as Primary Store (No Markdown Files)

Use SQLite as the only storage layer. Documents are stored as text blobs in the database. No `.md` files are written to the filesystem.

Rejected because it loses portability entirely. Users cannot edit documents outside Vedox (no Vim, no VS Code, no GitHub web editor). A corrupt or lost SQLite file means total data loss with no recovery path. Git history is not document history — you can only see "index.db changed," not which specific document changed and what the diff was. This option preserves none of the product's stated advantages over existing tools.

### Option C: Headless CMS (Contentful, Sanity, or Similar)

Use a cloud-hosted headless CMS as the document store and use the CMS API for reads and writes.

Rejected because it introduces a hard cloud dependency. The system cannot run offline. It costs money (relevant for solo developers and small teams who are the primary persona). It is not open-source-friendly. Vendor lock-in is high — migrating off a headless CMS requires a bulk export operation and a migration script. This option is the exact pattern Vedox is positioned against.

### Option D: Git Submodule Per Project

Store each project's documentation as a separate Git repository, linked to the workspace via a Git submodule.

Rejected because Git submodules are notoriously difficult for developers to work with. New team members frequently get into a detached HEAD state or fail to initialize submodules, producing a broken workspace. The onboarding friction contradicts the "zero-friction local install" requirement. A flat file structure within a single workspace repository achieves the same isolation without the operational complexity.
