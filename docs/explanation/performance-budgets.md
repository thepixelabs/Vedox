---
title: "Performance Budgets"
type: explanation
status: published
date: 2026-04-11
project: "vedox"
tags: ["flagship-ux", "performance", "ci", "budgets"]
author: "Vedox Team"
---

# Performance Budgets

Vedox defines five p95 latency budgets for the most common user actions, enforces them in CI on every pull request that touches the editor or CLI, and ships a deterministic 10,000-file reference workspace so the measurements are repeatable. This document explains the budgets, the CI wiring, the reference workspace, and how to run the checks locally.

Sources:
- `tests/perf/budgets.json`
- `.github/workflows/perf.yml`
- `tests/perf/build-reference-workspace.ts`
- `tests/perf/run-budgets.ts`

---

## The five budgets

All budgets are p95 latencies (the 95th percentile of 10 samples) in milliseconds.

| Budget | Value | What it measures |
|---|---|---|
| `coldLoadP95` | 250 ms | Time to first byte on the editor root route after a server cold start |
| `cmdKFirstResultP95` | 50 ms | Time from opening the command palette to the first matching result |
| `fileOpenP95` | 50 ms | Time from clicking a sidebar item to the document being visible and focused |
| `modeToggleP95` | 100 ms | Time to swap between WYSIWYG and source modes on the active pane |
| `fileSwitchP95` | 30 ms | Time to switch between two documents that are both already in memory |

The budgets live in `tests/perf/budgets.json` as a single flat object. Raising a budget requires a PR that changes that file — the values are under source control, so every increase is a visible, reviewable decision.

The numbers are not aspirational. They are the numbers the editor meets today on a reference MacBook with the reference workspace. If a change makes the editor slower, the PR fails CI. If a change makes the editor faster, the improvement is free — the budget does not auto-tighten.

---

## How CI enforces them

The `.github/workflows/perf.yml` workflow runs on every pull request that touches `apps/editor/**`, `apps/cli/**`, `tests/perf/**`, or the workflow file itself. It can also be triggered manually via `workflow_dispatch`.

Workflow steps:

1. **Checkout and toolchain setup.** Actions for `pnpm`, Node 20 with pnpm cache, Go 1.23 with the `apps/cli/go.sum` cache.
2. **Install dependencies.** `pnpm install --frozen-lockfile` with `VEDOX_SKIP_FONTS=1` to skip fontsource binary downloads that are not needed for perf runs.
3. **Build the CLI.** `go build ./apps/cli/...` — required because the dev server talks to the CLI over HTTP.
4. **Generate the reference workspace.** `pnpm exec tsx tests/perf/build-reference-workspace.ts` writes 10,000 deterministic Markdown files under `tests/perf/reference-workspace/`.
5. **Start the dev server.** `cd apps/editor && pnpm dev &` in the background, then `pnpm exec wait-on http://localhost:5151 --timeout 60000` to block until it is ready.
6. **Run the budget checker.** `pnpm exec tsx tests/perf/run-budgets.ts` against `PERF_BASE_URL=http://localhost:5151`.
7. **Upload results.** The `tests/perf/results/` directory is uploaded as a workflow artifact regardless of pass/fail, so a failure is inspectable after the fact.

The job has a 15-minute timeout. Any single step that hangs longer than 15 minutes fails the whole job, which means a runaway dev server or a stuck perf test does not burn CI minutes indefinitely.

---

## The reference workspace

A performance test that runs against a hand-curated workspace is not a performance test — it is a vibe check. The number of files, their sizes, their directory layout, and their content all affect indexing, search, and rendering speed. To get comparable numbers across machines and across time, Vedox generates a synthetic workspace from scratch on every run.

`build-reference-workspace.ts` produces:

- **10,000 Markdown files** (constant `FILE_COUNT`).
- **Five top-level directories**: `docs`, `guides`, `api`, `tutorials`, `reference`.
- **Subdirectories every 100 files**: `section-0`, `section-1`, ..., `section-99`. Each directory holds up to 100 files, which exercises the file-tree walker's branching behavior without producing pathologically deep trees.
- **Deterministic content.** The generator uses a seeded linear congruential generator (`seed = 42`). The same seed produces byte-identical files every time. Two CI runs on different machines generate the same workspace.
- **Realistic content mix.** Every file has a frontmatter block with title, type, status, tags, and date. Bodies contain 2-10 paragraphs of 10-50 word pseudo-sentences (from a small vocabulary of 2,000 pseudo-words). Every fifth file contains a TypeScript code fence; every seventh file contains a three-column table.

