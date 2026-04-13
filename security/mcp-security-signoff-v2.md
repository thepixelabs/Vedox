# MCP Security Sign-off v2 — Blocking Condition Re-Review

**Date:** 2026-04-12
**Reviewer:** security-engineer
**Epic:** pylon-mcp-security
**Base commit reviewed against:** d976dab
**Prior sign-off:** mcp-security-signoff.md (Phase 7 output)

---

## Summary

All six blocking conditions (BC-1 through BC-6) have been verified against the live
codebase. Each condition is addressed by concrete, auditable implementation — not
documentation or configuration alone.

**Final verdict: APPROVED**

Command execution (`commandExecEnabled`) is cleared to ship under the conditions
documented in the "What Ships" section below.

---

## BC-1 — FINDING-001 CRITICAL — Command Allowlist

**File:** `packages/app/src/lib/mcp/security.ts`

**Verdict: PASS**

| Check | Result | Evidence |
|---|---|---|
| `DEFAULT_ALLOWED_COMMANDS` constant exists | PASS | Line 170 — `readonly string[]` of 28 binaries |
| `NEVER_ALLOW_COMMANDS` Set exists | PASS | Line 184 — `ReadonlySet<string>` of 40+ entries |
| `ALLOWED_BIN_DIRS` exists | PASS | Line 213 — 9 trusted paths including `/opt/homebrew/bin` |
| NEVER_ALLOW checked before allowlist | PASS | Line 357 — NEVER_ALLOW check is first in `validateCommand()` |
| Allowlist checked second | PASS | Lines 378-391 |
| `which` resolution checked third | PASS | Lines 393-400 via `resolveCommandPath()` |
| Bin-dir check fourth | PASS | Lines 402-410 |
| NEVER_ALLOW cannot be overridden by config | PASS | Lines 382-384 — loop deletes NEVER_ALLOW entries from effective `allowedSet` even when present in `additionalAllowed` |

**Observation (non-blocking):** `DEFAULT_ALLOWED_COMMANDS` includes `curl`, `wget`,
`python3`, `ruby`, `node`, and `go`, all of which also appear in `NEVER_ALLOW_COMMANDS`.
The double-deletion logic at lines 382-384 correctly resolves this overlap. However,
having these entries in both lists increases maintenance surface. Recommend a follow-up
cleanup to remove the overlap from `DEFAULT_ALLOWED_COMMANDS` so the lists are
semantically non-overlapping — this is a documentation hygiene issue only, not a
security defect, because NEVER_ALLOW takes hard precedence at both check points
(line 357 and lines 382-384).

---

## BC-2 — FINDING-002 HIGH — Per-Argument Metacharacter Validation

**File:** `packages/app/src/lib/mcp/security.ts`

**Verdict: PASS**

