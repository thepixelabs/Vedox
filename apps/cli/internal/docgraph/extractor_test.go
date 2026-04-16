package docgraph

import (
	"strings"
	"testing"
)

// ── helpers ──────────────────────────────────────────────────────────────────

// refsOfKind filters refs by LinkType.
func refsOfKind(refs []DocRef, kind LinkType) []DocRef {
	var out []DocRef
	for _, r := range refs {
		if r.LinkType == kind {
			out = append(out, r)
		}
	}
	return out
}

// findRef returns the first DocRef whose TargetPath equals target, or nil.
func findRef(refs []DocRef, target string) *DocRef {
	for i := range refs {
		if refs[i].TargetPath == target {
			return &refs[i]
		}
	}
	return nil
}

// ── md-link tests ─────────────────────────────────────────────────────────────

func TestExtract_MDLink_Basic(t *testing.T) {
	content := []byte(`# Title

See [the ADR](./decisions/001-auth.md) for details.
`)
	refs := Extract("docs/runbooks/deploy.md", content)
	mdLinks := refsOfKind(refs, LinkTypeMD)
	if len(mdLinks) != 1 {
		t.Fatalf("expected 1 md-link, got %d", len(mdLinks))
	}
	want := "docs/runbooks/decisions/001-auth.md"
	if mdLinks[0].TargetPath != want {
		t.Errorf("TargetPath = %q, want %q", mdLinks[0].TargetPath, want)
	}
	if mdLinks[0].AnchorText != "the ADR" {
		t.Errorf("AnchorText = %q, want %q", mdLinks[0].AnchorText, "the ADR")
	}
}

func TestExtract_MDLink_RelativeParent(t *testing.T) {
	content := []byte(`[overview](../overview.md)`)
	refs := Extract("docs/runbooks/deploy.md", content)
	mdLinks := refsOfKind(refs, LinkTypeMD)
	if len(mdLinks) != 1 {
		t.Fatalf("expected 1 md-link, got %d", len(mdLinks))
	}
	if mdLinks[0].TargetPath != "docs/overview.md" {
		t.Errorf("TargetPath = %q, want %q", mdLinks[0].TargetPath, "docs/overview.md")
	}
}

func TestExtract_MDLink_AbsolutePath(t *testing.T) {
	content := []byte(`[see](/docs/api.md)`)
	refs := Extract("src/readme.md", content)
	mdLinks := refsOfKind(refs, LinkTypeMD)
	if len(mdLinks) != 1 {
		t.Fatalf("expected 1 md-link, got %d", len(mdLinks))
	}
	if mdLinks[0].TargetPath != "/docs/api.md" {
		t.Errorf("TargetPath = %q, want %q", mdLinks[0].TargetPath, "/docs/api.md")
	}
}

func TestExtract_MDLink_WithAnchor(t *testing.T) {
	content := []byte(`[see section](./api.md#authentication)`)
	refs := Extract("docs/readme.md", content)
	mdLinks := refsOfKind(refs, LinkTypeMD)
	if len(mdLinks) != 1 {
		t.Fatalf("expected 1 md-link, got %d", len(mdLinks))
	}
	if mdLinks[0].TargetPath != "docs/api.md" {
		t.Errorf("TargetPath = %q, want %q", mdLinks[0].TargetPath, "docs/api.md")
	}
	// AnchorText may be display text or heading anchor; the link has "see section".
	if mdLinks[0].AnchorText == "" {
		t.Error("AnchorText should not be empty for a link with display text")
	}
}

func TestExtract_MDLink_SkipsHTTPLinks(t *testing.T) {
	content := []byte(`[external](https://example.com/page.md)`)
	refs := Extract("docs/readme.md", content)
	mdLinks := refsOfKind(refs, LinkTypeMD)
	if len(mdLinks) != 0 {
		t.Errorf("http link should not be extracted as md-link, got %d", len(mdLinks))
	}
}

