/**
 * +page.ts — load function for the document editor route.
 *
 * Fetches the document from the Go backend before the page mounts so the
 * editor component always receives its initial content synchronously. This
 * avoids a flash of the loading skeleton if the doc loads quickly, while
 * still producing a proper error state for missing or inaccessible documents.
 *
 * The [...path] catch-all gives us the workspace-relative path segment after
 * the project name, e.g. for:
 *   /projects/myapp/docs/architecture/adr-001.md
 * params.project = "myapp"
 * params.path    = "architecture/adr-001.md"
 */

import type { PageLoad } from './$types';
import { api, ApiError, type Doc } from '$lib/api/client';

export const ssr = false;
export const prerender = false;

export interface DocPageData {
  project: string;
  path: string;
  doc: Doc | null;
  error: string | null;
}

export const load: PageLoad = async ({ params }): Promise<DocPageData> => {
  const project = params.project;
  const path = params.path ?? '';

  if (!project || !path) {
    return { project, path, doc: null, error: 'Invalid document path.' };
  }

  try {
    const doc = await api.getDoc(project, path);
    return { project, path, doc, error: null };
  } catch (err) {
    let error = 'Could not load document.';
    if (err instanceof ApiError) {
      if (err.status === 404) {
        error = 'Document not found.';
      } else {
        error = `[${err.code}] ${err.message}`;
      }
    } else if (err instanceof Error) {
      error = err.message;
    }
    return { project, path, doc: null, error };
  }
};
