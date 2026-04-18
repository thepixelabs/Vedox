/**
 * projects.ts — ProjectsStore
 *
 * In Phase 1 this store holds the in-memory project list. The data will be
 * wired to the Go backend API in a later ticket (VDX-P1-003 or similar).
 * For now it provides the shape and empty-state logic that the app shell needs.
 *
 * A project has:
 *   - id:    unique slug (used as the URL segment)
 *   - name:  display name
 *   - docs:  flat list of doc entries (path + title + type + folder)
 *
 * The store exposes `hasProjects` as a derived boolean used by the layout to
 * choose between the empty state CTA and the full sidebar.
 */

import { writable, derived } from "svelte/store";

/**
 * Diataxis-aligned doc type taxonomy.
 * "other" is the catch-all for docs that don't fit a known category.
 */
export type DocType =
  | "how-to"
  | "explanation"
  | "reference"
  | "tutorial"
  | "adr"
  | "runbook"
  | "readme"
  | "other";

/** Folder name segments known to imply a doc type when not set in frontmatter. */
const FOLDER_TYPE_MAP: Record<string, DocType> = {
  "how-to": "how-to",
  "how_to": "how-to",
  "howto": "how-to",
  "explanation": "explanation",
  "explanations": "explanation",
  "reference": "reference",
  "references": "reference",
  "ref": "reference",
  "tutorial": "tutorial",
  "tutorials": "tutorial",
  "adr": "adr",
  "adrs": "adr",
  "architecture": "adr",
  "runbook": "runbook",
  "runbooks": "runbook",
  "ops": "runbook",
};

/**
 * Infer a DocType from frontmatter metadata and/or filesystem path.
 * Frontmatter `type` field wins; folder name is the fallback heuristic.
 */
export function inferDocType(
  path: string,
  metadata?: Record<string, unknown>
): DocType {
  // 1. Trust explicit frontmatter type.
  const fm = metadata?.["type"];
  if (typeof fm === "string" && fm.trim()) {
    const normalized = fm.trim().toLowerCase() as DocType;
    const known: DocType[] = [
      "how-to",
      "explanation",
      "reference",
      "tutorial",
      "adr",
      "runbook",
      "readme",
      "other",
    ];
    if (known.includes(normalized)) return normalized;
  }

  // 2. README heuristic — file named readme.md (any case).
  const filename = path.split("/").pop() ?? "";
  if (/^readme\.md$/i.test(filename)) return "readme";

  // 3. Folder name heuristic — check each path segment.
  const segments = path.split("/").slice(0, -1); // exclude filename
  for (const seg of segments) {
    const mapped = FOLDER_TYPE_MAP[seg.toLowerCase()];
    if (mapped) return mapped;
  }

  return "other";
}

/**
 * Derive the display folder from a doc path.
 * Returns the parent directory segment(s) relative to the project root,
 * or an empty string for top-level docs.
 *
 * Examples:
 *   "architecture/adr-001.md"     → "architecture"
 *   "how-to/deploy/fly.md"        → "how-to/deploy"
 *   "readme.md"                   → ""
 */
export function deriveFolder(path: string): string {
  const parts = path.split("/");
  if (parts.length <= 1) return "";
  return parts.slice(0, -1).join("/");
}

export interface DocEntry {
  /** Path relative to the project root, e.g. "architecture/adr-001.md" */
  path: string;
  /** Display title — falls back to filename if frontmatter title is absent */
  title: string;
  /**
   * Diataxis-aligned doc type.
   * Derived from frontmatter `type` field or folder name heuristic.
   * Phase 2 addition — defaults to "other" for backwards compatibility.
   */
  type: DocType;
  /**
   * Parent folder path relative to the project root.
   * Empty string for top-level docs.
   * Phase 2 addition.
   */
  folder: string;
}

export interface Project {
  id: string;
  name: string;
  docs: DocEntry[];
  /**
   * Total document count as reported by the backend at project-list time.
   * More accurate than docs.length because the docs array is populated lazily
   * by child routes — it starts empty until a project page fetches the full list.
   */
  docCount?: number;
}

function createProjectsStore() {
  const { subscribe, set, update } = writable<Project[]>([]);

  return {
    subscribe,

    /** Replace the full project list (called when backend data loads). */
    setProjects(projects: Project[]): void {
      set(projects);
    },

    /** Add a single project. */
    addProject(project: Project): void {
      update((list) => [...list, project]);
    },

    /** Remove a project by id. */
    removeProject(id: string): void {
      update((list) => list.filter((p) => p.id !== id));
    },

    /**
     * Set the doc list for a project, deriving type+folder from each doc's
     * path and metadata. Called by project/doc pages after api.getProjectDocs().
     *
     * The `projectId` segment is stripped from each API path — e.g.
     * "myproject/architecture/adr-001.md" → "architecture/adr-001.md".
     */
    setProjectDocs(
      projectId: string,
      docs: Array<{ path: string; metadata?: Record<string, unknown> }>
    ): void {
      update((list) =>
        list.map((p) => {
          if (p.id !== projectId) return p;
          const prefix = projectId + "/";
          const entries: DocEntry[] = docs.map((doc) => {
            // Strip the leading project-name segment.
            const relPath = doc.path.startsWith(prefix)
              ? doc.path.slice(prefix.length)
              : doc.path;
            // Derive title: prefer frontmatter, fall back to filename.
            const fm = doc.metadata?.["title"];
            const title =
              typeof fm === "string" && fm.trim()
                ? fm.trim()
                : relPath.split("/").pop()?.replace(/\.md$/i, "") ?? relPath;
            return {
              path: relPath,
              title,
              type: inferDocType(relPath, doc.metadata),
              folder: deriveFolder(relPath),
            };
          });
          return { ...p, docs: entries };
        })
      );
    },
  };
}

export const projectsStore = createProjectsStore();

/** True when at least one project exists. Drives empty-state / full-UI fork. */
export const hasProjects = derived(
  projectsStore,
  ($projects) => $projects.length > 0
);
