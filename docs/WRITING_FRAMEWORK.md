---
title: "Vedox Writing Framework"
type: explanation
status: published
date: 2026-04-07
project: "vedox"
tags: ["framework", "schema", "governance", "agent-contract", "writing", "constitution"]
author: "Vedox CEO"
supersedes: ""
audience: "all-writers-human-and-agent"
---

# Vedox Writing Framework

This document is the law for any agent — human or AI — that writes content into the Vedox documentation tree. It defines the content types, their schemas, their lifecycles, and the mechanically verifiable rules that constitute "valid Vedox content." A document that does not conform is not Vedox content. It will be rejected. An agent that submits non-conforming content repeatedly will not be invited back.

This framework covers the **content/editorial/schema/governance** layer. The **visual/IA/component/accessibility/editor-UX** layer is governed by [DESIGN_FRAMEWORK.md](./DESIGN_FRAMEWORK.md), maintained in parallel by the creative-technologist role. Any concern about how a page looks, how the editor surfaces a field, how a component should be reused, or how the IA navigates between docs lives in DESIGN_FRAMEWORK.md, not here. When the two documents disagree on a point, file an issue and escalate; do not silently make the call yourself.

This is not a tutorial. It is a contract. There is no "you might consider." Where the framework says "MUST" the rule is mechanically enforced. Where it says "SHOULD" a human reviewer will reject violations during PR review. Where it says "MAY" the writer chooses.

---

## 0. Read Order

1. This document, top to bottom, in one pass.
2. [DESIGN_FRAMEWORK.md](./DESIGN_FRAMEWORK.md), top to bottom, in one pass.
3. The how-to template that matches the content type you are about to write (`docs/how-to/use-*-template.md`).
4. Then, and only then, write the document.

If you skip step 1 or step 2, your work will be rejected on first review and you will not be assigned content tasks again.

---

## 1. The Agent Contract (Read This Part First)

This section is addressed to AI agents and humans operating under agent-style PR workflows. It is blunt because every minute spent re-reviewing non-conforming submissions is a minute not spent shipping the product.

### 1.1 You MUST

1. **Read this entire file before writing.** No exceptions. Reading the file the first time you write content for Vedox is mandatory; reading it again whenever it is updated is mandatory.
2. **Read DESIGN_FRAMEWORK.md before writing.** Visual and IA decisions are not optional cosmetic concerns; they are part of the contract.
3. **Pick exactly one content type from Section 4.** Do not invent new types. Do not mix types. A document that is half how-to, half explanation is two documents.
4. **Use the correct directory and filename pattern from Section 4 for that type.** A misplaced file is rejected by the linter and never reaches review.
5. **Include all required frontmatter fields from Section 3 plus the type-specific fields from Section 4.** Frontmatter is mandatory. A document without frontmatter is not a Vedox document.
6. **Validate your frontmatter against the JSON Schema in Section 11 before submitting.** Mechanical validation is not a suggestion. CI runs it and rejects failures.
7. **Write to a staging branch.** Agent writes never go to `main` directly. This is enforced by the Phase 3 review queue and is non-negotiable.
8. **Run the preflight checklist in Section 1.3 below before submitting.**
9. **Defer all visual and component questions to DESIGN_FRAMEWORK.md.** Do not invent layout, typography, color, or component conventions in your prose.
10. **When in doubt, BLOCK and ask, do not guess.** A blocked task with a clear question is acceptable. A merged document with a wrong assumption is not.

### 1.2 You MUST NOT

1. **Never write code.** Documentation agents write Markdown only. Editing source files in `apps/`, `packages/`, `internal/`, or any non-`docs/` path is a permanent prohibition. Phase 3 enforces this in CI.
2. **Never write `.env` content, API keys, secrets, customer names, real emails, real phone numbers, or any personally identifiable information** into any document, frontmatter field, code block, comment, or commit message. The secret file blocklist (Section 9.3) is enforced server-side; do not test it.
3. **Never invent frontmatter fields.** Adding a field that is not in this schema is rejected. If you genuinely need a new field, file an ADR proposing the schema change first.
4. **Never use emoji** anywhere in any document or commit message. The single exception is human-authored release notes destined for a public changelog page (and even then, sparingly). Agents never use emoji. Ever.
5. **Never use first-person plural marketing voice** ("we are excited to announce", "our amazing platform"). Vedox documentation is direct, factual, and timeless.
6. **Never duplicate content** that lives in another document. Link to it. If the linked document is wrong, fix the linked document; do not create a divergent copy.
7. **Never modify a document's `id`, `title`, or filename after publication** without writing a `redirect_from` entry (Section 9.5). Permanent URLs are a contract with readers and external links.
8. **Never leave a document in `draft` state for more than 14 days.** Drafts that age out are mechanically detected and the writer is notified. Three abandoned drafts cost an agent its assignment privileges.
9. **Never write speculative or aspirational content.** "Vedox will support X in the future" belongs in an ADR or epic, not in product documentation. Documentation describes reality at the date in `date:`.
10. **Never copy text from `~/.vedox/`, `~/.claude/`, `~/.ssh/`, `.git/`, `.tasks/`, or any other directory outside `docs/`** into a document. Path traversal is enforced; copy-paste is a slower version of the same violation.

### 1.3 Preflight Checklist

Before submitting any document, mechanically work through this checklist. Every item is verifiable. If any item fails, fix it and restart the checklist from item 1.

```
[ ] 1.  I have read WRITING_FRAMEWORK.md (this file) in full this session.
[ ] 2.  I have read DESIGN_FRAMEWORK.md in full this session.
[ ] 3.  I have identified exactly one content type from Section 4.
[ ] 4.  My filename matches the naming pattern in Section 4 for that type.
[ ] 5.  My file is in the correct directory from Section 4 for that type.
[ ] 6.  My frontmatter contains every required universal field from Section 3.
[ ] 7.  My frontmatter contains every required type-specific field from Section 4.
[ ] 8.  Every frontmatter field uses the correct type and a value from any
        defined enum.
[ ] 9.  My `date` field is today's ISO 8601 date (YYYY-MM-DD), not a guess.
[ ] 10. I have validated the frontmatter against the JSON Schema in Section 11.
[ ] 11. The required headings for my type appear in the required order with no
        extra top-level headings inserted.
[ ] 12. There is exactly one H1 in the document and it matches `title` exactly.
[ ] 13. No emoji appear anywhere in the document or commit message.
[ ] 14. No PII, secrets, credentials, real customer names, or real personal data
        appear anywhere.
[ ] 15. Every internal link uses a workspace-relative path that resolves.
[ ] 16. Every code reference uses the `path/to/file.go:LINE` form (Section 9.4).
[ ] 17. Every image and asset is in the correct `_assets/` directory and has
        meaningful alt text.
[ ] 18. The document is under the word-count cap for its type (Section 4).
[ ] 19. Trailing whitespace, tab characters, and CRLF line endings are absent.
[ ] 20. I am writing to a staging branch, not `main`.
```

If any item is unclear, stop and re-read the relevant section. Do not proceed by guessing.

### 1.4 Rejection Policy

A submission that fails any MUST rule in this document is rejected automatically by the linter with a pointer to the failing rule. A submission that fails a SHOULD rule is rejected by a human reviewer with a comment. Three rejections of the same kind from the same agent in any 30-day window result in that agent being removed from the writing assignment pool. There is no appeal process; the framework exists to make appeals unnecessary.

---

## 2. Global Rules (Apply to Every Content Type)

These rules apply to **every** document in `docs/`, regardless of type. Type-specific rules in Section 4 are additive; they never relax these.

### 2.1 File-level rules

- **Encoding** is UTF-8. No BOM. (`MUST`)
- **Line endings** are LF only. CRLF is rejected by the linter. (`MUST`)
- **No trailing whitespace** on any line. (`MUST`)
- **No tab characters** in Markdown source. Indent with spaces. (`MUST`)
- **Max line length is 100 characters** for prose lines. Code blocks are exempt. URLs that exceed 100 characters are exempt. (`SHOULD`)
- **Filename is kebab-case**, lowercase ASCII letters, digits, and hyphens only. Maximum 80 characters including extension. No leading or trailing hyphen. (`MUST`)
- **File extension is `.md`** for all written content. MDX is not used in Vedox documentation; `.mdx` is reserved for future component-driven pages and is currently rejected. (`MUST`)
- **One blank line at end of file**, not zero, not two. (`MUST`)
- **One blank line between paragraphs**, never two consecutive blank lines. (`MUST`)
- **No HTML tags** in Markdown source except where Section 9.6 explicitly permits them. Agents may never use HTML. Humans may use the permitted subset. (`MUST`)

### 2.2 Frontmatter rules

- **Frontmatter is mandatory** on every document. A document with no `---` block at the top is rejected. (`MUST`)
- **Frontmatter is YAML 1.2.** No JSON-in-frontmatter. No TOML. (`MUST`)
- **String values are double-quoted** when they contain a colon, a hash, a leading number, or any non-ASCII character. Otherwise quotes are optional but recommended for `title`. (`MUST`)
- **Date values are ISO 8601** in `YYYY-MM-DD` form. Time-of-day is not allowed in any date field. (`MUST`)
- **Tag arrays are flow-style** (`tags: ["a", "b", "c"]`), kebab-case, between 2 and 8 entries, lowercase. (`MUST`)
- **Unknown frontmatter keys are rejected.** The schema in Section 11 is closed by content type. Adding a key requires an ADR. (`MUST`)

