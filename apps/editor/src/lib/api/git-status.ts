/**
 * git-status.ts
 *
 * Frontend client for the git status endpoint.
 * GET /api/projects/{id}/git/status
 */

export interface GitStatus {
  branch: string;
  dirty: boolean;
  ahead: number;
  behind: number;
}

const API_BASE =
  typeof window !== 'undefined' &&
  (window as unknown as { __VEDOX_API_BASE?: string }).__VEDOX_API_BASE
    ? (window as unknown as { __VEDOX_API_BASE?: string }).__VEDOX_API_BASE!
    : '';

/**
 * Fetch git status for the given project.
 * Throws on network errors or non-2xx responses.
 */
export async function fetchGitStatus(projectId: string): Promise<GitStatus> {
  const url = `${API_BASE}/api/projects/${encodeURIComponent(projectId)}/git/status`;
  const res = await fetch(url, {
    method: 'GET',
    headers: { Accept: 'application/json' }
  });

  if (!res.ok) {
    throw new Error(`git status failed: ${res.status}`);
  }

  const data = (await res.json()) as {
    branch?: string;
    dirty?: boolean;
    ahead?: number;
    behind?: number;
  };

  return {
    branch: data.branch ?? 'unknown',
    dirty: data.dirty ?? false,
    ahead: data.ahead ?? 0,
    behind: data.behind ?? 0
  };
}
