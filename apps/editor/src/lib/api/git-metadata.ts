/**
 * git-metadata.ts — fetch client for the doc metadata endpoint.
 *
 * Returns git-derived metadata (last modified, contributors, branch, commit)
 * for a single document file. Used by MetadataSidecar to populate the
 * right-rail panel.
 */

export interface GitFileMetadata {
  lastModified: string; // ISO timestamp
  contributors: Array<{ name: string; email: string }>;
  branch: string;
  commitHash: string;
}

export async function fetchGitMetadata(
  projectId: string,
  docPath: string,
): Promise<GitFileMetadata | null> {
  try {
    const res = await fetch(
      `/api/projects/${encodeURIComponent(projectId)}/docs/${encodeURIComponent(docPath)}/metadata`,
    );
    if (!res.ok) return null;
    return await res.json();
  } catch {
    return null;
  }
}