### 2.3 Heading rules

- **Exactly one H1 per document** and its text MUST match the `title` frontmatter field exactly (after Markdown unescaping). Generators that produce HTML may suppress the H1 in favor of the page title. (`MUST`)
- **Heading levels descend without skipping.** H1 -> H2 -> H3. You may not jump from H2 to H4. (`MUST`)
- **Heading text is sentence case**, not title case, except for proper nouns. ("How to add a project," not "How To Add A Project.") (`MUST`)
- **No trailing punctuation in headings** except for `?` where the heading is genuinely a question. (`MUST`)
- **Required heading order per type** is defined in Section 4. You may add subheadings under a required heading; you may not add new top-level (H2) headings outside the required set unless the type's section explicitly permits an "extras" zone. (`MUST`)

### 2.4 Voice and tone

- **Direct, factual, present tense.** "Vedox writes the file." Not "Vedox will write the file" and not "Vedox wrote the file." (`MUST`)
- **Imperative for instructions.** "Run `vedox dev`." Not "You should run `vedox dev`." (`MUST`)
- **No hedging.** No "might," "perhaps," "we think," "could possibly." If you don't know, find out or do not write the sentence. (`MUST`)
- **No marketing adjectives.** Forbidden words include `amazing`, `awesome`, `powerful`, `flexible`, `modern`, `cutting-edge`, `revolutionary`, `seamless`, `robust`, `world-class`, `next-generation`, `simple to use`, `user-friendly`. Replace with a concrete fact. (`MUST`)
- **No second-person plural.** "You" is fine. "You guys" is not. "Folks" is not. (`MUST`)
- **British vs. American spelling:** American English. `color`, not `colour`. `behavior`, not `behaviour`. (`MUST`)
- **Oxford comma is required.** "ports, protocols, and security." (`MUST`)

### 2.5 Numbers, units, and dates

- **ISO 8601 dates** in body text whenever a calendar date is referenced: `2026-04-07`, never `April 7, 2026` or `7/4/26`. (`MUST`)
- **Times are 24-hour with timezone**: `14:30 UTC`. Local times are not used in documentation. (`MUST`)
- **Durations** are spelled out with units: `30 seconds`, `5 minutes`, `2 hours`. SI symbols (`s`, `min`, `h`) are acceptable inside tables and code blocks. (`MUST`)
- **Sizes** use binary prefixes for memory and disk (`128 MiB`, `1 GiB`) and decimal prefixes for network and bandwidth (`1 Mbps`, `100 MB/s`). (`MUST`)
- **Ports** are bare integers: `3001`, never `:3001` in prose. (`MUST`)
- **Always use the same number once for the same concept.** If the dev port is `3001` in one doc, it is `3001` in every doc. Hard-coded duplication is fine; mismatch is the failure. (`MUST`)

### 2.6 Terminology

These exact spellings and capitalizations are required across all documentation. Inconsistent terminology is rejected.

| Correct | Wrong |
|---|---|
| Vedox | vedox, VEDOX, vedox.dev (in prose) |
| Markdown | markdown, MarkDown |
| Git | git (in prose), GIT |
| GitHub | Github, github |
| SQLite | Sqlite, sqlite (in prose; lowercase OK in code) |
| FTS5 | fts5 |
| SvelteKit | Sveltekit, svelte-kit |
| Tiptap | TipTap, tip-tap |
| Go | go (when referring to the language; lowercase OK in code blocks) |
| ADR | adr (in prose), Adr |
| README | Readme, readme (when referring to the document type) |
| frontmatter | front matter, front-matter |
| local-first | local first, localfirst |
| dogfooding | dog-fooding |
| `vedox dev`, `vedox build`, `vedox reindex` | Always in code spans |
| `127.0.0.1` | localhost (use only when explicitly distinguishing from `127.0.0.1`) |

### 2.7 Code blocks

- **Every fenced code block has a language tag.** ` ```sh `, ` ```go `, ` ```typescript `, ` ```yaml `, ` ```json `, ` ```sql `, ` ```markdown `. A code fence with no language is rejected. (`MUST`)
- **Use `sh` for shell commands**, not `bash`, `shell`, `console`, or `terminal`. The single canonical tag prevents the linter from generating false positives. (`MUST`)
- **Commands are runnable as shown.** Pseudocode is forbidden in how-tos and runbooks. Pseudocode is permitted in explanations and ADRs only when clearly labeled as such. (`MUST`)
- **Expected output is shown immediately below the command** with a comment line explaining what to expect, or as a separate code block introduced by the words "Expected output:" on the line above. (`MUST`)
- **Never include shell prompts** (`$`, `#`, `>`) in `sh` blocks. The reader copies the line as-is. (`MUST`)
- **Long lines wrap with backslash continuation**, not unwrapped. (`SHOULD`)

---

## 3. Universal Frontmatter Schema

Every document, regardless of type, MUST contain these fields. Type-specific fields in Section 4 are added to this set. Unknown fields are rejected.

```yaml
---
title: "<string, 5..120 chars>"
type: "<enum: see Section 4>"
status: "<enum: draft | review | published | deprecated | superseded>"
date: "<YYYY-MM-DD>"
project: "<kebab-case slug>"
tags: ["<2..8 kebab-case tags>"]
author: "<string or '@persona-name'>"
---
```

| Field | Type | Required | Validation |
|---|---|---|---|
| `title` | string | yes | 5..120 characters; no leading/trailing whitespace; matches the H1 |
| `type` | enum | yes | One of the values in Section 4.0 |
| `status` | enum | yes | `draft`, `review`, `published`, `deprecated`, or `superseded` |
| `date` | date | yes | ISO 8601 `YYYY-MM-DD`; not in the future |
| `project` | string | yes | kebab-case, 2..40 characters, matches a project that exists in `vedox.config.ts` |
| `tags` | array<string> | yes | 2..8 entries; each kebab-case; lowercase; alphanumeric + hyphen |
| `author` | string | yes | Either a human-readable name or `@persona-name` (e.g. `@staff-engineer`) for AI agents |

Optional universal fields permitted on any document:

| Field | Type | Validation |
|---|---|---|
| `summary` | string | 50..280 chars; one-sentence elevator pitch surfaced in search results and link previews |
| `redirect_from` | array<string> | List of old paths that should redirect to this file (Section 9.5) |
| `superseded_by` | string | Path of the document that replaces this one; only valid when `status: superseded` |
| `last_reviewed` | date | ISO 8601; the date a human last verified the content is still accurate |
| `audience` | enum | `developer`, `operator`, `agent`, `all-writers-human-and-agent`, `internal-team` |
| `reading_time_minutes` | integer | 1..60; computed by tooling, may be set manually |

Forbidden top-level fields (these are reserved for future use and any usage is rejected):

`id`, `slug`, `permalink`, `layout`, `template`, `category`, `categories`, `published`, `draft`, `weight`, `nav_order`, `parent`, `children`. The IA layer (parent/children/order) is computed from directory structure plus DESIGN_FRAMEWORK.md sidebar config; it is never declared per-file.

---

## 4. Content Types

Vedox documentation has exactly **eleven** content types. Every document is exactly one of these. Pick the right one before you start writing.

### 4.0 The complete enum

```yaml
type: adr             # Architecture Decision Record
type: how-to          # Task-oriented procedural guide
type: runbook         # Operational/incident response procedure
type: readme          # Project front-door overview
type: api-reference   # Interface reference (HTTP API, CLI, SDK)
type: explanation     # Conceptual / background "why" document
type: issue           # Bug report, feature request, or postmortem
type: platform        # Product/platform documentation page
type: infrastructure  # Deployment, environment, IaC reference
type: network         # Network, ports, protocols, security boundary doc
type: logging         # Log format, observability, log convention reference
```

There are no other types. If a document does not fit any of these, it does not belong in `docs/`. The most common mistake is reaching for "explanation" as a fallback; explanation is for genuine conceptual background (the "why"), not "I have content but no slot."

**Tutorial is explicitly out of scope for v1.** Diataxis defines four content types (tutorial, how-to, reference, explanation); Vedox ships only three (how-to, reference variants, explanation) plus operational types. Task-oriented learning content is expressed as a `how-to` — the how-to template is rich enough to carry both teaching and procedural content. Do not propose adding a `tutorial` type. If a document genuinely needs a teaching narrative that a how-to cannot carry, raise it with the CEO; the answer in v1 will still be "write a how-to."

### 4.1 type: `adr` — Architecture Decision Record

**Purpose.** A short, durable record of a significant technical decision: what was decided, what alternatives were rejected, and what the consequences are. ADRs exist to prevent re-litigating the same decisions at every architecture review.

**Not for.** Trivial implementation choices. Restating a feature spec. Writing a meeting minutes summary. Documenting code that is its own documentation.

**File location.** `docs/adr/`
**Glob.** `docs/adr/[0-9][0-9][0-9]-*.md`
**Filename pattern.** `NNN-kebab-case-title.md` where `NNN` is a zero-padded three-digit sequential ID. Example: `003-redis-for-sessions.md`. Numbers are permanent and never reused; gaps are acceptable, reuse is not.
**ID prefix in title.** `ADR-NNN: <Decision summary>`. The `title` field MUST start with `ADR-NNN: `.

