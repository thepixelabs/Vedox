---
title: "How to Write an API Reference Using the Vedox Template"
type: how-to
status: published
date: 2026-04-07
project: "vedox"
tags: ["api", "reference", "http", "endpoints", "templates"]
author: "Vedox Team"
difficulty: "intermediate"
estimated_time_minutes: 15
prerequisites:
  - "vedox dev running at http://127.0.0.1:3001"
  - "A project set up in Vedox for the service or SDK being documented"
---

The API Reference template is for documenting any interface that a caller must understand to use correctly: an HTTP API, a CLI, or a programmatic SDK. It is structured for scanning, not reading — engineers arrive at a reference doc looking for one specific thing, and leave the moment they find it.

Use this template when you need to document what an interface does, what it accepts, and what it returns. For a conceptual explanation of why an API is designed a certain way, write an ADR instead.

## Prerequisites

- `vedox dev` is running at http://127.0.0.1:3001
- You have a project set up in Vedox for the service or SDK you are documenting

## Steps

1. **Create a new document** and select the **API Reference** template.

   In the sidebar, click your project, then **New Document**, then choose **API Reference** from the template picker.

2. **Fill in the frontmatter fields.**

   ```yaml
   ---
   title: "Documents API"
   type: api-reference
   status: published
   date: 2026-04-07
   project: "my-api"
   version: "v1"
   tags: ["documents", "crud", "rest"]
   author: "Vedox Team"
   ---
   ```

   | Field | What to put here |
   |---|---|
   | `title` | The resource or endpoint group name. Keep it short — callers will search by this. |
   | `type` | Always `api-reference` — do not change this. |
   | `status` | `published` for live APIs; `draft` while writing; `deprecated` when the API is retired. |
   | `date` | The date this version of the reference was last updated. Update this whenever you change endpoint behavior. |
   | `project` | The project slug this API belongs to. |
   | `version` | The API version this reference describes: `v1`, `v2`, etc. |
   | `tags` | Keywords that help search find this doc. Include the resource name and the operations it supports. |
   | `author` | Your name or team name. |

3. **Write the Overview section.**

   Two to four sentences. State what resource this API manages and its primary use case. Do not repeat the title.

   ```markdown
   ## Overview

   The `/documents` resource manages the full lifecycle of Markdown documents
   within a Vedox workspace. Clients use it to create, read, update, and delete
   documents and to trigger publish workflows.

   **Base URL:** `http://127.0.0.1:3001/api/v1`

   **Content-Type:** `application/json`
   ```

4. **Write the Authentication section.**

   State plainly what authentication is required. Never omit this section even if the answer is "none."

   ```markdown
   ## Authentication

   Phase 1 endpoints require no authentication. The server binds to `127.0.0.1`
   only — requests from any other origin are rejected at the network layer.

   Phase 3 agentic endpoints require an HMAC-SHA256 signed API key in the
   `X-Vedox-Signature` header. See [Agentic API Auth](../adr/004-agentic-api-auth.md).
   ```

5. **Document each endpoint in its own subsection.**

   Use `### METHOD /path` as the heading. Include the full request/response cycle: parameters, request body, all possible response codes, and at least one working example.

   ```markdown
   ### `GET /documents`

   Returns all documents in the specified project, ordered by last-modified date descending.

   **Query parameters:**

   | Parameter | Type | Required | Description |
   |---|---|---|---|
   | `project` | string | yes | Project slug to filter by |
   | `type` | string | no | Filter by document type: `adr`, `runbook`, `how-to`, `readme`, `api-reference` |
   | `limit` | integer | no | Maximum results to return. Default: `50`. Max: `200`. |

   **Response `200 OK`:**

   ```json
   {
     "documents": [
       {
         "id": "01HX4K2M3N",
         "title": "ADR-001: Markdown as Source of Truth",
         "type": "adr",
         "status": "accepted",
         "project": "vedox",
         "date": "2026-04-07",
         "path": "adr/001-markdown-as-source-of-truth.md"
       }
     ],
     "total": 1
   }
   ```
   ```

6. **Document the error codes your API returns.**

   List every distinct error response with its HTTP status, error code string, what causes it, and what the caller should do. Do not omit error codes that "shouldn't happen" — they will happen.

   ```markdown
   ## Error Codes

   | HTTP Status | Error Code | Description | Action |
   |---|---|---|---|
   | `400` | `INVALID_BODY` | Request body failed schema validation | Fix the request; see the `errors` array in the response body |
   | `404` | `NOT_FOUND` | Document does not exist at the given path | Verify the `id` or `path` parameter |
   | `409` | `CONFLICT` | Optimistic lock mismatch — document was modified since your last fetch | Re-fetch the document and re-apply your changes |
   | `429` | `RATE_LIMITED` | Exceeded 60 writes/minute | Retry after the number of seconds in the `Retry-After` header |
   | `500` | `INTERNAL_ERROR` | Unexpected server error | Retry once; if it persists, check `~/.vedox/logs/` |
   ```

---

## Verification

After saving, confirm:

- The document appears in the project sidebar
- All code blocks render correctly (no broken JSON)
- Every endpoint heading is unique within the document — duplicate headings break the in-page navigation

---

## Troubleshooting

### Problem: The document renders but JSON code blocks are malformed

**Cause:** A code fence (``` ``` ```) inside an endpoint subsection was not closed before the next subsection heading.

**Fix:** Toggle to Code Mode (raw Markdown view) and find the unclosed fence. Every opening ` ```json ` must have a matching closing ` ``` ` on its own line.

---

## Tips

**One API Reference per resource, not per endpoint.**
Group all operations on a single resource (`/documents`) in one file. A separate file per endpoint creates dozens of tiny documents that are hard to navigate. The exception is when an endpoint group is so large (20+ endpoints) that a single file becomes unwieldy — split by sub-resource, not by HTTP method.

**Date the reference at the top of every update.**
The `date` field in frontmatter is not the creation date — it is the "last updated" date. Update it every time you change endpoint behavior. A reader looking at a reference with a date 18 months old has no idea if it is still accurate.

**Link to the error catalog, not to individual error codes.**
If you have a central error catalog (for example, an `errors.md` reference), link to it from the Authentication section rather than duplicating error descriptions in every API Reference file.
