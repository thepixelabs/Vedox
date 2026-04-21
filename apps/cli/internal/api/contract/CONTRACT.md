# Vedox Daemon HTTP API Contract

This document is the authoritative, human-readable specification for every
public HTTP endpoint exposed by the Vedox daemon (`apps/cli`). It is kept in
sync with `contract_test.go` — any change to a path, method, auth model,
required field, or response shape MUST update both files simultaneously.

Auth levels used in this document:

- **open** — no authentication required; any local caller may invoke.
- **bootstrap-token** — the caller must supply the 64-hex-char daemon token as
  `Authorization: Bearer <token>`. The token is stored at
  `~/.vedox/daemon-token` (mode 0600). Missing or wrong token → 401.
- **HMAC** — reserved for the Doc Agent ingest endpoints (not yet in alpha);
  callers must provide a signed HMAC-SHA256 header.

CORS: mutating verbs (POST/PUT/PATCH/DELETE) require an `Origin` header in the
allowed list (`http://localhost:5151` or `http://127.0.0.1:5151`). Missing or
non-allowlisted origin → 403. GET/HEAD requests pass through without an Origin
check.

Error shape: every 4xx/5xx response is a JSON object:

```json
{ "code": "VDX-xxx", "message": "human-readable explanation" }
```

---

## Endpoints

### GET /api/health

| Property    | Value |
|-------------|-------|
| Auth        | open  |
| Description | Liveness probe. Returns immediately if the binary is running. |

**Response 200**

```json
{ "status": "ok" }
```

---

### GET /api/projects

| Property    | Value |
|-------------|-------|
| Auth        | open  |
| Description | Returns the project list from the last completed workspace scan. Runs a synchronous scan on first call if no cached result exists. Also includes projects registered in .vedox/links.json. |

**Response 200** — JSON array, never null

```json
[
  {
    "name": "my-project",
    "path": "/absolute/path",
    "relPath": "relative/path",
    "docCount": 12,
    "detectedFramework": "docusaurus",
    "lastScanned": "2026-04-01T12:00:00Z"
  }
]
```

---

### POST /api/projects

| Property    | Value |
|-------------|-------|
| Auth        | open (CORS Origin header required) |
| Description | Creates a new project directory inside the workspace root. Optionally writes a README.md. Registers the project in the local registry. |

**Request body**

```json
{
  "name": "my-project",       // required; single path segment, no separators
  "tagline": "...",           // optional
  "description": "..."        // optional
}
```

**Response 201**

```json
{ "name": "my-project", "path": "/abs/path", "docCount": 0 }
```

**Error codes**

| Status | Code    | Condition |
|--------|---------|-----------|
| 400    | VDX-000 | Malformed JSON body |
| 400    | VDX-300 | `name` empty, contains path separators, or is "." / ".." |
| 409    | VDX-301 | Project already exists on disk |

---

### GET /api/scan

| Property    | Value |
|-------------|-------|
| Auth        | open  |
| Description | Synchronous summary of the last completed scan. Runs a scan inline on first call. Frontend onboarding uses this instead of POST+poll. |

**Response 200**

```json
{
  "projects": [
    { "path": "/abs/path", "name": "proj", "hasGit": true, "docCount": 7 }
  ]
}
```

---

### POST /api/scan

| Property    | Value |
|-------------|-------|
| Auth        | open (CORS Origin required) |
| Description | Starts an async workspace scan. Returns a job ID immediately; caller polls GET /api/scan/{jobId} for status. |

**Request body** (optional)

```json
{ "workspaceRoot": "/override/path" }
```

**Response 202**

```json
{ "jobId": "8b1f41a7..." }
```

---

### GET /api/scan/{jobId}

| Property    | Value |
|-------------|-------|
| Auth        | open  |
| Description | Returns the current state of a scan job. Jobs are lost on server restart. |

**Response 200** — full ScanJob object (see `internal/scanner` for field list)

**Error codes**

| Status | Code    | Condition |
|--------|---------|-----------|
| 404    | VDX-101 | Unknown job ID |

---

### GET /api/repos

| Property    | Value |
|-------------|-------|
| Auth        | open  |
| Description | Lists all repos from the global registry (~/.vedox/global.db). Returns 503 when GlobalDB is unavailable (dev-server mode). |

**Response 200** — JSON array, never null