func TestExtract_MDLink_SkipsBareAnchors(t *testing.T) {
	content := []byte(`[top](#heading)`)
	refs := Extract("docs/readme.md", content)
	mdLinks := refsOfKind(refs, LinkTypeMD)
	if len(mdLinks) != 0 {
		t.Errorf("bare anchor should not be extracted as md-link, got %d", len(mdLinks))
	}
}

func TestExtract_MDLink_SkipsNonMarkdownFiles(t *testing.T) {
	content := []byte(`[binary](./image.png) [source](./main.go)`)
	refs := Extract("docs/readme.md", content)
	mdLinks := refsOfKind(refs, LinkTypeMD)
	if len(mdLinks) != 0 {
		t.Errorf("non-.md links should be skipped, got %d", len(mdLinks))
	}
}

func TestExtract_MDLink_MultipleLinks(t *testing.T) {
	content := []byte(`
See [foo](./foo.md) and [bar](./bar.md) and [baz](../baz.md).
`)
	refs := Extract("docs/sub/readme.md", content)
	mdLinks := refsOfKind(refs, LinkTypeMD)
	if len(mdLinks) != 3 {
		t.Fatalf("expected 3 md-links, got %d", len(mdLinks))
	}
}

// ── wikilink tests ────────────────────────────────────────────────────────────

func TestExtract_Wikilink_Basic(t *testing.T) {
	content := []byte(`See [[ADR-001]] for the decision.`)
	refs := Extract("docs/readme.md", content)
	wl := refsOfKind(refs, LinkTypeWikilink)
	if len(wl) != 1 {
		t.Fatalf("expected 1 wikilink, got %d", len(wl))
	}
	if wl[0].TargetPath != "ADR-001" {
		t.Errorf("TargetPath = %q, want %q", wl[0].TargetPath, "ADR-001")
	}
	if wl[0].AnchorText != "ADR-001" {
		t.Errorf("AnchorText = %q, want %q", wl[0].AnchorText, "ADR-001")
	}
}

func TestExtract_Wikilink_WithAlias(t *testing.T) {
	content := []byte(`[[path/to/doc|Display Name]]`)
	refs := Extract("docs/readme.md", content)
	wl := refsOfKind(refs, LinkTypeWikilink)
	if len(wl) != 1 {
		t.Fatalf("expected 1 wikilink, got %d", len(wl))
	}
	if wl[0].TargetPath != "path/to/doc" {
		t.Errorf("TargetPath = %q, want %q", wl[0].TargetPath, "path/to/doc")
	}
	if wl[0].AnchorText != "Display Name" {
		t.Errorf("AnchorText = %q, want %q", wl[0].AnchorText, "Display Name")
	}
}

func TestExtract_Wikilink_WithSlashPath(t *testing.T) {
	content := []byte(`[[project/my-doc]]`)
	refs := Extract("docs/readme.md", content)
	wl := refsOfKind(refs, LinkTypeWikilink)
	if len(wl) != 1 {
		t.Fatalf("expected 1 wikilink, got %d", len(wl))
	}
	if wl[0].TargetPath != "project/my-doc" {
		t.Errorf("TargetPath = %q, want %q", wl[0].TargetPath, "project/my-doc")
	}
}

func TestExtract_Wikilink_Multiple(t *testing.T) {
	content := []byte(`[[Doc A]] and [[Doc B]] and [[Doc C|Alias C]]`)
	refs := Extract("docs/readme.md", content)
	wl := refsOfKind(refs, LinkTypeWikilink)
	if len(wl) != 3 {
		t.Fatalf("expected 3 wikilinks, got %d", len(wl))
	}
}

func TestExtract_Wikilink_LineNumber(t *testing.T) {
	content := []byte("line one\nline two\n[[Target Doc]]\nline four\n")
	refs := Extract("docs/readme.md", content)
	wl := refsOfKind(refs, LinkTypeWikilink)
	if len(wl) != 1 {
		t.Fatalf("expected 1 wikilink, got %d", len(wl))
	}
	// Line 3 in body; body starts at line 1 (no frontmatter), so lineNum = 3.
	if wl[0].LineNum != 3 {
		t.Errorf("LineNum = %d, want 3", wl[0].LineNum)
	}
}

