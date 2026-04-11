/**
 * api/client.ts — typed HTTP client for the Vedox Go backend.
 *
 * All methods throw ApiError on non-2xx responses so callers can pattern-match
 * on error.code (e.g. "VDX-003") and present meaningful messages.
 *
 * Base URL is relative so the Vite dev proxy (/api → 127.0.0.1:5150) handles
 * routing transparently. In production the SvelteKit static bundle is served by
 * the same Go binary, so relative URLs resolve correctly there too.
 */

// ---------------------------------------------------------------------------
// Error type
// ---------------------------------------------------------------------------

export class ApiError extends Error {
  /** Optional structured body from the server (e.g. conflict response). */
  conflictBody?: Record<string, unknown>;

  constructor(
    public readonly code: string,
    message: string,
    public readonly status: number,
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

// ---------------------------------------------------------------------------
// Response shapes — mirror the Go JSON structs in internal/api/
// ---------------------------------------------------------------------------

export interface Doc {
  /** Workspace-relative path, e.g. "myproject/architecture/adr-001.md" */
  path: string;
  content: string;
  /**
   * Parsed YAML frontmatter plus synthetic fields injected by the backend.
   *
   * Symlinked (read-only) docs carry three extra fields injected by SymlinkAdapter:
   *   metadata._source      = "symlink"
   *   metadata._editable    = false
   *   metadata._source_path = "/absolute/path/to/original/file"
   *
   * Editable (LocalAdapter) docs do not set these fields. Treat their absence
   * as implying _editable = true.
   */
  metadata: Record<string, unknown>;
  /** RFC3339 timestamp */
  modTime: string;
  size: number;
}

export interface LinkResult {
  /** Logical project name as it appears inside Vedox. */
  projectName: string;
  /** Number of Markdown files found in the linked directory. */
  docCount: number;
  /** Detected documentation framework (e.g. "MkDocs", "Docusaurus", "README"). */
  framework: string;
}

export interface Project {
  /** Directory name — used as the URL slug */
  name: string;
  /** Absolute path on the server filesystem */
  path: string;
  docCount: number;
}

export interface Task {
  id: string;
  project: string;
  title: string;
  status: 'todo' | 'in-progress' | 'done';
  /** Fractional index position used for drag-to-reorder ordering. */
  position: number;
  createdAt: string;
  updatedAt: string;
}

export interface SearchResult {
  /** FTS row id — also the workspace-relative doc path */
  id: string;
  project: string;
  title: string;
  type: string;
  status: string;
  snippet: string;
  score: number;
}

// ---------------------------------------------------------------------------
// Import & Migrate types
// ---------------------------------------------------------------------------

/**
 * Result returned by POST /api/import.
 * Mirrors importer.ImportResult in apps/cli/internal/importer/importer.go.
 */
export interface ImportResult {
  /** Workspace-relative destination paths of files successfully imported. */
  imported: string[];
  /** Source-relative paths that were skipped, with a reason in parentheses. */
  skipped: string[];
  /**
   * Advisory messages — always includes the Git removal reminder when at
   * least one file was imported. May also include indexing warnings.
   */
  warnings: string[];
}

export type ScanStatus = 'pending' | 'running' | 'done' | 'error';

export interface ScanJob {
  id: string;
  status: ScanStatus;
  /** Total paths examined so far */
  total: number;
  /** Number of paths fully scanned */
  scanned: number;
  /** Projects discovered so far */
  projects: Project[];
  /** Present when status === 'error' */
  error?: string;
}

export interface BrowseResponse {
  path: string;
  parent: string;
  directories: Array<{ name: string; path: string }>;
}

export interface CreateProjectResult {
  name: string;
  path: string;
  docCount: number;
}

export interface ProviderInfo {
  id: string;
  name: string;
  available: boolean;
  version?: string;
}

export interface AltergoAccount {
  name: string;
  providers: string[];
}

export interface AIProvidersResponse {
  providers: ProviderInfo[];
  altergo: {
    available: boolean;
    accounts: AltergoAccount[];
  };
}

export interface GenerationParams {
  categories: string[];
  platform: string;
  os: string;
  interface: string;
  audience: string;
  tone: string;
  nameLength: string;
  languageStyle: string;
}

export interface RefinementInput {
  mode: 'exact' | 'style';
  likedNames: string[];
}

export interface GenerateNamesRequest {
  provider: string;
  account: string;
  params: GenerationParams;
  count: number;
  refinement?: RefinementInput | null;
}

export type GenerationStatus = 'pending' | 'running' | 'done' | 'error';

export interface GenerationJob {
  id: string;
  status: GenerationStatus;
  names?: string[];
  error?: string;
  providerUsed?: string;
  accountUsed?: string;
  durationMs?: number;
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

/** Parse the Go error envelope { code, message } or fall back to status text. */
async function parseApiError(res: Response): Promise<ApiError> {
  let code = 'VDX-000';
  let message = res.statusText || `HTTP ${res.status}`;
  let rawBody: Record<string, unknown> | undefined;
  try {
    const body = await res.json() as Record<string, unknown>;
    rawBody = body;
    if (typeof body?.code === 'string') code = body.code;
    // 409 conflict uses { error: "conflict", currentEtag, ... } — map it.
    if (typeof body?.error === 'string' && body.error === 'conflict') code = 'conflict';
    if (typeof body?.message === 'string') message = body.message;
  } catch {
    // Response body was not JSON — keep defaults above.
  }
  const err = new ApiError(code, message, res.status);
  if (rawBody !== undefined) err.conflictBody = rawBody;
  return err;
}

async function get<T>(path: string): Promise<T> {
  const res = await fetch(path, {
    headers: { Accept: 'application/json' },
  });
  if (!res.ok) throw await parseApiError(res);
  return res.json() as Promise<T>;
}

async function post<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(path, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Accept: 'application/json',
    },
    body: JSON.stringify(body),
  });
  if (!res.ok) throw await parseApiError(res);
  // 204 No Content has no body — return undefined cast to T.
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

async function del(path: string): Promise<void> {
  const res = await fetch(path, {
    method: 'DELETE',
    headers: { Accept: 'application/json' },
  });
  if (!res.ok) throw await parseApiError(res);
}

async function patch<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(path, {
    method: 'PATCH',
    headers: {
      'Content-Type': 'application/json',
      Accept: 'application/json',
    },
    body: JSON.stringify(body),
  });
  if (!res.ok) throw await parseApiError(res);
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

/** Encode a slash-separated doc path for use inside a URL segment. */
function encodePath(docPath: string): string {
  // Encode each segment individually so forward slashes are preserved as
  // URL path separators (the Go router uses /*  wildcards, not query params).
  return docPath.split('/').map(encodeURIComponent).join('/');
}

// ---------------------------------------------------------------------------
// Public API client
// ---------------------------------------------------------------------------

export const api = {
  /**
   * GET /api/browse?path=
   * Lists subdirectories for the given absolute path.
   */
  browse(path?: string): Promise<BrowseResponse> {
    const params = new URLSearchParams();
    if (path) params.set('path', path);
    return get<BrowseResponse>(`/api/browse?${params.toString()}`);
  },

  /**
   * GET /api/projects
   * Returns the list of projects in the workspace.
   */
  getProjects(): Promise<Project[]> {
    return get<Project[]>('/api/projects');
  },

  /**
   * GET /api/projects/:project/docs
   * Returns all documents in the given project.
   */
  getProjectDocs(project: string): Promise<Doc[]> {
    return get<Doc[]>(`/api/projects/${encodeURIComponent(project)}/docs`);
  },

  /**
   * GET /api/projects/:project/docs/:path
   * Returns a single document, preferring the unsaved draft if newer.
   */
  getDoc(project: string, path: string): Promise<Doc> {
    return get<Doc>(`/api/projects/${encodeURIComponent(project)}/docs/${encodePath(path)}`);
  },

  /**
   * POST /api/projects/:project/docs/:path
   * Auto-saves content to the draft location. Does not commit to Git.
   */
  saveDoc(project: string, path: string, content: string): Promise<Doc> {
    return post<Doc>(
      `/api/projects/${encodeURIComponent(project)}/docs/${encodePath(path)}`,
      { content },
    );
  },

  /**
   * DELETE /api/projects/:project/docs/:path
   * Deletes both the committed file and any draft.
   */
  deleteDoc(project: string, path: string): Promise<void> {
    return del(`/api/projects/${encodeURIComponent(project)}/docs/${encodePath(path)}`);
  },

  /**
   * POST /api/projects/:project/docs/:path/publish
   * Promotes the draft to Git with the given commit message.
   * message defaults to "docs: update <filename>" if omitted.
   */
  publishDoc(project: string, path: string, message?: string): Promise<void> {
    const commitMessage = message?.trim() || `docs: update ${path.split('/').pop()}`;
    return post<void>(
      `/api/projects/${encodeURIComponent(project)}/docs/${encodePath(path)}/publish`,
      { message: commitMessage },
    );
  },

  /**
   * GET /api/projects/:project/search?q=:query
   * Full-text search scoped to the project. Empty query returns [].
   */
  search(project: string, query: string): Promise<SearchResult[]> {
    if (!query.trim()) return Promise.resolve([]);
    const params = new URLSearchParams({ q: query });
    return get<SearchResult[]>(`/api/projects/${encodeURIComponent(project)}/search?${params}`);
  },

  /**
   * POST /api/scan
   * Kicks off a background workspace scan. Returns a job id for polling.
   */
  startScan(workspaceRoot?: string): Promise<{ jobId: string }> {
    return post<{ jobId: string }>('/api/scan', workspaceRoot ? { workspaceRoot } : {});
  },

  /**
   * GET /api/scan/:jobId
   * Polls a running scan job for status and progress.
   */
  getScanJob(jobId: string): Promise<ScanJob> {
    return get<ScanJob>(`/api/scan/${encodeURIComponent(jobId)}`);
  },

  /**
   * POST /api/import
   * Copies all .md files from srcProjectRoot into the Vedox workspace under
   * a sub-directory named projectName, then indexes them in SQLite.
   */
  importProject(srcProjectRoot: string, projectName: string): Promise<ImportResult> {
    return post<ImportResult>('/api/import', { srcProjectRoot, projectName });
  },

  /**
   * POST /api/link
   * Links an external project directory into Vedox as a read-only source.
   */
  linkProject(externalRoot: string, projectName: string): Promise<LinkResult> {
    return post<LinkResult>('/api/link', { externalRoot, projectName });
  },

  // ---------------------------------------------------------------------------
  // Task backlog (VDX-P2-H)
  // ---------------------------------------------------------------------------

  /**
   * GET /api/projects/:project/tasks
   * Returns all tasks for the project ordered by position ascending.
   */
  getTasks(project: string): Promise<Task[]> {
    return get<Task[]>(`/api/projects/${encodeURIComponent(project)}/tasks`);
  },

  /**
   * POST /api/projects/:project/tasks
   * Creates a new task. Status defaults to "todo" if omitted.
   */
  createTask(project: string, title: string, status?: Task['status']): Promise<Task> {
    return post<Task>(
      `/api/projects/${encodeURIComponent(project)}/tasks`,
      { title, ...(status ? { status } : {}) },
    );
  },

  /**
   * PATCH /api/projects/:project/tasks/:id
   * Partially updates a task. Only the fields present in patch are written.
   *
   * When a position update causes fractional index exhaustion the server
   * returns { renumbered: true, tasks: Task[] }. Callers must handle this
   * envelope by replacing their local task list wholesale.
   */
  async updateTask(
    project: string,
    id: string,
    patch: Partial<Pick<Task, 'title' | 'status' | 'position'>>,
  ): Promise<Task | { renumbered: true; tasks: Task[] }> {
    const res = await fetch(
      `/api/projects/${encodeURIComponent(project)}/tasks/${encodeURIComponent(id)}`,
      {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
        body: JSON.stringify(patch),
      },
    );
    if (!res.ok) throw await parseApiError(res);
    return res.json();
  },

  /**
   * DELETE /api/projects/:project/tasks/:id
   * Deletes a task. Returns void on success.
   */
  deleteTask(project: string, id: string): Promise<void> {
    return del(`/api/projects/${encodeURIComponent(project)}/tasks/${encodeURIComponent(id)}`);
  },

  /**
   * POST /api/projects
   * Creates a new blank project in the workspace.
   */
  createProject(name: string, tagline?: string, description?: string): Promise<CreateProjectResult> {
    return post<CreateProjectResult>('/api/projects', {
      name,
      ...(tagline ? { tagline } : {}),
      ...(description ? { description } : {}),
    });
  },

  /**
   * GET /api/ai/providers
   * Returns available AI CLI providers and AlterGo accounts.
   */
  getAIProviders(): Promise<AIProvidersResponse> {
    return get<AIProvidersResponse>('/api/ai/providers');
  },

  /**
   * POST /api/ai/generate-names
   * Submits a name generation job. Returns immediately with jobId for polling.
   */
  generateNames(req: GenerateNamesRequest): Promise<{ jobId: string }> {
    return post<{ jobId: string }>('/api/ai/generate-names', req);
  },

  /**
   * GET /api/ai/generate-names/:jobId
   * Polls a name generation job for status and results.
   */
  getGenerationJob(jobId: string): Promise<GenerationJob> {
    return get<GenerationJob>(`/api/ai/generate-names/${encodeURIComponent(jobId)}`);
  },
};