```json
[
  {
    "id": "uuid",
    "name": "my-repo",
    "type": "private",
    "root_path": "/abs/path",
    "remote_url": "https://...",
    "status": "active",
    "created_at": "2026-04-01T12:00:00Z",
    "updated_at": "2026-04-01T12:00:00Z"
  }
]
```

---

### POST /api/repos

| Property    | Value |
|-------------|-------|
| Auth        | open (CORS Origin required) |
| Description | Creates a new repo entry in the global registry. Does NOT scaffold a directory or run git init — use POST /api/repos/create for that. |

**Request body**

```json
{
  "name": "my-repo",          // required
  "type": "private",          // required; one of: private | public | inbox
  "root_path": "/abs/path",   // required
  "remote_url": "https://..." // optional
}
```

**Response 201** — same shape as GET /api/repos array element

**Error codes**

| Status | Code    | Condition |
|--------|---------|-----------|
| 400    | VDX-400 | Missing name, root_path, or type; invalid type value |
| 503    | VDX-503 | GlobalDB not available |

---

### POST /api/repos/create

| Property    | Value |
|-------------|-------|
| Auth        | **bootstrap-token** |
| Description | Scaffolds a new local documentation repo: creates the directory, runs `git init`, and registers the result in GlobalDB. |

**Request body**

```json
{
  "name": "my-repo",       // required
  "path": "~/docs/my-repo",// required; expanded and made absolute; must be within $HOME
  "type": "private"        // optional; default "private"
}
```

**Response 201** — same shape as GET /api/repos array element

**Error codes**

| Status | Code    | Condition |
|--------|---------|-----------|
| 400    | VDX-400 | Missing name or path; invalid type; path outside $HOME or via symlink |
| 401    | VDX-401 | Missing or wrong bootstrap token |
| 500    | VDX-500 | git init failed or DB write failed |

---

### POST /api/repos/register

| Property    | Value |
|-------------|-------|
| Auth        | **bootstrap-token** |
| Description | Registers an existing local git repository in GlobalDB. The directory must already exist and contain a .git entry. Does NOT run git init. |

**Request body**

```json
{
  "path": "/abs/path",  // required; must be within $HOME and be a git repo
  "name": "my-repo",   // optional; defaults to directory basename
  "type": "private"    // optional; default "private"
}
```

**Response 201** — same shape as GET /api/repos array element

**Error codes**

| Status | Code    | Condition |
|--------|---------|-----------|
| 400    | VDX-400 | Missing path; not a directory; no .git found; path outside $HOME |
| 401    | VDX-401 | Missing or wrong bootstrap token |
| 503    | VDX-503 | GlobalDB not available |

---

### GET /api/agent/list

| Property    | Value |
|-------------|-------|
| Auth        | open  |
| Description | Lists all installed Doc Agent configurations by reading provider receipt files from ~/.vedox/install-receipts/. Returns an empty array when none are installed. |

**Response 200** — JSON array, never null

```json
[
  {
    "provider": "claude",
    "version": "1.0.0",
    "authKeyID": "uuid",
    "installedAt": "2026-04-01T12:00:00Z"
  }
]
```

---

### POST /api/agent/install

| Property    | Value |
|-------------|-------|
| Auth        | open (CORS Origin required); KeyStore must be configured |
| Description | Installs the Vedox Doc Agent into the specified AI provider. Runs Probe → Plan → Install → Save. Returns 503 if no KeyStore is configured (daemon not started with auth). Returns 409 if already installed. |

**Request body**

```json
{ "provider": "claude" }  // required; one of: claude | codex | copilot | gemini
```

**Response 201**

```json
{
  "provider": "claude",
  "version": "1.0.0",
  "authKeyID": "uuid",
  "daemonURL": "http://127.0.0.1:5150",
  "fileCount": 3,
  "installedAt": "2026-04-01T12:00:00Z"
}
```

**Error codes**

| Status | Code    | Condition |
|--------|---------|-----------|
| 400    | VDX-400 | Unknown provider; malformed JSON |
| 409    | VDX-409 | Agent already installed for this provider |
| 503    | VDX-503 | KeyStore not configured |

---

### POST /api/agent/uninstall

| Property    | Value |
|-------------|-------|
| Auth        | open (CORS Origin required); KeyStore must be configured |
| Description | Removes the Vedox Doc Agent from the specified provider. Revokes the HMAC key and strips all Vedox-managed config fragments. |

**Request body**

```json
{ "provider": "claude" }  // required; one of: claude | codex | copilot | gemini
```

