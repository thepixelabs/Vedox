package cmd

// vedox server — daemon lifecycle commands (WS-A, D2-02, D3-05)
//
// Subcommands:
//   vedox server start     [--foreground] [--no-supervisor] [--port <int>] [--dev] [--deploy-mode <mode>]
//   vedox server stop      [--timeout <int>] [--force]
//   vedox server status    [--json]
//   vedox server restart   [--timeout <int>]
//   vedox server logs      [-n <int>] [--follow]
//   vedox server install   [--auto-start] [--force]
//   vedox server uninstall
//
// All file paths are derived from VedoxHome (default ~/.vedox) via
// internal/daemon.NewPaths().
//
// Implementation rules (spec §1.2, R11, R13):
//   - Bind-guard: daemon refuses if VEDOX_BIND env is set and != 127.0.0.1.
//   - Bootstrap token: 32-byte hex, written to ~/.vedox/daemon-token (0o600).
//   - PID file: ~/.vedox/run/vedoxd.pid, advisory lock on ~/.vedox/run/vedoxd.pid.lock.
//   - SIGHUP: reload stub (Week 3 real implementation).
//   - SIGTERM: 30s graceful drain via http.Server.Shutdown.
//   - SIGUSR1: backup safe-point stub (WS-B hook).
//   - --no-supervisor: self-re-exec with --foreground, log to ~/.vedox/logs/vedoxd.log.
//   - Default (no flag): print supervisor-not-implemented message, fall back to --no-supervisor.

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/vedox/vedox/internal/agentauth"
	"github.com/vedox/vedox/internal/ai"
	"github.com/vedox/vedox/internal/analytics"
	"github.com/vedox/vedox/internal/api"
	"github.com/vedox/vedox/internal/daemon"
	globaldb "github.com/vedox/vedox/internal/db"
	"github.com/vedox/vedox/internal/portcheck"
	"github.com/vedox/vedox/internal/registry"
	"github.com/vedox/vedox/internal/scanner"
	"github.com/vedox/vedox/internal/store"
	"github.com/vedox/vedox/internal/voice"
)

// serverCmd is the parent of `vedox server <subcommand>`.
// It does nothing on its own — operators always invoke a subcommand.
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Manage the Vedox daemon process",
	Long: `Manage the Vedox background daemon.

The daemon runs as a persistent process managed by launchd (macOS) or
systemd (Linux). It watches registered documentation repos, maintains
the SQLite index, and serves the HTTP API on 127.0.0.1.`,
}

// ── server start ─────────────────────────────────────────────────────────────

var serverStartFlags struct {
	foreground   bool
	noSupervisor bool
	port         int
	dev          bool
	deployMode   string
	voice        bool
}

var serverStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the Vedox daemon",
	Long: `Start the Vedox daemon process.

By default the daemon detaches and is managed by the OS service manager
(launchd on macOS, systemd on Linux). Use --foreground to keep it in the
current terminal session, or --no-supervisor to run as a bare background
process without OS service registration.

A bootstrap token is generated at startup and written to ~/.vedox/daemon-token
(mode 0600). In --foreground mode it is also printed to stdout.`,
	RunE: runServerStart,
}

func runServerStart(cmd *cobra.Command, _ []string) error {
	// §6.2 Bind-guard: refuse if VEDOX_BIND env is not exactly "127.0.0.1".
	if bindEnv := os.Getenv("VEDOX_BIND"); bindEnv != "" && bindEnv != portcheck.BindAddr {
		fmt.Fprintf(os.Stderr,
			"[VDX-D10] refusing to start: VEDOX_BIND=%s is not %s. "+
				"The Vedox daemon binds loopback only. "+
				"See https://vedox.pixelabs.sh/docs/security/bind-policy\n",
			bindEnv, portcheck.BindAddr)
		os.Exit(78)
	}

	// §12 dev-mode refused under launchd/systemd supervision.
	if serverStartFlags.dev && os.Getenv("VEDOX_SUPERVISED") == "1" {
		return fmt.Errorf("--dev is not permitted in a supervised (launchd/systemd) invocation; " +
			"unset VEDOX_SUPERVISED or start manually without a supervisor")
	}

	vedoxHome, err := daemon.DefaultVedoxHome()
	if err != nil {
		return fmt.Errorf("[VDX-D08] %w", err)
	}
	p := daemon.NewPaths(vedoxHome)

	if err := daemon.EnsureDirs(p); err != nil {
		return fmt.Errorf("[VDX-D08] %w", err)
	}

	// Determine effective mode:
	//   --foreground          → run in foreground, block.
	//   --no-supervisor (R11) → daemonize via self-re-exec, return.
	//   default               → print supervisor stub warning, fall back to --no-supervisor.
	switch {
	case serverStartFlags.foreground:
		return runForeground(p)
	case serverStartFlags.noSupervisor:
		return runNoSupervisor(p)
	default:
		fmt.Fprintln(os.Stderr,
			"launchd/systemd registration not yet implemented, falling back to --no-supervisor mode")
		return runNoSupervisor(p)
	}
}

