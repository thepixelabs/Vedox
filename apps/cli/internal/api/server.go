package api

import (
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/vedox/vedox/internal/agentauth"
	"github.com/vedox/vedox/internal/ai"
	"github.com/vedox/vedox/internal/analytics"
	"github.com/vedox/vedox/internal/db"
	"github.com/vedox/vedox/internal/docgraph"
	"github.com/vedox/vedox/internal/providers"
	"github.com/vedox/vedox/internal/scanner"
	"github.com/vedox/vedox/internal/store"
	"github.com/vedox/vedox/internal/voice"
)

// eventEmitter is the narrow slice of *analytics.Collector that the API
// handlers depend on. Defined as an interface so tests can supply a fake
// that records every Emit call without needing a real SQLite-backed
// Collector. Production code injects *analytics.Collector directly.
type eventEmitter interface {
	Emit(e analytics.Event) error
}

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

	// voiceServer is the optional voice pipeline HTTP facade. When non-nil,
	// Mount registers POST /api/voice/ptt and GET /api/voice/status on the chi
	// router so that corsMiddleware and loggingMiddleware apply. It is nil in
	// dev-server mode and when --voice is not passed to the daemon.
	voiceServer *voice.VoiceServer

	// bootstrapToken is the 64-hex-char daemon token used to authenticate
	// privileged GET endpoints (e.g. /api/browse). An empty string means
	// no token has been configured; requireBootstrapToken will return 401
	// for every request when this is unset (fail-closed). Production code
	// must call SetBootstrapToken before Mount().
	bootstrapToken string

	// graphStore is the doc-reference graph store. It is nil when the workspace
	// db has not been opened (dev-server mode without --graph) or when no
	// documents have been indexed yet. handleGraph returns 503 when nil.
	// Inject with SetGraphStore after NewServer.
	graphStore *docgraph.GraphStore

	// collector is the analytics write-side. When non-nil, handlers that
	// represent analytically-interesting user actions (document.published,
	// repo.registered, agent.installed, onboarding.completed) fire-and-forget
	// an Emit call after the action succeeds. It is nil in dev-server mode
	// and in unit tests that do not care about event emission — every Emit
	// call is nil-guarded. Inject with SetCollector after NewServer.
	collector eventEmitter

	// installerFactoryOverride, if non-nil, replaces buildInstaller. Tests-only
	// seam: production code never sets it. Allows agent_test.go to inject a
	// stub ProviderInstaller (with deterministic Probe/Plan/Install/Uninstall
	// outcomes) without spinning up a real provider that touches ~/.claude/
	// or runs `claude --version`. The returned ReceiptStore is used by the
	// install handler to persist the resulting receipt — tests can supply a
	// real ReceiptStore rooted at t.TempDir() to verify on-disk side effects.
	installerFactoryOverride func(provider string) (providers.ProviderInstaller, *providers.ReceiptStore, error)
}

// SetHomeDirOverride replaces the home directory used for user-global provider
// config paths. Tests only — production code must not call this.
func (s *Server) SetHomeDirOverride(home string) {
	s.homeDirOverride = home
}