**Required frontmatter (in addition to universal):**

```yaml
type: adr
status: "<enum: proposed | accepted | superseded | deprecated>"
superseded_by: ""              # path to replacement ADR, or empty string
decision_date: "<YYYY-MM-DD>"  # the date the decision was made (may differ from `date`)
deciders: ["@persona-1", "@persona-2"]   # who signed off
```

Status enum for ADRs is more restrictive than the global enum: only `proposed`, `accepted`, `superseded`, or `deprecated`. `draft` is forbidden — an ADR is either proposed and being decided, or it's not an ADR yet.

**Required heading order.**

```
H1: ADR-NNN: <Decision summary>      (matches title exactly)
H2: Context
H2: Decision
H2: Consequences
H2: Alternatives Considered
```

`Alternatives Considered` MUST contain at least two alternatives, each as an H3 subheading, and each MUST state explicitly why it was rejected. An ADR with one alternative was a foregone conclusion and is not a decision.

**Tone.** Past-tense for what was decided. Present-tense for the consequences. No hedging.
**Length.** 600..2000 words. ADRs longer than 2000 words are usually two ADRs.

**Cross-linking.**
- Reference other ADRs by their ID and a relative link: `[ADR-001](./001-markdown-as-source-of-truth.md)`.
- Reference code by `path/from/repo/root.go:LINE` (Section 9.4).
- Reference epics or tickets in `.tasks/` only by their epic name and never embed task-tracker URLs (the blackboard is internal state, not documentation).

**Lifecycle.**
- `proposed` -> `accepted` (the team agreed)
- `proposed` -> `superseded` (a later ADR replaces this one before it was accepted; rare)
- `accepted` -> `superseded` (a later ADR explicitly replaces it; `superseded_by` MUST point to the replacement)
- `accepted` -> `deprecated` (the decision is no longer relevant but no replacement exists)

**Acceptance criteria checklist.**
- Filename matches `^[0-9]{3}-[a-z0-9-]+\.md$`
- `title` starts with `ADR-NNN: ` and matches the H1
- All four required H2 sections present in order
- `Alternatives Considered` has >= 2 H3 subsections, each with a "Rejected because" sentence
- `status` is one of the four ADR statuses
- `superseded_by` empty unless `status: superseded`
- Word count between 600 and 2000

### 4.2 type: `how-to` — Task-Oriented Procedural Guide

**Purpose.** Tells a competent reader how to accomplish one specific task. The reader knows what they want; they just need the steps.

**Not for.** Tutorials (which teach concepts as they go), explanations, or any document that needs a "background" section.

**File location.** `docs/how-to/`
**Glob.** `docs/how-to/*.md`
**Filename pattern.** `<verb>-<noun>.md`, kebab-case. Example: `add-a-project.md`, `deploy-to-staging.md`. Filename starts with an imperative verb (add, configure, deploy, debug, install, update, remove, enable, disable, build, run, test).

**Required frontmatter (in addition to universal):**

```yaml
type: how-to
difficulty: "<enum: beginner | intermediate | advanced>"
estimated_time_minutes: <integer 1..60>
prerequisites: ["<short prereq>", "<short prereq>"]   # 1..6 items
```

**Required heading order.**

```
H1: How to <verb> <noun>     (matches title)
[one-sentence purpose statement, no heading]
H2: Prerequisites
H2: Steps
H2: Verification
H2: Troubleshooting           (optional but strongly recommended)
```

The one-sentence purpose statement immediately after the H1, before any H2, is mandatory. It is the answer to "what will I have at the end?"

**Tone.** Imperative, direct, copy-pasteable.
**Length.** Hard cap of 500 words. The cap exists to prevent background creep; if you cannot fit the task in 500 words, you are documenting two tasks or you are mixing in an explanation.

**Cross-linking.** A how-to MUST link to the relevant explanation document if there is one ("For background, see [...]"). It MUST link to the relevant runbook if it documents a recovery action.

**Lifecycle.** Standard global lifecycle (`draft -> review -> published -> deprecated`).

**Acceptance criteria checklist.**
- Filename starts with an imperative verb
- One-sentence purpose statement present between H1 and Prerequisites
- All required H2 sections present in order
- Word count <= 500
- Every code block has a language tag
- Every numbered step is one action; no step contains the word "and" joining two verbs

### 4.3 type: `runbook` — Operational / Incident Response Procedure

**Purpose.** A documented procedure for responding to a specific operational event. Written for an engineer paged at 3am with no prior context.

**Not for.** Architecture explanations. Postmortems (use `type: issue` for those). Routine maintenance that does not have a paging trigger.

**File location.** `docs/runbooks/`
**Glob.** `docs/runbooks/*.md`
**Filename pattern.** `<symptom-or-system>-<short-symptom>.md`. Example: `vedox-not-loading-workspace.md`, `indexer-stuck-on-large-workspace.md`. Symptom-first naming is preferred over system-first because runbooks are searched by symptom in the heat of an incident.

**Required frontmatter (in addition to universal):**

```yaml
type: runbook
on_call_severity: "<enum: P1 | P2 | P3 | P4>"
last_tested: "<YYYY-MM-DD>"
target_time_to_mitigate_minutes: <integer>
related_error_codes: ["VDX-NNN", "VDX-NNN"]   # may be empty array
service: "<service slug>"
```

**Required heading order.**

```
H1: <Problem statement>          (matches title; symptom-first wording)
H2: Symptoms
H2: Immediate Actions
H2: Root Cause Investigation
H2: Resolution Steps
H2: Verification
H2: Prevention
```

`Resolution Steps` SHOULD contain one H3 subsection per root cause case (Case A, Case B, ...). Each case maps to one check from `Root Cause Investigation`.

**Tone.** Imperative, ultra-direct, zero ambiguity. Numbered steps; one action per step.
**Length.** No hard cap, but Immediate Actions MUST be 5 steps or fewer. Steps that contain the word "and" joining two verbs are split.

**The `last_tested` rule.** A runbook with a `last_tested` date older than 90 days is mechanically flagged as **stale** by CI and surfaces a warning badge in the UI. A stale runbook is presumed incorrect until proven otherwise. The writer of the runbook is responsible for scheduling drills and updating `last_tested`.

**Lifecycle.** Standard, plus the implicit `stale` overlay computed from `last_tested`.

**Acceptance criteria checklist.**
- Filename starts with system or symptom keyword
- All seven required H2 sections present in order
- `Immediate Actions` has <= 5 numbered steps
- Every command has expected output or an explicit "no output expected"
- `last_tested` <= 90 days from `date` at submission time
- `on_call_severity` is one of `P1`, `P2`, `P3`, `P4`
- `related_error_codes` references only codes that exist in the error catalog

### 4.4 type: `readme` — Project Front Door

**Purpose.** The first thing a new engineer reads when they encounter a project. Answers in under 2 minutes: what is this, how do I run it, how do I contribute.

**Not for.** Anything that needs more than 500 lines. Anything that documents a specific procedure (use how-to). Anything that documents a decision (use ADR).

**File location.** Project root within the workspace; surfaces in `docs/` only as a symlink or imported copy via the workspace scanner.
**Glob.** `**/README.md`
**Filename pattern.** Exactly `README.md`. Always uppercase. No exceptions.

**Required frontmatter (in addition to universal):**

```yaml
type: readme
project_status: "<enum: experimental | beta | production | maintenance | archived>"
primary_language: "<string, e.g. go, typescript, python>"
license: "<SPDX identifier, e.g. PolyForm Shield 1.0.0, Apache-2.0>"
```

**Required heading order.**

```
H1: <project name>             (matches title and the package/repo name exactly)
[one-sentence headline blockquote, no heading]
H2: Overview
H2: Installation
H2: Usage
H2: Configuration              (only if non-trivial config exists)
H2: Contributing
H2: License
```

**Tone.** Factual, pragmatic.
**Length.** Hard cap of 500 lines. README documents that exceed this MUST extract sections into linked documents.

**Acceptance criteria checklist.**
- Filename is exactly `README.md`
- One-sentence blockquote headline immediately after H1
- All required H2 sections present in order (Configuration optional)
- Line count <= 500
- License section present and links to a `LICENSE` file
- Installation section's commands are runnable as shown

### 4.5 type: `api-reference` — Interface Reference

**Purpose.** Documents what an interface accepts and what it returns. Structured for scanning, not reading.

**File location.** `docs/api-reference/<surface>/`. Surfaces include `http`, `cli`, `mcp`, `sdk`.
**Glob.** `docs/api-reference/(http|cli|mcp|sdk)/*.md`
**Filename pattern.** `<resource-or-command>.md`. Example: `documents.md`, `vedox-dev.md`.

**Required frontmatter (in addition to universal):**

```yaml
type: api-reference
api_surface: "<enum: http | cli | mcp | sdk>"
api_version: "<semver or vN, e.g. v1, 0.3.0>"
stability: "<enum: stable | beta | experimental | deprecated>"
base_url: "<string>"             # http surface only
```

**Required heading order (HTTP surface).**

