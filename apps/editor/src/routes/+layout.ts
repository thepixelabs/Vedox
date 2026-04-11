/**
 * +layout.ts — root layout load function.
 *
 * Fetches the project list from the Go backend on every navigation that hits
 * the root layout. The list is used by Sidebar, the projects page, and the
 * hasProjects gate in +layout.svelte.
 *
 * Error handling strategy:
 *   - Network / server errors: return empty projects + an error string so
 *     +layout.svelte can show an inline banner rather than crashing the shell.
 *   - Empty workspace: return [] gracefully — layout shows the empty state CTA.
 */

import type { LayoutLoad } from './$types';
import { api, ApiError } from '$lib/api/client';
import type { Project as ApiProject } from '$lib/api/client';
import type { Project } from '$lib/stores/projects';

export const ssr = false; // SPA mode for the editor shell (local-only app)
export const prerender = false;

/**
 * Map the API project shape to the store shape.
 *
 * The store uses `id` (URL slug) and carries a `docs` array for tree rendering.
 * On layout load we only know the project list, not each project's doc list,
 * so `docs` starts empty. Individual pages (project home, doc editor) load
 * doc lists on demand via api.getProjectDocs().
 */
function toStoreProject(p: ApiProject): Project {
  return {
    id: p.name,       // name is the directory slug used in URLs
    name: p.name,
    docs: [],         // populated lazily by child routes
    docCount: p.docCount,
  };
}

export const load: LayoutLoad = async () => {
  try {
    const apiProjects: ApiProject[] = await api.getProjects();
    const projects: Project[] = (apiProjects ?? []).map(toStoreProject);
    return { projects, error: null };
  } catch (err) {
    // Surface a human-readable message; preserve the code for diagnostics.
    let errorMessage = 'Could not connect to the Vedox backend.';
    if (err instanceof ApiError) {
      errorMessage = `[${err.code}] ${err.message} (HTTP ${err.status})`;
    } else if (err instanceof Error) {
      errorMessage = err.message;
    }
    // Return an empty project list so the shell still mounts.
    return { projects: [] as Project[], error: errorMessage };
  }
};