**Response 200**

```json
{ "provider": "claude", "status": "uninstalled" }
```

**Error codes**

| Status | Code    | Condition |
|--------|---------|-----------|
| 400    | VDX-400 | Unknown provider |
| 503    | VDX-503 | KeyStore not configured |

---

### POST /api/onboarding/complete

| Property    | Value |
|-------------|-------|
| Auth        | open (CORS Origin required) |
| Description | Fire-and-forget analytics event. The frontend posts here when the user reaches the AllDone onboarding step. Body is optional. Always returns 204 — a malformed body is swallowed so the UX flow is never gated on telemetry. |

**Request body** (optional)

```json
{
  "skippedSteps": [2, 3],
  "selectedProviders": ["claude"],
  "registeredRepos": 1
}
```

**Response 204** — empty body

---

### GET /api/settings

| Property    | Value |
|-------------|-------|
| Auth        | open  |
| Description | Returns the contents of ~/.vedox/user-prefs.json. Returns `{}` when the file does not exist (first run) or is malformed. |

**Response 200** — JSON object (schema open; preserves unknown keys)

---

### PUT /api/settings

| Property    | Value |
|-------------|-------|
| Auth        | open (CORS Origin required) |
| Description | PATCH-merge semantics: only top-level keys present in the request body are overwritten; absent keys are preserved. Write is atomic (rename). |

**Request body** — JSON object; top-level keys are the preference categories

```json
{ "editor": { "spellCheck": true }, "theme": "dark" }
```

**Response 200** — the full merged prefs object after writing

**Error codes**

| Status | Code    | Condition |
|--------|---------|-----------|
| 400    | VDX-400 | Malformed JSON; key too long; value nesting > 32 levels |

---

### GET /api/graph

| Property    | Value |
|-------------|-------|
| Auth        | open  |
| Description | Returns the per-project doc-reference graph as a flat {nodes, edges, …} envelope consumed directly by the DocGraph frontend. Read-only. |

**Query parameters**

| Param   | Required | Description |
|---------|----------|-------------|
| project | yes      | Project name as returned by GET /api/projects. An unknown project returns 200 with an empty graph (matching /api/projects/{project}/docs). |

**Response 200**

```json
{
  "nodes": [
    {
      "id": "my-project/docs/adr/001.md",
      "project": "my-project",
      "slug": "adr-001",
      "title": "ADR 001: Storage",
      "type": "adr",
      "status": "published",
      "degree_in": 4,
      "degree_out": 2,
      "modified": "2026-04-20T14:02:11Z"
    }
  ],
  "edges": [
    {
      "id": "e:src->tgt#0",
      "source": "my-project/docs/adr/001.md",
      "target": "my-project/docs/adr/002.md",
      "kind": "mdlink",
      "broken": false
    }
  ],
  "truncated": false,
  "total_nodes": 142,
  "total_edges": 389
}
```

Notes:

- `kind` is one of `mdlink` | `wikilink` | `frontmatter` | `vedox_ref`. The backend normalises its internal LinkType enum (`md-link`, etc.) to these canonical values at the wire edge.
- Broken edges (target does not resolve to an indexed doc) still appear, with `broken: true` and a synthesised target node carrying `type: "missing"`, `status: "broken"`. This keeps dangling links visible in the UI instead of silently dropping them.
- Vedox-scheme edges (`vedox://file/...` source-code cross-links) are intentionally excluded in v1.
- When `total_nodes` exceeds the server cap (default 2000) the response is capped and sorted by `degree_in + degree_out` DESC then `modified` DESC; `truncated` is `true`.

**Error codes**

| Status | Code    | Condition |
|--------|---------|-----------|
| 400    | VDX-400 | Missing `project` query parameter |
| 503    | VDX-503 | GraphStore not available |
| 500    | VDX-500 | Database read error |

---

### GET /api/analytics/summary

| Property    | Value |
|-------------|-------|
| Auth        | open  |
| Description | Returns aggregated analytics from the global DB. Uses the precomputed cache when the aggregator has run at least once; falls back to live range queries otherwise. Returns 503 when GlobalDB is unavailable. |

**Response 200**

```json
{
  "total_docs": 42,
  "docs_last_7_days": 5,
  "docs_last_30_days": 18,
  "agent_triggered_last_7_days": 12,
  "agent_triggered_last_30_days": 44,
  "change_velocity_7d": 5,
  "change_velocity_30d": 18,
  "docs_per_project": "{\"my-project\":7}",
  "pipeline_ready": true
}
```

