package api

import (
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/vedox/vedox/internal/agentauth"
	"github.com/vedox/vedox/internal/ai"
	"github.com/vedox/vedox/internal/db"
	"github.com/vedox/vedox/internal/providers"
	"github.com/vedox/vedox/internal/scanner"
	"github.com/vedox/vedox/internal/store"
)

// Server holds the shared dependencies for all API handlers. It is constructed
// once in cmd/dev.go and its routes are mounted onto the main http.ServeMux.
//
// Thread safety: Server itself carries no mutable state after construction.
// DocStore and db.Store are safe for concurrent use per their own contracts.
// JobStore and ProjectRegistry are internally mutex-protected.
type Server struct {
	store         store.DocStore
	db            *db.Store
	workspaceRoot string
	jobStore      *scanner.JobStore
	// aiJobStore holds in-flight and recently completed AI name generation jobs.
	aiJobStore *ai.JobStore
	// registry holds all registered DocStore instances keyed by project name.
	// LocalAdapter projects are registered on startup; SymlinkAdapter projects
	// are registered dynamically via POST /api/link and persisted to
	// .vedox/links.json so they survive restarts.
	registry *store.ProjectRegistry

	// requireAgent is the auth middleware applied to agent-only routes
	// (POST /docs, POST /decisions — landing in VDX-P3-INGEST). It is set by
	// NewServer from the KeyStore loaded at dev-server startup. Until
	// ingestion routes exist, this field is unused on the hot path, but the
	// server accepts it now so VDX-P3-INGEST can wire routes without having
	// to retouch the server constructor.
	requireAgent agentauth.Middleware

	// globalDB is the cross-workspace database (~/.vedox/global.db) that holds
	// the repo registry, agent install state, and daily event roll-ups. It is
	// nil in dev-server mode (where only the workspace DB is open). Handlers
	// that depend on it must check for nil and return 503 if absent.
	globalDB *db.GlobalDB

	// keyStore is the HMAC key store used by the Doc Agent installer.
	// It is nil in dev-server mode (where the daemon has not loaded keys).
	// Handlers that depend on it must check for nil and return 503 if absent.
	keyStore providers.KeyIssuer

	// homeDirOverride, if non-empty, replaces os.UserHomeDir() when resolving
	// user-global provider config paths (e.g. ~/.codex/config.toml). Production
	// code leaves this empty; tests set it to a t.TempDir() for isolation.
	homeDirOverride string
}

// SetHomeDirOverride replaces the home directory used for user-global provider
// config paths. Tests only — production code must not call this.
func (s *Server) SetHomeDirOverride(home string) {
	s.homeDirOverride = home
}

// SetGlobalDB injects the GlobalDB handle into the server. Call this after
// NewServer when the daemon has successfully opened ~/.vedox/global.db.
// The server does not own the GlobalDB lifecycle — the caller is responsible
// for closing it on shutdown.
func (s *Server) SetGlobalDB(g *db.GlobalDB) {
	s.globalDB = g
}

// SetKeyStore injects the HMAC KeyStore into the server. Call this after
// NewServer when the daemon has successfully loaded the agent key store.
// The ks value must implement providers.KeyIssuer — in production this is
// always *agentauth.KeyStore. Tests that do not exercise agent install can
// leave this nil, which causes the agent handlers to return 503.
func (s *Server) SetKeyStore(ks providers.KeyIssuer) {
	s.keyStore = ks
}

// userHome returns homeDirOverride when set, otherwise os.UserHomeDir().
func (s *Server) userHome() (string, error) {
	if s.homeDirOverride != "" {
		return s.homeDirOverride, nil
	}
	return os.UserHomeDir()
}

