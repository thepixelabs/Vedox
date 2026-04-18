package api

// handlers for the Doc Agent install/uninstall/list endpoints:
//
//	POST /api/agent/install   — install the Vedox Doc Agent into a provider
//	POST /api/agent/uninstall — remove the Vedox Doc Agent from a provider
//	GET  /api/agent/list      — list all installed agent configurations
//
// These endpoints delegate to the ProviderInstaller adapters in
// internal/providers. The Server must have a KeyStore and a ReceiptStore
// injected (via SetKeyStore) for these handlers to work. Without a KeyStore
// they return 503 — the same pattern used by globalDB-dependent handlers.
//
// Security:
//   - The plaintext HMAC secret is issued once by the KeyStore and stored only
//     in the OS keychain. It is NEVER returned in any API response.
//   - The JSON receipt contains only the key ID (a UUID) and public metadata.
//   - Handlers always check s.keyStore != nil before proceeding.

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/vedox/vedox/internal/agentauth"
	"github.com/vedox/vedox/internal/db"
	"github.com/vedox/vedox/internal/providers"
)

// ---------------------------------------------------------------------------
// Agent install/uninstall request/response shapes
// ---------------------------------------------------------------------------

// agentInstallRequest is the body for POST /api/agent/install.
type agentInstallRequest struct {
	// Provider is the target AI provider. One of: claude, codex, copilot, gemini.
	Provider string `json:"provider"`
}

// agentUninstallRequest is the body for POST /api/agent/uninstall.
type agentUninstallRequest struct {
	// Provider is the target AI provider. One of: claude, codex, copilot, gemini.
	Provider string `json:"provider"`
}

// agentReceiptResponse is the JSON shape returned after a successful install.
// It mirrors providers.InstallReceipt but omits secret material. The HMAC
// secret is NEVER included — only the key ID is returned.
//
// MED-05 re-audit fix: FileCount replaces the former FileHashes map. The map
// exposed absolute home-directory paths in the HTTP response (e.g.
// "/Users/alice/.claude/agents/vedox-doc.md"), which is unnecessary
// information disclosure. The count is sufficient for the frontend to
// confirm installation completeness; the full hashes remain in the
// on-disk receipt at ~/.vedox/install-receipts/<provider>.json (0600).
type agentReceiptResponse struct {
	Provider    string `json:"provider"`
	Version     string `json:"version"`
	AuthKeyID   string `json:"authKeyID"`
	DaemonURL   string `json:"daemonURL"`
	FileCount   int    `json:"fileCount"`
	InstalledAt string `json:"installedAt"`
}

// agentListItem is one entry in the GET /api/agent/list response.
type agentListItem struct {
	Provider    string `json:"provider"`
	Version     string `json:"version"`
	AuthKeyID   string `json:"authKeyID"`
	InstalledAt string `json:"installedAt"`
}

// ---------------------------------------------------------------------------
// POST /api/agent/install
// ---------------------------------------------------------------------------

