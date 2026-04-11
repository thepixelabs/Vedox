package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/vedox/vedox/internal/ai"
	"github.com/vedox/vedox/internal/api"
	"github.com/vedox/vedox/internal/config"
	"github.com/vedox/vedox/internal/db"
	"github.com/vedox/vedox/internal/gitcheck"
	"github.com/vedox/vedox/internal/indexer"
	"github.com/vedox/vedox/internal/links"
	"github.com/vedox/vedox/internal/portcheck"
	"github.com/vedox/vedox/internal/scanner"
	"github.com/vedox/vedox/internal/store"
)

// devCmd starts the Vedox development server.
//
// Startup sequence:
//  1. Load vedox.config.toml (VDX-002 if missing)
//  2. Check git identity (VDX-003 if unset)
//  3. Bind-test 127.0.0.1:<port> (VDX-001 if in use)
//  4. Open DocStore (LocalAdapter) and SQLite index
//  5. Mount API server; build HTTP mux
//  6. Start HTTP server; block until SIGINT/SIGTERM, then gracefully shut down
var devCmd = &cobra.Command{
	Use:   "dev",
	Short: "Start the Vedox development server on localhost",
	Long: `Start the Vedox development server.

The server binds to 127.0.0.1 only (loopback). It watches your workspace
for Markdown changes and serves the WYSIWYG editor UI.

NETWORK POLICY: No outbound network calls are made. The server only
listens on the configured localhost port.`,
	RunE: runDev,
}

func runDev(cmd *cobra.Command, args []string) error {
	// 1. Load config. Use the global --config flag value.
	cfg, err := config.LoadConfig(globalFlags.configPath)
	if err != nil {
		return err
	}

	slog.Info("config loaded",
		"port", cfg.Port,
		"workspace", cfg.Workspace,
		"profile", string(cfg.Profile),
	)

	// 2. Verify Git identity. Vedox commits on Publish; fail fast if unset
	//    rather than silently producing commits with no author.
	identity, err := gitcheck.Check()
	if err != nil {
		return err
	}
	slog.Info("git identity verified", "name", identity.Name, "email", identity.Email)

	// 3. Bind-test the port before starting the server. This gives a clean
	//    VDX-001 error instead of a raw kernel "address already in use" panic.
	if err := portcheck.CheckPort(cfg.Port); err != nil {
		return err
	}

	listenAddr := portcheck.ListenAddr(cfg.Port)

	// 4. Open DocStore and SQLite index.
	adapter, err := store.NewLocalAdapter(cfg.Workspace, nil)
	if err != nil {
		return fmt.Errorf("could not initialise DocStore: %w", err)
	}

	dbStore, err := db.Open(db.Options{WorkspaceRoot: cfg.Workspace})
	if err != nil {
		return fmt.Errorf("could not open SQLite index: %w", err)
	}
	defer func() {
		if closeErr := dbStore.Close(); closeErr != nil {
			slog.Error("failed to close db", "error", closeErr.Error())
		}
	}()

	// 4b. Start the background file indexer. It watches the workspace for .md
	// changes and keeps the SQLite FTS5 index in sync without blocking HTTP.
	// We derive a context that is cancelled when the process receives SIGINT/SIGTERM
	// so the indexer shuts down cleanly alongside the HTTP server.
	idxCtx, idxCancel := context.WithCancel(context.Background())
	defer idxCancel()

	ix := indexer.New(adapter, dbStore, cfg.Workspace)
	go func() {
		if err := ix.Start(idxCtx); err != nil {
			slog.Error("indexer error", "error", err.Error())
		}
	}()

	// 5. Build the project registry and restore any previously-linked projects.
	// Linked projects are persisted to .vedox/links.json and re-registered on
	// every startup so they survive a `vedox dev` restart. Failures to restore
	// an individual link are logged and skipped — a missing or moved external
	// directory should not prevent the server from starting.
	registry := store.NewProjectRegistry()
	linkedProjects, err := links.Load(cfg.Workspace)
	if err != nil {
		slog.Warn("could not load linked projects; starting without them",
			"error", err.Error(),
		)
	} else {
		for _, lp := range linkedProjects {
			symAdapter, linkErr := store.NewSymlinkAdapter(lp.ExternalRoot, lp.ProjectName, cfg.Workspace)
			if linkErr != nil {
				slog.Warn("could not restore linked project; skipping",
					"projectName", lp.ProjectName,
					"externalRoot", lp.ExternalRoot,
					"error", linkErr.Error(),
				)
				continue
			}
			registry.Register(lp.ProjectName, symAdapter)
			slog.Info("linked project restored",
				"projectName", lp.ProjectName,
				"externalRoot", lp.ExternalRoot,
			)
		}
	}

	// 6. Build the HTTP handler.
	// SvelteKit Vite dev server runs on port 5151 and proxies /api/* to this
	// Go server on port 5150. The Go server owns /api/*; SvelteKit owns everything else.
	jobStore := scanner.NewJobStore()
	aiJobStore := ai.NewJobStore(3)
	mux := buildDevMux(cfg, adapter, dbStore, jobStore, aiJobStore, registry)

	srv := &http.Server{
		Addr:         listenAddr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 7. Start and block until signal.
	fmt.Printf("Vedox dev server running at http://%s\n", listenAddr)
	fmt.Println("Press Ctrl+C to stop.")
	slog.Info("dev server starting", "addr", listenAddr)

	return runServer(srv)
}

// buildDevMux returns the HTTP mux for the dev server.
//
// Route ownership:
//   - /api/*   — Go API server (this binary); SvelteKit Vite proxies here.
//   - /healthz — lightweight probe used by process supervisors and health checks.
//   - /        — placeholder until SvelteKit static assets are served in VDX-P1-005.
//
// SvelteKit Vite dev server proxies /api/* to this Go server on port 3001+1=3002
// (or configurable offset). The Go server owns /api/*; SvelteKit owns everything else.
func buildDevMux(cfg *config.Config, docStore store.DocStore, dbStore *db.Store, jobStore *scanner.JobStore, aiJobStore *ai.JobStore, registry *store.ProjectRegistry) *http.ServeMux {
	mux := http.NewServeMux()

	apiServer := api.NewServer(docStore, dbStore, cfg.Workspace, jobStore, aiJobStore, registry)
	apiServer.Mount(mux)

	// Healthcheck endpoint. Used by process supervisors and the SvelteKit
	// frontend to confirm the Go backend is alive before making API calls.
	// This endpoint intentionally duplicates /api/health so external probes
	// that don't know about the /api prefix can reach it.
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		// Security: CSP on every response, including localhost dev server.
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; script-src 'none'; object-src 'none'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok","version":%q}`, version)
	})

	// Root: placeholder until SvelteKit static assets are served (VDX-P1-005).
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; script-src 'none'; object-src 'none'")
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "Vedox dev server %s — editor UI coming in VDX-P1-005\n", version)
	})

	return mux
}

// runServer starts srv and blocks until SIGINT or SIGTERM, then performs
// a graceful shutdown with a 10-second deadline.
func runServer(srv *http.Server) error {
	// Channel to receive OS signals.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Channel to receive server errors.
	serverErr := make(chan error, 1)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		slog.Error("server error", "error", err.Error())
		return fmt.Errorf("dev server error: %w", err)

	case sig := <-quit:
		slog.Info("shutdown signal received", "signal", sig.String())
		fmt.Println("\nShutting down...")

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			slog.Error("graceful shutdown failed", "error", err.Error())
			return fmt.Errorf("graceful shutdown failed: %w", err)
		}

		slog.Info("server stopped cleanly")
		return nil
	}
}
