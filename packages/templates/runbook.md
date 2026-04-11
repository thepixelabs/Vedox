---
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

Update `last_tested` every time you drill this runbook in a game day or real incident.
A runbook that has never been tested is a guess, not a procedure.
-->

## Symptoms

<!--
What does the engineer on call observe that leads them to this runbook?
List observable signals, not internal states.

Good: "The Vedox dev server process is absent from `ps aux`"
Bad: "The server has crashed" (too vague — how do you know?)

Include:
- Monitoring alert name or description that fires
- What the user reports ("I get a blank screen when I open Vedox")
- Log patterns to look for (include the exact log line or grep command)

```
grep "level=error" ~/.vedox/logs/vedox-$(date +%Y-%m-%d).log | tail -20
```
-->

## Immediate Actions

<!--
Write these steps for someone paged at 3am on their first week.
Each step must be a single, complete, testable action.
Number the steps. Do not group multiple actions in one step.

The goal of this section is to STOP THE BLEEDING, not to fix root cause.
Defer investigation to the next section.

1. [Action] — [expected outcome, how you know it worked]
2. [Action] — [expected outcome]
-->

## Root Cause Investigation

<!--
Once the system is stable (or the incident is contained), find out why.

Structure this as a decision tree or ordered checklist.
Each check should have a clear yes/no or value to look for.

Example format:
- [ ] Check A — if yes, go to Resolution Steps > Case A
- [ ] Check B — if no, check C
- [ ] Check C — ...

Include specific commands, file paths, and example output.

```
# Check disk space
df -h ~/.vedox/

# Check SQLite integrity
sqlite3 ~/.vedox/index.db "PRAGMA integrity_check;"
# Expected output: ok
```
-->

## Resolution Steps

<!--
One subsection per root cause identified above.

### Case A: [Root cause name]

Step-by-step remediation. Commands must be copy-pasteable.
State the expected outcome after each step.

### Case B: [Root cause name]

...

At the end of all resolution steps, include a verification check:
confirm the system is healthy before closing the incident.
-->

## Prevention

<!--
What should we do to prevent this from happening again?

Format as action items with owners and due dates where known:
- [ ] [Action] — owner: @name, due: YYYY-MM-DD
- [ ] [Action]

If this runbook was triggered by a real incident, link the post-mortem here.
-->
