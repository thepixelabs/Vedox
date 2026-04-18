// Package docgraph extracts the reference graph from Markdown documents and
// persists it in SQLite. It is designed to hook into the indexer pipeline:
// call Extract after every successful file read; call SaveRefs to persist.
//
// Four link kinds are recognised:
//
//   - md-link    — standard Markdown [text](path) links whose target ends in .md
//   - wikilink   — [[Doc Name]] and [[Doc Name|display]] syntax
//   - frontmatter — YAML `related:`, `see_also:`, `supersedes:`, `superseded_by:` lists
//   - vedox-scheme — vedox://file/path/to/file#L10-L25 inline code cross-links
//
// All four kinds are stored even when the target cannot be resolved (broken
// links). Resolution is intentionally left to the store layer so the extractor
// stays pure and testable without a database.
package docgraph

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"gopkg.in/yaml.v3"
)

// LinkType classifies the syntactic origin of a reference edge.
type LinkType string

const (
	LinkTypeMD          LinkType = "md-link"
	LinkTypeWikilink    LinkType = "wikilink"
	LinkTypeFrontmatter LinkType = "frontmatter"
	LinkTypeVedoxScheme LinkType = "vedox-scheme"
)

// DocRef is one extracted outgoing reference from a source document.
type DocRef struct {
	// SourcePath is the workspace-relative slash-normalised path of the file
	// that contains this link.
	SourcePath string

	// TargetPath is the raw unresolved target string as it appears in the
	// source (relative path, slug, vedox:// URI, etc.).
	TargetPath string

	// LinkType identifies the syntactic form of the link.
	LinkType LinkType

	// LineNum is the 1-based line number in the source file. 0 when unknown.
	LineNum int

	// AnchorText is the display text for md-link/wikilink, the heading anchor
	// for in-doc jumps, or the line anchor for vedox-scheme links (e.g. "L10-L25").
	AnchorText string
}

// wikilinkRE matches [[target]] and [[target|display]].
var wikilinkRE = regexp.MustCompile(`\[\[([^\]|]+?)(?:\|([^\]]*))?\]\]`)

// vedoxSchemeRE matches vedox://file/<path>[#anchor].
var vedoxSchemeRE = regexp.MustCompile(`vedox://file/([^#\s"')\]]+)(?:#([^\s"')\]]+))?`)

// trimTrailingPunct removes trailing sentence punctuation that gets captured
// when a vedox:// URL appears at the end of a prose sentence.
func trimTrailingPunct(s string) string {
	return strings.TrimRight(s, ".,:;!?")
}

// frontmatterRelFields are the YAML keys whose values are treated as references.
var frontmatterRelFields = []string{
	"related",
	"see_also",
	"supersedes",
	"superseded_by",
}

// Extract parses content (raw Markdown bytes) and returns all outgoing
// references. sourcePath must be a workspace-relative slash-normalised path
// (e.g. "docs/adr/001-foo.md"). It is used as DocRef.SourcePath and as the
// base for resolving relative md-link targets.
func Extract(sourcePath string, content []byte) []DocRef {
	var refs []DocRef

	// 1. Split frontmatter from body. We extract frontmatter first so the
	//    goldmark parse below operates on the body text only, keeping line
	//    numbers relative to the body start.
	fm, body, fmLineOffset := splitFrontmatter(content)

	// 2. Frontmatter refs — parsed before goldmark so they are always extracted
	//    even if the body has parse errors.
	refs = append(refs, extractFrontmatterRefs(sourcePath, fm)...)

	// 3. goldmark AST walk for md-links and vedox-scheme links embedded in
	//    standard link/image nodes.
	refs = append(refs, extractASTRefs(sourcePath, body, fmLineOffset)...)

	// 4. Wikilink regex scan across the full body (goldmark does not know about
	//    [[...]] — they are not CommonMark).
	refs = append(refs, extractWikilinks(sourcePath, body, fmLineOffset)...)

	// 5. vedox:// links that appear outside standard Markdown link syntax (e.g.
	//    bare URIs in prose or code spans). The AST walk already handles links
	//    inside [text](vedox://...) nodes; this pass catches the rest.
	refs = append(refs, extractBareVedoxRefs(sourcePath, body, fmLineOffset, refs)...)

	return refs
}

// splitFrontmatter separates YAML frontmatter from the Markdown body.
// Returns the raw YAML string, the body bytes, and the 1-based line number
// at which the body starts (so we can offset line numbers correctly).
func splitFrontmatter(content []byte) (fm string, body []byte, bodyStartLine int) {
	s := string(content)
	if !strings.HasPrefix(s, "---") {
		return "", content, 1
	}
	// Skip the opening --- and optional newline.
	rest := s[3:]
	if strings.HasPrefix(rest, "\r\n") {
		rest = rest[2:]
	} else if strings.HasPrefix(rest, "\n") {
		rest = rest[1:]
	}
	end := strings.Index(rest, "\n---")
	if end == -1 {
		return "", content, 1
	}
	fm = rest[:end]
	// Count lines consumed by the frontmatter block: opening ---, fm content, closing ---.
	fmBlock := "---\n" + fm + "\n---"
	lineCount := strings.Count(fmBlock, "\n") + 1
	body = []byte(rest[end+4:]) // skip \n---
	// Consume optional trailing newline after closing ---.
	if len(body) > 0 && body[0] == '\n' {
		body = body[1:]
		lineCount++
	}
	return fm, body, lineCount + 1
}

