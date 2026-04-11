---
title: "How to Write an ADR Using the Vedox Template"
type: how-to
status: published
date: 2026-04-07
project: "vedox"
tags: ["adr", "architecture", "decisions", "templates"]
author: "Vedox Team"
---

An Architecture Decision Record (ADR) is a short document that captures a significant technical decision: what was decided, why, what alternatives were rejected, and what the consequences are. ADRs are the audit trail that keeps a team from relitigating the same decisions at every architecture review.

Write an ADR when a decision is hard to reverse, affects multiple people, or has non-obvious trade-offs. Do not write one for trivial implementation details — the code documents those.

## Prerequisites

- `vedox dev` is running at http://127.0.0.1:3001
- You are working in a project that already exists in Vedox

## Steps

1. **Create a new document** in your project using the ADR template.

   In the sidebar, click your project name, then click **New Document** and select the **ADR** template from the template picker. Vedox prefills the frontmatter with today's date and the project name.

2. **Fill in the frontmatter fields.**

   ```yaml
   ---
   title: "ADR-003: Use Redis for Session Storage"
   type: adr
   status: proposed
   date: 2026-04-07
   project: "my-api"
   tags: ["session", "storage", "redis", "auth"]
   author: "Vedox Team"
   superseded_by: ""
   ---
   ```

   | Field | What to put here |
   |---|---|
   | `title` | `ADR-<NNN>: <Short decision title>`. Number sequentially. |
   | `type` | Always `adr` — do not change this. |
   | `status` | Start at `proposed`. Change to `accepted` when the team agrees. |
   | `date` | The date you write the ADR, not the date the decision was implemented. |
   | `project` | The slug of the project this decision belongs to. |
   | `tags` | Two to five keywords that help search surface this ADR. |
   | `author` | Your name or team name. |
   | `superseded_by` | Leave empty. Fill in only if a later ADR replaces this one. |

3. **Write the Context section.**

   Describe the situation as it is now. What problem are you solving? What constraints exist? What forces are in tension?

   ```markdown
   ## Context

   Our API stores user sessions in-process memory. This works for a single instance
   but breaks under horizontal scaling — session affinity is either required or users
   are logged out on every request that lands on a different pod.

   We are planning a Kubernetes deployment next quarter. The session store must
   survive pod restarts and work across multiple replicas.

   Constraints: the ops team does not want to manage an additional persistent database.
   The chosen solution must be operable by the existing team with no new on-call runbooks.
   ```

   Write in present tense. Describe reality, not aspirations.

4. **Write the Decision section.**

   Start with a single declarative sentence: "We will [action]." Then explain the reasoning.

   ```markdown
   ## Decision

   We will use Redis (managed via AWS ElastiCache) as the session store.

   Redis is the lowest-friction path to a horizontally-scalable session store.
   The ops team already manages an ElastiCache cluster for the rate limiter, so
   this adds no new infrastructure. The existing Redis client library (go-redis)
   is already in the dependency tree, so no new dependency is introduced.
   ```

   Do not hedge. If the decision is not yet final, set `status: proposed` in the frontmatter — do not write "we might" or "we could" in the body.

5. **Write the Consequences section.**

   List all effects — positive and negative. Do not omit the negatives. A consequences section that is only positive signals that no one thought it through.

   ```markdown
   ## Consequences

   Positive:
   - Session state survives pod restarts and is consistent across all replicas.
   - No sticky sessions required in the load balancer config.
   - Existing ElastiCache cluster absorbs the additional load without resizing.

   Negative:
   - Redis is now in the critical path for every authenticated request. A Redis
     outage means all logged-in users are immediately logged out.
   - Adds a network round-trip per session validation (~0.5ms within the same VPC).
   - Session data is now visible to anyone with Redis access — requires encryption
     at rest and in transit (ElastiCache TLS is already enabled).

   Follow-on work:
   - ADR-004: session data encryption standard
   - Runbook: Redis unavailable (session layer)
   ```

6. **Write the Alternatives Considered section.**

   Document every realistic alternative you rejected and why. Be specific — "too complex" is not a reason; "requires a new operational runbook and we have no Redis expertise" is.

   ```markdown
   ## Alternatives Considered

   ### Option A: Database-backed sessions (PostgreSQL)

   Store sessions in the existing Postgres database. Rejected because Postgres is
   already under write pressure, and session validation adds a read on every request.
   Our DBA declined to support this without read replicas, which are out of scope.

   ### Option B: JWT (stateless)

   Encode session state in signed JWTs and eliminate the session store entirely.
   Rejected because we need server-side session invalidation (logout, security
   revocation). JWTs cannot be reliably invalidated without a blocklist, which
   reintroduces a shared store.

   ### Option C: Sticky sessions (load balancer affinity)

   Pin each user to a single pod. Rejected because it breaks graceful pod shutdown
   and creates hotspots when sessions are unevenly distributed.
   ```

7. **Set the status to `accepted`** once the team has agreed.

   Change `status: proposed` to `status: accepted` in the frontmatter. If the decision is later reversed by a new ADR, set `status: superseded` and fill in `superseded_by: "ADR-007"`.

---

## Verification

After saving, the ADR should appear:

- In the project sidebar under its document title
- In search results when you query any of the words in the title or tags
- With the correct status badge (Proposed / Accepted) in the document list view

---

## Common Mistakes

**Writing the ADR after the fact without recording alternatives.**
If you write the ADR a month after the decision was made, the alternatives section is the most important part. Even if you are reconstructing from memory, record what was considered and why it lost. An ADR without alternatives is just a changelog entry.

**Leaving status as "proposed" indefinitely.**
A proposed ADR is a decision waiting to happen. If it sits at `proposed` for more than two weeks, either the decision has been made and the ADR was not updated, or the decision is blocked and someone needs to escalate. Neither state is acceptable. Set a calendar reminder to close it.

**Writing consequences that are only positive.**
Every real decision has trade-offs. If your consequences section has no negatives, the ADR will not be trusted by anyone who reads it. Name the costs.

**Numbering ADRs non-sequentially or reusing numbers.**
ADR numbers are permanent identifiers. Pad with leading zeros (`ADR-001`, not `ADR-1`) to sort correctly at three digits. Never reuse a number, even if an ADR is deleted.