// handleAgentInstall implements POST /api/agent/install.
//
// It instantiates the appropriate ProviderInstaller, runs Probe → Plan →
// Install, persists the receipt, and records the install in GlobalDB.
//
// Returns 503 when:
//   - the keyStore is nil (daemon not started with auth support).
//   - the globalDB is nil (dev server mode).
//
// Returns 400 for unknown/unsupported provider IDs.
// Returns 409 when the agent is already installed (probe returns Installed=true).
func (s *Server) handleAgentInstall(w http.ResponseWriter, r *http.Request) {
	if s.keyStore == nil {
		writeError(w, http.StatusServiceUnavailable, "VDX-503",
			"key store not available; start the daemon to enable agent management")
		return
	}

	// Install payload is just { provider: "..." }. Cap at 4 KB so we never
	// buffer a multi-MB garbage body.
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)

	var req agentInstallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "VDX-400", "invalid JSON body")
		return
	}
	req.Provider = strings.TrimSpace(strings.ToLower(req.Provider))

	installer, receiptStore, err := s.buildInstaller(req.Provider)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VDX-400", err.Error())
		return
	}

	ctx := r.Context()

	probe, err := installer.Probe(ctx)
	if err != nil {
		slog.Error("agent/install: probe failed", "provider", req.Provider, "error", err.Error())
		writeError(w, http.StatusInternalServerError, "VDX-500",
			fmt.Sprintf("probe failed: %s", err.Error()))
		return
	}
	if probe.Installed {
		writeError(w, http.StatusConflict, "VDX-409",
			fmt.Sprintf("vedox doc agent is already installed for %s; use repair to re-apply", req.Provider))
		return
	}

	plan, err := installer.Plan(ctx)
	if err != nil {
		slog.Error("agent/install: plan failed", "provider", req.Provider, "error", err.Error())
		writeError(w, http.StatusInternalServerError, "VDX-500",
			fmt.Sprintf("plan failed: %s", err.Error()))
		return
	}

	receipt, err := installer.Install(ctx, plan)
	if err != nil {
		slog.Error("agent/install: install failed", "provider", req.Provider, "error", err.Error())
		writeError(w, http.StatusInternalServerError, "VDX-500",
			fmt.Sprintf("install failed: %s", err.Error()))
		return
	}

	if err := receiptStore.Save(receipt); err != nil {
		slog.Error("agent/install: save receipt", "provider", req.Provider, "error", err.Error())
		// Install succeeded on disk — saving the receipt is best-effort. We
		// log and continue rather than returning 500 (the agent IS installed).
	}

	// Record the install in GlobalDB if available. Not fatal if absent.
	if s.globalDB != nil {
		agentRow := db.AgentInstall{
			ID:           uuid.New().String(),
			Provider:     agentProviderToDB(receipt.Provider),
			Version:      receipt.Version,
			InstallDate:  receipt.InstalledAt.Format("2006-01-02T15:04:05Z"),
			HealthStatus: "healthy",
		}
		if upsertErr := s.globalDB.UpsertAgentInstall(ctx, agentRow); upsertErr != nil {
			slog.Warn("agent/install: upsert globalDB record",
				"provider", req.Provider, "error", upsertErr.Error())
		}
	}

	// Emit agent.installed only after the install actually landed on disk.
	// We use the DB-normalised provider name (e.g. "claude-code" rather than
	// "claude") so the dashboard aggregator doesn't have to rewrite values.
	s.emitEvent("agent.installed", map[string]any{
		"provider": agentProviderToDB(receipt.Provider),
		"version":  receipt.Version,
	})

	writeJSON(w, http.StatusCreated, agentReceiptResponse{
		Provider:    string(receipt.Provider),
		Version:     receipt.Version,
		AuthKeyID:   receipt.AuthKeyID,
		DaemonURL:   receipt.DaemonURL,
		FileCount:   len(receipt.FileHashes),
		InstalledAt: receipt.InstalledAt.Format("2006-01-02T15:04:05Z"),
	})
}

// ---------------------------------------------------------------------------
// POST /api/agent/uninstall
// ---------------------------------------------------------------------------

// handleAgentUninstall implements POST /api/agent/uninstall.
//
// It instantiates the appropriate ProviderInstaller and calls Uninstall, which
// revokes the HMAC key and strips all Vedox-managed content.
func (s *Server) handleAgentUninstall(w http.ResponseWriter, r *http.Request) {
	if s.keyStore == nil {
		writeError(w, http.StatusServiceUnavailable, "VDX-503",
			"key store not available; start the daemon to enable agent management")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)

	var req agentUninstallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "VDX-400", "invalid JSON body")
		return
	}
	req.Provider = strings.TrimSpace(strings.ToLower(req.Provider))

	installer, _, err := s.buildInstaller(req.Provider)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VDX-400", err.Error())
		return
	}

	if err := installer.Uninstall(r.Context()); err != nil {
		slog.Error("agent/uninstall: failed", "provider", req.Provider, "error", err.Error())
		writeError(w, http.StatusInternalServerError, "VDX-500",
			fmt.Sprintf("uninstall failed: %s", err.Error()))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"provider": req.Provider,
		"status":   "uninstalled",
	})
}

// ---------------------------------------------------------------------------
// GET /api/agent/list
// ---------------------------------------------------------------------------