```
H1: <Resource name>
H2: Overview
H2: Authentication
H2: Endpoints                   (each endpoint is an H3: METHOD /path)
H2: Error Codes
H2: Examples                    (optional)
```

**Required heading order (CLI surface).**

```
H1: <Command name>
H2: Synopsis
H2: Description
H2: Flags
H2: Exit Codes
H2: Examples
```

**Required heading order (MCP surface).**

```
H1: <Tool name>
H2: Description
H2: Input Schema
H2: Output Schema
H2: Errors
H2: Examples
```

**Tone.** Reference, not narrative. Tables wherever possible. Each endpoint or command is independently complete; do not chain ("see above").

**Length.** No cap. Group operations on one resource into one file rather than per-endpoint files.

**Acceptance criteria checklist.**
- Located under `docs/api-reference/<surface>/`
- `api_surface` matches the parent directory
- All required headings for the surface are present
- Every endpoint/command lists every error code it can return
- `Authentication` section present even when the answer is "none"

### 4.6 type: `explanation` — Conceptual Background

**Purpose.** Explains why the system works the way it does. For readers who want to understand a concept rather than perform a task.

**Not for.** Step-by-step procedures (how-to). Decision records (ADR). API specs (api-reference).

**File location.** `docs/explanation/`
**Glob.** `docs/explanation/*.md`
**Filename pattern.** `<concept>.md`, kebab-case. Example: `markdown-as-source-of-truth.md`, `the-blackboard-pattern.md`.

**Required frontmatter (in addition to universal):**

```yaml
type: explanation
related_adrs: ["adr/001-markdown-as-source-of-truth.md"]   # 0..N
audience: "<enum: developer | operator | agent | internal-team>"
```

**Required heading order.**

```
H1: <Concept name>
H2: Overview
H2: <topic-specific H2s>
H2: See Also
```

The body between H2s is free-form prose, but must remain prose, not a checklist. Explanations that devolve into checklists are how-tos in disguise and should be split.

**Tone.** Considered, expository, present-tense.
**Length.** 800..3000 words. Longer than 3000, split.

### 4.7 type: `issue` — Bug Report, Feature Request, or Postmortem

**Purpose.** Captures a defect, a request, or an after-action review of a real incident. Issues are documentation, not tickets — they live alongside the system they describe and survive after the ticket tracker forgets them.

**Not for.** Open ideas (those live in `.tasks/`). Vague wishlists. Anything still being triaged.

**File location.** `docs/issues/<kind>/`. Kinds: `bug`, `feature`, `postmortem`.
**Glob.** `docs/issues/(bug|feature|postmortem)/[A-Z]{3}-[0-9]{4}-*.md`
**Filename pattern.** `<PREFIX>-NNNN-<kebab-summary>.md`. Prefixes: `BUG`, `FRQ` (feature request), `PMR` (postmortem). Numbers are sequential per prefix and never reused. Example: `BUG-0042-port-conflict-on-restart.md`.

**Required frontmatter (in addition to universal):**

```yaml
type: issue
issue_kind: "<enum: bug | feature | postmortem>"
issue_id: "<PREFIX-NNNN>"                  # matches filename
severity: "<enum: P1 | P2 | P3 | P4>"      # bug & postmortem only
state: "<enum: open | triaged | in-progress | resolved | wont-fix | duplicate>"
related_runbooks: ["runbooks/...md"]       # 0..N
related_adrs: ["adr/...md"]                # 0..N
incident_start: "<YYYY-MM-DD>"             # postmortem only
incident_end: "<YYYY-MM-DD>"               # postmortem only
detection_method: "<string>"               # postmortem only
```

**Required heading order (`bug`).**

```
H1: BUG-NNNN: <one-sentence symptom>
H2: Summary
H2: Steps to Reproduce
H2: Expected Behavior
H2: Actual Behavior
H2: Environment
H2: Workaround
H2: Resolution
```

**Required heading order (`feature`).**

```
H1: FRQ-NNNN: <one-sentence request>
H2: Problem
H2: Proposed Solution
H2: User Stories
H2: Acceptance Criteria
H2: Out of Scope
```

**Required heading order (`postmortem`).**

```
H1: PMR-NNNN: <one-sentence incident summary>
H2: Summary
H2: Timeline
H2: Impact
H2: Root Cause
H2: Detection
H2: Response
H2: Resolution
H2: Lessons Learned
H2: Action Items
```

Postmortems are blameless. Names of individuals are forbidden in postmortem text; refer to roles (`@on-call`, `@release-engineer`).

**Length.** Bugs <= 800 words. Feature requests <= 1200 words. Postmortems <= 3000 words.

**Acceptance criteria checklist.**
- Filename matches `^(BUG|FRQ|PMR)-[0-9]{4}-[a-z0-9-]+\.md$`
- `issue_id` matches the filename prefix and number
- `issue_kind` matches the directory under `docs/issues/`
- `severity` present for bugs and postmortems, absent for feature requests
- Postmortem contains zero individual names

### 4.8 type: `platform` — Product / Platform Documentation

**Purpose.** End-user documentation of a product surface or feature: what it does, when to use it, what its boundaries are. The "manual" reader, not the "engineer integrating" reader (who reads `api-reference`).

**File location.** `docs/platform/<area>/`. Areas: `editor`, `workspace`, `templates`, `search`, `publishing`, `agents`. New areas require an ADR.
**Glob.** `docs/platform/*/*.md`
**Filename pattern.** `<feature>.md`, kebab-case.

**Required frontmatter (in addition to universal):**

```yaml
type: platform
platform_area: "<enum: editor | workspace | templates | search | publishing | agents>"
ui_surface: "<string, e.g. sidebar, command-palette, settings>"
since_version: "<semver, e.g. 0.1.0>"
stability: "<enum: stable | beta | experimental | deprecated>"
```

**Required heading order.**

```
H1: <Feature name>
H2: Overview
H2: When to Use This
H2: How It Works
H2: Limits and Edge Cases
H2: Related
```

**Tone.** End-user voice; assume the reader is a user, not an engineer. Avoid implementation detail. Link to ADRs and explanation docs for the "why."

**Length.** 400..1500 words.

### 4.9 type: `infrastructure` — Deployment / Environment / IaC

**Purpose.** Documents the infrastructure that runs Vedox or a service: environments, deployment procedures, IaC modules, machine images, container layouts.

**File location.** `docs/infrastructure/<environment>/`. Environments: `local`, `dev`, `staging`, `production`, `shared`.
**Glob.** `docs/infrastructure/*/*.md`
**Filename pattern.** `<topic>.md`, kebab-case.

**Required frontmatter (in addition to universal):**

```yaml
type: infrastructure
environment: "<enum: local | dev | staging | production | shared>"
provider: "<string, e.g. local, gcp, aws, fly, docker, k8s>"
iac_tool: "<enum: terraform | pulumi | helm | ansible | docker-compose | makefile | none>"
data_classification: "<enum: public | internal | confidential | restricted>"
related_runbooks: ["runbooks/...md"]
```

**Required heading order.**

```
H1: <Topic>
H2: Overview
H2: Topology
H2: Configuration
H2: Deployment
H2: Rollback
H2: Observability
H2: Disaster Recovery
H2: Cost
```

`Disaster Recovery` MUST cite the runbook that executes it. `Rollback` MUST be a complete procedure, not "see git history."

**Tone.** Operations-first. Commands are copy-pasteable. Variables are placeholders the reader substitutes (`<region>`, `<project-id>`).

**Length.** No cap, but each section should fit on one screen.

### 4.10 type: `network` — Network, Ports, Protocols, Security Boundaries

**Purpose.** Documents how a service or component talks to the network: bind addresses, listening ports, protocols, security boundaries, firewall rules, ingress, egress.

**File location.** `docs/network/`
**Glob.** `docs/network/*.md`
**Filename pattern.** `<service>-<aspect>.md`. Example: `vedox-dev-server.md`, `vedox-api-tls-boundary.md`.

**Required frontmatter (in addition to universal):**

```yaml
type: network
service: "<service slug>"
bind_address: "<string, e.g. 127.0.0.1, 0.0.0.0>"
ports: [<integer>, ...]
protocols: ["<enum: http | https | sse | websocket | grpc | tcp | udp | unix>"]
direction: "<enum: ingress-only | egress-only | bidirectional>"
exposed_to: "<enum: loopback | lan | vpn | internet>"
authentication: "<enum: none | hmac | bearer | mtls | basic | os-keychain>"
encryption_in_transit: "<enum: none | tls-1.2 | tls-1.3>"
egress_destinations: ["<host:port>", ...]   # may be empty array
```

**Required heading order.**

```
H1: <Service> <aspect>
H2: Overview
H2: Bind and Ports
H2: Protocols
H2: Authentication and Encryption
H2: Egress
H2: Threat Model
H2: Operational Notes
```

**Critical rule.** Vedox is loopback-only by default. Any network document that proposes a non-loopback bind MUST link to an ADR that approved it. The linter rejects `bind_address: "0.0.0.0"` without a corresponding `related_adrs` entry.

**Length.** 500..2000 words.

### 4.11 type: `logging` — Log Format and Observability

**Purpose.** Documents what a service logs, how it logs it, where the logs go, and what fields callers can rely on. Also documents observability: metrics, traces, dashboards.