// runNoSupervisor daemonizes the current process via self-re-exec with
// --foreground appended to the original argv. Per R11.
func runNoSupervisor(p daemon.Paths) error {
	// Check if daemon is already running before forking.
	rec, err := daemon.ReadPIDFile(p.PIDFile)
	if err == nil && daemon.IsAlive(rec.PID) {
		return fmt.Errorf("[VDX-D01] vedox daemon is already running (pid %d)", rec.PID)
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine binary path: %w", err)
	}

	// Reconstruct the server start args without --no-supervisor (child gets
	// --foreground instead) and pass through --port if set.
	childArgs := []string{"server", "start"}
	if serverStartFlags.port != 0 && serverStartFlags.port != portcheck.DefaultPort {
		childArgs = append(childArgs, "--port", fmt.Sprintf("%d", serverStartFlags.port))
	}
	if serverStartFlags.dev {
		childArgs = append(childArgs, "--dev")
	}

	return daemon.Daemonize(exe, childArgs, p.LogFile)
}

// runForeground is the real daemon body. It is invoked either when --foreground
// is explicitly passed, or when the self-re-exec child starts up.
func runForeground(p daemon.Paths) error {
	startTime := time.Now()

	// §4.2 Step 1: Acquire advisory lock. Exit VDX-D01 if held.
	lock, err := daemon.AcquireLock(p.LockFile)
	if err != nil {
		if err == daemon.ErrAlreadyRunning {
			// Try to read the existing PID for a better message.
			if rec, readErr := daemon.ReadPIDFile(p.PIDFile); readErr == nil {
				return fmt.Errorf("[VDX-D01] vedox daemon is already running (pid %d, port %d)", rec.PID, rec.Port)
			}
			return fmt.Errorf("[VDX-D01] vedox daemon is already running (cannot acquire lock)")
		}
		return fmt.Errorf("[VDX-D08] %w", err)
	}
	// Lock released in shutdown sequence.

	// §4.2 Step 7: Port selection.
	port, err := portcheck.SelectPort(serverStartFlags.port)
	if err != nil {
		lock.Release()
		return err
	}
	listenAddr := portcheck.ListenAddr(port)

	// R13: Generate bootstrap token, write to ~/.vedox/daemon-token (0o600).
	token, err := daemon.GenerateBootstrapToken()
	if err != nil {
		lock.Release()
		return fmt.Errorf("token generation failed: %w", err)
	}
	if err := daemon.WriteTokenFile(p.TokenFile, token); err != nil {
		lock.Release()
		return fmt.Errorf("[VDX-D08] %w", err)
	}
	if serverStartFlags.foreground {
		fmt.Printf("vedox daemon token: %s\n", token)
		fmt.Printf("editor URL: http://%s?token=%s\n", listenAddr, token)
	}

	// Open the global database (~/.vedox/global.db).
	// Failure is non-fatal: the API server degrades gracefully (repo and
	// analytics endpoints return 503) while all workspace-scoped endpoints
	// remain functional.
	//
	// p.Home is already ~/.vedox; db.GlobalDBPath is ".vedox/global.db" relative
	// to the user home directory. We derive the path from the user home directly
	// so we match the contract documented in db.GlobalDBPath.
	globalDBPath := filepath.Join(p.Home, "global.db")
	globalDB, globalDBErr := globaldb.OpenGlobalDB(globalDBPath)
	if globalDBErr != nil {
		slog.Warn("could not open global database; repo registry and analytics unavailable",
			"path", globalDBPath,
			"error", globalDBErr,
		)
	}

	// Build a minimal mux (no config loaded yet — full startup sequence is Week 2).
	// For D2-02 we wire /healthz plus the API server routes.
	mux := http.NewServeMux()

	// Mount the healthz handler (unauthenticated, per spec §5.1).
	healthzHandler := daemon.HealthzHandler(version, commit, buildDate, listenAddr, startTime)
	api.MountHealthz(mux, healthzHandler)

	// Minimal API server wiring — mirrors the dev server pattern from dev.go.
	// workspaceRoot defaults to VedoxHome for daemon mode; real workspace
	// selection from repos.json lands in WS-B.
	workspaceRoot := p.Home
	docStore, _ := store.NewLocalAdapter(workspaceRoot, nil) // best-effort; errors degrade gracefully
	jobStore := scanner.NewJobStore()
	aiJobStore := ai.NewJobStore(3)
	projectRegistry := store.NewProjectRegistry()
	requireAgent := agentauth.PassthroughAuth()

	// Open the workspace-scoped DB so the analytics Collector can write events.
	// Failure is non-fatal — analytics degrades gracefully; the rest of the daemon
	// continues without it.
	wsDB, wsDBErr := globaldb.Open(globaldb.Options{WorkspaceRoot: workspaceRoot})
	if wsDBErr != nil {
		slog.Warn("could not open workspace database; analytics collection unavailable",
			"path", workspaceRoot,
			"error", wsDBErr,
		)
	}

	// Generate a session ID for this daemon run (used by the analytics Collector
	// to tag every event with the current session). We reuse the bootstrap token
	// truncated to 16 hex chars — unique enough per process lifetime.
	sessionID := token
	if len(sessionID) > 16 {
		sessionID = sessionID[:16]
	}

	// Declare analytics pipeline handles; started below after ctx is created.
	var (
		collector  *analytics.Collector
		aggregator *analytics.Aggregator
	)

	if docStore != nil {
		apiServer := api.NewServer(docStore, wsDB, workspaceRoot, jobStore, aiJobStore, projectRegistry, requireAgent)
		if globalDB != nil {
			apiServer.SetGlobalDB(globalDB)
		}
		apiServer.Mount(mux)
	}

	// Root placeholder.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "vedox daemon %s — API on /api/\n", version)
	})

	// Voice pipeline — only started when --voice flag is set (off by default).
	// The pipeline is always stub-mode until a Whisper model is installed at
	// ~/.vedox/models/ggml-base.en.bin and the binary is built with -tags whisper.
	var voicePipeline *voice.Pipeline
	if serverStartFlags.voice {
		vSrc := voice.NewStubAudioSource("")
		vTrans := voice.NewStubTranscriber(nil)

		vPipeline, vErr := voice.NewPipeline(voice.PipelineConfig{
			Source:      vSrc,
			Transcriber: vTrans,
			DaemonURL:   fmt.Sprintf("http://%s", listenAddr),
		})
		if vErr != nil {
			slog.Warn("voice pipeline could not be created; voice disabled", "error", vErr)
		} else {
			voicePipeline = vPipeline

			// Register voice HTTP endpoints before the server starts accepting
			// connections.  VoiceServer.Mount uses http.ServeMux patterns
			// (method + path), which require Go 1.22+.
			vServer := voice.NewVoiceServer(voicePipeline)
			vServer.Mount(mux)

			slog.Info("voice: enabled (stub mode — install whisper model for real STT)")
			if serverStartFlags.foreground {
				fmt.Println("voice: enabled (stub mode — install whisper model for real STT)")
			}
		}
	}

	srv := &http.Server{
		Addr:         listenAddr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// §4.2 Step 15: Write PID file.
	if err := daemon.WritePIDFile(p.PIDFile, daemon.PIDRecord{
		PID:         os.Getpid(),
		Port:        port,
		StartUnixNS: startTime.UnixNano(),
		Version:     version,
	}); err != nil {
		lock.Release()
		return fmt.Errorf("[VDX-D08] %w", err)
	}

	// §11.3: Write port sidecar for editor hot-path read.
	if err := daemon.WritePortSidecar(p.PortFile, port); err != nil {
		slog.Warn("could not write port sidecar", "error", err)
	}

	slog.Info("vedox daemon starting",
		"addr", listenAddr,
		"pid", os.Getpid(),
		"version", version,
	)
	if serverStartFlags.foreground {
		fmt.Printf("vedox daemon running at http://%s\n", listenAddr)
	}

	// §7 Multi-repo registry — open the global repos.json for SIGHUP-driven reload.
	reposJSONPath := filepath.Join(p.Home, "repos.json")
	repoRegistry, regErr := registry.NewFileRegistry(reposJSONPath)
	if regErr != nil {
		slog.Warn("could not open repo registry; SIGHUP reload unavailable", "error", regErr)
	}

	// Root context that is cancelled on SIGTERM/SIGINT.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	// Start the analytics pipeline now that the root context exists.
	// Both Collector and Aggregator respond to ctx cancellation and flush
	// their buffers before exiting.
	if wsDB != nil && globalDB != nil {
		collector = analytics.NewCollector(wsDB, sessionID)
		collector.Start(ctx)

		aggregator = analytics.NewAggregator(wsDB, globalDB)
		aggregator.Start(ctx)
	}

	// Start the voice pipeline if it was successfully constructed above.
	if voicePipeline != nil {
		if err := voicePipeline.Start(ctx); err != nil {
			slog.Warn("voice pipeline failed to start; voice disabled for this session", "error", err)
			voicePipeline = nil
		} else {
			slog.Info("voice pipeline started")
		}
	}

	// §4.3 SIGHUP handler — reload registry from ~/.vedox/repos.json and
	// re-read any per-repo config. Reload is safe to call concurrently with
	// List/Get (registry.FileRegistry uses sync.RWMutex internally).
	hupCh := make(chan os.Signal, 1)
	signal.Notify(hupCh, syscall.SIGHUP)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-hupCh:
				slog.Info("SIGHUP received, reloading config and registry")
				if repoRegistry != nil {
					if err := repoRegistry.Reload(); err != nil {
						slog.Error("registry reload failed", "error", err)
					} else {
						repos, _ := repoRegistry.List()
						slog.Info("registry reloaded", "repos", len(repos))
					}
				}
			}
		}
	}()

	// §4.6 SIGUSR1 stub — backup safe-point hook for WS-B.
	usr1Ch := make(chan os.Signal, 1)
	signal.Notify(usr1Ch, syscall.SIGUSR1)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-usr1Ch:
				slog.Info("SIGUSR1 received, backup safe-point stub (WS-B will implement)")
			}
		}
	}()

	// Start HTTP server in background.
	serverErr := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	// Block until signal or server error.
	select {
	case err := <-serverErr:
		// Server died before we received a signal — clean up and return the error.
		lock.Release()
		daemon.CleanupRunFiles(p)
		return fmt.Errorf("daemon server error: %w", err)

	case <-ctx.Done():
		slog.Info("shutdown signal received, draining (30s budget)")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("graceful shutdown failed", "error", err)
		}
	}

	// §4.4 cleanup sequence: PID file, port sidecar, lock — in that order.
	daemon.CleanupRunFiles(p)
	lock.Release()

	// Stop the voice pipeline before closing DBs.
	if voicePipeline != nil {
		if err := voicePipeline.Stop(); err != nil {
			slog.Warn("voice pipeline stop error", "error", err)
		}
	}

	// Stop the analytics pipeline before closing DBs so in-flight events
	// are flushed and the final aggregation cycle completes cleanly.
	if collector != nil {
		collector.Stop()
	}
	if aggregator != nil {
		aggregator.Stop()
	}

	// Close the workspace DB after the analytics pipeline has stopped.
	if wsDB != nil {
		if err := wsDB.Close(); err != nil {
			slog.Warn("error closing workspace database", "error", err)
		}
	}

	// Close the global database after the HTTP server has drained so no
	// in-flight requests attempt to use it during shutdown.
	if globalDB != nil {
		if err := globalDB.Close(); err != nil {
			slog.Warn("error closing global database", "error", err)
		} else {
			slog.Info("global database closed")
		}
	}

	slog.Info("vedox daemon stopped")
	return nil
}