---

### GET /api/preview

| Property    | Value |
|-------------|-------|
| Auth        | open  |
| Description | Resolves a `vedox://` URL and returns source code for Shiki rendering. Read-only. |

**Query parameters**

| Param | Required | Description |
|-------|----------|-------------|
| url   | yes      | vedox:// URL, e.g. `vedox://file/apps/cli/main.go#L10-L25` |

**Response 200**

```json
{
  "file_path": "apps/cli/main.go",
  "language": "go",
  "content": "package main\n...",
  "start_line": 10,
  "end_line": 25,
  "total_lines": 300,
  "truncated": false
}
```

**Error codes**

| Status | Code    | Condition |
|--------|---------|-----------|
| 400    | VDX-400 | Missing url param |
| 403    | VDX-403 | File matches secret blocklist |
| 404    | VDX-404 | File not found |
| 415    | VDX-415 | Binary file |
| 422    | VDX-422 | Invalid vedox:// URL (bad scheme, path, anchor, etc.) |

---

### GET /api/projects/{project}/git/status

| Property    | Value |
|-------------|-------|
| Auth        | open  |
| Description | Best-effort git status for the editor status bar. Always returns 200 — never errors; falls back to sentinel values when git is unavailable or the project has no git repo. |

**Response 200**

```json
{ "branch": "main", "dirty": false, "ahead": 0, "behind": 0 }
```

Sentinel value when git is not available or not a git repo: `branch = "(no git)"`.

---

### GET /api/projects/{project}/docs/{docPath}/history

| Property    | Value |
|-------------|-------|
| Auth        | open  |
| Description | Returns the git-backed commit timeline for a single document. Supports `?limit=N` (default 50, max 500). Results are ordered most-recent first. |

**Response 200**

```json
{
  "docPath": "relative/path/to/doc.md",
  "entries": [
    {
      "sha": "abc123",
      "author": "Alice",
      "message": "fix typo",
      "date": "2026-04-01T12:00:00Z"
    }
  ]
}
```

---

### GET /api/browse

| Property    | Value |
|-------------|-------|
| Auth        | **bootstrap-token** |
| Description | Lists subdirectories for the given path. Used by the frontend folder picker. Only directories within $HOME are served; paths outside $HOME return 403. Symlink-escape attacks are blocked. |

**Query parameters**

| Param | Required | Description |
|-------|----------|-------------|
| path  | no       | Absolute path to list; defaults to $HOME |

**Response 200**

```json
{
  "path": "/Users/alice/docs",
  "parent": "/Users/alice",
  "directories": [
    { "name": "notes", "path": "/Users/alice/docs/notes" }
  ]
}
```

**Error codes**

| Status | Code    | Condition |
|--------|---------|-----------|
| 401    | VDX-401 | Missing or wrong bootstrap token |
| 403    | VDX-403 | Path outside $HOME or symlink escape |
| 404    | n/a     | Path does not exist |

---

### POST /api/voice/ptt

| Property    | Value |
|-------------|-------|
| Auth        | open (CORS Origin required) |
| Description | Push-to-talk voice input. Only registered when the daemon is started with `--voice`. When no VoiceServer is injected, this path returns 404. |

Delegates to `voice.VoiceServer.HandlePTT` — see `internal/voice` for request/response shape.

---

### GET /api/voice/status

| Property    | Value |
|-------------|-------|
| Auth        | open  |
| Description | Voice pipeline health. Only registered when the daemon is started with `--voice`. Returns 404 otherwise. |

Delegates to `voice.VoiceServer.HandleStatus` — see `internal/voice` for response shape.

---

## Endpoints not covered by contract tests (implementation details)

These endpoints are tested in other packages and excluded from this contract
file because their schemas are either intentionally schema-loose or their
contracts are owned by sub-packages:

- `GET/POST /api/projects/{project}/docs/*` — doc CRUD (see `docs.go`, `api_integration_test.go`)
- `POST /api/import`, `POST /api/link` — import/link flows
- `GET /api/projects/{project}/search` — full-text search
- `GET/POST/PATCH/DELETE /api/projects/{project}/tasks` — task backlog
- `GET/POST/PUT/DELETE /api/projects/{project}/providers/*` — AI provider config
- `GET/POST /api/ai/*` — AI name generation jobs
