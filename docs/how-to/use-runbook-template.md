---
title: "How to Write a Runbook Using the Vedox Template"
type: how-to
status: published
date: 2026-04-07
project: "vedox"
tags: ["runbook", "incident-response", "operations", "templates", "on-call"]
author: "Vedox Team"
difficulty: "intermediate"
estimated_time_minutes: 15
prerequisites:
  - "vedox dev running at http://127.0.0.1:3001"
  - "A documented incident pattern or recurring maintenance task to describe"
---

A runbook is a documented procedure for responding to a specific operational event — an incident, a degradation, or a scheduled maintenance task. It is written for an engineer paged at 3am on their first week who needs to stop the bleeding without fully understanding the system.

Reach for the runbook template when:

- A P1 or P2 incident has occurred and you are writing the response procedure afterward
- You are setting up a recurring maintenance task and need a repeatable checklist
- A postmortem identifies a gap in operational documentation

Do not write a runbook for incidents that have no documented pattern — if you don't know what to do, write an ADR to decide, then a runbook to document the response.

## Prerequisites

- `vedox dev` is running at http://127.0.0.1:3001
- You have a project set up in Vedox for the service this runbook covers

## Steps

1. **Create a new document** and select the **Runbook** template.

   In the sidebar, click your project, then **New Document**, then choose **Runbook** from the template picker.

2. **Fill in the frontmatter fields.**

   ```yaml
   ---
   title: "Vedox Not Loading Workspace"
   type: runbook
   status: published
   date: 2026-04-07
   project: "vedox"
   on_call_severity: P2
   last_tested: 2026-04-07
   tags: ["workspace", "startup", "incident-response"]
   author: "Vedox Team"
   ---
   ```

   | Field | What to put here |
   |---|---|
   | `title` | Describe the problem, not the system. "Database connection failing" not "PostgreSQL runbook". |
   | `type` | Always `runbook` — do not change this. |
   | `status` | `published` when the runbook has been reviewed. `draft` while writing. |
   | `date` | Date the runbook was written or last updated. |
   | `project` | The project slug this runbook covers. |
   | `on_call_severity` | `P1` for user-facing outage or data loss risk. `P2` for degraded functionality. `P3` for minor degradation in business hours. |
   | `last_tested` | The date this runbook was last executed — in a real incident or a drill. Update this every time. |
   | `tags` | Keywords that surface this runbook during search. Include symptom words, not just system names. |
   | `author` | Your name or team name. |

3. **Write the Symptoms section.**

   List what the engineer on call observes — not internal states, but externally observable signals. Include the user report, the monitoring alert, and relevant log lines.

   ```markdown
   ## Symptoms

   - The browser shows a blank white screen at http://127.0.0.1:3001
   - The terminal running `vedox dev` shows error code `VDX-001` (port conflict)
   - A user reports "No projects found" on the projects page
   - The log file contains lines matching:

   ```sh
   grep "level=error" ~/.vedox/logs/vedox-$(date +%Y-%m-%d).log | tail -20
   ```
   ```

4. **Write the Immediate Actions section.**

   These steps stop the bleeding. They are not root cause analysis. Write them for someone who does not know the system.

   Number every step. One action per step. State the expected outcome so the responder knows if it worked.

   ```markdown
   ## Immediate Actions

   1. Check whether `vedox dev` is still running in the terminal. If the process
      has exited, restart it:

      ```sh
      vedox dev
      ```

      Expected: the terminal prints `ready at http://127.0.0.1:3001` within 10 seconds.

   2. If the process starts but immediately exits with `VDX-001`, another process
      is using port 3001. Find and stop it:

      ```sh
      lsof -i :3001
      kill <PID>
      vedox dev
      ```
   ```

5. **Write the Root Cause Investigation section.**

   Structure this as a checklist. Each check has a clear observable value to look for and a pointer to the resolution step to follow.

   ```markdown
   ## Root Cause Investigation

   - [ ] Is the `vedox dev` process running?
         ```sh
         ps aux | grep "vedox dev"
         ```
         If not running: go to Resolution Steps > Case A (process not running).

   - [ ] Is port 3001 in use by another process?
         ```sh
         lsof -i :3001
         ```
         If yes: go to Resolution Steps > Case B (port conflict).

   - [ ] Is `vedox.config.ts` present in the directory where you ran `vedox dev`?
         ```sh
         ls vedox.config.ts
         ```
         If missing: go to Resolution Steps > Case C (wrong directory).
   ```

6. **Write one Resolution subsection per root cause.**

   Each case maps to one check from the investigation section. Commands must be copy-pasteable. State the expected outcome after each command.

   ```markdown
   ## Resolution Steps

   ### Case A: Process not running

   1. Restart the dev server:
      ```sh
      vedox dev
      ```
   2. Verify the UI loads at http://127.0.0.1:3001.

   ### Case B: Port conflict

   1. Find the process using port 3001:
      ```sh
      lsof -i :3001
      ```
   2. Stop the conflicting process or change Vedox's port in `vedox.config.ts`:
      ```ts
      export default { dev: { port: 3002 } }
      ```
   3. Restart `vedox dev` and verify the UI loads on the new port.
   ```

7. **Write the Prevention section.**

   What can be done to prevent this class of incident? Format as action items.

   ```markdown
   ## Prevention

   - [ ] Add `vedox dev` to a process manager (e.g. `launchd` on macOS) so it
         restarts automatically after crashes — owner: @yourname, due: 2026-04-14
   - [ ] Document the port number in the project README so team members know
         not to bind other services to 3001
   ```

---

## Verification

After saving:

- The runbook appears in the project sidebar
- The severity badge (`P1`, `P2`, or `P3`) displays correctly in the document list
- `last_tested` is set to today's date (you will update it after your first drill)

---

## The Most Important Rule

**Update `last_tested` after every incident or drill.** A runbook with a `last_tested` date more than 90 days old is a guess, not a procedure. Systems change. Commands that worked in January may fail in April because a file moved, a port changed, or a dependency was upgraded. The only way to know a runbook works is to run it. Schedule a quarterly drill.

---

## Troubleshooting

### Problem: The runbook steps are so long that responders skip steps

**Cause:** Each "step" contains multiple actions bundled together.

**Fix:** Break any step that contains the word "and" into two steps. If step 3 says "restart the process and verify the UI loads," that is two steps: restart the process, then verify.

### Problem: The Immediate Actions section is being used for root cause analysis

**Cause:** The distinction between stopping the bleeding and finding the cause is blurry.

**Fix:** Keep Immediate Actions to five steps or fewer. If you are writing more than five, the extra steps belong in Root Cause Investigation or Resolution Steps.