// SetInstallerFactoryOverride installs a tests-only seam that replaces
// buildInstaller. Production code must not call this. It exists so agent
// handler tests can inject a deterministic stub ProviderInstaller in place
// of the real adapters (which touch ~/.claude/, run `claude --version`, etc).
func (s *Server) SetInstallerFactoryOverride(fn func(provider string) (providers.ProviderInstaller, *providers.ReceiptStore, error)) {
	s.installerFactoryOverride = fn
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

// SetVoiceServer injects the VoiceServer into the API server so that voice
// routes are registered inside the chi router (and therefore inherit CORS and
// logging middleware). Call this after NewServer and before Mount. The vs value
// may be nil — Mount performs a nil-guard and skips voice route registration
// when no voice pipeline was constructed.
func (s *Server) SetVoiceServer(vs *voice.VoiceServer) {
	s.voiceServer = vs
}

// SetBootstrapToken records the daemon bootstrap token that callers must
// present on privileged GET endpoints such as /api/browse. Call this after
// NewServer and before Mount. An empty token keeps the fail-closed default
// (every request returns 401); callers that genuinely need an open endpoint
// must explicitly document why before omitting this call.
func (s *Server) SetBootstrapToken(token string) {
	s.bootstrapToken = token
}

// SetGraphStore injects the doc-reference GraphStore into the server. Call
// this after NewServer when the daemon has successfully opened the workspace
// db and constructed a GraphStore. Handlers that depend on it check for nil
// and return 503 if absent, so not calling this is safe in dev-server mode.
func (s *Server) SetGraphStore(gs *docgraph.GraphStore) {
	s.graphStore = gs
}

// SetCollector injects the analytics Collector used by handlers to emit
// audit-able user actions (document published, repo registered, agent
// installed, onboarding completed). Call this after NewServer when the
// daemon has successfully constructed and Start()-ed a Collector. Passing
// nil disables event emission, which is the dev-server default — all
// emitEvent call sites are nil-guarded.
func (s *Server) SetCollector(c eventEmitter) {
	s.collector = c
}

// emitEvent is the single fan-out helper that every handler uses to fire an
// analytics event. It centralises three invariants:
//
//  1. nil-guard — a nil collector is always valid (dev-server, unit tests).
//  2. fire-and-forget — Emit errors are logged at debug level but never
//     surface to the caller. Analytics must never break the HTTP response.
//  3. timestamp — callers never have to set it; we stamp time.Now() here so
//     every event carries a consistent UTC wall-clock from the handler.
//
// Properties may be nil when the event has no attributes.
func (s *Server) emitEvent(kind string, props map[string]any) {
	if s.collector == nil {
		return
	}
	err := s.collector.Emit(analytics.Event{
		Kind:       kind,
		Timestamp:  time.Now(),
		Properties: props,
	})
	if err != nil {
		// Most likely Validate() rejected the event. A bad kind constant
		// would be a programmer error, not user-facing — log and move on.
		slog.Debug("api: analytics emit dropped", "kind", kind, "error", err.Error())
	}
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
		// FIX-SEC-10: fail-closed construction. Silently substituting
		// PassthroughAuth here produced an unauthenticated agent surface
		// whenever a caller forgot to wire auth — exactly the class of
		// mistake that made the daemon serve /docs without auth on a
		// keystore-load failure. Panic instead so the miswiring is caught
		// immediately at process start, not after a production incident.
		// Callers that genuinely want no auth (integration tests) must
		// explicitly pass agentauth.PassthroughAuth().
		panic("api.NewServer: requireAgent must not be nil; pass agentauth.RequireAgent(ks), agentauth.RejectAllAuth(), or (tests only) agentauth.PassthroughAuth()")
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
	// Requires the bootstrap token (CRIT-02 / FIX-SEC-01): unauthenticated
	// callers receive 401; paths outside $HOME receive 403 (boundary enforced
	// inside handleBrowse).
	r.With(s.requireBootstrapToken).Get("/api/browse", s.handleBrowse)

	// Project listing — returns results from the last completed scan (or runs
	// a synchronous scan on first call if no cached results exist).
	r.Get("/api/projects", s.handleListProjects)

	// Workspace scan — async job-based scanning with progress polling.
	// GET returns a synchronous summary of the last completed scan (running
	// one if none is cached) in the lightweight shape the editor onboarding
	// step expects; see handleGetScanSummary for rationale.
	r.Get("/api/scan", s.handleGetScanSummary)
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

	// Git status — branch, dirty flag, ahead/behind counters for the editor
	// status bar. Best-effort: never returns an error status (see handler doc).
	r.Get("/api/projects/{project}/git/status", s.handleGitStatus)

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
	//
	// FIX-SEC-07: both mutating endpoints scaffold directories and register
	// repos in GlobalDB — operations that write to disk under $HOME and touch
	// global daemon state. Any local process (or drive-by page that tricks the
	// browser into a same-origin POST) could previously create/register repos
	// without credentials. Require the bootstrap token so only callers with
	// file-read access to ~/.vedox/daemon-token (0600) can drive onboarding.
	r.With(s.requireBootstrapToken).Post("/api/repos/create", s.handleCreateRepoWithInit)
	r.With(s.requireBootstrapToken).Post("/api/repos/register", s.handleRegisterRepo)

	// Doc Agent management — install/uninstall/list across all supported providers.
	// Requires a KeyStore (SetKeyStore); returns 503 in dev-server mode.
	r.Get("/api/agent/list", s.handleAgentList)
	r.Post("/api/agent/install", s.handleAgentInstall)
	r.Post("/api/agent/uninstall", s.handleAgentUninstall)

	// User preferences — persisted to ~/.vedox/user-prefs.json.
	// PUT uses PATCH semantics (R3): only supplied top-level keys are overwritten;
	// all other keys in the stored file are preserved.
	r.Get("/api/settings", s.handleGetSettings)
	r.Put("/api/settings", s.handlePutSettings)

	// Doc reference graph — returns Cytoscape-compatible {nodes, edges} for the
	// given project. Read-only; no auth required at alpha (consistent with all
	// other project GET endpoints; bootstrap token scope is a GA gate).
	r.Get("/api/graph", s.handleGraph)

	// Inline code preview — resolves a vedox:// URL and returns source content
	// for Shiki rendering. Read-only; no agent auth required.
	r.Get("/api/preview", s.handlePreview)

	// Analytics summary — cross-workspace event aggregates from GlobalDB.
	r.Get("/api/analytics/summary", s.handleAnalyticsSummary)

	// Onboarding completion — a narrow write-only endpoint the SvelteKit
	// AllDone step posts to so the analytics Collector can emit
	// onboarding.completed. Returns 204 No Content; the body is optional.
	r.Post("/api/onboarding/complete", s.handleOnboardingComplete)

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

	// Voice pipeline routes — only wired when a VoiceServer has been injected
	// via SetVoiceServer. Both endpoints inherit corsMiddleware and
	// loggingMiddleware from the chi router's middleware stack (HIGH-03 fix).
	if s.voiceServer != nil {
		r.Post("/api/voice/ptt", s.voiceServer.HandlePTT)
		r.Get("/api/voice/status", s.voiceServer.HandleStatus)
	}

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
