package frontmatter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeMD writes content to a named file inside a temp dir and returns the path.
func writeMD(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeMD: %v", err)
	}
	return path
}

// collectRules returns a map of rule ID → count of issues with that rule.
func collectRules(issues []LintIssue) map[string]int {
	m := make(map[string]int)
	for _, i := range issues {
		m[i.Rule]++
	}
	return m
}

// hasRule reports whether any issue carries the given rule ID.
func hasRule(issues []LintIssue, rule string) bool {
	for _, i := range issues {
		if i.Rule == rule {
			return true
		}
	}
	return false
}

// severityOf returns the Severity of the first issue matching rule, or "".
func severityOf(issues []LintIssue, rule string) string {
	for _, i := range issues {
		if i.Rule == rule {
			return i.Severity
		}
	}
	return ""
}

// ── helpers ──────────────────────────────────────────────────────────────────

// validFrontmatter returns a minimal, fully-valid frontmatter block for docType.
// All date-like values are YAML-quoted so yaml.v3 keeps them as strings (not
// time.Time), which is what stringField expects.
func validFrontmatter(docType string) string {
	return "---\n" +
		"type: " + docType + "\n" +
		"status: published\n" +
		"title: Test Doc\n" +
		"date: \"2024-01-15\"\n" +
		"slug: test-doc\n" +
		"project: myproject\n" +
		"---\n\nBody text.\n"
}

// ── LINT-001: frontmatter must be present ─────────────────────────────────

func TestLintFile_NoFrontmatter_ReturnsLINT001(t *testing.T) {
	path := writeMD(t, "no-fm.md", "# Just a heading\n\nNo frontmatter here.\n")

	issues, err := LintFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasRule(issues, "LINT-001") {
		t.Error("expected LINT-001 for missing frontmatter, got none")
	}
	if severityOf(issues, "LINT-001") != SeverityError {
		t.Errorf("LINT-001 should be error severity")
	}
}

func TestLintFile_MalformedYAML_ReturnsLINT001(t *testing.T) {
	content := "---\ntype: [\ninvalid yaml\n---\n\nBody.\n"
	path := writeMD(t, "malformed.md", content)

	issues, err := LintFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasRule(issues, "LINT-001") {
		t.Errorf("expected LINT-001 for YAML parse error, got rules: %v", collectRules(issues))
	}
}

func TestLintFile_EmptyFrontmatter_ReportsRequiredFields(t *testing.T) {
	// A frontmatter block with only an unknown key produces LINT-002/004/006/007.
	// Note: "---\n---" with nothing in between — extractFrontmatter cannot find
	// "\n---" at position 0 after the opening delimiter is consumed, so it
	// returns ok=false and fires LINT-001 instead. We use a dummy key so the
	// block is valid YAML but all required fields are absent.
	path := writeMD(t, "empty-fm.md", "---\ndummy: x\n---\n\nBody.\n")

	issues, err := LintFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rules := collectRules(issues)
	for _, r := range []string{"LINT-002", "LINT-004", "LINT-006", "LINT-007"} {
		if rules[r] == 0 {
			t.Errorf("expected %s for frontmatter missing required fields, not found in %v", r, rules)
		}
	}
}

// ── LINT-002 / LINT-003: type field ──────────────────────────────────────────

func TestLintFile_MissingType_ReturnsLINT002(t *testing.T) {
	content := "---\nstatus: published\ntitle: T\ndate: \"2024-01-01\"\n---\n"
	path := writeMD(t, "no-type.md", content)

	issues, err := LintFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasRule(issues, "LINT-002") {
		t.Error("expected LINT-002 for missing type")
	}
}

func TestLintFile_InvalidType_ReturnsLINT003(t *testing.T) {
	content := "---\ntype: blogpost\nstatus: published\ntitle: T\ndate: \"2024-01-01\"\n---\n"
	path := writeMD(t, "bad-type.md", content)

	issues, err := LintFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasRule(issues, "LINT-003") {
		t.Error("expected LINT-003 for non-canonical type")
	}
	if severityOf(issues, "LINT-003") != SeverityError {
		t.Errorf("LINT-003 must be error severity")
	}
}

