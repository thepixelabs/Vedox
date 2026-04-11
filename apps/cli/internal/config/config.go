// Package config loads and validates the vedox.config.toml workspace config.
//
// The config file is TOML. The canonical name is "vedox.config.toml". Users
// may override the path with the --config CLI flag.
//
// Design note: we use TOML (not JSON) because the CTO spec calls the file
// "vedox.config.ts" conceptually, but we parse a TOML file — no TS/JS runtime
// dependency in the Go binary. A future Phase 2 ticket can evaluate a thin
// TS evaluator if dynamic config is needed; Phase 1 does not require it.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	vdxerr "github.com/vedox/vedox/internal/errors"
)

const DefaultConfigName = "vedox.config.toml"

// DefaultPort is the port used when no port is specified in the config.
// Chosen to be memorable (Van Halen "5150") and uncommon in the wild to
// avoid collisions with other dev tools that squat on 3000/3001/8080.
// The SvelteKit Vite dev server runs on DefaultPort+1 (5151) and proxies
// /api/* to this port.
const DefaultPort = 5150

// Profile controls whether the dev server uses development or production
// settings (CSP strictness, minification, source maps, etc.).
type Profile string

const (
	ProfileDev  Profile = "dev"
	ProfileProd Profile = "prod"
)

// Config is the in-memory representation of vedox.config.toml.
// All fields have safe defaults so a minimal config file works.
type Config struct {
	// Port is the TCP port the dev server listens on. Defaults to 5150.
	// The server always binds to 127.0.0.1; use --network to allow 0.0.0.0
	// (that flag is not implemented in Phase 1 — documented for future use).
	Port int `toml:"port"`

	// Workspace is the path to the documentation root directory. Relative
	// paths are resolved against the directory containing the config file.
	// Defaults to "." (same directory as the config file).
	Workspace string `toml:"workspace"`

	// Profile controls environment-specific behaviour. Valid values: "dev", "prod".
	// Defaults to "dev".
	Profile Profile `toml:"profile"`
}

// defaults returns a Config with all fields set to their default values.
func defaults() Config {
	return Config{
		Port:      DefaultPort,
		Workspace: ".",
		Profile:   ProfileDev,
	}
}

// LoadConfig reads and parses the TOML config file at the given path.
//
// If path is empty, LoadConfig looks for DefaultConfigName in the current
// working directory. Returns VDX-002 if the file does not exist.
//
// Returned paths in Config.Workspace are always absolute.
func LoadConfig(path string) (*Config, error) {
	if path == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("could not determine working directory: %w", err)
		}
		path = filepath.Join(cwd, DefaultConfigName)
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("could not resolve config path: %w", err)
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return nil, vdxerr.ConfigNotFound(absPath)
	}

	cfg := defaults()
	if _, err := toml.DecodeFile(absPath, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file '%s': %w", absPath, err)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	// Resolve workspace path relative to the directory containing the config file.
	if !filepath.IsAbs(cfg.Workspace) {
		cfg.Workspace = filepath.Join(filepath.Dir(absPath), cfg.Workspace)
	}
	cfg.Workspace = filepath.Clean(cfg.Workspace)

	return &cfg, nil
}

// validate checks Config for invalid values and returns a descriptive error.
func (c *Config) validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("config: port %d is out of valid range [1, 65535]", c.Port)
	}
	switch c.Profile {
	case ProfileDev, ProfileProd:
		// valid
	default:
		return fmt.Errorf("config: profile %q is invalid; must be %q or %q", c.Profile, ProfileDev, ProfileProd)
	}
	return nil
}
