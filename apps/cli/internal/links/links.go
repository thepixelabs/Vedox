// Package links manages the persistence of symlinked (read-only) external
// projects across `vedox dev` restarts. The registry is saved to
// .vedox/links.json inside the workspace root.
//
// File format:
//
//	{
//	  "links": [
//	    {"projectName": "my-api", "externalRoot": "/Users/alice/projects/my-api"},
//	    ...
//	  ]
//	}
//
// The file is written atomically (temp + fsync + rename) to avoid corruption
// on crashes. If the file does not exist an empty list is returned — this is
// normal on first startup.
package links

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const linksFile = ".vedox/links.json"

// LinkedProject is a single entry in the links registry.
type LinkedProject struct {
	ProjectName  string `json:"projectName"`
	ExternalRoot string `json:"externalRoot"`
}

// registry is the top-level JSON shape for links.json.
type registry struct {
	Links []LinkedProject `json:"links"`
}

// Load reads linked projects from .vedox/links.json inside workspaceRoot. If
// the file does not exist an empty (non-nil) slice is returned. Any other I/O
// or parse error is returned to the caller.
func Load(workspaceRoot string) ([]LinkedProject, error) {
	path := filepath.Join(workspaceRoot, linksFile)

	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return []LinkedProject{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("links.Load: read %s: %w", linksFile, err)
	}

	var reg registry
	if err := json.Unmarshal(raw, &reg); err != nil {
		return nil, fmt.Errorf("links.Load: parse %s: %w", linksFile, err)
	}

	if reg.Links == nil {
		reg.Links = []LinkedProject{}
	}
	return reg.Links, nil
}

// Save writes the given linked projects to .vedox/links.json inside
// workspaceRoot. The write is atomic: temp file → fsync → rename.
func Save(workspaceRoot string, projects []LinkedProject) error {
	reg := registry{Links: projects}
	if reg.Links == nil {
		reg.Links = []LinkedProject{}
	}

	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return fmt.Errorf("links.Save: marshal: %w", err)
	}

	dir := filepath.Join(workspaceRoot, ".vedox")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("links.Save: mkdir %s: %w", dir, err)
	}

	target := filepath.Join(workspaceRoot, linksFile)

	// Atomic write: temp → fsync → rename.
	tmp, err := os.CreateTemp(dir, ".vedox-links-*")
	if err != nil {
		return fmt.Errorf("links.Save: create temp file: %w", err)
	}
	tmpName := tmp.Name()

	success := false
	defer func() {
		if !success {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("links.Save: write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("links.Save: fsync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("links.Save: close temp file: %w", err)
	}
	if err := os.Rename(tmpName, target); err != nil {
		return fmt.Errorf("links.Save: rename to %s: %w", linksFile, err)
	}

	success = true
	return nil
}

// Add appends a linked project to the registry, replacing any existing entry
// with the same projectName. The updated list is saved atomically.
func Add(workspaceRoot string, entry LinkedProject) error {
	existing, err := Load(workspaceRoot)
	if err != nil {
		return fmt.Errorf("links.Add: load: %w", err)
	}

	// Replace if already present (re-linking with a different path).
	replaced := false
	for i, p := range existing {
		if p.ProjectName == entry.ProjectName {
			existing[i] = entry
			replaced = true
			break
		}
	}
	if !replaced {
		existing = append(existing, entry)
	}

	return Save(workspaceRoot, existing)
}

// Remove deletes the entry with the given projectName from the registry. If
// the name is not found, Save is still called to normalise the file (no-op
// save). Returns an error only for I/O failures.
func Remove(workspaceRoot, projectName string) error {
	existing, err := Load(workspaceRoot)
	if err != nil {
		return fmt.Errorf("links.Remove: load: %w", err)
	}

	filtered := existing[:0]
	for _, p := range existing {
		if p.ProjectName != projectName {
			filtered = append(filtered, p)
		}
	}

	return Save(workspaceRoot, filtered)
}