// ── server stop ──────────────────────────────────────────────────────────────

var serverStopFlags struct {
	timeout int
	force   bool
}

var serverStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the running Vedox daemon",
	Long: `Send SIGTERM to the running Vedox daemon and wait for it to exit cleanly.

The daemon flushes pending SQLite writes and closes open file watchers
before exiting. Use --timeout to adjust the grace window (default 30s).
Use --force to send SIGKILL if the daemon does not exit within --timeout.`,
	RunE: runServerStop,
}

func runServerStop(_ *cobra.Command, _ []string) error {
	vedoxHome, err := daemon.DefaultVedoxHome()
	if err != nil {
		return err
	}
	p := daemon.NewPaths(vedoxHome)

	rec, err := daemon.ReadPIDFile(p.PIDFile)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(os.Stderr, "[VDX-D07] vedox daemon is not running (no PID file)")
			os.Exit(69)
		}
		return fmt.Errorf("cannot read PID file: %w", err)
	}

	if !daemon.IsAlive(rec.PID) {
		fmt.Printf("daemon (pid %d) is not running; cleaning up stale PID file\n", rec.PID)
		daemon.CleanupRunFiles(p)
		return nil
	}

	fmt.Printf("stopping vedox daemon (pid %d)...\n", rec.PID)
	if err := daemon.SendSignal(rec.PID, syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send SIGTERM: %w", err)
	}

	timeout := time.Duration(serverStopFlags.timeout) * time.Second
	if daemon.WaitForExit(rec.PID, timeout) {
		fmt.Println("vedox daemon stopped")
		daemon.CleanupRunFiles(p)
		return nil
	}

	if serverStopFlags.force {
		fmt.Printf("daemon (pid %d) did not exit within %ds; sending SIGKILL\n", rec.PID, serverStopFlags.timeout)
		if err := daemon.SendSignal(rec.PID, syscall.SIGKILL); err != nil {
			slog.Warn("SIGKILL failed", "pid", rec.PID, "error", err)
		}
		daemon.CleanupRunFiles(p)
		return nil
	}

	fmt.Fprintf(os.Stderr,
		"[VDX-D05] daemon (pid %d) did not exit within %ds grace window. "+
			"Use --force to send SIGKILL.\n", rec.PID, serverStopFlags.timeout)
	os.Exit(75)
	return nil
}