| Check | Result | Evidence |
|---|---|---|
| `validateCommandArgs(command, args)` exported | PASS | Line 296 — `export function validateCommandArgs` |
| Called by `validateCommand()` | PASS | Line 414 — called after bin-dir check |
| Shell metacharacters blocked (`;`, `&&`, `||`, `|`, `>`, `` ` ``, `$()`, `${}`) | PASS | Lines 249-257 — one pattern per metacharacter |
| Path traversal `../` blocked | PASS | Line 260 |
| Null bytes blocked | PASS | Line 264 |
| Git `-c` flag blocked | PASS | Line 269 |
| Git `--upload-pack`, `--receive-pack`, `--exec` blocked | PASS | Lines 270-272 |
| `find -exec`, `-execdir` blocked | PASS | Lines 275-276 |

**Observation (non-blocking):** Pattern `[/>/, 'shell metacharacter ">"']` at line 254
blocks `>` in any argument, which will also deny legitimate redirect literals that some
programs accept as argument syntax (e.g. `git log --format=">%s"`). This is the correct
conservative choice for a security gate — false positives are preferable to false
negatives here. No change required.

---

## BC-3 — FINDING-004 HIGH — Feature Flag Two-Factor Confirmation

**Files:** `packages/app/src/lib/config.ts`, `packages/app/src/hooks/useMcp.ts`,
`packages/app/bin/pylon.ts`

**Verdict: PASS**

| Check | Result | Evidence |
|---|---|---|
| `commandExecConfirmedAt` field in config schema | PASS | config.ts line 45 — `z.string().optional()` with BC-3 comment |
| `commandExecEnabled` defaults to false | PASS | config.ts line 36 |
| `useMcp` requires BOTH flag AND timestamp | PASS | useMcp.ts lines 134-138 — validates `commandExecEnabled === true`, timestamp present, non-empty, and passes `Date.parse()` |
| `effectiveCommandExecEnabled` gates McpManager instantiation | PASS | useMcp.ts line 140, line 194 — `McpManager` receives `effectiveCommandExecEnabled` |
| `pylon config --confirm-command-exec` CLI command exists | PASS | pylon.ts lines 84-112 — sets both `commandExecEnabled: true` and `commandExecConfirmedAt: new Date().toISOString()` |
| Startup warning when flag set without confirmation | PASS | pylon.ts lines 258-270 — early stderr warning before TUI renders |

**Observation (non-blocking):** The startup warning at pylon.ts lines 258-270 checks
`commandExecEnabled === true && commandExecConfirmedAt === undefined || trim === ''`
but does not check `Date.parse()` validity. The runtime enforcement in `useMcp.ts`
does check `Date.parse()`. A corrupt timestamp (non-ISO string) would not trigger the
startup warning but would be caught at the hook level and silently disable command exec.
This is safe — defence-in-depth works correctly here — but the startup warning could
be hardened to match the hook's validation for consistency. Non-blocking.

---

## BC-4 — FINDING-006 MEDIUM — ApprovalDialog Hardening

**File:** `packages/app/src/components/mcp/ApprovalDialog.tsx`

**Verdict: PASS**

| Check | Result | Evidence |
|---|---|---|
| Command + args shown as PRIMARY display | PASS | Lines 73-83 — `{fullCommand}` in `bold` white, section labelled "Command:", appears first after the header |
| LLM description labeled "AI description (unverified):" | PASS | Line 97 — exact string `"AI description (unverified):"` |
| LLM description de-emphasized with `dimColor` | PASS | Lines 97-101 — both the label and the description body use `dimColor` |
| No default accept (user must choose Y or N) | PASS | Lines 42-52 — only `y` approves, `n` or Escape denies; no other key has a default |
| Working directory displayed when set | PASS | Lines 86-90 — cwd shown in gray when present |

---

## BC-5 — FINDING-008 MEDIUM — Rate Limiting

**Files:** `packages/app/src/hooks/useMcp.ts`, `packages/app/src/hooks/useStream.ts`

**Verdict: PASS**

| Check | Result | Evidence |
|---|---|---|
| `MAX_TOOL_CALLS_PER_CONVERSATION = 100` exported | PASS | useMcp.ts line 34 — `export const MAX_TOOL_CALLS_PER_CONVERSATION = 100` |
| Counter resets on `conversationId` change | PASS | useMcp.ts lines 241-243 — `useEffect(..., [conversationId])` sets `toolCallCountRef.current = 0` |
| Calls beyond limit return error result without calling through | PASS | useMcp.ts lines 265-275 — increments counter first, returns error object when `> MAX_TOOL_CALLS_PER_CONVERSATION`, never calls `originalExecute` |
| `maxSteps` reduced to 5 | PASS | useStream.ts line 114 — `maxSteps: 5` (was 10) |

**Observation (non-blocking):** The counter increments before checking the limit
(line 264 increments, line 265 checks). This means call number 101 is the first to
be blocked. At limit 100, calls 1-100 are allowed and call 101 is the first rejection.
This is off-by-one relative to the stated limit name but is not a security concern —
the window is 1 extra call, not unbounded. Acceptable as-is; if the intent is strictly
"no more than 100 calls ever", increment should be moved after the limit check. The
current behaviour is also defensible as "100 is the last allowed call number."

---

## BC-6 — FINDING-007 MEDIUM — Audit Log

**File:** `packages/app/src/lib/mcp/McpManager.ts`

**Verdict: PASS**

| Check | Result | Evidence |
|---|---|---|
| `AuditEntry` interface exported | PASS | Line 62 — `export interface AuditEntry` with `ts`, `tool`, `args`, `outcome`, `detail?`, `conversationId?` |
| `auditLogPath` resolved in constructor to `~/.pylon/mcp-audit.log` | PASS | Lines 163-172 — `join(getPylonDir(), 'mcp-audit.log')` set in constructor |
| `logToolCall()` uses `appendFileSync` | PASS | Line 192 — `appendFileSync(this.auditLogPath, line, { encoding: 'utf-8', flag: 'a' })` |
| `logToolCall()` never throws (swallows errors) | PASS | Lines 193-197 — try/catch writes to stderr but never rethrows |
| Coverage — `mcp_file_read` | PASS | Lines 499, 505, 508 — denied (path), denied (size), allowed |
| Coverage — `mcp_file_list` | PASS | Lines 522, 526 — denied, allowed |
| Coverage — `mcp_file_write` | PASS | Lines 543, 554, 568, 572 — denied (path), denied (size), denied (user), allowed |
| Coverage — `mcp_file_patch` | PASS | Lines 588, 603, 607 — denied (path), denied (user), allowed |
| Coverage — `mcp_command_exec` | PASS | Lines 639, 654, 658 — denied (validation), denied (user), allowed |
| Coverage — `mcp_git_status` | PASS | Lines 711-716 — allowed |
| Coverage — `mcp_git_diff` | PASS | Lines 735-742 — allowed |
| Coverage — `mcp_git_stage` | PASS | Lines 771-778, 782-788 — denied, allowed |
| Coverage — `mcp_git_commit` | PASS | Lines 818-825, 829-834 — denied, allowed |
| Coverage — `mcp_rag_search` | PASS | Lines 871-877 — allowed |
| Coverage — `mcp_rag_index` | PASS | Lines 895-901, 907-912 — denied, allowed |
| File content NOT logged | PASS | `mcp_file_read`: logs `path` and `resolvedPath` only (line 508); `mcp_file_write`: logs `byteLength`, not `content` (line 572) |
| Arg values NOT logged for command-exec | PASS | Line 639/658 — logs `argCount` (integer), not the args array itself |

---

## Findings Not Blocking Shipment

The following observations are recorded for follow-up but do not prevent
`commandExecEnabled` from shipping:

1. **BC-1 overlap cleanup** — `DEFAULT_ALLOWED_COMMANDS` and `NEVER_ALLOW_COMMANDS`
   share entries (`curl`, `wget`, `python3`, `ruby`, `node`, `go`). The runtime
   deduplication is correct; the overlap is a maintenance hazard, not a security defect.

2. **BC-3 startup warning completeness** — startup warning does not validate
   `commandExecConfirmedAt` as a parseable ISO date; the hook-level enforcement does.
   Safe as-is (defence-in-depth), but the warning could be more consistent.

3. **BC-5 off-by-one** — `toolCallCountRef` increments before the limit check, so
   call 101 is the first blocked. Acceptable; does not constitute a bypass.

---

## What Ships

**v0.2 — released upon completion of this sign-off (Phase 8 DONE)**

| Tool | Status | Notes |
|---|---|---|
| `mcp_file_read` | APPROVED | Path validation + blocked patterns + size limit + audit |
| `mcp_file_list` | APPROVED | Path validation + audit |
| `mcp_file_write` | APPROVED | Path validation + size limit + approval gate + audit |
| `mcp_file_patch` | APPROVED | Path validation + approval gate + audit |
| `mcp_command_exec` | APPROVED | Allowlist + per-arg validation + approval gate + audit + rate limit + two-factor flag |
| `mcp_git_status` | APPROVED | repoPath validation + audit |
| `mcp_git_diff` | APPROVED | repoPath validation + audit |
| `mcp_git_stage` | APPROVED | repoPath validation + approval gate + audit |
| `mcp_git_commit` | APPROVED | repoPath validation + approval gate + audit |
| `mcp_rag_search` | APPROVED | Audit; query string not logged |
| `mcp_rag_index` | APPROVED | Directory validation against allowedRoots + audit |

**Activation requirement for `mcp_command_exec`:** Users must explicitly run
`pylon config --confirm-command-exec` to stamp `commandExecConfirmedAt`. Setting
`commandExecEnabled: true` alone in `config.json` is insufficient — the feature
remains off until the interactive confirmation is completed.

---

## Sign-off

All six blocking conditions from the original security review are implemented correctly
and verified against the live codebase at commit d976dab (plus all Phase 1-6 changes
in the pylon-mcp-security epic).

**APPROVED to ship `commandExecEnabled` in v0.2.**

-- security-engineer, 2026-04-12
