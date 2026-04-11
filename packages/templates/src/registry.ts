/**
 * Template registry for Vedox document templates.
 *
 * Templates are stored as inline string literals — no `fs` or Node.js built-ins
 * are used at runtime. This keeps the package purely functional and usable in
 * any JavaScript environment (browser, Deno, Node, edge runtime).
 *
 * The canonical source of truth for template content is the `.md` files in
 * `packages/templates/`. This file must be kept in sync with those files.
 * When updating a template, update both the `.md` file and the string here.
 */

// ---------------------------------------------------------------------------
// Public types
// ---------------------------------------------------------------------------

export interface TemplateInfo {
  type: string;
  title: string;
  description: string;
}

// ---------------------------------------------------------------------------
// Template content (inline — no runtime I/O)
// ---------------------------------------------------------------------------

const TEMPLATES: Record<string, string> = {
  adr: `---
title: "ADR-NNN: [Decision Title]"
type: adr
status: proposed   # proposed | accepted | deprecated | superseded
date: YYYY-MM-DD
project: ""
tags: []
author: ""
superseded_by: ""  # fill in when status: superseded; leave empty otherwise
---

## Context

<!--
Describe the situation that forces a decision. Include:
- What problem you are solving and why it matters now
- Constraints (technical, organizational, time, cost)
- Forces in tension (e.g. consistency vs. speed, simplicity vs. flexibility)
- Any prior decisions this one builds on or conflicts with

Write in present tense. Describe the world as it is, not as you wish it were.
-->

## Decision

<!--
State the decision in one clear sentence, then explain it.

Start with: "We will [action]."

Follow with the reasoning — why this option over the alternatives.
Do not hedge. If the decision is tentative, the status should be "proposed", not a hedge in the body.
-->

## Consequences

<!--
List all effects of this decision, positive and negative.

Positive:
- What becomes easier or possible

Negative / trade-offs:
- What becomes harder, slower, or impossible
- Technical debt incurred
- What must now be monitored or revisited

Neutral / follow-on work:
- New tickets, spikes, or constraints this decision creates
-->

## Alternatives Considered

<!--
For each rejected alternative, document:
1. What it was
2. Why it was rejected (one or two sentences — be specific)

This section prevents re-litigation. If someone later asks "why didn't you use X?",
the answer is here.

Format:

### Option A: [Name]
[Brief description]. Rejected because [specific reason].

### Option B: [Name]
[Brief description]. Rejected because [specific reason].
-->
`,

  "api-reference": `---
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
-->

**Base URL:** \`https://localhost:3001/api/v1\`

**Content-Type:** \`application/json\`

## Authentication

No authentication is required for Phase 1 endpoints. The server binds to \`127.0.0.1\` only.
Requests from any other origin are rejected at the network layer.

## Endpoints

### \`GET /example\`

<!--Brief description of what this endpoint returns.-->

**Query parameters:**

| Parameter | Type | Required | Description |
|---|---|---|---|
| \`param\` | string | no | Example parameter |

**Response \`200 OK\`:**

\`\`\`json
{
  "example": "value"
}
\`\`\`

---

### \`POST /example\`

<!--Brief description of what this endpoint creates or triggers.-->

**Request body:**

\`\`\`json
{
  "field": "value"
}
\`\`\`

**Response \`201 Created\`:**

\`\`\`json
{
  "id": "abc123",
  "field": "value",
  "created_at": "2026-04-07T12:00:00Z"
}
\`\`\`

## Error Codes

| HTTP Status | Error Code | Description | Action |
|---|---|---|---|
| \`400\` | \`INVALID_BODY\` | Request body failed validation | Fix the request body; see the field-level \`errors\` array in the response |
| \`404\` | \`NOT_FOUND\` | Resource does not exist | Verify the \`id\` is correct |
| \`409\` | \`CONFLICT\` | Optimistic lock mismatch | Re-fetch the resource and re-apply your changes |
| \`429\` | \`RATE_LIMITED\` | Exceeded 60 writes/minute per API key | Retry after \`Retry-After\` seconds |
| \`500\` | \`INTERNAL_ERROR\` | Unexpected server error | Retry once; if it persists, check \`~/.vedox/logs/\` |

**Error response shape:**

\`\`\`json
{
  "error": "INVALID_BODY",
  "message": "Human-readable description",
  "errors": [
    { "field": "title", "message": "title is required" }
  ]
}
\`\`\`
`,

  runbook: `---
title: "[Incident/Procedure Name]"
type: runbook
status: published   # draft | published | deprecated
date: YYYY-MM-DD
project: ""
on_call_severity: P1   # P1 | P2 | P3
last_tested: YYYY-MM-DD
tags: []
author: ""
---

<!--
SEVERITY GUIDE
P1 — User-facing outage or data loss risk. Page immediately. Target time-to-mitigate: 30 min.
P2 — Degraded functionality. Significant user impact. Target time-to-mitigate: 2 hours.
P3 — Minor degradation. No immediate user impact. Resolve in business hours.

Update \`last_tested\` every time you drill this runbook in a game day or real incident.
-->

## Symptoms

<!--
What does the engineer on call observe that leads them to this runbook?
List observable signals, not internal states. Include exact log patterns or commands.

\`\`\`
grep "level=error" ~/.vedox/logs/vedox-$(date +%Y-%m-%d).log | tail -20
\`\`\`
-->

## Immediate Actions

<!--
Steps for someone paged at 3am on their first week.
Each step must be a single, complete, testable action.
Goal: STOP THE BLEEDING. Defer investigation to the next section.

1. [Action] — [expected outcome, how you know it worked]
2. [Action] — [expected outcome]
-->

## Root Cause Investigation

<!--
Once the system is stable, find out why.
Structure as a decision tree or ordered checklist.

- [ ] Check A — if yes, go to Resolution Steps > Case A
- [ ] Check B — if no, check C

\`\`\`
# Check SQLite integrity
sqlite3 ~/.vedox/index.db "PRAGMA integrity_check;"
# Expected output: ok
\`\`\`
-->

## Resolution Steps

<!--
One subsection per root cause identified above.

### Case A: [Root cause name]

Step-by-step remediation. Commands must be copy-pasteable.
State the expected outcome after each step.
-->

## Prevention

<!--
Action items to prevent recurrence.

- [ ] [Action] — owner: @name, due: YYYY-MM-DD
- [ ] [Action]

Link the post-mortem here if this was triggered by a real incident.
-->
`,

  readme: `---
title: "[Project Name]"
type: readme
status: published   # draft | published | deprecated
date: YYYY-MM-DD
project: ""
tags: []
author: ""
---

<!--
Badges — replace the placeholders or remove this block.
[![CI](https://github.com/org/repo/actions/workflows/ci.yml/badge.svg)](https://github.com/org/repo/actions/workflows/ci.yml)
[![License: PolyForm Shield 1.0.0](https://img.shields.io/badge/License-PolyForm Shield 1.0.0-yellow.svg)](https://polyformproject.org/licenses/shield/1.0.0)
-->

# [Project Name]

> One sentence that tells a reader what this project does and why they should care.

## Overview

<!--
Two to four paragraphs covering:
1. What problem this project solves
2. Who it is for (the primary user)
3. What makes it different from alternatives (if applicable)
4. Current maturity / stability signal (alpha, beta, production-ready)
-->

## Installation

<!--
List every prerequisite before the first install command.
State the minimum supported versions.

**Prerequisites:**
- Node.js >= 20
- pnpm >= 9
-->

\`\`\`sh
npm install -g [package-name]
\`\`\`

## Usage

\`\`\`sh
[command] --flag value
\`\`\`

## Configuration

<!--
Optional section. Document each config option as a table:
| Key | Type | Default | Description |
|---|---|---|---|
| \`port\` | number | \`3001\` | Dev server port |
-->

## Contributing

\`\`\`sh
git clone https://github.com/org/repo
cd repo
pnpm install
pnpm test
\`\`\`

## License

[PolyForm Shield 1.0.0](./LICENSE)
`,

  "how-to": `---
title: "How to [accomplish X]"
type: how-to
status: published   # draft | published | deprecated
date: YYYY-MM-DD
project: ""
tags: []
author: ""
---

<!--
A how-to guide answers: "How do I accomplish X?"
Keep it tightly scoped to one task.
The reader knows what they want. Give them the shortest correct path to done.
-->

## Prerequisites

<!--
List everything the reader must have in place before step 1.
Be specific about versions and state.

- Prerequisite 1
- Prerequisite 2
-->

## Steps

1. **[First action]**

   \`\`\`sh
   # command
   \`\`\`

   Expected output: \`...\`

2. **[Second action]**

   \`\`\`sh
   # command
   \`\`\`

3. **[Third action]**

## Verification

<!--
How does the reader confirm the task is complete?
Give them a concrete check — a command with expected output.

\`\`\`sh
# verification command
expected output
\`\`\`
-->

## Troubleshooting

<!--
The two or three most common failure modes only.

### Problem: [Error message or symptom]

**Cause:** [One sentence explanation]

**Fix:**
\`\`\`sh
# fix command
\`\`\`
-->
`,
};

