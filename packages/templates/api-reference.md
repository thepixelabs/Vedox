---
title: "[Resource/Endpoint Name]"
type: api-reference
status: published   # draft | published | deprecated
date: YYYY-MM-DD
project: ""
version: "v1"
tags: []
author: ""
---

## Overview

<!--
Two to four sentences describing what this resource or endpoint group does.
State the resource model (what entity does it represent?) and the primary use case.

Example: "The `/documents` resource manages the full lifecycle of Markdown documents
within a Vedox workspace. Clients use it to create, read, update, and delete documents,
and to trigger publish workflows."
-->

**Base URL:** `https://localhost:3001/api/v1`

**Content-Type:** `application/json`

## Authentication

<!--
Describe the auth mechanism for this API.
For Vedox: HMAC-SHA256 signed API keys in the `X-Vedox-Signature` header (Phase 3).
For Phase 1 local-only endpoints: state that no auth is required and why (localhost-only binding).
-->

No authentication is required for Phase 1 endpoints. The server binds to `127.0.0.1` only.
Requests from any other origin are rejected at the network layer.

## Endpoints

<!--
Document each endpoint in its own subsection. Use the format below.
Include at least one complete request/response example per endpoint.
-->

### `GET /example`

<!--Brief description of what this endpoint returns.-->

**Query parameters:**

| Parameter | Type | Required | Description |
|---|---|---|---|
| `param` | string | no | Example parameter |

**Response `200 OK`:**

```json
{
  "example": "value"
}
```

---

### `POST /example`

<!--Brief description of what this endpoint creates or triggers.-->

**Request body:**

```json
{
  "field": "value"
}
```

**Response `201 Created`:**

```json
{
  "id": "abc123",
  "field": "value",
  "created_at": "2026-04-07T12:00:00Z"
}
```

---

### `PUT /example/:id`

<!--Brief description of the update semantics (partial vs full replacement).-->

**Path parameters:**

| Parameter | Type | Description |
|---|---|---|
| `id` | string | Resource identifier |

**Request body:**

```json
{
  "field": "updated value"
}
```

**Response `200 OK`:**

```json
{
  "id": "abc123",
  "field": "updated value",
  "updated_at": "2026-04-07T13:00:00Z"
}
```

---

### `DELETE /example/:id`

**Response `204 No Content`:** (empty body)

## Error Codes

<!--
List all error responses this API can return. Include the HTTP status, the
error code string, a description, and what the caller should do.
-->

| HTTP Status | Error Code | Description | Action |
|---|---|---|---|
| `400` | `INVALID_BODY` | Request body failed validation | Fix the request body; see the field-level `errors` array in the response |
| `404` | `NOT_FOUND` | Resource does not exist | Verify the `id` is correct |
| `409` | `CONFLICT` | Optimistic lock mismatch — resource was modified since last fetch | Re-fetch the resource and re-apply your changes |
| `429` | `RATE_LIMITED` | Exceeded 60 writes/minute per API key | Retry after `Retry-After` seconds |
| `500` | `INTERNAL_ERROR` | Unexpected server error | Retry once; if it persists, check `~/.vedox/logs/` for details |

**Error response shape:**

```json
{
  "error": "INVALID_BODY",
  "message": "Human-readable description",
  "errors": [
    { "field": "title", "message": "title is required" }
  ]
}
```
