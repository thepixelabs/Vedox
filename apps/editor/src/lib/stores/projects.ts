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
 *   - docs:  flat list of doc entries (path + title)
 *
 * The store exposes `hasProjects` as a derived boolean used by the layout to
 * choose between the empty state CTA and the full sidebar.
 */

import { writable, derived } from "svelte/store";

export interface DocEntry {
  /** Path relative to the project root, e.g. "architecture/adr-001.md" */
  path: string;
  /** Display title — falls back to filename if frontmatter title is absent */
  title: string;
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
  };
}

export const projectsStore = createProjectsStore();

/** True when at least one project exists. Drives empty-state / full-UI fork. */
export const hasProjects = derived(
  projectsStore,
  ($projects) => $projects.length > 0
);