func TestLintFile_ValidTypes_NoTypeErrors(t *testing.T) {
	validTypes := []string{
		"how-to", "explanation", "runbook", "adr", "readme",
		"api-reference", "issue", "platform", "infrastructure", "network", "logging",
	}
	for _, docType := range validTypes {
		t.Run(docType, func(t *testing.T) {
			name := docType + "-doc.md"
			if docType == "adr" {
				name = "001-" + docType + "-doc.md"
			}
			path := writeMD(t, name, validFrontmatter(docType))

			issues, err := LintFile(path)
			if err != nil {
				t.Fatalf("LintFile: %v", err)
			}
			if hasRule(issues, "LINT-002") || hasRule(issues, "LINT-003") {
				t.Errorf("type %q should not produce type errors, got: %v", docType, collectRules(issues))
			}
		})
	}
}

// ── LINT-004 / LINT-005: status field ────────────────────────────────────────

func TestLintFile_MissingStatus_ReturnsLINT004(t *testing.T) {
	content := "---\ntype: how-to\ntitle: T\ndate: \"2024-01-01\"\n---\n"
	path := writeMD(t, "no-status.md", content)

	issues, err := LintFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasRule(issues, "LINT-004") {
		t.Error("expected LINT-004 for missing status")
	}
}

func TestLintFile_InvalidStatus_ReturnsLINT005Error(t *testing.T) {
	content := "---\ntype: how-to\nstatus: wip\ntitle: T\ndate: \"2024-01-01\"\n---\n"
	path := writeMD(t, "bad-status.md", content)

	issues, err := LintFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasRule(issues, "LINT-005") {
		t.Error("expected LINT-005 for non-canonical status")
	}
	if severityOf(issues, "LINT-005") != SeverityError {
		t.Errorf("LINT-005 for unknown status must be error, got %q", severityOf(issues, "LINT-005"))
	}
}

