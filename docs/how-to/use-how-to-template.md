---
title: "How to Write a How-To Guide Using the Vedox Template"
type: how-to
status: published
date: 2026-04-07
project: "vedox"
tags: ["how-to", "templates", "diataxis", "writing", "documentation"]
author: "Vedox Team"
---

This document explains how to write a how-to guide using Vedox's how-to template. It is intentionally self-referential — a how-to about how-tos — so reading it is also a demonstration of the format.

## Prerequisites

- `vedox dev` is running at http://127.0.0.1:3001
- You know what task you want to document (if you are still deciding what to write, read the section on Diataxis below first)

## The Diataxis Framework in 90 Seconds

Vedox uses the Diataxis framework to keep documentation purposeful. There are four document types. Each serves one audience need.

| Type | Question it answers | Example |
|---|---|---|
| **How-To** | How do I accomplish X? | How to add a project to Vedox |
| **Tutorial** | How do I learn to use this system? | Building your first Vedox workspace from scratch |
| **Reference** | What are the exact details of X? | API Reference for `/documents` |
| **Explanation** | Why does the system work this way? | Why Vedox uses Markdown as the source of truth |

The most common mistake is mixing types. A how-to that explains background theory is no longer a how-to — the background belongs in an explanation doc, linked from the how-to. Keep each document to exactly one type.

**How-To vs. Tutorial:** A how-to assumes the reader knows what they want and just needs the steps. A tutorial starts from zero and teaches as it goes. If your document contains sentences like "this is important because..." or teaches a concept before a step, it is either a tutorial or has embedded explanation that should be extracted.

## Steps

1. **Create a new document** and select the **How-To** template.

   In the sidebar, click your project, then **New Document**, then choose **How-To** from the template picker.

2. **Fill in the frontmatter fields.**

   ```yaml
   ---
   title: "How to Deploy to Staging"
   type: how-to
   status: draft
   date: 2026-04-07
   project: "my-api"
   tags: ["deploy", "staging", "ci"]
   author: "Vedox Team"
   ---
   ```

   Set `status: draft` while writing. Change to `published` when at least one other person has verified the steps work.

3. **Write a one-sentence purpose statement** immediately after the frontmatter.

   Before the Prerequisites heading, write a single sentence that states what the reader will have accomplished by the end of this guide. This is not a heading — it is the opening line of the document.

   ```markdown
   This guide walks you through deploying a reviewed pull request to the staging environment.
   ```

4. **Write the Prerequisites section.**

   List everything the reader must have in place before step 1. Be specific. Include version numbers when they matter. State the expected system state, not just installed tools.

   ```markdown
   ## Prerequisites

   - `kubectl` >= 1.28 installed and configured for the staging cluster
   - The pull request you want to deploy has been approved and all CI checks pass
   - You have write access to the `staging` namespace
   ```

   Good prerequisites are testable. The reader can verify each one before proceeding. Avoid prerequisites like "you understand Kubernetes" — that is vague and cannot be verified.

5. **Write numbered steps, one action per step.**

   Each step is a single discrete action. Use an imperative verb to start each one. Show the command or UI action in a code block. If the step produces output the reader needs to verify, show it.

   ```markdown
   1. **Retrieve the image tag** for the pull request build.

      ```sh
      gh run list --branch my-feature-branch --limit 1 --json headSha
      ```

      Expected output: `[{"headSha":"a3f2b1c..."}]`

   2. **Update the staging deployment** with the new image tag.

      ```sh
      kubectl set image deployment/my-api my-api=ghcr.io/acme/my-api:a3f2b1c -n staging
      ```

   3. **Watch the rollout** until it completes.

      ```sh
      kubectl rollout status deployment/my-api -n staging
      ```

      Expected output: `deployment "my-api" successfully rolled out`
   ```

6. **Write the Verification section.**

   Give the reader one concrete check that confirms the task is done. A command with expected output is best. "You should see X" without saying where to look is not enough.

   ```markdown
   ## Verification

   ```sh
   curl -s https://staging.acme.example.com/health | jq .status
   # Expected output: "ok"
   ```
   ```

7. **Write the Troubleshooting section.**

   Cover the two or three most common failures only. For each: the observable symptom, why it happens, and the fix. Do not try to be exhaustive — an exhaustive troubleshooting section signals the guide needs to be rewritten.

   ```markdown
   ## Troubleshooting

   ### Problem: `kubectl set image` succeeds but pods crash immediately

   **Cause:** The image tag does not exist in the registry, or the container fails
   its health check.

   **Fix:**
   ```sh
   kubectl describe pod -l app=my-api -n staging | grep -A 10 Events
   ```
   Look for `ImagePullBackOff` (wrong tag) or `CrashLoopBackOff` (health check
   failure). Check the image tag against the CI build artifacts.
   ```

---

## Verification

After saving your how-to:

- Ask one other person to follow the steps cold and report where they got confused or stuck
- Any step that requires explanation is a step that needs to be broken into smaller steps or have a prerequisite added
- The total word count should be under 500 words (use the word count indicator in the editor status bar)

---

## The 500-Word Rule

Keep each how-to under 500 words. This constraint forces two good outcomes: it prevents background theory from creeping in (move that to an explanation doc), and it forces you to identify the one task the guide is actually about. If you cannot fit it in 500 words, you have either included too much explanation (extract it) or you are documenting two tasks (split them into two how-tos).

---

## Troubleshooting

### Problem: The how-to keeps growing — every step requires context

**Cause:** The reader needs background knowledge before they can execute the steps. That background belongs in an explanation doc.

**Fix:** Identify the background content, move it to a new explanation document, and replace it in the how-to with a single sentence and a link: "For background on how the deployment pipeline works, see [Deployment Pipeline Architecture](../explanations/deployment-pipeline.md)."

### Problem: The steps work but the reader does not know if they succeeded

**Cause:** The Verification section is missing or too vague ("you should now see the new version").

**Fix:** Add a concrete, testable check — a `curl` command with expected output, a UI state that is unambiguous, or a test command that passes.
