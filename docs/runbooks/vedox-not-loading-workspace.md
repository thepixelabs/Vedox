---
title: "Vedox Not Loading Workspace"
type: runbook
status: published
date: 2026-04-07
project: "vedox"
on_call_severity: P2
last_tested: 2026-04-07
tags: ["workspace", "startup", "port", "git", "database", "incident-response"]
author: "Vedox Team"
---

**Severity:** P2 — Degraded functionality. The Vedox workspace is inaccessible but no data loss has occurred. Target time-to-mitigate: 2 hours.

**Update `last_tested`** every time you work through this runbook — in a real incident or a scheduled drill. A runbook that has never been executed is a guess.

---

## Symptoms

Look for any of the following. Each maps to a numbered error code in Vedox's CLI error taxonomy.

- **Blank white screen** at http://127.0.0.1:3001 — the browser connects but nothing renders
- **"No projects found"** message on the projects page — the server is up but the workspace is not indexed
- **`VDX-001` in the terminal** — port 3001 is already in use; `vedox dev` failed to bind
- **`VDX-003` in the terminal** — Git identity is unset (`git config user.email` or `user.name` is missing); Vedox cannot commit
- **`VDX-008` in the terminal** — SQLite database error on startup; the index file may be corrupt

Confirm which error is present before proceeding:

```sh
grep "level=error" ~/.vedox/logs/vedox-$(date +%Y-%m-%d).log | tail -20
```

---

## Immediate Actions

Write these steps for someone paged with no prior context. Complete each step before moving to the next.

1. **Check whether the `vedox dev` process is running.**

   ```sh
   ps aux | grep "vedox dev" | grep -v grep
   ```

   If no output appears, the process has exited. Go to **Root Cause Investigation** to determine why before restarting.

2. **If the process is running but the UI is blank,** check that the browser is connecting to the correct address.

   Open http://127.0.0.1:3001 — not `localhost:3001`. On some systems `localhost` resolves to `::1` (IPv6), which Vedox does not bind to by default.

3. **If the terminal shows `VDX-001` (port conflict),** find what is using the port and stop it:

   ```sh
   lsof -i :3001
   ```

   Note the PID in the second column. If it is a process you own:

   ```sh
   kill <PID>
   ```

   Then restart `vedox dev`. If the conflicting process should not be stopped, change Vedox's port instead — see Resolution Steps > Case B.

4. **If the terminal shows `VDX-003` (Git identity missing),** set your Git identity immediately. Vedox cannot commit documents without it:

   ```sh
   git config --global user.email "you@example.com"
   git config --global user.name "Your Name"
   ```

   Restart `vedox dev` after setting these values.

5. **If the terminal shows `VDX-008` (database error),** do not restart the process repeatedly. Go directly to **Root Cause Investigation > Check C** to determine whether the SQLite file is corrupt before taking further action.

---

## Root Cause Investigation

Work through each check in order. Stop at the first one that fails — that is your root cause. Follow the pointer to the Resolution Steps.

- [ ] **Check A — Is the process running in the right directory?**

  ```sh
  # In the terminal where you run vedox dev, confirm vedox.config.ts exists
  ls vedox.config.ts
  ```

  Expected: `vedox.config.ts` — the file exists.
  If missing: go to **Resolution Steps > Case A** (wrong working directory).

- [ ] **Check B — Is port 3001 available?**

  ```sh
  lsof -i :3001
  ```

  Expected: no output (port is free).
  If output appears: go to **Resolution Steps > Case B** (port conflict).

- [ ] **Check C — Is the SQLite index intact?**

  ```sh
  sqlite3 ~/.vedox/index.db "PRAGMA integrity_check;"
  ```

  Expected output: `ok`
  If the file does not exist or outputs anything other than `ok`: go to **Resolution Steps > Case C** (corrupt or missing index).

- [ ] **Check D — Is Git identity configured?**

  ```sh
  git config user.email && git config user.name
  ```

  Expected: two non-empty lines (your email and name).
  If either is blank or the command exits with no output: go to **Resolution Steps > Case D** (unset Git identity).

- [ ] **Check E — Are there Markdown files in the workspace?**

  ```sh
  find ~/.vedox/workspace -name "*.md" | head -5
  ```

  Expected: one or more file paths.
  If no output: the workspace directory is empty. Go to **Resolution Steps > Case E** (empty workspace).

- [ ] **Check F — Is the log showing a file watcher limit error?**

  ```sh
  grep "inotify\|kqueue\|too many open files\|VDX-009" ~/.vedox/logs/vedox-$(date +%Y-%m-%d).log
  ```

  If any output appears: go to **Resolution Steps > Case F** (file watcher limit reached).

---

## Resolution Steps

### Case A: Wrong Working Directory

`vedox dev` was started from a directory that does not contain `vedox.config.ts`. Vedox cannot locate the workspace root and fails silently or shows an empty project list.

1. Stop `vedox dev` (Ctrl+C in the terminal).

2. Change to the correct directory — the one containing `vedox.config.ts`:

   ```sh
   cd /path/to/your/workspace
   ls vedox.config.ts   # confirm it exists
   ```

3. Restart:

   ```sh
   vedox dev
   ```

4. Expected: the terminal prints `ready at http://127.0.0.1:3001` and the sidebar shows your projects.

---

### Case B: Port Conflict (VDX-001)