**File location.** `docs/logging/`
**Glob.** `docs/logging/*.md`
**Filename pattern.** `<service>-<aspect>.md`. Example: `vedox-cli-logs.md`, `vedox-http-access.md`.

**Required frontmatter (in addition to universal):**

```yaml
type: logging
service: "<service slug>"
log_format: "<enum: json | logfmt | text>"
log_destination: "<string, e.g. ~/.vedox/logs/, stdout, stderr, syslog>"
retention_days: <integer>
contains_pii: false                   # MUST be literal false; PII logs are forbidden
schema_version: "<integer>"           # increments on breaking field changes
fields: ["<field name>", ...]         # canonical list of log fields
related_runbooks: ["runbooks/...md"]
```

`contains_pii` MUST be literal `false`. Vedox logs contain no PII or file contents. A document that requires `contains_pii: true` is documenting a violation; the violation is fixed in code, not the document.

**Required heading order.**

```
H1: <Service> <aspect> logs
H2: Overview
H2: Format
H2: Field Reference
H2: Levels
H2: Rotation and Retention
H2: PII Policy
H2: Operational Queries
```

`Field Reference` is a table: field name, type, required, description, example. Every field listed in frontmatter `fields:` MUST appear in this table.

**Length.** 400..1500 words.

---

## 5. Naming Conventions Reference

This section consolidates every naming rule scattered across Section 4 so a writer can verify mechanically.

### 5.1 Filenames

| Type | Directory | Pattern | Example |
|---|---|---|---|
| `adr` | `docs/adr/` | `^[0-9]{3}-[a-z0-9-]+\.md$` | `003-redis-for-sessions.md` |
| `how-to` | `docs/how-to/` | `^(add|build|configure|debug|deploy|disable|enable|install|remove|run|test|update|use)-[a-z0-9-]+\.md$` | `deploy-to-staging.md` |
| `runbook` | `docs/runbooks/` | `^[a-z0-9][a-z0-9-]+\.md$` | `vedox-not-loading-workspace.md` |
| `readme` | project root | `^README\.md$` | `README.md` |
| `api-reference` | `docs/api-reference/(http|cli|mcp|sdk)/` | `^[a-z0-9-]+\.md$` | `documents.md` |
| `explanation` | `docs/explanation/` | `^[a-z0-9-]+\.md$` | `markdown-as-source-of-truth.md` |
| `issue` | `docs/issues/(bug|feature|postmortem)/` | `^(BUG|FRQ|PMR)-[0-9]{4}-[a-z0-9-]+\.md$` | `BUG-0042-port-conflict-on-restart.md` |
| `platform` | `docs/platform/*/` | `^[a-z0-9-]+\.md$` | `command-palette.md` |
| `infrastructure` | `docs/infrastructure/*/` | `^[a-z0-9-]+\.md$` | `local-dev-environment.md` |
| `network` | `docs/network/` | `^[a-z0-9-]+\.md$` | `vedox-dev-server.md` |
| `logging` | `docs/logging/` | `^[a-z0-9-]+\.md$` | `vedox-cli-logs.md` |

### 5.2 Identifier prefixes

| Prefix | Used by | Format | Permanence |
|---|---|---|---|
| `ADR-NNN` | ADR titles | three-digit zero-padded | permanent, never reused |
| `VDX-NNN` | Error codes (in code, referenced in runbooks) | three-digit zero-padded | permanent, never reused |
| `BUG-NNNN` | Bug issue files | four-digit zero-padded | permanent, never reused |
| `FRQ-NNNN` | Feature request issue files | four-digit zero-padded | permanent, never reused |
| `PMR-NNNN` | Postmortem issue files | four-digit zero-padded | permanent, never reused |

### 5.3 Slugs and project names

- A project slug is kebab-case, 2..40 chars, alphanumeric + hyphen, starts with a letter.
- A project slug MUST exist in the `projects:` allowlist of `vedox.config.ts` before any document with `project: "<slug>"` is accepted. The allowlist is the single source of truth; the linter reads it directly.
- Reserved slugs (rejected if used as project names): `vedox-internal`, `system`, `agents`, `admin`.

### 5.3.1 Sub-workspace pattern: `docs/projects/<slug>/`