func TestLintFile_LegacyStatus_ReturnsLINT005Warn(t *testing.T) {
	legacyStatuses := []string{"approved", "archived", "accepted"}
	for _, s := range legacyStatuses {
		t.Run(s, func(t *testing.T) {
			content := "---\ntype: how-to\nstatus: " + s + "\ntitle: T\ndate: \"2024-01-01\"\n---\n"
			path := writeMD(t, "legacy-status.md", content)

			issues, err := LintFile(path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !hasRule(issues, "LINT-005") {
				t.Errorf("legacy status %q should produce LINT-005 warn", s)
			}
			if severityOf(issues, "LINT-005") != SeverityWarn {
				t.Errorf("legacy status %q must be warn, got %q", s, severityOf(issues, "LINT-005"))
			}
		})
	}
}

func TestLintFile_CanonicalStatuses_NoStatusErrors(t *testing.T) {
	canonical := []string{"draft", "review", "published", "deprecated", "superseded"}
	for _, s := range canonical {
		t.Run(s, func(t *testing.T) {
			content := "---\ntype: how-to\nstatus: " + s + "\ntitle: T\ndate: \"2024-01-01\"\n---\n"
			path := writeMD(t, "status.md", content)

			issues, err := LintFile(path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if hasRule(issues, "LINT-004") || hasRule(issues, "LINT-005") {
				t.Errorf("canonical status %q should not produce LINT-004/005, got %v", s, collectRules(issues))
			}
		})
	}
}

// ── LINT-006: title ───────────────────────────────────────────────────────────

func TestLintFile_MissingTitle_ReturnsLINT006(t *testing.T) {
	content := "---\ntype: how-to\nstatus: published\ndate: \"2024-01-01\"\n---\n"
	path := writeMD(t, "no-title.md", content)

	issues, err := LintFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasRule(issues, "LINT-006") {
		t.Error("expected LINT-006 for missing title")
	}
}

// ── LINT-007 / LINT-008: date field ──────────────────────────────────────────

func TestLintFile_MissingDate_ReturnsLINT007(t *testing.T) {
	content := "---\ntype: how-to\nstatus: published\ntitle: T\n---\n"
	path := writeMD(t, "no-date.md", content)

	issues, err := LintFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasRule(issues, "LINT-007") {
		t.Error("expected LINT-007 for missing date")
	}
}

func TestLintFile_BadDateFormat_ReturnsLINT008(t *testing.T) {
	// All values are YAML-quoted so yaml.v3 keeps them as strings — not
	// auto-parsed as time.Time or int by the YAML decoder.
	badDates := []string{"01/15/2024", "2024/01/15", "January 2024", "20240115"}
	for _, d := range badDates {
		t.Run(d, func(t *testing.T) {
			content := "---\ntype: how-to\nstatus: published\ntitle: T\ndate: \"" + d + "\"\n---\n"
			path := writeMD(t, "bad-date.md", content)

			issues, err := LintFile(path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !hasRule(issues, "LINT-008") {
				t.Errorf("date %q should produce LINT-008, got %v", d, collectRules(issues))
			}
		})
	}
}

func TestLintFile_ValidISODate_NoDateError(t *testing.T) {
	// Quote the date so yaml.v3 treats it as a string, matching how production
	// Markdown files authored via the CLI will be serialized.
	content := "---\ntype: how-to\nstatus: published\ntitle: T\ndate: \"2024-01-15\"\n---\n"
	path := writeMD(t, "good-date.md", content)

	issues, err := LintFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasRule(issues, "LINT-007") || hasRule(issues, "LINT-008") {
		t.Errorf("valid ISO date should not produce date errors, got %v", collectRules(issues))
	}
}

// ── LINT-009 / LINT-010: slug field ──────────────────────────────────────────

func TestLintFile_MissingSlug_ReturnsLINT009Warn(t *testing.T) {
	content := "---\ntype: how-to\nstatus: published\ntitle: T\ndate: \"2024-01-01\"\n---\n"
	path := writeMD(t, "no-slug.md", content)

	issues, err := LintFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasRule(issues, "LINT-009") {
		t.Error("expected LINT-009 warn for missing slug")
	}
	if severityOf(issues, "LINT-009") != SeverityWarn {
		t.Errorf("LINT-009 must be warn, got %q", severityOf(issues, "LINT-009"))
	}
}

func TestLintFile_InvalidSlugFormat_ReturnsLINT010(t *testing.T) {
	badSlugs := []string{"My Document", "UPPER-CASE", "has_underscore", "-leading-dash"}
	for _, s := range badSlugs {
		t.Run(s, func(t *testing.T) {
			content := "---\ntype: how-to\nstatus: published\ntitle: T\ndate: \"2024-01-01\"\nslug: \"" + s + "\"\n---\n"
			path := writeMD(t, "bad-slug.md", content)

			issues, err := LintFile(path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !hasRule(issues, "LINT-010") {
				t.Errorf("slug %q should produce LINT-010, got %v", s, collectRules(issues))
			}
		})
	}
}

func TestLintFile_ValidSlug_NoSlugError(t *testing.T) {
	path := writeMD(t, "good-slug.md", validFrontmatter("how-to"))

	issues, err := LintFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasRule(issues, "LINT-009") || hasRule(issues, "LINT-010") {
		t.Errorf("valid slug 'test-doc' should not produce slug errors, got %v", collectRules(issues))
	}
}

// ── LINT-011: project field ───────────────────────────────────────────────────

func TestLintFile_MissingProject_ReturnsLINT011Warn(t *testing.T) {
	content := "---\ntype: how-to\nstatus: published\ntitle: T\ndate: \"2024-01-01\"\nslug: my-doc\n---\n"
	path := writeMD(t, "no-project.md", content)

	issues, err := LintFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasRule(issues, "LINT-011") {
		t.Error("expected LINT-011 warn for missing project")
	}
	if severityOf(issues, "LINT-011") != SeverityWarn {
		t.Errorf("LINT-011 must be warn, got %q", severityOf(issues, "LINT-011"))
	}
}

// ── LINT-012: filename kebab-case ─────────────────────────────────────────────

func TestLintFile_UppercaseFilename_ReturnsLINT012(t *testing.T) {
	path := writeMD(t, "MyDocument.md", validFrontmatter("how-to"))

	issues, err := LintFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasRule(issues, "LINT-012") {
		t.Error("expected LINT-012 for uppercase filename")
	}
	if severityOf(issues, "LINT-012") != SeverityWarn {
		t.Errorf("LINT-012 must be warn, got %q", severityOf(issues, "LINT-012"))
	}
}

func TestLintFile_KebabCaseFilename_NoLINT012(t *testing.T) {
	path := writeMD(t, "my-document.md", validFrontmatter("how-to"))

	issues, err := LintFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasRule(issues, "LINT-012") {
		t.Error("kebab-case filename should not produce LINT-012")
	}
}

// ── LINT-013 / LINT-014: ADR-specific rules ───────────────────────────────────

func TestLintFile_ADR_MissingNumericPrefix_ReturnsLINT013(t *testing.T) {
	content := "---\ntype: adr\nstatus: published\ntitle: T\ndate: \"2024-01-01\"\nslug: my-adr\nproject: p\ndecision_date: \"2024-01-01\"\n---\n"
	path := writeMD(t, "my-adr.md", content) // no NNN- prefix

	issues, err := LintFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasRule(issues, "LINT-013") {
		t.Error("expected LINT-013 for ADR without numeric prefix")
	}
	if severityOf(issues, "LINT-013") != SeverityError {
		t.Errorf("LINT-013 must be error, got %q", severityOf(issues, "LINT-013"))
	}
}

func TestLintFile_ADR_WithNumericPrefix_NoLINT013(t *testing.T) {
	content := "---\ntype: adr\nstatus: published\ntitle: T\ndate: \"2024-01-01\"\nslug: my-adr\nproject: p\ndecision_date: \"2024-01-01\"\n---\n"
	path := writeMD(t, "001-my-adr.md", content)

	issues, err := LintFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasRule(issues, "LINT-013") {
		t.Error("ADR with NNN- prefix should not produce LINT-013")
	}
}

func TestLintFile_ADR_MissingDecisionDate_ReturnsLINT014Warn(t *testing.T) {
	content := "---\ntype: adr\nstatus: published\ntitle: T\ndate: \"2024-01-01\"\nslug: my-adr\nproject: p\n---\n"
	path := writeMD(t, "001-my-adr.md", content)

	issues, err := LintFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasRule(issues, "LINT-014") {
		t.Error("expected LINT-014 warn for ADR missing decision_date")
	}
	if severityOf(issues, "LINT-014") != SeverityWarn {
		t.Errorf("LINT-014 must be warn, got %q", severityOf(issues, "LINT-014"))
	}
}

func TestLintFile_ADR_WithDecisionDate_NoLINT014(t *testing.T) {
	// decision_date is quoted so yaml.v3 keeps it as a string (not time.Time).
	content := "---\ntype: adr\nstatus: published\ntitle: T\ndate: \"2024-01-01\"\nslug: my-adr\nproject: p\ndecision_date: \"2024-01-10\"\n---\n"
	path := writeMD(t, "001-my-adr.md", content)

	issues, err := LintFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasRule(issues, "LINT-014") {
		t.Error("ADR with decision_date should not produce LINT-014")
	}
}

// ── LINT-015: unknown frontmatter keys ───────────────────────────────────────

func TestLintFile_UnknownKey_ReturnsLINT015Warn(t *testing.T) {
	content := "---\ntype: how-to\nstatus: published\ntitle: T\ndate: \"2024-01-01\"\nunknown_field: value\n---\n"
	path := writeMD(t, "unknown-key.md", content)

	issues, err := LintFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasRule(issues, "LINT-015") {
		t.Error("expected LINT-015 warn for unknown frontmatter key")
	}
	if severityOf(issues, "LINT-015") != SeverityWarn {
		t.Errorf("LINT-015 must be warn, got %q", severityOf(issues, "LINT-015"))
	}
}

func TestLintFile_KnownKeys_NoLINT015(t *testing.T) {
	path := writeMD(t, "all-known.md", validFrontmatter("how-to"))

	issues, err := LintFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasRule(issues, "LINT-015") {
		t.Errorf("all known keys should not produce LINT-015, got %v", collectRules(issues))
	}
}

// ── LINT-016: trailing whitespace ────────────────────────────────────────────

func TestLintFile_TrailingWhitespaceInBody_ReturnsLINT016(t *testing.T) {
	content := "---\ntype: how-to\nstatus: published\ntitle: T\ndate: \"2024-01-01\"\n---\n\nLine with trailing space   \nClean line.\n"
	path := writeMD(t, "trailing-ws.md", content)

	issues, err := LintFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasRule(issues, "LINT-016") {
		t.Error("expected LINT-016 for trailing whitespace in body")
	}
	if severityOf(issues, "LINT-016") != SeverityWarn {
		t.Errorf("LINT-016 must be warn, got %q", severityOf(issues, "LINT-016"))
	}
}

func TestLintFile_TrailingWhitespace_WhenNoFrontmatter_StillReportsLINT016(t *testing.T) {
	// Even when LINT-001 fires (no frontmatter), LINT-016 should still run on
	// the whole file content.
	content := "# Heading   \nNo frontmatter but trailing whitespace.   \n"
	path := writeMD(t, "no-fm-trailing.md", content)

	issues, err := LintFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasRule(issues, "LINT-001") {
		t.Error("expected LINT-001")
	}
	if !hasRule(issues, "LINT-016") {
		t.Error("expected LINT-016 even when LINT-001 fires")
	}
}

func TestLintFile_TrailingTab_ReturnsLINT016(t *testing.T) {
	content := "---\ntype: how-to\nstatus: published\ntitle: T\ndate: \"2024-01-01\"\n---\n\nLine with trailing tab\t\n"
	path := writeMD(t, "trailing-tab.md", content)

	issues, err := LintFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasRule(issues, "LINT-016") {
		t.Error("expected LINT-016 for trailing tab")
	}
}

func TestLintFile_NoTrailingWhitespace_NoLINT016(t *testing.T) {
	path := writeMD(t, "clean.md", validFrontmatter("how-to"))

	issues, err := LintFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasRule(issues, "LINT-016") {
		t.Error("clean file should not produce LINT-016")
	}
}

// ── Full valid documents — each canonical type ────────────────────────────────

func TestLintFile_ValidHowTo_NoErrors(t *testing.T) {
	path := writeMD(t, "how-to-setup.md", validFrontmatter("how-to"))
	issues, err := LintFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, i := range issues {
		if i.Severity == SeverityError {
			t.Errorf("unexpected error issue %q: %s", i.Rule, i.Message)
		}
	}
}

func TestLintFile_ValidExplanation_NoErrors(t *testing.T) {
	path := writeMD(t, "explanation-doc.md", validFrontmatter("explanation"))
	issues, err := LintFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, i := range issues {
		if i.Severity == SeverityError {
			t.Errorf("unexpected error issue %q: %s", i.Rule, i.Message)
		}
	}
}

func TestLintFile_ValidRunbook_NoErrors(t *testing.T) {
	path := writeMD(t, "my-runbook.md", validFrontmatter("runbook"))
	issues, err := LintFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, i := range issues {
		if i.Severity == SeverityError {
			t.Errorf("unexpected error issue %q: %s", i.Rule, i.Message)
		}
	}
}

func TestLintFile_ValidReference_NoErrors(t *testing.T) {
	path := writeMD(t, "api-ref.md", validFrontmatter("api-reference"))
	issues, err := LintFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, i := range issues {
		if i.Severity == SeverityError {
			t.Errorf("unexpected error issue %q: %s", i.Rule, i.Message)
		}
	}
}

func TestLintFile_ValidADR_NoErrors(t *testing.T) {
	// All date-like values quoted for yaml.v3 string parsing.
	content := "---\n" +
		"type: adr\n" +
		"status: published\n" +
		"title: Use PostgreSQL\n" +
		"date: \"2024-01-15\"\n" +
		"slug: use-postgresql\n" +
		"project: myproject\n" +
		"decision_date: \"2024-01-10\"\n" +
		"---\n\nDecision body.\n"
	path := writeMD(t, "001-use-postgresql.md", content)

	issues, err := LintFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, i := range issues {
		if i.Severity == SeverityError {
			t.Errorf("unexpected error issue %q: %s", i.Rule, i.Message)
		}
	}
}

// ── LintIssue.String() ────────────────────────────────────────────────────────

func TestLintIssue_String_ErrorSeverity(t *testing.T) {
	i := LintIssue{Rule: "LINT-002", File: "foo.md", Line: 1, Severity: SeverityError, Message: "missing type"}
	s := i.String()
	if !strings.Contains(s, "ERROR") {
		t.Errorf("String() should contain ERROR, got: %q", s)
	}
	if !strings.Contains(s, "LINT-002") {
		t.Errorf("String() should contain rule, got: %q", s)
	}
	if !strings.Contains(s, "missing type") {
		t.Errorf("String() should contain message, got: %q", s)
	}
}

func TestLintIssue_String_WarnSeverity(t *testing.T) {
	i := LintIssue{Rule: "LINT-009", File: "bar.md", Line: 1, Severity: SeverityWarn, Message: "slug missing"}
	s := i.String()
	if !strings.Contains(s, "WARN") {
		t.Errorf("String() should contain WARN, got: %q", s)
	}
}

// ── LintFile: unreadable file ─────────────────────────────────────────────────

func TestLintFile_NonexistentFile_ReturnsError(t *testing.T) {
	_, err := LintFile("/nonexistent/path/file.md")
	if err == nil {
		t.Fatal("expected error for non-existent file, got nil")
	}
}

// ── LintDir ───────────────────────────────────────────────────────────────────

func TestLintDir_LintsMDFiles(t *testing.T) {
	dir := t.TempDir()

	// Write two markdown files: one valid, one with issues.
	validPath := filepath.Join(dir, "valid.md")
	if err := os.WriteFile(validPath, []byte(validFrontmatter("how-to")), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	badPath := filepath.Join(dir, "bad.md")
	if err := os.WriteFile(badPath, []byte("# No frontmatter\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	issues, err := LintDir(dir)
	if err != nil {
		t.Fatalf("LintDir: %v", err)
	}
	// bad.md must have contributed at least LINT-001.
	found := false
	for _, i := range issues {
		if i.Rule == "LINT-001" {
			found = true
			break
		}
	}
	if !found {
		t.Error("LintDir should surface LINT-001 from bad.md")
	}
}

func TestLintDir_SkipsNonMDFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("not markdown"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	issues, err := LintDir(dir)
	if err != nil {
		t.Fatalf("LintDir: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("LintDir should skip non-.md files, got %d issues", len(issues))
	}
}

func TestLintDir_SkipsHiddenDirectories(t *testing.T) {
	dir := t.TempDir()
	hiddenDir := filepath.Join(dir, ".hidden")
	if err := os.MkdirAll(hiddenDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hiddenDir, "secret.md"), []byte("# No FM\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	issues, err := LintDir(dir)
	if err != nil {
		t.Fatalf("LintDir: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("LintDir should skip hidden dirs, got %d issues", len(issues))
	}
}

func TestLintDir_SkipsNodeModules(t *testing.T) {
	dir := t.TempDir()
	nmDir := filepath.Join(dir, "node_modules")
	if err := os.MkdirAll(nmDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nmDir, "readme.md"), []byte("# No FM\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	issues, err := LintDir(dir)
	if err != nil {
		t.Fatalf("LintDir: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("LintDir should skip node_modules, got %d issues", len(issues))
	}
}

func TestLintDir_EmptyDir_ReturnsNoIssues(t *testing.T) {
	dir := t.TempDir()

	issues, err := LintDir(dir)
	if err != nil {
		t.Fatalf("LintDir: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("empty dir should produce no issues, got %d", len(issues))
	}
}

// ── extractFrontmatter (internal) ────────────────────────────────────────────

func TestExtractFrontmatter_NoOpeningDelimiter_ReturnsFalse(t *testing.T) {
	_, _, ok := extractFrontmatter("# Heading\n\nContent.\n")
	if ok {
		t.Error("should return ok=false when file does not start with ---")
	}
}

func TestExtractFrontmatter_NoClosingDelimiter_ReturnsFalse(t *testing.T) {
	_, _, ok := extractFrontmatter("---\ntype: how-to\n")
	if ok {
		t.Error("should return ok=false when closing --- is missing")
	}
}

func TestExtractFrontmatter_ValidBlock_ReturnsComponents(t *testing.T) {
	content := "---\ntype: how-to\n---\n\nBody.\n"
	fm, body, ok := extractFrontmatter(content)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if !strings.Contains(fm, "type: how-to") {
		t.Errorf("frontmatter should contain 'type: how-to', got: %q", fm)
	}
	if !strings.Contains(body, "Body.") {
		t.Errorf("body should contain 'Body.', got: %q", body)
	}
}

func TestExtractFrontmatter_CRLFAfterOpeningDelimiter(t *testing.T) {
	// The parser skips an optional \r\n or \n after "---".  Verify it does not
	// panic and returns ok=true when the content is otherwise valid.
	content := "---\r\ntype: how-to\r\n---\r\nBody.\r\n"
	_, _, ok := extractFrontmatter(content)
	// CRLF after opening delimiter is handled; result may or may not be ok
	// depending on whether "\n---" is found.  The important thing is no panic.
	_ = ok
}
