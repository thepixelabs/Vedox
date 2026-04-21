/**
 * /projects/[project]/graph load function.
 *
 * Fetches the per-project doc reference graph server-side (or client-side on
 * navigation) via the api client. ApiError surfaces as a SvelteKit `error`
 * so the nearest +error.svelte boundary can render it consistently with
 * every other route.
 *
 * Runs as a plain PageLoad (not PageServerLoad) because /api/* is reachable
 * from the browser via the Vite proxy in dev and the Go binary in prod —
 * there is no SSR-only concern here.
 */

import { error } from '@sveltejs/kit';
import { api, ApiError } from '$lib/api/client';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ params, fetch: _fetch }) => {
  const project = params.project;
  try {
    const graph = await api.getGraph(project);
    return { project, graph };
  } catch (err) {
    if (err instanceof ApiError) {
      throw error(err.status, `${err.code} — ${err.message}`);
    }
    throw error(500, 'failed to load project graph');
  }
};