When a workspace hosts documentation for more than one project, each non-primary project lives under `docs/projects/<slug>/`. The primary project (the workspace's own product) keeps the root layout (`docs/adr/`, `docs/how-to/`, etc.). Sub-projects mirror the same per-type subdirectories scoped to their slug:

```
docs/
  adr/                        # primary project (vedox)
  how-to/
  projects/
    <sub-project-slug>/
      adr/
      how-to/
      explanation/
```

Rules:
- A document under `docs/projects/<slug>/` MUST set `project: "<slug>"` in frontmatter and that slug MUST appear in the `vedox.config.ts` allowlist.
- A document in the root tree (`docs/adr/`, `docs/how-to/`, ...) MUST set `project:` to the primary project slug (currently `vedox`).
- Files placed at `docs/<slug>/...` (without the `projects/` prefix) are non-conforming and rejected by the linter; they must move to `docs/projects/<slug>/...`.

### 5.4 Tags

- Lowercase kebab-case.
- 2..8 tags per document.
- Each tag is 2..40 characters.
- Tags are de-duplicated and alphabetized by the linter on save (do not rely on order).
- A tag MUST be reused from the existing tag set if a near-synonym exists. The lint rule warns on tags with edit distance <= 2 from an existing tag.

### 5.5 Heading anchors

Anchors are derived deterministically from heading text by lowercasing, replacing non-alphanumerics with hyphens, and collapsing consecutive hyphens. Do not author anchors manually. Internal links to a heading MUST use the derived anchor and MUST be verified to resolve.

---

## 6. Required Sections Quick Reference

A single-screen index of every required heading in every type. If your document is missing any of the headings listed for its type, it is invalid.

| Type | H2 sections (in order) |
|---|---|
| `adr` | Context, Decision, Consequences, Alternatives Considered |
| `how-to` | Prerequisites, Steps, Verification, Troubleshooting (optional) |
| `runbook` | Symptoms, Immediate Actions, Root Cause Investigation, Resolution Steps, Verification, Prevention |
| `readme` | Overview, Installation, Usage, Configuration (optional), Contributing, License |
| `api-reference` (http) | Overview, Authentication, Endpoints, Error Codes, Examples (optional) |
| `api-reference` (cli) | Synopsis, Description, Flags, Exit Codes, Examples |
| `api-reference` (mcp) | Description, Input Schema, Output Schema, Errors, Examples |
| `explanation` | Overview, ..., See Also |
| `issue` (bug) | Summary, Steps to Reproduce, Expected Behavior, Actual Behavior, Environment, Workaround, Resolution |
| `issue` (feature) | Problem, Proposed Solution, User Stories, Acceptance Criteria, Out of Scope |
| `issue` (postmortem) | Summary, Timeline, Impact, Root Cause, Detection, Response, Resolution, Lessons Learned, Action Items |
| `platform` | Overview, When to Use This, How It Works, Limits and Edge Cases, Related |
| `infrastructure` | Overview, Topology, Configuration, Deployment, Rollback, Observability, Disaster Recovery, Cost |
| `network` | Overview, Bind and Ports, Protocols, Authentication and Encryption, Egress, Threat Model, Operational Notes |
| `logging` | Overview, Format, Field Reference, Levels, Rotation and Retention, PII Policy, Operational Queries |

---

## 7. Lifecycle and States

### 7.1 Universal lifecycle

```
draft -> review -> published -> deprecated
                              -> superseded   (ADRs only — see §4.1)
```

**Canonical status enum (CEO decision, 2026-04-07):** the five states above are the only legal `status:` values for non-ADR content. ADRs additionally use `proposed` and `accepted` (see §4.1). The values `approved` and `archived` — present in early scaffold code at `apps/editor/src/lib/editor/utils/frontmatter.ts` — are **not** part of the canonical enum; they are synonyms (`approved` = `published`, `archived` = `deprecated`) and the linter rejects them. The editor frontmatter helper file is on the Phase 2 migration list and must be updated by the creative-technologist to ship only the canonical enum.

**Linter alias rule for `accepted`:** the linter normalizes `status: accepted` on a non-ADR document to `status: published` and emits a warning (`LINT-W-001`). On an ADR document `accepted` is a valid terminal state and is left untouched. This is the only legal status alias; all others are hard rejects.

| State | Meaning | Set by | Visible where |
|---|---|---|---|
| `draft` | Being written. May be incomplete or wrong. | Author | Editor only; not surfaced in search by default |
| `review` | Submitted for human review. Frozen until accepted or rejected. | Author | Search, with a "review" badge |
| `published` | Accurate as of `date`. The default state for live content. | Reviewer | Search, sidebar, all surfaces |
| `deprecated` | Still accurate as historical record but no longer the recommended path. | Reviewer | Search with a "deprecated" badge; demoted in ranking |
| `superseded` | Replaced by another document at `superseded_by`. Reads should redirect after a grace period. | Reviewer | Search with a "superseded" badge; redirect in UI after 30 days |

### 7.2 Aging rules (mechanically enforced)

- A document in `draft` for more than **14 days** is flagged. The author receives a notification. After 30 days, the draft is moved to `docs/.attic/` and removed from the index.
- A document in `review` for more than **7 days** without a reviewer touching it is auto-escalated to a planner role (CEO or CTO).
- A `published` document with `last_reviewed` older than **180 days** is flagged for re-review but remains visible.
- A `runbook` with `last_tested` older than **90 days** is marked **stale** and surfaces a warning badge in the UI.

### 7.3 Deprecation procedure

To deprecate a document:

1. Set `status: deprecated` and update `date` to today.
2. Add a `> Deprecated as of YYYY-MM-DD. See [<replacement>](<path>) for the current guidance.` blockquote immediately after the H1.
3. If a replacement exists, set `superseded_by: "<path>"` and `status: superseded` instead.
4. Add the old path to the replacement's `redirect_from:` list.

---

## 8. Acceptance Criteria (Reviewer / Linter Checklist)

A reviewer (or the CI linter) verifies the following on every submission. Items marked **L** are linter-enforced; items marked **R** are reviewer-enforced.

```
[L] File path matches the type's directory and filename regex
[L] Frontmatter present, valid YAML, no unknown keys
[L] All required universal frontmatter fields present
[L] All required type-specific frontmatter fields present
[L] All enum values are within the allowed set
[L] `date` is ISO 8601 and not in the future
[L] H1 exists exactly once and matches `title`
[L] All required H2 sections present in declared order
[L] No emoji present
[L] No forbidden marketing words present
[L] No tab characters; LF line endings; UTF-8 no-BOM
[L] No trailing whitespace
[L] Word count within type's bounds
[L] Every code fence has a language tag
[L] No `bash` / `shell` / `console` language tags (must be `sh`)
[L] No HTML tags except permitted subset
[L] Every internal link resolves
[L] Every code reference of the form `path:LINE` resolves to an existing line
[L] No PII patterns matched (email, phone, credit card, SSN, IP-other-than-127.0.0.1, GitHub username followed by colon, AWS access key prefix, GCP service account email)
[L] No content from secret blocklist paths (`.env`, `*.pem`, `*.key`, `id_rsa`, `*.p12`, `credentials.json`, `~/.ssh/`)
[L] `contains_pii: false` for all `type: logging` documents
[L] `bind_address: "0.0.0.0"` requires a `related_adrs:` entry (network type)
[L] `last_tested` <= 90 days for runbooks
[R] Tone matches Section 2.4 voice rules
[R] Document is the right type (not a how-to disguised as an explanation, etc.)
[R] Cross-links are correct and relevant
[R] No duplication of content that already lives elsewhere
[R] Diagrams or images are correct and have meaningful alt text
[R] The document fulfils its declared purpose (Section 4)
```

---

## 9. Cross-Reference and Linking Rules

### 9.1 Internal document links

- Use workspace-relative paths from the current document, not absolute paths. (`MUST`)
- Use `./` and `../` explicitly for adjacent and parent directories. (`MUST`)
- Link text is the target document's `title` field, not "click here," "this," or the bare URL. (`MUST`)
- Link to a heading using the deterministic anchor (Section 5.5). (`MUST`)

Correct: `See [ADR-001: Markdown as Source of Truth](../adr/001-markdown-as-source-of-truth.md).`
Wrong: `See [this ADR](https://example.com/adr/001).`

### 9.2 External links

- HTTPS only. (`MUST`)
- Long-lived destinations only (RFCs, language docs, vendor official docs). (`SHOULD`)
- Never link to a Slack message, an internal ticket URL, or a Google Doc. Those are not durable. (`MUST`)
- Add a `last-checked` HTML comment after each external link: `<!-- last-checked: 2026-04-07 -->`. (`SHOULD`)

### 9.3 Code references

Reference source code by path and line number using the canonical form:

```
path/from/repo/root.go:LINE
path/from/repo/root.go:LINE-LINE       (range)
```

Examples:

```
apps/cli/internal/errors/errors.go:27
apps/cli/internal/db/migrations/001_initial.sql:7-25
```

The linter verifies that the file exists, the line range is in bounds, and (optionally) that the line content has not drifted since `date`. (`MUST`)

### 9.4 Ticket and epic references

- Reference epics by their epic name only: `EPIC-001 Vedox Platform`. Do not link to `.tasks/` paths from `docs/`. (`MUST`)
- Reference an issue document by its ID: `BUG-0042`, `PMR-0001`. The ID resolves to the file in `docs/issues/` via the linter. (`MUST`)

### 9.5 Redirects after rename

When a document is renamed or moved:

1. Add the **old** path to the renamed document's `redirect_from:` array.
2. The build emits a redirect from the old path to the new path.
3. Old paths are kept in `redirect_from:` indefinitely. They are append-only.

```yaml
redirect_from:
  - "explanation/markdown-source.md"
  - "explanation/why-markdown.md"
```

### 9.6 Permitted HTML subset

HTML in Markdown is forbidden for agents. For human authors, the permitted subset is:

| Tag | Allowed | Notes |
|---|---|---|
| `<details>` / `<summary>` | yes | For collapsible sections in long references |
| `<kbd>` | yes | For keyboard shortcuts |
| `<sub>` / `<sup>` | yes | For technical notation |
| `<br>` | no | Use a blank line |
| `<a>` | no | Use Markdown link syntax |
| `<img>` | no | Use Markdown image syntax |
| `<div>`, `<span>`, `<style>`, `<script>` | no | Permanently forbidden |

---

## 10. Assets, Images, and Diagrams

### 10.1 Where assets live

- Each document directory has a `_assets/` subdirectory for that directory's assets.
- Asset filenames are kebab-case and start with the parent document's slug: `<doc-slug>-<descriptor>.<ext>`.
- Example: `docs/adr/_assets/001-markdown-source-of-truth-overview.svg` is owned by `docs/adr/001-markdown-as-source-of-truth.md`.
- Cross-document assets that are genuinely shared go in `docs/_shared-assets/` and require a comment in the file explaining ownership.

### 10.2 File formats

| Use case | Format | Notes |
|---|---|---|
| Diagrams | SVG (preferred) or Mermaid in-line | Mermaid is rendered client-side; see DESIGN_FRAMEWORK.md for theming |
| Screenshots | PNG | 2x DPI minimum; max width 2400 px; max file size 500 KB |
| Photos | WebP | max 500 KB |
| Animations | none | Animated GIFs and videos forbidden in `docs/`. Link to an external host if essential. |

### 10.3 Alt text

- Every image MUST have non-empty alt text. (`MUST`)
- Alt text describes the image's content, not the file. ("Sequence diagram: client publishes a doc, server commits, returns 201" not "diagram.svg.") (`MUST`)
- Alt text is a complete sentence ending in a period. (`MUST`)

### 10.4 Diagrams as code

Diagrams that change over time SHOULD be authored as Mermaid in the document body, not as exported images. Static SVGs are acceptable when the diagram tool is something else (Excalidraw, draw.io); in that case the source file (`.excalidraw`, `.drawio`) is committed alongside the SVG in `_assets/`.

---

## 11. Machine-Readable Schemas

The following schemas are the authoritative machine-readable contract. Linters and CI MUST validate every document against the appropriate schema. Editor tooling MAY use them for autocomplete and inline validation.

### 11.1 Universal frontmatter schema (JSON Schema 2020-12)

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://vedox.dev/schemas/frontmatter/universal.json",
  "title": "Vedox Universal Frontmatter",
  "type": "object",
  "additionalProperties": false,
  "required": ["title", "type", "status", "date", "project", "tags", "author"],
  "properties": {
    "title":   { "type": "string", "minLength": 5, "maxLength": 120 },
    "type":    {
      "type": "string",
      "enum": [
        "adr", "how-to", "runbook", "readme", "api-reference",
        "explanation", "issue", "platform", "infrastructure",
        "network", "logging"
      ]
    },
    "status":  {
      "type": "string",
      "enum": ["draft", "review", "published", "deprecated", "superseded",
               "proposed", "accepted"]
    },
    "date":    { "type": "string", "format": "date" },
    "project": { "type": "string", "pattern": "^[a-z][a-z0-9-]{1,39}$" },
    "tags":    {
      "type": "array",
      "minItems": 2,
      "maxItems": 8,
      "uniqueItems": true,
      "items": { "type": "string", "pattern": "^[a-z0-9][a-z0-9-]{1,39}$" }
    },
    "author":  { "type": "string", "minLength": 2, "maxLength": 80 },

    "summary":              { "type": "string", "minLength": 50, "maxLength": 280 },
    "redirect_from":        { "type": "array", "items": { "type": "string" } },
    "superseded_by":        { "type": "string" },
    "last_reviewed":        { "type": "string", "format": "date" },
    "audience":             {
      "type": "string",
      "enum": ["developer", "operator", "agent",
               "all-writers-human-and-agent", "internal-team"]
    },
    "reading_time_minutes": { "type": "integer", "minimum": 1, "maximum": 60 }
  }
}
```

### 11.2 Per-type schemas (additive)

Each per-type schema extends the universal schema by adding required and optional fields. The linter applies the universal schema first, then the per-type schema.

#### 11.2.1 `adr`

```json
{
  "$id": "https://vedox.dev/schemas/frontmatter/adr.json",
  "allOf": [{ "$ref": "universal.json" }],
  "type": "object",
  "required": ["decision_date", "deciders"],
  "properties": {
    "type":   { "const": "adr" },
    "status": { "enum": ["proposed", "accepted", "superseded", "deprecated"] },
    "superseded_by": { "type": "string" },
    "decision_date": { "type": "string", "format": "date" },
    "deciders": {
      "type": "array",
      "minItems": 1,
      "items": { "type": "string", "pattern": "^@?[a-z][a-z0-9-]+$" }
    }
  }
}
```

#### 11.2.2 `how-to`

```json
{
  "$id": "https://vedox.dev/schemas/frontmatter/how-to.json",
  "allOf": [{ "$ref": "universal.json" }],
  "type": "object",
  "required": ["difficulty", "estimated_time_minutes", "prerequisites"],
  "properties": {
    "type":       { "const": "how-to" },
    "difficulty": { "enum": ["beginner", "intermediate", "advanced"] },
    "estimated_time_minutes": { "type": "integer", "minimum": 1, "maximum": 60 },
    "prerequisites": {
      "type": "array",
      "minItems": 1,
      "maxItems": 6,
      "items": { "type": "string", "minLength": 5, "maxLength": 200 }
    }
  }
}
```

#### 11.2.3 `runbook`

```json
{
  "$id": "https://vedox.dev/schemas/frontmatter/runbook.json",
  "allOf": [{ "$ref": "universal.json" }],
  "type": "object",
  "required": ["on_call_severity", "last_tested",
               "target_time_to_mitigate_minutes", "service"],
  "properties": {
    "type":             { "const": "runbook" },
    "on_call_severity": { "enum": ["P1", "P2", "P3", "P4"] },
    "last_tested":      { "type": "string", "format": "date" },
    "target_time_to_mitigate_minutes": { "type": "integer", "minimum": 1 },
    "related_error_codes": {
      "type": "array",
      "items": { "type": "string", "pattern": "^VDX-[0-9]{3}$" }
    },
    "service": { "type": "string", "pattern": "^[a-z][a-z0-9-]+$" }
  }
}
```

#### 11.2.4 `readme`

```json
{
  "$id": "https://vedox.dev/schemas/frontmatter/readme.json",
  "allOf": [{ "$ref": "universal.json" }],
  "type": "object",
  "required": ["project_status", "primary_language", "license"],
  "properties": {
    "type": { "const": "readme" },
    "project_status": {
      "enum": ["experimental", "beta", "production", "maintenance", "archived"]
    },
    "primary_language": { "type": "string" },
    "license": { "type": "string" }
  }
}
```

#### 11.2.5 `api-reference`

```json
{
  "$id": "https://vedox.dev/schemas/frontmatter/api-reference.json",
  "allOf": [{ "$ref": "universal.json" }],
  "type": "object",
  "required": ["api_surface", "api_version", "stability"],
  "properties": {
    "type":        { "const": "api-reference" },
    "api_surface": { "enum": ["http", "cli", "mcp", "sdk"] },
    "api_version": { "type": "string", "pattern": "^(v[0-9]+|[0-9]+\\.[0-9]+\\.[0-9]+)$" },
    "stability":   { "enum": ["stable", "beta", "experimental", "deprecated"] },
    "base_url":    { "type": "string", "format": "uri" }
  }
}
```

#### 11.2.6 `explanation`

```json
{
  "$id": "https://vedox.dev/schemas/frontmatter/explanation.json",
  "allOf": [{ "$ref": "universal.json" }],
  "type": "object",
  "required": ["audience"],
  "properties": {
    "type":         { "const": "explanation" },
    "audience":     {
      "enum": ["developer", "operator", "agent", "internal-team"]
    },
    "related_adrs": { "type": "array", "items": { "type": "string" } }
  }
}
```

#### 11.2.7 `issue`

```json
{
  "$id": "https://vedox.dev/schemas/frontmatter/issue.json",
  "allOf": [{ "$ref": "universal.json" }],
  "type": "object",
  "required": ["issue_kind", "issue_id", "state"],
  "properties": {
    "type":       { "const": "issue" },
    "issue_kind": { "enum": ["bug", "feature", "postmortem"] },
    "issue_id":   { "type": "string", "pattern": "^(BUG|FRQ|PMR)-[0-9]{4}$" },
    "severity":   { "enum": ["P1", "P2", "P3", "P4"] },
    "state":      {
      "enum": ["open", "triaged", "in-progress", "resolved",
               "wont-fix", "duplicate"]
    },
    "related_runbooks": { "type": "array", "items": { "type": "string" } },
    "related_adrs":     { "type": "array", "items": { "type": "string" } },
    "incident_start":   { "type": "string", "format": "date" },
    "incident_end":     { "type": "string", "format": "date" },
    "detection_method": { "type": "string" }
  }
}
```

#### 11.2.8 `platform`

```json
{
  "$id": "https://vedox.dev/schemas/frontmatter/platform.json",
  "allOf": [{ "$ref": "universal.json" }],
  "type": "object",
  "required": ["platform_area", "ui_surface", "since_version", "stability"],
  "properties": {
    "type":          { "const": "platform" },
    "platform_area": {
      "enum": ["editor", "workspace", "templates", "search",
               "publishing", "agents"]
    },
    "ui_surface":    { "type": "string" },
    "since_version": { "type": "string", "pattern": "^[0-9]+\\.[0-9]+\\.[0-9]+$" },
    "stability":     { "enum": ["stable", "beta", "experimental", "deprecated"] }
  }
}
```

#### 11.2.9 `infrastructure`

```json
{
  "$id": "https://vedox.dev/schemas/frontmatter/infrastructure.json",
  "allOf": [{ "$ref": "universal.json" }],
  "type": "object",
  "required": ["environment", "provider", "iac_tool", "data_classification"],
  "properties": {
    "type":        { "const": "infrastructure" },
    "environment": { "enum": ["local", "dev", "staging", "production", "shared"] },
    "provider":    { "type": "string" },
    "iac_tool":    {
      "enum": ["terraform", "pulumi", "helm", "ansible",
               "docker-compose", "makefile", "none"]
    },
    "data_classification": {
      "enum": ["public", "internal", "confidential", "restricted"]
    },
    "related_runbooks": { "type": "array", "items": { "type": "string" } }
  }
}
```

#### 11.2.10 `network`

```json
{
  "$id": "https://vedox.dev/schemas/frontmatter/network.json",
  "allOf": [{ "$ref": "universal.json" }],
  "type": "object",
  "required": ["service", "bind_address", "ports", "protocols",
               "direction", "exposed_to", "authentication",
               "encryption_in_transit"],
  "properties": {
    "type":         { "const": "network" },
    "service":      { "type": "string", "pattern": "^[a-z][a-z0-9-]+$" },
    "bind_address": { "type": "string" },
    "ports": {
      "type": "array",
      "minItems": 1,
      "items": { "type": "integer", "minimum": 1, "maximum": 65535 }
    },
    "protocols": {
      "type": "array",
      "minItems": 1,
      "items": {
        "enum": ["http", "https", "sse", "websocket",
                 "grpc", "tcp", "udp", "unix"]
      }
    },
    "direction":  { "enum": ["ingress-only", "egress-only", "bidirectional"] },
    "exposed_to": { "enum": ["loopback", "lan", "vpn", "internet"] },
    "authentication": {
      "enum": ["none", "hmac", "bearer", "mtls", "basic", "os-keychain"]
    },
    "encryption_in_transit": { "enum": ["none", "tls-1.2", "tls-1.3"] },
    "egress_destinations":   { "type": "array", "items": { "type": "string" } },
    "related_adrs":          { "type": "array", "items": { "type": "string" } }
  },
  "allOf": [
    {
      "if":   { "properties": { "bind_address": { "const": "0.0.0.0" } } },
      "then": { "required": ["related_adrs"] }
    }
  ]
}
```

#### 11.2.11 `logging`

```json
{
  "$id": "https://vedox.dev/schemas/frontmatter/logging.json",
  "allOf": [{ "$ref": "universal.json" }],
  "type": "object",
  "required": ["service", "log_format", "log_destination",
               "retention_days", "contains_pii", "schema_version", "fields"],
  "properties": {
    "type":            { "const": "logging" },
    "service":         { "type": "string", "pattern": "^[a-z][a-z0-9-]+$" },
    "log_format":      { "enum": ["json", "logfmt", "text"] },
    "log_destination": { "type": "string" },
    "retention_days":  { "type": "integer", "minimum": 0, "maximum": 3650 },
    "contains_pii":    { "const": false },
    "schema_version":  { "type": "integer", "minimum": 1 },
    "fields":          {
      "type": "array",
      "minItems": 1,
      "items": { "type": "string", "pattern": "^[a-z][a-z0-9_]+$" }
    },
    "related_runbooks": { "type": "array", "items": { "type": "string" } }
  }
}
```

---

## 12. Cross-Functional Concerns Addressed

This framework incorporates the explicit demands of three roles. Each subsection lists what that role requires of the framework and how the framework satisfies it.

### 12.1 Data engineer

| Requirement | How the framework satisfies it |
|---|---|
| Stable, validated frontmatter schema | Section 11 JSON Schemas; closed schemas reject unknown keys |
| Indexable fields for SQLite FTS5 and metadata table | Universal frontmatter mirrors `apps/cli/internal/db/migrations/001_initial.sql` columns: `title`, `type`, `status`, `date`, `tags`, `author`, `project` |
| Schema versioning | Per-type schemas have `$id` URLs; breaking changes require an ADR and bump the migration framework |
| Idempotent re-indexing | Frontmatter is the source of truth (ADR-001); `vedox reindex` rebuilds from disk; framework is consistent with that contract |
| Deterministic content hashing | `content_hash` in DB depends on stable file contents; the framework's no-trailing-whitespace and one-trailing-newline rules keep hashes stable across editors |
| Search ranking signals | `tags` are bounded to 2..8, kebab-case, and de-duplicated, giving FTS5 a clean signal; `summary` field gives a high-quality ranking surface |
| PII protection | Section 9.4 PII linter rule; `contains_pii: false` constraint on logging type |
| Migration story | Adding required fields requires an ADR and a `vedox reindex` run; existing docs are flagged invalid until updated |

### 12.2 Tech writer

| Requirement | How the framework satisfies it |
|---|---|
| Clear voice and tone rules | Section 2.4 |
| Diataxis-aligned type taxonomy | Types map cleanly: how-to, reference (api-reference), explanation, tutorial-substitute (the README acts as orientation, dedicated tutorials are out of scope for v1) |
| Structural templates per type | Section 4 required heading order per type; existing how-to templates in `docs/how-to/use-*-template.md` remain canonical examples |
| Terminology consistency | Section 2.6 forbidden words and canonical spellings |
| Lifecycle (draft -> published -> deprecated) | Section 7 |
| Reusable content avoidance of duplication | Section 1.2 #6 forbids duplication; redirect_from supports rename without breaking links |
| Cross-linking discipline | Section 9 |
| Length discipline | Per-type word/line caps in Section 4 |
| Reviewer checklist | Section 8 |

### 12.3 Product engineer

| Requirement | How the framework satisfies it |
|---|---|
| Discoverability via type filters | The `type` enum maps directly to UI filter chips in the editor sidebar (defined in DESIGN_FRAMEWORK.md) |
| Search ranking that respects user intent | `summary`, `tags`, `type`, `status` all feed FTS5 with clean inputs |
| Status badges in UI | `status`, `stability`, `severity`, `last_tested` (stale flag) all have UI surfaces defined in DESIGN_FRAMEWORK.md |
| Empty-state and first-run flows | Templates per type; required prerequisites field on how-to gives the editor a "what do I need" panel |
| Instrumentation hooks | Per-type frontmatter is dense enough that the editor can offer field-aware autocomplete and inline validation (instrumented by the same JSON Schemas in Section 11) |
| Cross-doc relationships surfaced in UI | `related_adrs`, `related_runbooks`, `superseded_by`, `redirect_from` give the UI a relationship graph to render |
| Lifecycle automation | Section 7.2 aging rules drive notifications and auto-escalation, all from frontmatter dates |
| Editor field validation | JSON Schemas in Section 11 are loadable into the Tiptap editor's frontmatter form; validation surfaces field-by-field |

---

## 13. Enforcement Plan

This section is a map from rule -> enforcement mechanism. It is the spec for the future linter ticket. No ticket is created here; this is the inventory.

**Linter owner: staff-engineer.** The framework linter (CLI binary `vedox lint`, plus the in-editor live diagnostics path) is owned by the staff-engineer persona. The migration ticket VDX-P2-M (see §14) is a hard dependency: the linter ships with the §14 file list as its initial exemption set, and that exemption set must drain to zero before the linter can be marked GA. The creative-technologist owns only the editor-side frontmatter helper at `apps/editor/src/lib/editor/utils/frontmatter.ts` and must align it to the canonical status enum (see §7.1).

### 13.1 Linter-enforced rules (CI mechanical rejection)

Implementable as a single Go command (`vedox lint docs/`) that exits non-zero on any failure. Rules are listed in priority order (cheapest first).

```
LINT-001  Frontmatter parses as YAML
LINT-002  Frontmatter validates against universal JSON Schema
LINT-003  Frontmatter validates against per-type JSON Schema
LINT-004  No unknown frontmatter keys
LINT-005  Filename matches type's directory and pattern regex
LINT-006  Exactly one H1 in document
LINT-007  H1 text matches `title` field
LINT-008  Required H2 sections present in declared order for type
LINT-009  Heading levels descend without skipping
LINT-010  No emoji characters anywhere
LINT-011  No forbidden marketing words (case-insensitive substring match)
LINT-012  No tab characters in body
LINT-013  No trailing whitespace
LINT-014  LF line endings only
LINT-015  UTF-8 encoding without BOM
LINT-016  File ends with exactly one newline
LINT-017  Word count within type's bounds
LINT-018  Every fenced code block has a language tag
LINT-019  No `bash` / `shell` / `console` / `terminal` language tags
LINT-020  No HTML tags except permitted subset
LINT-021  Internal links resolve to existing files
LINT-022  Internal links to anchors resolve to existing headings
LINT-023  Code references `path:LINE` resolve to existing files and lines
LINT-024  PII patterns absent (email, phone, SSN, IP-other-than-127.0.0.1, AWS access key, GCP service account, GitHub username:token)
LINT-025  Secret blocklist file paths absent (`.env`, `*.pem`, `*.key`, `id_rsa`, `*.p12`, `credentials.json`, `~/.ssh/`)
LINT-026  `contains_pii: false` for `type: logging`
LINT-027  `bind_address: "0.0.0.0"` requires `related_adrs:` (network type)
LINT-028  `last_tested` <= 90 days for runbooks (warning at 90, error at 180)
LINT-029  `date` is not in the future
LINT-030  `project` matches an existing project slug in `vedox.config.ts`
LINT-031  `tags` are kebab-case, lowercase, 2..8 entries, no near-duplicates
LINT-032  ADR `Alternatives Considered` has >= 2 H3 subsections
LINT-033  How-to numbered steps have one verb per step (no "and" joining)
LINT-034  Postmortem contains zero individual person names (heuristic: look for capitalized two-word phrases not in glossary)
LINT-035  Image alt text non-empty and ends with a period
LINT-036  Asset paths inside `_assets/` and start with parent doc slug
LINT-037  No animated GIF or video files in `docs/`
LINT-038  `redirect_from` is append-only across commits (CI git diff check)
LINT-039  Document is on a staging branch when authored by an `author: @<persona>` value
LINT-040  Frontmatter `summary` (when present) is 50..280 chars and a single sentence
```

### 13.2 Reviewer-enforced rules (human PR review)

These cannot be reliably automated. A human reviewer signs off on each.

```
REV-001  Document is the right type for its content
REV-002  Tone matches Section 2.4 voice rules
REV-003  Content does not duplicate another document
REV-004  Cross-links are correct, relevant, and not over-linked
REV-005  Diagrams accurately depict the system
REV-006  The document fulfils its declared purpose (Section 4)
REV-007  Examples are realistic and run on a clean system
REV-008  No author name slips into postmortem prose
REV-009  Forbidden marketing words have been replaced with concrete facts (linter catches the words; reviewer judges the replacement)
REV-010  ADR alternatives are realistic, not strawmen
```

### 13.3 Runtime / build-time validations

```
RUN-001  vedox dev refuses to load a document that fails LINT-001..LINT-005
RUN-002  vedox build fails on any LINT-* error
RUN-003  vedox publish refuses to publish a document with `status: draft`
RUN-004  Search index demotes `status: deprecated` and `status: superseded` documents
RUN-005  Editor field-aware autocomplete uses JSON Schemas from Section 11
RUN-006  CI computes word count diff per PR and posts as a comment
```

### 13.4 What this framework does NOT enforce

These are explicitly deferred or out of scope:

- Style of prose beyond the Section 2.4 rules and the forbidden-word list. Beautiful sentences are aspirational; correct sentences are mandatory.
- Diagram quality. The reviewer judges this.
- Whether the **content** is accurate. The framework is about form, not facts.
- Search relevance tuning. That belongs in the search component.
- Visual rendering, typography, color, and accessibility — those are governed by [DESIGN_FRAMEWORK.md](./DESIGN_FRAMEWORK.md).

---

## §14 — Migration Window

All §14 entries migrated by VDX-P2-M on 2026-04-13.

---

## 16. Changelog

| Date | Author | Change |
|---|---|---|
| 2026-04-07 | @ceo | Initial framework. Defines 11 content types, JSON Schemas, lint rule inventory, and the agent contract. Supersedes nothing — this is the constitution. |

Future amendments to this framework MUST go through an ADR (`type: adr`) and update this changelog. Silent amendments are forbidden.

---

## 17. See Also

- [DESIGN_FRAMEWORK.md](./DESIGN_FRAMEWORK.md) — visual, IA, component, accessibility, editor UX
- [ADR-001: Markdown as Source of Truth](./adr/001-markdown-as-source-of-truth.md) — the architectural premise this framework relies on
- [How to Write an ADR](./how-to/use-adr-template.md) — template how-to for ADRs
- [How to Write a Runbook](./how-to/use-runbook-template.md) — template how-to for runbooks
- [How to Write a How-To](./how-to/use-how-to-template.md) — template how-to for how-tos
- [How to Write a README](./how-to/use-readme-template.md) — template how-to for READMEs
- [How to Write an API Reference](./how-to/use-api-reference-template.md) — template how-to for API references