// ── frontmatter tests ─────────────────────────────────────────────────────────

func TestExtract_Frontmatter_Related(t *testing.T) {
	content := []byte(`---
title: My Doc
related:
  - adr-001
  - adr-002
---

Body text.
`)
	refs := Extract("docs/readme.md", content)
	fm := refsOfKind(refs, LinkTypeFrontmatter)
	if len(fm) != 2 {
		t.Fatalf("expected 2 frontmatter refs, got %d", len(fm))
	}
	if findRef(fm, "adr-001") == nil {
		t.Error("expected ref to adr-001")
	}
	if findRef(fm, "adr-002") == nil {
		t.Error("expected ref to adr-002")
	}
}

func TestExtract_Frontmatter_SeeAlso(t *testing.T) {
	content := []byte(`---
title: My Doc
see_also:
  - how-to-deploy
---
`)
	refs := Extract("docs/readme.md", content)
	fm := refsOfKind(refs, LinkTypeFrontmatter)
	if len(fm) != 1 {
		t.Fatalf("expected 1 frontmatter ref, got %d", len(fm))
	}
	if fm[0].TargetPath != "how-to-deploy" {
		t.Errorf("TargetPath = %q, want %q", fm[0].TargetPath, "how-to-deploy")
	}
	if fm[0].AnchorText != "see_also" {
		t.Errorf("AnchorText = %q, want %q", fm[0].AnchorText, "see_also")
	}
}

func TestExtract_Frontmatter_Supersedes(t *testing.T) {
	content := []byte(`---
title: ADR-002
supersedes: adr-001-old-auth
superseded_by: adr-003-new-auth
---
`)
	refs := Extract("docs/adr/002.md", content)
	fm := refsOfKind(refs, LinkTypeFrontmatter)
	if len(fm) != 2 {
		t.Fatalf("expected 2 frontmatter refs (supersedes + superseded_by), got %d", len(fm))
	}
}

func TestExtract_Frontmatter_SingleStringValue(t *testing.T) {
	// related: can be a bare string, not a list.
	content := []byte(`---
related: adr-001
---
`)
	refs := Extract("docs/readme.md", content)
	fm := refsOfKind(refs, LinkTypeFrontmatter)
	if len(fm) != 1 {
		t.Fatalf("expected 1 frontmatter ref for bare string, got %d", len(fm))
	}
	if fm[0].TargetPath != "adr-001" {
		t.Errorf("TargetPath = %q, want %q", fm[0].TargetPath, "adr-001")
	}
}

func TestExtract_Frontmatter_NoFrontmatter(t *testing.T) {
	content := []byte(`# Title

No frontmatter here.
`)
	refs := Extract("docs/readme.md", content)
	fm := refsOfKind(refs, LinkTypeFrontmatter)
	if len(fm) != 0 {
		t.Errorf("expected 0 frontmatter refs when no frontmatter, got %d", len(fm))
	}
}

func TestExtract_Frontmatter_MalformedYAML(t *testing.T) {
	// Malformed YAML must not panic; extractor falls back to zero refs.
	content := []byte("---\n: bad yaml {[}\n---\n\nBody.\n")
	refs := Extract("docs/readme.md", content)
	// We just verify it doesn't panic; ref count may be 0.
	_ = refs
}

// ── vedox-scheme tests ────────────────────────────────────────────────────────

func TestExtract_VedoxScheme_InMDLink(t *testing.T) {
	content := []byte(`See [main.go L10-L25](vedox://file/apps/cli/main.go#L10-L25).`)
	refs := Extract("docs/readme.md", content)
	vs := refsOfKind(refs, LinkTypeVedoxScheme)
	if len(vs) != 1 {
		t.Fatalf("expected 1 vedox-scheme ref, got %d", len(vs))
	}
	if vs[0].TargetPath != "vedox://file/apps/cli/main.go" {
		t.Errorf("TargetPath = %q, want %q", vs[0].TargetPath, "vedox://file/apps/cli/main.go")
	}
	if vs[0].AnchorText != "L10-L25" {
		t.Errorf("AnchorText = %q, want %q", vs[0].AnchorText, "L10-L25")
	}
}