The output lives in `tests/perf/reference-workspace/` and is gitignored. Each CI run rebuilds it fresh. Local runs can rebuild it or reuse an existing one — the determinism guarantees they are equivalent.

Why 10,000 files? It is a plausible upper bound for a large documentation set (the Vedox docs corpus itself is ~1,200 files, the largest public docs site surveyed during design was ~4,800). At 10,000 files, any linear-scan behavior in the file watcher or indexer becomes visible as a non-trivial latency. Smaller numbers hide regressions.

---

## Current state of the measurements

`run-budgets.ts` currently measures two of the five budgets directly:

| Budget | Measurement |
|---|---|
| `coldLoadP95` | 10 fetches to `${BASE_URL}/`, sorted, p95 of response times |
| `cmdKFirstResultP95` | 10 fetches to `${BASE_URL}/api/search?q=doc`, sorted, p95 of response times |

The other three (`fileOpenP95`, `modeToggleP95`, `fileSwitchP95`) are printed as `[stub — needs Playwright in v2]`. They require browser automation to drive real click events and measure DOM-paint timings, which the v1 runner does not do. The budgets exist so the target is committed to source; they are not yet enforced end-to-end.

A v2 runner using Playwright is on the roadmap. It will drive the three UI actions through a headless browser, measure `performance.now()` inside the page, and report back via a structured JSON artifact.

For now, a PR that only regresses `fileOpenP95` on paper (say, by adding 200 ms of JavaScript work on file open) will not fail this workflow. It will still fail other CI jobs that run the editor's Vitest suite and type checks, but the perf workflow itself is best-effort on three of five axes.

---

## The sampling method

For the two measured budgets, the runner:

1. Fires 10 sequential `fetch()` calls against the target URL.
2. Awaits the response body on each (`await res.text()`) so the timing includes transport and parse, not just headers.
3. Sorts the 10 samples ascending.
4. Takes the value at index `ceil(0.95 * N) - 1`, which for N=10 is index 9 — the maximum. For N=20 it would be index 18, the second-largest.

This is a conservative p95: with N=10, "p95" is effectively "worst of 10". The conservatism is intentional for a CI gate. A flaky sample pushes the number up, which either fails the PR (and the author investigates) or passes if the worst sample is still under budget.

The runner exits with code 1 if any measured budget exceeds its target. In CI, a non-zero exit fails the workflow step, which fails the job, which marks the PR check red.

---

## How to run locally

Requirements:

- pnpm installed and dependencies fetched (`pnpm install`).
- Go 1.23 available on PATH.
- Node 20.

Steps:

1. **Generate the reference workspace once.**

   ```
   pnpm exec tsx tests/perf/build-reference-workspace.ts
   ```

   This writes ~40 MB of Markdown under `tests/perf/reference-workspace/`. Re-running the command regenerates the same files.

2. **Start the editor dev server.** From the repo root:

   ```
   cd apps/editor && pnpm dev
   ```

   Wait for it to log `Local: http://localhost:5173/` (or whichever port Vite picks).

3. **Run the budget checker.** In a second terminal:

   ```
   PERF_BASE_URL=http://localhost:5173 pnpm exec tsx tests/perf/run-budgets.ts
   ```

   The `PERF_BASE_URL` env var defaults to `http://localhost:5173` if unset. Override it if your dev server runs on a different port (CI uses 5151).

Output looks like:

```
Vedox Performance Budget Check
   Base URL: http://localhost:5173
   Samples per measurement: 10

  Cold load p95:       142ms  (budget: 250ms)  PASS
  Cmd+K first result:  34ms   (budget: 50ms)   PASS
  File open p95:       [stub — needs Playwright in v2]  (budget: 50ms)
  Mode toggle p95:     [stub — needs Playwright in v2]  (budget: 100ms)
  File switch p95:     [stub — needs Playwright in v2]  (budget: 30ms)

PASS: All measured budgets met.
```

A failing run exits with code 1 and prints `FAIL: One or more perf budgets exceeded.`

---

## Changing a budget

To raise or lower a budget, edit `tests/perf/budgets.json` and open a PR. Every change is visible in the diff. A budget increase should be justified in the PR description with the reason the latency grew and why the new target is still acceptable.

A budget decrease (tightening) is always welcome but should be grounded in observed performance, not optimism. Tightening a budget below what the code actually achieves makes CI flaky.

---

## File reference

| File | Role |
|---|---|
| `tests/perf/budgets.json` | Budget values, single source of truth |
| `tests/perf/build-reference-workspace.ts` | Deterministic 10k-file workspace generator |
| `tests/perf/run-budgets.ts` | Sampling runner, pass/fail gate |
| `.github/workflows/perf.yml` | CI workflow that runs on PRs |