// extractFrontmatterRefs scans the parsed YAML map for the well-known
// reference fields and returns a DocRef per target value.
func extractFrontmatterRefs(sourcePath, fm string) []DocRef {
	if strings.TrimSpace(fm) == "" {
		return nil
	}
	var meta map[string]interface{}
	if err := yaml.Unmarshal([]byte(fm), &meta); err != nil {
		return nil
	}
	var refs []DocRef
	for _, field := range frontmatterRelFields {
		v, ok := meta[field]
		if !ok {
			continue
		}
		targets := toStringSlice(v)
		for _, t := range targets {
			t = strings.TrimSpace(t)
			if t == "" {
				continue
			}
			refs = append(refs, DocRef{
				SourcePath: sourcePath,
				TargetPath: t,
				LinkType:   LinkTypeFrontmatter,
				LineNum:    0, // frontmatter line numbers not tracked at this level
				AnchorText: field,
			})
		}
	}
	return refs
}

// extractASTRefs walks the goldmark AST and collects md-link and
// vedox-scheme link nodes.
func extractASTRefs(sourcePath string, body []byte, lineOffset int) []DocRef {
	if len(body) == 0 {
		return nil
	}

	md := goldmark.New()
	reader := text.NewReader(body)
	doc := md.Parser().Parse(reader)

	// Build a line-start offset index so we can convert byte offsets to line
	// numbers cheaply.
	lineStarts := buildLineIndex(body)

	var refs []DocRef
	sourceDir := filepath.Dir(sourcePath)

	// ast.Walk returns an error only when the walker itself returns one;
	// our walker never does, so we can safely ignore the return value here.
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		var dest []byte
		var displayText string

		switch v := n.(type) {
		case *ast.Link:
			dest = v.Destination
			displayText = extractNodeText(v, body)
		case *ast.Image:
			// Images can contain vedox:// refs for inline previews but are
			// not doc-to-doc edges; include only vedox-scheme targets.
			dest = v.Destination
		default:
			return ast.WalkContinue, nil
		}

		target := string(dest)
		if target == "" {
			return ast.WalkContinue, nil
		}

		// Compute line number from node position.
		lineNum := 0
		if seg, ok := firstSegment(n); ok {
			lineNum = byteOffsetToLine(lineStarts, seg) + lineOffset
		}

		if strings.HasPrefix(target, "vedox://") {
			// vedox-scheme link inside a standard MD link node.
			anchor := ""
			if m := vedoxSchemeRE.FindStringSubmatch(target); m != nil {
				target = "vedox://file/" + m[1]
				if len(m) > 2 {
					anchor = m[2]
				}
			}
			refs = append(refs, DocRef{
				SourcePath: sourcePath,
				TargetPath: target,
				LinkType:   LinkTypeVedoxScheme,
				LineNum:    lineNum,
				AnchorText: anchor,
			})
			return ast.WalkContinue, nil
		}

		// Only track local .md links (not http://, mailto:, #anchors, etc.).
		if isLocalMDLink(target) {
			resolved := resolveRelPath(sourceDir, target)
			// Strip any in-doc anchor from the target but preserve it as AnchorText.
			anchor := ""
			clean := resolved
			if idx := strings.Index(resolved, "#"); idx != -1 {
				clean = resolved[:idx]
				anchor = resolved[idx+1:]
			}
			if displayText == "" {
				displayText = anchor
			}
			refs = append(refs, DocRef{
				SourcePath: sourcePath,
				TargetPath: clean,
				LinkType:   LinkTypeMD,
				LineNum:    lineNum,
				AnchorText: displayText,
			})
		}

		return ast.WalkContinue, nil
	})

	return refs
}

// extractWikilinks scans body line by line for [[...]] patterns.
func extractWikilinks(sourcePath string, body []byte, lineOffset int) []DocRef {
	if len(body) == 0 {
		return nil
	}
	var refs []DocRef
	lines := strings.Split(string(body), "\n")
	for i, line := range lines {
		matches := wikilinkRE.FindAllStringSubmatch(line, -1)
		for _, m := range matches {
			target := strings.TrimSpace(m[1])
			display := ""
			if len(m) > 2 {
				display = strings.TrimSpace(m[2])
			}
			if display == "" {
				display = target
			}
			refs = append(refs, DocRef{
				SourcePath: sourcePath,
				TargetPath: target,
				LinkType:   LinkTypeWikilink,
				LineNum:    i + lineOffset,
				AnchorText: display,
			})
		}
	}
	return refs
}