// ── server status ─────────────────────────────────────────────────────────────

var serverStatusFlags struct {
	json bool
}

var serverStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show Vedox daemon health, uptime, and active repos",
	Long: `Print daemon health, uptime, listening port, and registered repos.

Use --json for machine-readable output suitable for scripting or monitoring.`,
	RunE: runServerStatus,
}

func runServerStatus(_ *cobra.Command, _ []string) error {
	vedoxHome, err := daemon.DefaultVedoxHome()
	if err != nil {
		return err
	}
	p := daemon.NewPaths(vedoxHome)

	rec, err := daemon.ReadPIDFile(p.PIDFile)
	if err != nil || !daemon.IsAlive(rec.PID) {
		if serverStatusFlags.json {
			fmt.Println(`{"status":"not_running"}`)
			os.Exit(69)
		}
		fmt.Fprintln(os.Stderr, "[VDX-D07] vedox daemon is not running")
		os.Exit(69)
	}

	baseURL := fmt.Sprintf("http://%s:%d", portcheck.BindAddr, rec.Port)
	h, err := daemon.QueryHealthz(baseURL)
	if err != nil {
		if serverStatusFlags.json {
			fmt.Printf(`{"status":"unreachable","pid":%d,"port":%d}%s`, rec.PID, rec.Port, "\n")
			return nil
		}
		fmt.Printf("running — pid %d, port %d (daemon unreachable: %v)\n", rec.PID, rec.Port, err)
		return nil
	}

	if serverStatusFlags.json {
		// Augment the healthz response with pid, listen_addr, supervisor fields.
		out := map[string]interface{}{
			"status":         h.Status,
			"version":        h.Version,
			"commit":         h.Commit,
			"build_date":     h.BuildDate,
			"uptime_seconds": h.UptimeSeconds,
			"pid":            rec.PID,
			"listen_addr":    h.ListenAddr,
			"supervisor":     "none", // launchd/systemd detection is Week 3
		}
		b, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(b))
		return nil
	}

	uptime := time.Duration(h.UptimeSeconds) * time.Second
	fmt.Printf("running — pid %d, port %d, uptime %s, status %s\n",
		rec.PID, rec.Port, formatDuration(uptime), h.Status)
	return nil
}

