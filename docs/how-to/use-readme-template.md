---
title: "How to Write a README Using the Vedox Template"
type: how-to
status: published
date: 2026-04-07
project: "vedox"
tags: ["readme", "templates", "onboarding", "documentation"]
author: "Vedox Team"
---

The README template produces the front door of a project — the first thing a new engineer reads when they encounter a codebase. Its job is to answer three questions in under two minutes: what does this do, how do I run it, and how do I contribute?

## Prerequisites

- `vedox dev` is running at http://127.0.0.1:3001
- You have a project set up in Vedox

## When to Use the README Template

Use the README template for the root-level documentation of any repository, service, or package. It is the right choice when a reader needs to orient themselves before diving into specifics.

Do not use it when you need to document:

- A specific task a reader must accomplish ("how to deploy to staging") — use a How-To
- An architectural decision — use an ADR
- A production incident response procedure — use a Runbook
- The full details of an HTTP API — use an API Reference

The README links to all of those. It does not contain them.

## Steps

1. **Create a new document** and select the **README** template.

   In the sidebar, click your project, then **New Document**, then choose **README** from the template picker.

2. **Fill in the frontmatter fields.**

   ```yaml
   ---
   title: "my-api"
   type: readme
   status: published
   date: 2026-04-07
   project: "my-api"
   tags: ["readme", "overview", "onboarding"]
   author: "Vedox Team"
   ---
   ```

   The `title` field should match the repository or package name exactly. This is what appears in search results and the sidebar.

3. **Write the project headline.**

   A single sentence below the project name heading. It must convey what the project does and why it exists. Avoid adjectives like "powerful," "flexible," or "modern" — they say nothing. Be specific.

   ```markdown
   # my-api

   > REST API for managing user accounts and billing in the Acme platform.
   ```

4. **Write the Overview section.**

   Two to four paragraphs. Cover: what problem it solves, who uses it, what makes it different if anything, and the current maturity level. Keep it factual.

   ```markdown
   ## Overview

   `my-api` is the core backend service for the Acme platform. It handles user
   registration, authentication, subscription management, and billing webhooks
   from Stripe.

   The primary consumers are the Acme web frontend and the mobile app. Internal
   services access it through the service mesh, not the public API.

   This service is in production and powers Acme's paying customer base. Breaking
   changes require a deprecation window of 30 days and a version bump.
   ```

5. **Write the Installation section.**

   List every prerequisite with minimum versions before the first command. Then show the commands in the exact order a new engineer runs them. Test these commands against the current codebase — stale install instructions are the most common README failure mode.

   ```markdown
   ## Installation

   **Prerequisites:**
   - Go >= 1.21
   - PostgreSQL >= 15 (or `docker compose up db`)

   ```sh
   git clone https://github.com/acme/my-api
   cd my-api
   cp .env.example .env
   go mod download
   go run ./cmd/server
   ```
   ```

6. **Write the Usage section.**

   Show the most common use case first with working commands and expected output. Do not show pseudocode or commands that require values the reader does not have yet.

   ```markdown
   ## Usage

   ```sh
   # Start the development server
   go run ./cmd/server
   # Output: {"level":"info","msg":"listening","addr":"0.0.0.0:8080"}

   # Run tests
   go test ./...
   ```
   ```

7. **Write the Configuration section** (only if the project has non-trivial configuration).

   A table of environment variables or config keys. Do not document every possible option here — link to the full configuration reference if one exists.

   ```markdown
   ## Configuration

   | Variable | Default | Description |
   |---|---|---|
   | `PORT` | `8080` | Port the HTTP server listens on |
   | `DATABASE_URL` | — | Postgres connection string (required) |
   | `LOG_LEVEL` | `info` | Log verbosity: `debug`, `info`, `warn`, `error` |
   ```

8. **Write the Contributing section.**

   Minimum: clone, install dependencies, run tests, run the linter. If a `CONTRIBUTING.md` exists, link to it rather than duplicating content here.

   ```markdown
   ## Contributing

   ```sh
   git clone https://github.com/acme/my-api
   cd my-api
   go mod download
   go test ./...
   golangci-lint run
   ```

   See [CONTRIBUTING.md](./CONTRIBUTING.md) for branch naming conventions and
   the pull request process.
   ```

9. **Write the License section.**

   One line. Link to the LICENSE file.

   ```markdown
   ## License

   [MIT](./LICENSE)
   ```

---

## Verification

After saving, confirm:

- The document appears at the top of the project sidebar (it should be pinned as the root document)
- All code blocks have syntax highlighting applied (Go blocks use ` ```sh ` or ` ```go ` as appropriate)
- Every link in the document resolves (use the link validator in the Vedox editor toolbar)

---

## Troubleshooting

### Problem: The README is getting too long — it has grown past 500 lines

**Cause:** Architecture explanations, full API docs, or operational runbooks have been written into the README instead of their own dedicated documents.

**Fix:** Extract each major section into its own document using the appropriate template (ADR, API Reference, How-To, Runbook). Replace the section in the README with a one-sentence summary and a link.

### Problem: The install commands in the README do not work for new team members

**Cause:** The commands were written when the project was first set up and have drifted as dependencies changed.

**Fix:** Run the README install steps from scratch in a clean environment before every major release. Treat "new engineer can follow the README" as a release criterion, not an afterthought.