// handleAgentList implements GET /api/agent/list.
//
// It reads all provider receipt files from disk and returns a JSON array.
// The array is empty (never null) when no providers are installed.
func (s *Server) handleAgentList(w http.ResponseWriter, r *http.Request) {
	home, err := s.userHome()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "VDX-500", "could not determine home directory")
		return
	}

	receiptStore, err := providers.NewReceiptStore(home + "/.vedox")
	if err != nil {
		slog.Error("agent/list: receipt store", "error", err.Error())
		writeError(w, http.StatusInternalServerError, "VDX-500", "could not open receipt store")
		return
	}

	receipts, err := receiptStore.List()
	if err != nil {
		slog.Error("agent/list: list receipts", "error", err.Error())
		writeError(w, http.StatusInternalServerError, "VDX-500", "could not read agent installations")
		return
	}

	out := make([]agentListItem, 0, len(receipts))
	for _, rec := range receipts {
		out = append(out, agentListItem{
			Provider:    string(rec.Provider),
			Version:     rec.Version,
			AuthKeyID:   rec.AuthKeyID,
			InstalledAt: rec.InstalledAt.Format("2006-01-02T15:04:05Z"),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// buildInstaller constructs the appropriate ProviderInstaller and a
// ReceiptStore for the given provider string. Returns an error for unknown
// providers. The ReceiptStore is returned so callers can Save after Install.
//
// When s.installerFactoryOverride is non-nil (tests only) it is consulted
// first and returned verbatim, bypassing the real provider constructors.
// agent_test.go uses this seam to inject a stub installer that can force
// Install/Uninstall errors deterministically.
func (s *Server) buildInstaller(provider string) (providers.ProviderInstaller, *providers.ReceiptStore, error) {
	if s.installerFactoryOverride != nil {
		return s.installerFactoryOverride(provider)
	}
	home, err := s.userHome()
	if err != nil {
		return nil, nil, fmt.Errorf("could not determine home directory: %w", err)
	}

	vedoxDir := home + "/.vedox"
	receiptStore, err := providers.NewReceiptStore(vedoxDir)
	if err != nil {
		return nil, nil, fmt.Errorf("could not open receipt store: %w", err)
	}

	daemonURL := "http://127.0.0.1:5150"

	// s.keyStore implements providers.KeyIssuer (IssueKey + RevokeKey).
	var ks providers.KeyIssuer = s.keyStore

	switch providers.ProviderID(provider) {
	case providers.ProviderClaude:
		inst, err := providers.NewClaudeInstaller("", daemonURL, ks, receiptStore)
		if err != nil {
			return nil, nil, fmt.Errorf("initialise claude installer: %w", err)
		}
		return inst, receiptStore, nil

	case providers.ProviderCodex:
		inst, err := providers.NewCodexInstaller("", daemonURL, ks, receiptStore)
		if err != nil {
			return nil, nil, fmt.Errorf("initialise codex installer: %w", err)
		}
		return inst, receiptStore, nil

	case providers.ProviderCopilot:
		inst, err := providers.NewCopilotInstaller("", "", daemonURL, ks, receiptStore)
		if err != nil {
			return nil, nil, fmt.Errorf("initialise copilot installer: %w", err)
		}
		return inst, receiptStore, nil

	case providers.ProviderGemini:
		inst, err := providers.NewGeminiInstaller("", daemonURL, ks, receiptStore)
		if err != nil {
			return nil, nil, fmt.Errorf("initialise gemini installer: %w", err)
		}
		return inst, receiptStore, nil

	default:
		return nil, nil, fmt.Errorf("unknown provider %q; must be one of: claude, codex, copilot, gemini", provider)
	}
}

// agentProviderToDB maps a ProviderID to the value expected by the DB CHECK
// constraint on agent_installs.provider.
func agentProviderToDB(p providers.ProviderID) string {
	switch p {
	case providers.ProviderClaude:
		return "claude-code"
	case providers.ProviderCodex:
		return "codex"
	case providers.ProviderCopilot:
		return "copilot"
	case providers.ProviderGemini:
		return "gemini"
	default:
		return string(p)
	}
}

// keyStoreIssuer is satisfied by *agentauth.KeyStore when injected into Server.
// This compile-time check ensures the KeyStore interface is not silently broken.
var _ providers.KeyIssuer = (*agentauth.KeyStore)(nil)