// ── server restart ─────────────────────────────────────────────────────────────

var serverRestartFlags struct {
	timeout int
}

var serverRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Gracefully restart the Vedox daemon",
	Long: `Stop then start the Vedox daemon.

The daemon drains active HTTP connections before stopping. Restart is
implemented as a sequential stop + start — in-flight requests complete
before the new process begins accepting connections.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Reuse stop logic, then start.
		serverStopFlags.timeout = serverRestartFlags.timeout
		serverStopFlags.force = false
		if err := runServerStop(cmd, args); err != nil {
			return err
		}
		// Brief pause to allow the OS to release the port binding.
		time.Sleep(500 * time.Millisecond)
		serverStartFlags.noSupervisor = true
		return runServerStart(cmd, args)
	},
}

// ── server logs ──────────────────────────────────────────────────────────────

var serverLogsFlags struct {
	follow bool
	lines  int
}

var serverLogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Tail the Vedox daemon log file",
	Long: `Stream daemon log output to stdout.

Reads from ~/.vedox/logs/vedoxd.log. Use -n to set how many trailing lines
to print (default 50). Use --follow to stream new lines as they arrive
(Ctrl-C to stop). --follow survives log rotation.`,
	RunE: runServerLogs,
}

func runServerLogs(cmd *cobra.Command, _ []string) error {
	vedoxHome, err := daemon.DefaultVedoxHome()
	if err != nil {
		return err
	}
	p := daemon.NewPaths(vedoxHome)

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	// Honour Ctrl-C in follow mode.
	if serverLogsFlags.follow {
		sigCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
		defer stop()
		ctx = sigCtx
	}

	return daemon.TailLog(ctx, p.LogFile, serverLogsFlags.lines, serverLogsFlags.follow, os.Stdout)
}

// ── server install ────────────────────────────────────────────────────────────

var serverInstallFlags struct {
	autoStart bool
	force     bool
}

var serverInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Register the Vedox daemon with the OS service manager",
	Long: `Install the Vedox daemon as a supervised OS service.

