// Package frontmatter validates Markdown files against the Vedox
// WRITING_FRAMEWORK frontmatter contract.
//
// Phase 2: warn-first, always exits 0. --strict (Phase 3) is wired but
// currently also exits 0 (TODO: harden after VDX-P2-M migration clears
// the exemption list to zero).
package frontmatter

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Severity levels for lint issues.
const (
	SeverityError = "error"
	SeverityWarn  = "warn"
)

// LintIssue describes a single violation found in a file.
type LintIssue struct {
	Rule     string
	File     string
	Line     int
	Message  string
	Severity string
}

func (i LintIssue) String() string {
	sev := strings.ToUpper(i.Severity)
	if i.Severity == SeverityError {
		sev = "ERROR"
	} else {
		sev = "WARN "
	}
	return fmt.Sprintf("%s  %-10s  %s:%d  %s", sev, i.Rule, i.File, i.Line, i.Message)
}

// canonical type and status sets per WRITING_FRAMEWORK.
var canonicalTypes = map[string]bool{
	"adr": true, "how-to": true, "runbook": true, "readme": true,
	"api-reference": true, "explanation": true, "issue": true,
	"platform": true, "infrastructure": true, "network": true, "logging": true,
}

var canonicalStatuses = map[string]bool{
	"draft": true, "review": true, "published": true,
	"deprecated": true, "superseded": true,
}

// legacyStatuses are deprecated but recognised — emit LINT-W-001 warn, not error.
var legacyStatuses = map[string]bool{
	"approved": true, "archived": true, "accepted": true,
}

// universalFrontmatterKeys are top-level keys valid on every content type.
var universalFrontmatterKeys = map[string]bool{
	"type": true, "status": true, "title": true, "date": true,
	"slug": true, "project": true, "tags": true, "author": true,
	"description": true, "draft": true,
	// per-type additive fields that are common enough to whitelist here
	"decision_date": true, "deciders": true, "superseded_by": true, // ADR
	"severity": true, "kind": true, "resolution": true,             // issue
	"owner": true, "runbook_url": true,                             // runbook
	"redirect_from": true,                                          // all
}

var (
	reDateISO    = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	reSlug       = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	reKebabFile  = regexp.MustCompile(`^[a-z0-9][a-z0-9.\-]*$`)
	reADRPrefix  = regexp.MustCompile(`^\d{3}-`)
	reTrailSpace = regexp.MustCompile(`[ \t]+$`)
)