// ---------------------------------------------------------------------------
// Template metadata
// ---------------------------------------------------------------------------

const TEMPLATE_META: TemplateInfo[] = [
  {
    type: "adr",
    title: "Architecture Decision Record",
    description:
      "Capture a significant architectural decision: the context that forced it, the decision itself, its consequences, and the alternatives considered.",
  },
  {
    type: "api-reference",
    title: "API Reference",
    description:
      "Document a REST API resource or endpoint group with authentication details, endpoint specifications, request/response examples, and error codes.",
  },
  {
    type: "runbook",
    title: "Runbook",
    description:
      "Step-by-step incident response or operational procedure for on-call engineers, including symptoms, immediate actions, root cause investigation, and prevention.",
  },
  {
    type: "readme",
    title: "README",
    description:
      "Project overview document with installation instructions, usage examples, configuration reference, and contributing guide.",
  },
  {
    type: "how-to",
    title: "How-To Guide",
    description:
      "Task-oriented guide that walks a reader through accomplishing a specific goal, with prerequisites, numbered steps, verification, and troubleshooting.",
  },
];

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

/**
 * Return the raw Markdown template string for the given document type.
 *
 * @param type  Document type string, e.g. "adr", "runbook".
 * @throws      Error if the type is not recognised. Callers should validate
 *              the type first using `listTemplates()` or catch the error.
 */
export function getTemplate(type: string): string {
  const template = TEMPLATES[type];
  if (template === undefined) {
    throw new Error(
      `Unknown template type "${type}". Valid types: ${Object.keys(TEMPLATES).join(", ")}`
    );
  }
  return template;
}

/**
 * Return metadata for all registered templates.
 *
 * Useful for rendering a "new document" picker in the UI.
 */
export function listTemplates(): TemplateInfo[] {
  // Return a copy — callers must not mutate the registry.
  return TEMPLATE_META.map((t) => ({ ...t }));
}