On macOS: writes a LaunchAgent plist to ~/Library/LaunchAgents/sh.pixelabs.vedoxd.plist
and bootstraps it with launchctl. The daemon will restart automatically after
a crash (KeepAlive.Crashed=true) but will NOT start after a clean exit — so
'vedox server stop' keeps the daemon off until you explicitly start it.

On Linux: writes a systemd user unit to ~/.config/systemd/user/vedoxd.service
and enables it. Use 'loginctl enable-linger $USER' to keep the daemon running
without an active login session.

By default the daemon is registered but not started. Use --auto-start to start
it immediately and enable RunAtLoad (macOS) / start now (Linux).

Use --force to overwrite an existing installation.`,
	RunE: runServerInstall,
}

func runServerInstall(_ *cobra.Command, _ []string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine binary path: %w", err)
	}

	switch runtime.GOOS {
	case "darwin":
		return daemon.InstallLaunchd(exe, serverInstallFlags.autoStart, serverInstallFlags.force)
	case "linux":
		return daemon.InstallSystemd(exe, serverInstallFlags.autoStart, serverInstallFlags.force)
	default:
		return fmt.Errorf("vedox server install is not supported on %s (macOS and Linux only)", runtime.GOOS)
	}
}

// ── server uninstall ──────────────────────────────────────────────────────────

var serverUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove the Vedox daemon from the OS service manager",
	Long: `Uninstall the Vedox daemon service registration.