func TestExtract_VedoxScheme_BareInProse(t *testing.T) {
	content := []byte(`The function is at vedox://file/apps/cli/server.go#L42.`)
	refs := Extract("docs/readme.md", content)
	vs := refsOfKind(refs, LinkTypeVedoxScheme)
	if len(vs) != 1 {
		t.Fatalf("expected 1 vedox-scheme bare ref, got %d", len(vs))
	}
	if vs[0].TargetPath != "vedox://file/apps/cli/server.go" {
		t.Errorf("TargetPath = %q, want %q", vs[0].TargetPath, "vedox://file/apps/cli/server.go")
	}
	if vs[0].AnchorText != "L42" {
		t.Errorf("AnchorText = %q, want %q", vs[0].AnchorText, "L42")
	}
}

func TestExtract_VedoxScheme_NoAnchor(t *testing.T) {
	content := []byte(`See vedox://file/apps/cli/main.go for context.`)
	refs := Extract("docs/readme.md", content)
	vs := refsOfKind(refs, LinkTypeVedoxScheme)
	if len(vs) != 1 {
		t.Fatalf("expected 1 vedox-scheme ref without anchor, got %d", len(vs))
	}
	if vs[0].AnchorText != "" {
		t.Errorf("AnchorText should be empty for no-anchor ref, got %q", vs[0].AnchorText)
	}
}

func TestExtract_VedoxScheme_NoDuplicates(t *testing.T) {
	// Same vedox:// URI appears in both a Markdown link and bare prose.
	// The bare-pass de-duplicates against the AST-captured one.
	content := []byte(`[see it](vedox://file/apps/main.go#L1) also at vedox://file/apps/main.go#L1.`)
	refs := Extract("docs/readme.md", content)
	vs := refsOfKind(refs, LinkTypeVedoxScheme)
	if len(vs) != 1 {
		t.Errorf("duplicate vedox refs should be de-duped; got %d", len(vs))
	}
}

// ── mixed content tests ───────────────────────────────────────────────────────

func TestExtract_AllFourKinds(t *testing.T) {
	content := []byte(`---
title: Mixed Doc
related:
  - adr-001
---

# Overview

See [[Wikilink Target]] for background.

The main file is at vedox://file/apps/cli/main.go#L10-L25.

Also see [the runbook](./runbook.md) for procedures.
`)
	refs := Extract("docs/readme.md", content)

	if len(refsOfKind(refs, LinkTypeFrontmatter)) != 1 {
		t.Errorf("expected 1 frontmatter ref, got %d", len(refsOfKind(refs, LinkTypeFrontmatter)))
	}
	if len(refsOfKind(refs, LinkTypeWikilink)) != 1 {
		t.Errorf("expected 1 wikilink, got %d", len(refsOfKind(refs, LinkTypeWikilink)))
	}
	if len(refsOfKind(refs, LinkTypeVedoxScheme)) != 1 {
		t.Errorf("expected 1 vedox-scheme ref, got %d", len(refsOfKind(refs, LinkTypeVedoxScheme)))
	}
	if len(refsOfKind(refs, LinkTypeMD)) != 1 {
		t.Errorf("expected 1 md-link, got %d", len(refsOfKind(refs, LinkTypeMD)))
	}
}

func TestExtract_EmptyContent(t *testing.T) {
	refs := Extract("docs/empty.md", []byte{})
	if len(refs) != 0 {
		t.Errorf("empty content should yield no refs, got %d", len(refs))
	}
}

func TestExtract_SourcePathPreserved(t *testing.T) {
	content := []byte(`[[Target]]`)
	refs := Extract("some/path/doc.md", content)
	if len(refs) == 0 {
		t.Fatal("expected at least one ref")
	}
	for _, r := range refs {
		if r.SourcePath != "some/path/doc.md" {
			t.Errorf("SourcePath = %q, want %q", r.SourcePath, "some/path/doc.md")
		}
	}
}

