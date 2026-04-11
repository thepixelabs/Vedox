package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/vedox/vedox/internal/config"
	vdxerr "github.com/vedox/vedox/internal/errors"
)

// writeConfig writes content to a temp file and returns its path.
func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, config.DefaultConfigName)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeConfig: %v", err)
	}
	return path
}

func TestLoadConfig_Defaults(t *testing.T) {
	path := writeConfig(t, "") // empty TOML = all defaults

	cfg, err := config.LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig returned unexpected error: %v", err)
	}
	if cfg.Port != config.DefaultPort {
		t.Errorf("Port = %d, want %d", cfg.Port, config.DefaultPort)
	}
	if cfg.Profile != config.ProfileDev {
		t.Errorf("Profile = %q, want %q", cfg.Profile, config.ProfileDev)
	}
}

func TestLoadConfig_CustomValues(t *testing.T) {
	toml := `
port      = 4000
workspace = "docs"
profile   = "prod"
`
	path := writeConfig(t, toml)
	cfg, err := config.LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig returned unexpected error: %v", err)
	}
	if cfg.Port != 4000 {
		t.Errorf("Port = %d, want 4000", cfg.Port)
	}
	if cfg.Profile != config.ProfileProd {
		t.Errorf("Profile = %q, want %q", cfg.Profile, config.ProfileProd)
	}
}

func TestLoadConfig_MissingFile_ReturnsVDX002(t *testing.T) {
	_, err := config.LoadConfig("/nonexistent/vedox.config.toml")

	var vdxErr *vdxerr.VedoxError
	if !errors.As(err, &vdxErr) {
		t.Fatalf("expected *vdxerr.VedoxError, got %T: %v", err, err)
	}
	if vdxErr.Code != vdxerr.ErrConfigNotFound {
		t.Errorf("Code = %q, want %q", vdxErr.Code, vdxerr.ErrConfigNotFound)
	}
}

func TestLoadConfig_InvalidPort(t *testing.T) {
	path := writeConfig(t, "port = 99999")
	_, err := config.LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for invalid port, got nil")
	}
}

func TestLoadConfig_InvalidProfile(t *testing.T) {
	path := writeConfig(t, `profile = "staging"`)
	_, err := config.LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for invalid profile, got nil")
	}
}

func TestLoadConfig_WorkspaceResolved_ToAbsolute(t *testing.T) {
	path := writeConfig(t, `workspace = "docs"`)
	cfg, err := config.LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig returned unexpected error: %v", err)
	}
	if !filepath.IsAbs(cfg.Workspace) {
		t.Errorf("Workspace should be absolute, got: %q", cfg.Workspace)
	}
}

func TestLoadConfig_InvalidTOML(t *testing.T) {
	path := writeConfig(t, "port = [not valid toml")
	_, err := config.LoadConfig(path)
	if err == nil {
		t.Fatal("expected parse error for invalid TOML, got nil")
	}
}