On macOS: runs 'launchctl bootout' and removes the plist from
~/Library/LaunchAgents/sh.pixelabs.vedoxd.plist.

On Linux: stops and disables the systemd user unit, removes the unit file
from ~/.config/systemd/user/vedoxd.service, and runs daemon-reload.

In both cases the PID file is cleaned up if present. Vedox data at ~/.vedox/
is left untouched. Use --purge (not yet implemented) to also remove runtime
files.`,
	RunE: runServerUninstall,
}

func runServerUninstall(_ *cobra.Command, _ []string) error {
	switch runtime.GOOS {
	case "darwin":
		return daemon.UninstallLaunchd()
	case "linux":
		return daemon.UninstallSystemd()
	default:
		return fmt.Errorf("vedox server uninstall is not supported on %s (macOS and Linux only)", runtime.GOOS)
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

// formatDuration formats a duration as "2h14m" style (omitting zero fields).
func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%dm", h, m)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

// serverLogFile returns the canonical daemon log path from VedoxHome.
func serverLogFile() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".vedox", "logs", "vedoxd.log")
}

func init() {
	// server start flags
	serverStartCmd.Flags().BoolVar(&serverStartFlags.foreground, "foreground", false,
		"run in the foreground without detaching (CI-friendly; used by launchd/systemd)")
	serverStartCmd.Flags().BoolVar(&serverStartFlags.noSupervisor, "no-supervisor", false,
		"start without launchd/systemd registration (bare background process; best-effort)")
	serverStartCmd.Flags().IntVar(&serverStartFlags.port, "port", 0,
		"port for the daemon HTTP API (default: auto-select from 5150-5199)")
	serverStartCmd.Flags().BoolVar(&serverStartFlags.dev, "dev", false,
		"developer mode: pretty logs, /debug/pprof, relaxed CORS (refused under launchd/systemd)")
	serverStartCmd.Flags().StringVar(&serverStartFlags.deployMode, "deploy-mode", "laptop",
		"deployment mode: laptop (default), container, headless")
	serverStartCmd.Flags().BoolVar(&serverStartFlags.voice, "voice", false,
		"enable the voice pipeline (stub STT by default; requires whisper model for real transcription)")

	// server stop flags
	serverStopCmd.Flags().IntVar(&serverStopFlags.timeout, "timeout", 30,
		"seconds to wait for graceful exit before giving up (0 = immediate SIGKILL)")
	serverStopCmd.Flags().BoolVar(&serverStopFlags.force, "force", false,
		"send SIGKILL if daemon does not exit within --timeout")

	// server status flags
	serverStatusCmd.Flags().BoolVar(&serverStatusFlags.json, "json", false,
		"output status as JSON (machine-readable)")

	// server restart flags
	serverRestartCmd.Flags().IntVar(&serverRestartFlags.timeout, "timeout", 30,
		"seconds to wait for the old daemon to exit during restart")

	// server logs flags
	serverLogsCmd.Flags().BoolVarP(&serverLogsFlags.follow, "follow", "f", false,
		"follow the log file (Ctrl-C to stop); survives log rotation")
	serverLogsCmd.Flags().IntVarP(&serverLogsFlags.lines, "lines", "n", 50,
		"number of trailing lines to show before following")

	// server install flags
	serverInstallCmd.Flags().BoolVar(&serverInstallFlags.autoStart, "auto-start", false,
		"start the daemon immediately and enable automatic start on login (RunAtLoad=true on macOS; --now on Linux)")
	serverInstallCmd.Flags().BoolVar(&serverInstallFlags.force, "force", false,
		"overwrite an existing plist/unit file without prompting")

	// wire subcommand tree
	serverCmd.AddCommand(serverStartCmd)
	serverCmd.AddCommand(serverStopCmd)
	serverCmd.AddCommand(serverStatusCmd)
	serverCmd.AddCommand(serverRestartCmd)
	serverCmd.AddCommand(serverLogsCmd)
	serverCmd.AddCommand(serverInstallCmd)
	serverCmd.AddCommand(serverUninstallCmd)

	rootCmd.AddCommand(serverCmd)
}