// ── edge cases ────────────────────────────────────────────────────────────────

func TestExtract_RelativePath_SameDir(t *testing.T) {
	// Source at root level; link to sibling file.
	content := []byte(`[sibling](sibling.md)`)
	refs := Extract("readme.md", content)
	mdLinks := refsOfKind(refs, LinkTypeMD)
	if len(mdLinks) != 1 {
		t.Fatalf("expected 1 md-link, got %d", len(mdLinks))
	}
	if mdLinks[0].TargetPath != "sibling.md" {
		t.Errorf("TargetPath = %q, want %q", mdLinks[0].TargetPath, "sibling.md")
	}
}

func TestExtract_MDLink_DotSlash(t *testing.T) {
	content := []byte(`[doc](./doc.md)`)
	refs := Extract("docs/guide.md", content)
	mdLinks := refsOfKind(refs, LinkTypeMD)
	if len(mdLinks) != 1 {
		t.Fatalf("expected 1 md-link, got %d", len(mdLinks))
	}
	if mdLinks[0].TargetPath != "docs/doc.md" {
		t.Errorf("TargetPath = %q, want %q", mdLinks[0].TargetPath, "docs/doc.md")
	}
}

func TestExtract_Wikilink_NotInCodeSpan(t *testing.T) {
	// The regex does not know about code spans; this tests the boundary.
	// Wikilinks inside backticks are lexically valid but should be considered
	// intentional — we extract them anyway (safe: extra refs are low cost,
	// missed refs are high cost).
	content := []byte("Use `[[Not A Link]]` syntax to link.")
	refs := Extract("docs/readme.md", content)
	wl := refsOfKind(refs, LinkTypeWikilink)
	// We accept 0 or 1 here — the exact behaviour is implementation-defined.
	// What we must not do is panic.
	_ = wl
}

func TestExtract_FrontmatterLineOffset(t *testing.T) {
	// With a 4-line frontmatter block the body wikilink should have lineNum >= 5.
	content := []byte("---\ntitle: T\nstatus: draft\n---\n\n[[Target]]\n")
	refs := Extract("docs/readme.md", content)
	wl := refsOfKind(refs, LinkTypeWikilink)
	if len(wl) != 1 {
		t.Fatalf("expected 1 wikilink, got %d", len(wl))
	}
	if wl[0].LineNum < 5 {
		t.Errorf("LineNum = %d, want >= 5 (body starts after 4-line frontmatter)", wl[0].LineNum)
	}
}

func TestExtract_BrokenLink_StillExtracted(t *testing.T) {
	// Links to non-existent targets must still be returned; resolution is the
	// store's responsibility, not the extractor's.
	content := []byte(`[missing](./does-not-exist.md)`)
	refs := Extract("docs/readme.md", content)
	mdLinks := refsOfKind(refs, LinkTypeMD)
	if len(mdLinks) != 1 {
		t.Fatalf("expected broken link to be extracted, got %d", len(mdLinks))
	}
	if !strings.HasSuffix(mdLinks[0].TargetPath, "does-not-exist.md") {
		t.Errorf("TargetPath = %q, expected suffix does-not-exist.md", mdLinks[0].TargetPath)
	}
}

func TestExtract_VedoxScheme_DeepPath(t *testing.T) {
	content := []byte(`vedox://file/apps/cli/internal/db/store.go#L120-L200`)
	refs := Extract("docs/readme.md", content)
	vs := refsOfKind(refs, LinkTypeVedoxScheme)
	if len(vs) != 1 {
		t.Fatalf("expected 1 vedox-scheme ref with deep path, got %d", len(vs))
	}
	if vs[0].TargetPath != "vedox://file/apps/cli/internal/db/store.go" {
		t.Errorf("TargetPath = %q", vs[0].TargetPath)
	}
	if vs[0].AnchorText != "L120-L200" {
		t.Errorf("AnchorText = %q, want L120-L200", vs[0].AnchorText)
	}
}