// NewServer constructs an API Server. workspaceRoot must be an absolute path;
// it is the boundary used for path traversal validation on every request.
// The DocStore and db.Store are used as-is — callers retain ownership of Close.
// jobStore must be non-nil; use scanner.NewJobStore() if no existing store exists.
// aiJobStore must be non-nil; use ai.NewJobStore(3) if no existing store exists.
// registry must be non-nil; use store.NewProjectRegistry() if no registry exists.
func NewServer(docStore store.DocStore, dbStore *db.Store, workspaceRoot string, jobStore *scanner.JobStore, aiJobStore *ai.JobStore, registry *store.ProjectRegistry, requireAgent agentauth.Middleware) *Server {
	if requireAgent == nil {
		// Fail-closed default: tests and callers that do not supply auth get
		// a passthrough so wiring does not explode, but this is explicit.
		requireAgent = agentauth.PassthroughAuth()
	}
	return &Server{
		store:         docStore,
		db:            dbStore,
		workspaceRoot: workspaceRoot,
		jobStore:      jobStore,
		aiJobStore:    aiJobStore,
		registry:      registry,
		requireAgent:  requireAgent,
	}
}

// Mount registers all /api/* routes on mux. The chi router handles sub-routing
// and wildcard path parameters; mux is the top-level http.ServeMux from dev.go.
//
// Middleware stack applied to every /api/* request:
//  1. corsMiddleware    — CORS headers + security headers
//  2. loggingMiddleware — structured request logging (bodies never logged)
//
// Route registration order matters for the docs subrouter: the /publish POST
// must be registered before the generic /* POST so chi matches the more-specific
// pattern first when both patterns could match a given path.
func (s *Server) Mount(mux *http.ServeMux) {
	r := chi.NewRouter()
	r.Use(corsMiddleware)
	r.Use(loggingMiddleware)

	// Health — used by the SvelteKit frontend to confirm the Go backend is alive.
	r.Get("/api/health", s.handleHealth)

	// Filesystem browsing — used by the frontend folder picker.
	r.Get("/api/browse", s.handleBrowse)

	// Project listing — returns results from the last completed scan (or runs
	// a synchronous scan on first call if no cached results exist).
	r.Get("/api/projects", s.handleListProjects)

	// Workspace scan — async job-based scanning with progress polling.
	r.Post("/api/scan", s.handleStartScan)
	r.Get("/api/scan/{jobId}", s.handleGetScanJob)

	// Create project — scaffold a new empty project inside the workspace root.
	r.Post("/api/projects", s.handleCreateProject)

	// Import & Migrate — copy an external project's Markdown docs into the workspace.
	r.Post("/api/import", s.handleImport)

	// Link (read-only) — register an external project as a SymlinkAdapter.
	// The project docs remain in their original location and are served
	// read-only. Use Import & Migrate to gain editing access.
	r.Post("/api/link", s.handleLinkProject)

	// AI name generation — provider discovery and async generation jobs.
	r.Route("/api/ai", func(r chi.Router) {
		r.Get("/providers", s.handleAIProviders)
		r.Post("/generate-names", s.handleGenerateNames)
		r.Get("/generate-names/{jobId}", s.handleGenerateNamesStatus)
	})

	// Full-text search within a project.
	r.Get("/api/projects/{project}/search", s.handleSearch)

	// Task backlog — per-project flat task list (VDX-P2-H).
	r.Get("/api/projects/{project}/tasks", s.handleListTasks)
	r.Post("/api/projects/{project}/tasks", s.handleCreateTask)
	r.Patch("/api/projects/{project}/tasks/{id}", s.handleUpdateTask)
	r.Delete("/api/projects/{project}/tasks/{id}", s.handleDeleteTask)

	// Document routes — all scoped under /api/projects/{project}/docs.
	// We use a single subrouter so that:
	//   a) The listing GET "/" and doc-level routes share the same {project} param.
	//   b) The /publish POST is registered before the generic /* POST inside the
	//      subrouter, ensuring chi matches the more-specific pattern first.
	r.Route("/api/projects/{project}/docs", func(dr chi.Router) {
		// List all docs directly inside the project root.
		dr.Get("/", s.handleListDocs)

		// GET a single document (includes draft precedence logic).
		// Routes ending in /metadata are dispatched to handleDocMetadata
		// for git-derived file metadata (similar to the POST /publish dispatch).
		dr.Get("/*", func(w http.ResponseWriter, r *http.Request) {
			p := chi.URLParam(r, "*")
			if p == "metadata" || strings.HasSuffix(p, "/metadata") {
				s.handleDocMetadata(w, r)
				return
			}
			if p == "history" || strings.HasSuffix(p, "/history") {
				s.handleDocHistory(w, r)
				return
			}
			s.handleGetDoc(w, r)
		})

		// Chi does not allow a wildcard mid-pattern (e.g. /*/publish), so a
		// single POST /* handler dispatches to handlePublish when the path ends
		// in "/publish", and to handleWriteDoc otherwise.
		dr.Post("/*", func(w http.ResponseWriter, r *http.Request) {
			p := chi.URLParam(r, "*")
			if p == "publish" || strings.HasSuffix(p, "/publish") {
				s.handlePublish(w, r)
				return
			}
			s.handleWriteDoc(w, r)
		})

		// Delete both the committed file and any draft.
		dr.Delete("/*", s.handleDeleteDoc)
	})

	// Global repo registry — backed by GlobalDB (~/.vedox/global.db).
	// Returns 503 when the daemon GlobalDB is not available (dev-server mode).
	r.Get("/api/repos", s.handleListRepos)
	r.Post("/api/repos", s.handleCreateRepo)
	// Onboarding-specific repo endpoints. These must be registered before the
	// generic POST /api/repos so chi matches the more-specific routes first.
	r.Post("/api/repos/create", s.handleCreateRepoWithInit)
	r.Post("/api/repos/register", s.handleRegisterRepo)

	// Doc Agent management — install/uninstall/list across all supported providers.
	// Requires a KeyStore (SetKeyStore); returns 503 in dev-server mode.
	r.Get("/api/agent/list", s.handleAgentList)
	r.Post("/api/agent/install", s.handleAgentInstall)
	r.Post("/api/agent/uninstall", s.handleAgentUninstall)

	// Inline code preview — resolves a vedox:// URL and returns source content
	// for Shiki rendering. Read-only; no agent auth required.
	r.Get("/api/preview", s.handlePreview)

	// Analytics summary — cross-workspace event aggregates from GlobalDB.
	r.Get("/api/analytics/summary", s.handleAnalyticsSummary)

	// AI provider config — manage Claude Code config, Codex global config.
	r.Route("/api/projects/{project}/providers", func(pr chi.Router) {
		pr.Get("/claude", s.handleGetClaudeConfig)
		pr.Put("/claude/memory", s.handlePutClaudeMemory)
		pr.Put("/claude/permissions", s.handlePutClaudePermissions)
		pr.Get("/claude/mcp", s.handleGetClaudeMCP)
		pr.Put("/claude/mcp", s.handlePutClaudeMCP)
		pr.Get("/claude/agents", s.handleListAgents)
		pr.Post("/claude/agents", s.handleCreateAgent)
		pr.Get("/claude/agents/{filename}", s.handleGetAgent)
		pr.Put("/claude/agents/{filename}", s.handlePutAgent)
		pr.Delete("/claude/agents/{filename}", s.handleDeleteAgent)
		pr.Get("/codex", s.handleGetCodexConfig)
		pr.Put("/codex/mcp", s.handlePutCodexMCP)
		pr.Put("/codex/settings", s.handlePutCodexSettings)
	})

	// Mount the chi router under /api/ on the stdlib mux. Everything that
	// hits /api/* will be dispatched by chi.
	mux.Handle("/api/", r)
}

// handleHealth responds with a simple JSON ok payload. This endpoint is
// intentionally minimal — it should never fail if the binary is running.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// MountHealthz registers the /healthz route on mux using the provided handler.
// This is called by cmd/server.go so the daemon's richer /healthz (with uptime,
// version, pid, etc.) replaces the dev-server placeholder. The handler is
// unauthenticated per spec §5.1 — HMAC middleware does not apply.
func MountHealthz(mux *http.ServeMux, handler http.HandlerFunc) {
	mux.HandleFunc("/healthz", handler)
}

// storeForProject returns the DocStore responsible for the given project name.
// It checks the ProjectRegistry first (SymlinkAdapter projects registered via
// POST /api/link), then falls back to the default LocalAdapter (s.store) for
// projects that live inside the Vedox workspace.
//
// This is the single routing point for all per-project doc operations. Adding
// a new store type only requires registering it in the registry at startup or
// via a POST /api/link call — no handler code needs to change.
func (s *Server) storeForProject(project string) store.DocStore {
	if s.registry != nil {
		if st, ok := s.registry.Get(project); ok {
			return st
		}
	}
	return s.store
}