Another process is bound to port 3001. Vedox hard-fails on startup when the port is unavailable.

**Option 1 — Stop the conflicting process:**

```sh
lsof -i :3001
# Note the PID in column 2
kill <PID>
vedox dev
```

**Option 2 — Change Vedox's port** (use this if the conflicting process should not be stopped):

Open `vedox.config.ts` in the workspace root and set a different port:

```ts
export default {
  dev: {
    port: 3002
  }
}
```

Then restart:

```sh
vedox dev
# Expected: ready at http://127.0.0.1:3002
```

Update any bookmarks or scripts that reference the old port.

---

### Case C: Corrupt or Missing SQLite Index (VDX-008)

The SQLite index is either missing or has failed its integrity check. Because Markdown files are the source of truth, a corrupt or deleted index is fully recoverable with no data loss.

1. Stop `vedox dev` (Ctrl+C).

2. Remove the corrupt index:

   ```sh
   rm ~/.vedox/index.db
   ```

3. Rebuild the index from the Markdown file tree:

   ```sh
   vedox reindex
   ```

   Expected output (last line): `reindex complete — N documents indexed`

4. Restart the dev server:

   ```sh
   vedox dev
   ```

5. Verify: open http://127.0.0.1:3001, confirm projects appear in the sidebar, and confirm search returns results.

> If `vedox reindex` itself fails with an error, check whether the Markdown files in `~/.vedox/workspace/` are intact:
> ```sh
> find ~/.vedox/workspace -name "*.md" | wc -l
> ```
> A non-zero count confirms the source files are present. If the count is zero, your workspace documents are missing — this is data loss and requires restoring from Git.

---

### Case D: Unset Git Identity (VDX-003)

Vedox sources commit authorship from the local Git config. If `user.email` or `user.name` is unset, Vedox cannot commit documents on Publish and fails with `VDX-003`.

1. Set the Git identity:

   ```sh
   git config --global user.email "you@example.com"
   git config --global user.name "Your Name"
   ```

2. Confirm both values are set:

   ```sh
   git config user.email
   git config user.name
   ```

3. Restart `vedox dev`. The VDX-003 error will not recur.

> If you are in a CI or container environment where a global `.gitconfig` is not available, set the identity in the workspace repository directly:
> ```sh
> cd /path/to/workspace
> git config user.email "ci@example.com"
> git config user.name "CI Bot"
> ```

---

### Case E: Empty Workspace

The workspace directory exists but contains no Markdown files. The project list is empty because there is nothing to index.

1. If you intended to migrate an existing project, follow the [Add a Project](../how-to/add-a-project.md) guide.

2. If you want to start fresh, create a new document from the Vedox UI:
   - Open http://127.0.0.1:3001
   - Click **New Document** in the first-run empty state
   - Choose a template and save — this creates the first `.md` file in the workspace

---

### Case F: File Watcher Limit Reached (VDX-009)

On Linux, the kernel limits the number of files `inotify` can watch. Vedox warns at 1000 watched files and logs `VDX-009`. This commonly happens when `node_modules/` or a build artifact directory is inside the workspace root.

1. Check the current inotify limit:

   ```sh
   cat /proc/sys/fs/inotify/max_user_watches
   ```

2. Add ignore patterns to `vedox.config.ts` to exclude high-volume directories:

   ```ts
   export default {
     dev: {
       ignore: ["**/node_modules/**", "**/dist/**", "**/.git/**"]
     }
   }
   ```

3. Restart `vedox dev`. The watched file count should drop below 1000.

4. If the legitimate workspace genuinely requires more than 1000 watched files, increase the kernel limit temporarily:

   ```sh
   sudo sysctl fs.inotify.max_user_watches=65536
   ```

   To persist across reboots on Ubuntu/Debian:

   ```sh
   echo "fs.inotify.max_user_watches=65536" | sudo tee -a /etc/sysctl.conf
   sudo sysctl -p
   ```

   This change is system-wide and permanent. Confirm with your system administrator before applying on a shared machine.

---

## Verification

Before closing the incident, confirm all of the following:

```sh
# 1. Process is running
ps aux | grep "vedox dev" | grep -v grep

# 2. UI loads
curl -s -o /dev/null -w "%{http_code}" http://127.0.0.1:3001
# Expected: 200

# 3. Index is intact
sqlite3 ~/.vedox/index.db "PRAGMA integrity_check;"
# Expected: ok

# 4. Documents are indexed
sqlite3 ~/.vedox/index.db "SELECT COUNT(*) FROM documents;"
# Expected: a number greater than 0
```

---

## Prevention

- [ ] Add `**/node_modules/**` and `**/dist/**` to the `ignore` list in `vedox.config.ts` before your workspace grows — owner: @yourname, due: ongoing
- [ ] Set your Git identity in `~/.gitconfig` on every new machine before running `vedox dev` for the first time — owner: @yourname, due: on each new machine setup
- [ ] Run `vedox reindex` as a scheduled drill quarterly to confirm the recovery path works on your current workspace — owner: team, due: quarterly
- [ ] If running on Linux, check `cat /proc/sys/fs/inotify/max_user_watches` and increase to 65536 proactively if the default is 8192 — owner: @yourname, due: 2026-04-14
- [ ] Pin `vedox dev` to a dedicated terminal or process manager so port 3001 is consistently reserved — owner: @yourname, due: ongoing