// extractBareVedoxRefs finds vedox:// URIs in prose that are NOT already
// captured as standard MD link destinations (to avoid duplicates).
func extractBareVedoxRefs(sourcePath string, body []byte, lineOffset int, existing []DocRef) []DocRef {
	if len(body) == 0 {
		return nil
	}

	// Build a set of already-captured vedox targets to de-duplicate.
	seen := make(map[string]bool, len(existing))
	for _, r := range existing {
		if r.LinkType == LinkTypeVedoxScheme {
			seen[r.TargetPath] = true
		}
	}

	var refs []DocRef
	lines := strings.Split(string(body), "\n")
	for i, line := range lines {
		matches := vedoxSchemeRE.FindAllStringSubmatch(line, -1)
		for _, m := range matches {
			path := trimTrailingPunct(m[1])
			key := "vedox://file/" + path
			if seen[key] {
				continue
			}
			seen[key] = true
			anchor := ""
			if len(m) > 2 {
				anchor = trimTrailingPunct(m[2])
			}
			refs = append(refs, DocRef{
				SourcePath: sourcePath,
				TargetPath: key,
				LinkType:   LinkTypeVedoxScheme,
				LineNum:    i + lineOffset,
				AnchorText: anchor,
			})
		}
	}
	return refs
}

// isLocalMDLink returns true if target is a relative or absolute path (not a
// URL scheme, not a bare anchor) that ends with .md or has no extension
// (wiki-style bare paths are already handled by extractWikilinks).
func isLocalMDLink(target string) bool {
	// Skip URLs, mailto, tel, etc.
	if strings.Contains(target, "://") {
		return false
	}
	// Skip bare in-doc anchors.
	if strings.HasPrefix(target, "#") {
		return false
	}
	// Accept paths that end in .md (with optional #anchor suffix).
	withoutAnchor := target
	if idx := strings.Index(target, "#"); idx != -1 {
		withoutAnchor = target[:idx]
	}
	return strings.HasSuffix(strings.ToLower(withoutAnchor), ".md")
}

// resolveRelPath resolves a link target against the source document's
// directory, returning a normalised slash path. Absolute paths are returned
// as-is (with leading slash stripped). The result still has the .md suffix
// and optional #anchor appended.
func resolveRelPath(sourceDir, target string) string {
	// Preserve anchor if present.
	anchor := ""
	base := target
	if idx := strings.Index(target, "#"); idx != -1 {
		base = target[:idx]
		anchor = target[idx:]
	}

	var resolved string
	if filepath.IsAbs(base) {
		// Absolute paths are left as-is but normalised.
		resolved = filepath.Clean(base)
	} else {
		resolved = filepath.Clean(filepath.Join(sourceDir, base))
	}
	// Normalise to forward slashes.
	resolved = filepath.ToSlash(resolved)
	return resolved + anchor
}

// buildLineIndex returns the byte offset of the start of each line (index 0 =
// line 1). Used to convert goldmark node segment offsets to line numbers.
func buildLineIndex(b []byte) []int {
	starts := []int{0}
	for i, c := range b {
		if c == '\n' {
			starts = append(starts, i+1)
		}
	}
	return starts
}

// byteOffsetToLine converts a byte offset into a 1-based line number using the
// pre-built line-start index. Returns 0 if the index is empty.
func byteOffsetToLine(lineStarts []int, offset int) int {
	lo, hi := 0, len(lineStarts)-1
	for lo <= hi {
		mid := (lo + hi) / 2
		if lineStarts[mid] <= offset {
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	return hi + 1 // 1-based
}

// firstSegment returns the byte start offset of the first text segment in n,
// searching first the node's own Lines(), then its first Text child, then the
// first text child recursively. Used to map an AST node to a byte offset so
// we can convert to a line number via buildLineIndex.
func firstSegment(n ast.Node) (int, bool) {
	// Block nodes (paragraph, heading) expose their source via Lines().
	// Only call Lines() on block/document nodes — inline nodes may panic.
	if n.Type() == ast.TypeBlock || n.Type() == ast.TypeDocument {
		if lines := n.Lines(); lines != nil && lines.Len() > 0 {
			return lines.At(0).Start, true
		}
	}
	// Inline nodes (link, image) embed their content in child Text nodes.
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch t := c.(type) {
		case *ast.Text:
			return t.Segment.Start, true
		case *ast.String:
			// ast.String nodes don't carry a source position; skip.
		default:
			if off, ok := firstSegment(t); ok {
				return off, true
			}
		}
	}
	return 0, false
}

// extractNodeText walks child text nodes to build the display text for a link.
func extractNodeText(n ast.Node, src []byte) string {
	var b strings.Builder
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch t := c.(type) {
		case *ast.Text:
			b.Write(t.Segment.Value(src))
		case *ast.String:
			b.Write(t.Value)
		}
	}
	return b.String()
}

// toStringSlice coerces a YAML value to a []string. Accepts []interface{},
// []string, and bare string (treated as single-element list).
func toStringSlice(v interface{}) []string {
	switch tv := v.(type) {
	case []interface{}:
		out := make([]string, 0, len(tv))
		for _, item := range tv {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return tv
	case string:
		return []string{tv}
	}
	return nil
}