// LintFile validates a single Markdown file and returns any issues found.
// All issues are collected — execution does not stop on first error.
func LintFile(path string) ([]LintIssue, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	content := string(raw)
	base := filepath.Base(path)

	var issues []LintIssue
	add := func(rule, sev, msg string) {
		issues = append(issues, LintIssue{Rule: rule, File: path, Line: 1, Severity: sev, Message: msg})
	}

	// LINT-001: frontmatter block must be present.
	fm, body, ok := extractFrontmatter(content)
	if !ok {
		add("LINT-001", SeverityError, "no YAML frontmatter block (expected --- delimiters at top of file)")
		// Can't proceed with field checks — but still run LINT-016 on whole file.
		issues = append(issues, trailingWhitespaceIssues(path, content)...)
		return issues, nil
	}

	// Parse YAML.
	var meta map[string]interface{}
	if err := yaml.Unmarshal([]byte(fm), &meta); err != nil {
		add("LINT-001", SeverityError, fmt.Sprintf("YAML frontmatter parse error: %s", err))
		return issues, nil
	}
	if meta == nil {
		meta = map[string]interface{}{}
	}

	docType := stringField(meta, "type")

	// LINT-002: type present.
	if docType == "" {
		add("LINT-002", SeverityError, "required field 'type' is missing")
	} else {
		// LINT-003: type is canonical.
		if !canonicalTypes[docType] {
			add("LINT-003", SeverityError, fmt.Sprintf("type %q is not a canonical type (valid: adr, how-to, runbook, readme, api-reference, explanation, issue, platform, infrastructure, network, logging)", docType))
		}
	}

	// LINT-004: status present.
	status := stringField(meta, "status")
	if status == "" {
		add("LINT-004", SeverityError, "required field 'status' is missing")
	} else {
		// LINT-005: status is canonical or known-legacy.
		if legacyStatuses[status] {
			add("LINT-005", SeverityWarn, fmt.Sprintf("status %q is deprecated (LINT-W-001); will be normalized — use 'published', 'deprecated', or 'draft'", status))
		} else if !canonicalStatuses[status] {
			add("LINT-005", SeverityError, fmt.Sprintf("status %q is not a canonical status (valid: draft, review, published, deprecated, superseded)", status))
		}
	}

	// LINT-006: title present and non-empty.
	if stringField(meta, "title") == "" {
		add("LINT-006", SeverityError, "required field 'title' is missing or empty")
	}

	// LINT-007: date present.
	date := stringField(meta, "date")
	if date == "" {
		add("LINT-007", SeverityError, "required field 'date' is missing")
	} else {
		// LINT-008: date is ISO 8601 YYYY-MM-DD.
		if !reDateISO.MatchString(date) {
			add("LINT-008", SeverityError, fmt.Sprintf("date %q must be ISO 8601 format YYYY-MM-DD", date))
		}
	}

	// LINT-009: slug present (warn only in Phase 2).
	slug := stringField(meta, "slug")
	if slug == "" {
		add("LINT-009", SeverityWarn, "field 'slug' is missing (required in Phase 3; derive from filename for now)")
	} else {
		// LINT-010: slug format.
		if !reSlug.MatchString(slug) {
			add("LINT-010", SeverityError, fmt.Sprintf("slug %q must be lowercase kebab-case (e.g. my-document)", slug))
		}
	}

	// LINT-011: project present.
	if stringField(meta, "project") == "" {
		add("LINT-011", SeverityWarn, "field 'project' is missing or empty")
	}

	// LINT-012: filename is kebab-case.
	if !reKebabFile.MatchString(base) {
		add("LINT-012", SeverityWarn, fmt.Sprintf("filename %q should be lowercase kebab-case with no spaces or uppercase", base))
	}

	// LINT-013: ADR filename must start with NNN-.
	if docType == "adr" && !reADRPrefix.MatchString(base) {
		add("LINT-013", SeverityError, fmt.Sprintf("ADR filename %q must start with a 3-digit prefix (e.g. 001-my-decision.md)", base))
	}

	// LINT-014: ADR must have decision_date.
	if docType == "adr" {
		if stringField(meta, "decision_date") == "" {
			add("LINT-014", SeverityWarn, "ADR is missing 'decision_date' field")
		}
	}

	// LINT-015: warn on unknown top-level keys.
	for k := range meta {
		if !universalFrontmatterKeys[k] {
			add("LINT-015", SeverityWarn, fmt.Sprintf("unknown frontmatter key %q (may be a valid per-type additive field; update universalFrontmatterKeys if intentional)", k))
		}
	}

	// LINT-016: no trailing whitespace in body.
	issues = append(issues, trailingWhitespaceIssues(path, body)...)

	return issues, nil
}

// LintDir recursively lints all .md files under dir, skipping hidden dirs
// and node_modules.
func LintDir(dir string) ([]LintIssue, error) {
	var all []LintIssue
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".md") {
			return nil
		}
		issues, ferr := LintFile(path)
		if ferr != nil {
			return ferr
		}
		all = append(all, issues...)
		return nil
	})
	return all, err
}

// extractFrontmatter splits a Markdown file into (frontmatter YAML, body, ok).
// Returns ok=false if the file does not start with ---.
func extractFrontmatter(content string) (fm, body string, ok bool) {
	if !strings.HasPrefix(content, "---") {
		return "", content, false
	}
	rest := content[3:]
	// skip optional \r\n or \n after opening ---
	if strings.HasPrefix(rest, "\r\n") {
		rest = rest[2:]
	} else if strings.HasPrefix(rest, "\n") {
		rest = rest[1:]
	}
	end := strings.Index(rest, "\n---")
	if end == -1 {
		return "", content, false
	}
	fm = rest[:end]
	body = rest[end+4:] // skip \n---
	return fm, body, true
}

func stringField(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

func trailingWhitespaceIssues(path, text string) []LintIssue {
	var issues []LintIssue
	scanner := bufio.NewScanner(strings.NewReader(text))
	line := 1
	for scanner.Scan() {
		if reTrailSpace.MatchString(scanner.Text()) {
			issues = append(issues, LintIssue{
				Rule:     "LINT-016",
				File:     path,
				Line:     line,
				Severity: SeverityWarn,
				Message:  "trailing whitespace",
			})
		}
		line++
	}
	return issues
}
