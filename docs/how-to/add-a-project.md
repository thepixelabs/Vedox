---
title: "How to Add a Project to Vedox"
type: how-to
status: published
date: 2026-04-07
project: "vedox"
tags: ["workspace", "project", "import", "symlink", "onboarding"]
author: "Vedox Team"
---

A how-to guide is task-oriented. It answers one question: how do I accomplish this specific thing? This guide covers adding a local Git repository to Vedox so its documentation appears in the sidebar. There are two paths depending on whether you want Vedox to own the files or read them in place.

## Prerequisites

- `vedox` >= 0.1.0 is installed (`vedox --version` prints a version number)
- `vedox dev` is running and the UI is accessible at http://127.0.0.1:3001
- You have a local Git repository whose documentation you want to add
- The repository has at least one Markdown file (`.md`) to confirm scanning works

## Choosing a Path

| | Path A: Import & Migrate | Path B: Symlink / Fetch |
|---|---|---|
| Files move to | `~/.vedox/workspace/<project>/` | Stay in their original location |
| Editing in Vedox | Yes — full read/write | No — read-only |
| Frontmatter injection | Yes | No |
| Git history in Vedox | New commits after import | Not tracked |
| Best for | Docs you actively maintain in Vedox | Reference docs owned by another repo |

---

## Path A: Import & Migrate

Use this path when you want Vedox to be the primary editor for the project's documentation.

1. **Open the Projects page.**

   Navigate to http://127.0.0.1:3001/projects and click **Add Project**.

2. **Select "Import & Migrate"** from the project type selector.

3. **Enter the source directory path** — the absolute path to the root of your Git repository.

   ```sh
   # Example
   /Users/yourname/code/my-api
   ```

   Vedox scans the directory and shows a preview of Markdown files it will import.

4. **Review the file list.** Deselect any files you do not want to migrate (for example, `node_modules`, generated docs, or third-party files).

5. **Click "Import".** Vedox copies the selected Markdown files into `~/.vedox/workspace/<project-slug>/`, preserving the directory structure. It indexes them immediately.

   Expected: the terminal running `vedox dev` logs lines like:
   ```
   {"level":"info","msg":"imported","path":"docs/architecture.md","project":"my-api"}
   ```

6. **Commit the removal from your origin repository.** The files now live in Vedox. Remove them from the source repo so the two copies do not diverge.

   ```sh
   cd /Users/yourname/code/my-api
   git rm -r docs/
   git commit -m "docs: migrate documentation to Vedox"
   git push
   ```

   > This step is your responsibility. Vedox does not modify the origin repository.

---

## Path B: Symlink / Fetch (Read-Only)

Use this path when another team or tool owns the files and you want Vedox to surface them without taking ownership.

1. **Open the Projects page.**

   Navigate to http://127.0.0.1:3001/projects and click **Add Project**.

2. **Select "Symlink / Fetch"** from the project type selector.

3. **Enter the source directory path** — the absolute path to the root of the repository.

   ```sh
   /Users/yourname/code/third-party-service
   ```

4. **Click "Add".** Vedox registers a `SymlinkAdapter` entry pointing at the directory. No files are copied. The project appears in the sidebar marked with a read-only badge.

5. **Verify the project appears** in the sidebar. Open a file — it will render in the WYSIWYG viewer but the editor toolbar will be disabled and a "Read-only" banner will appear at the top.

   > Vedox watches the resolved path on disk (not the symlink itself) for file changes and re-indexes automatically within 300ms of a change.

---

## Verification

After either path, confirm the project is indexed correctly:

1. Open http://127.0.0.1:3001 and look for the project name in the left sidebar.

2. Open the search bar (keyboard shortcut: `Cmd+K` / `Ctrl+K`) and search for a word you know appears in one of the imported documents.

3. The document should appear in results and open cleanly.

```sh
# You can also confirm the index from the terminal
sqlite3 ~/.vedox/index.db "SELECT slug, title FROM documents WHERE project = 'my-api' LIMIT 10;"
```

---

## Troubleshooting

### Problem: "No projects found" after import — the sidebar is empty

**Cause:** The import completed but the indexer did not finish, or `vedox.config.ts` is not in the directory where you started `vedox dev`.

**Fix:** Confirm `vedox.config.ts` exists in the directory you ran `vedox dev` from. If it does, trigger a manual reindex:

```sh
vedox reindex
```

Expected output ends with `reindex complete — N documents indexed`.

---

### Problem: Import fails partway through — some files appear, others do not

**Cause:** A file exceeded the size limit, contained a path that resolved outside the workspace boundary, or matched a blocked filename (`.env`, `*.key`, etc.).

**Fix:** Check the log for the exact file that caused the failure:

```sh
grep "level=error" ~/.vedox/logs/vedox-$(date +%Y-%m-%d).log | tail -20
```

Remove or rename the offending file, then re-run the import. Vedox skips files it has already imported, so a re-run will only process the ones that failed.

---

### Problem: Scanning finds nothing — the file preview in step 3 is empty

**Cause:** The directory contains no `.md` files, or all Markdown files are nested under a path that matches an ignore pattern (for example, `node_modules/`).

**Fix:** Confirm at least one `.md` file exists at a path that is not excluded:

```sh
find /Users/yourname/code/my-api -name "*.md" -not -path "*/node_modules/*" | head -10
```

If this returns results, the files are present. Refresh the preview in the Vedox UI. If this returns nothing, the repository has no Markdown documentation to import.
