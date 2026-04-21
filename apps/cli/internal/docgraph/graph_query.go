package docgraph

import (
	"context"
	"database/sql"
	"fmt"
	"path"
	"sort"
	"strings"
)

// Graph is the full response payload for GET /api/graph?project=<name>.
// Field tags are the wire contract — the frontend GraphData interface
// (apps/editor/src/lib/components/graph/DocGraph.svelte) depends on these
// exact JSON names. Do not rename without a coordinated frontend change.
type Graph struct {
	Nodes       []GraphNode `json:"nodes"`
	Edges       []GraphEdge `json:"edges"`
	Truncated   bool        `json:"truncated"`
	TotalNodes  int         `json:"total_nodes"`
	TotalEdges  int         `json:"total_edges"`
}

// GraphNode is one document in the reference graph. Broken/missing targets
// are synthesised with Type="missing" and Status="broken" so the UI can
// render dangling edges instead of silently dropping them.
type GraphNode struct {
	ID        string `json:"id"`
	Project   string `json:"project"`
	Slug      string `json:"slug"`
	Title     string `json:"title"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	DegreeIn  int    `json:"degree_in"`
	DegreeOut int    `json:"degree_out"`
	Modified  string `json:"modified"`
}

// GraphEdge is one directed reference from Source doc id to Target doc id.
// Kind uses the frontend-canonical enum (mdlink | wikilink | frontmatter |
// vedox_ref); the backend normalises internal LinkType values at the edge.
// Broken=true when Target does not resolve to an indexed doc (and is not
// a vedox-scheme cross-link to source code).
type GraphEdge struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
	Kind   string `json:"kind"`
	Broken bool   `json:"broken"`
}

// graphNodeCap is the maximum number of nodes emitted from GetGraphForProject.
// Nodes are ranked by ref_count + backlink_count DESC, then mod_time DESC, so
// the cap drops leaf/peripheral docs first. Exposed as a package var so tests
// can lower it without seeding a huge fixture.
var graphNodeCap = 2000

// GetGraphForProject returns the enriched reference graph scoped to a single
// project. The returned Graph matches the /api/graph wire shape 1:1 — the
// handler just marshals it. The queries inside this method are the only place
// that joins documents, doc_references, and doc_reference_counts; handler and
// tests consume the typed result directly.
//
// vedox-scheme edges (source-code cross-links like vedox://file/foo.go#L10)
// are intentionally excluded from the doc graph in v1 — they pollute the
// doc-type chip list with a "code" category and the user's primary use case
// is doc-to-doc navigation. Re-enable in a follow-up with a dedicated target
// shape if we need to surface them here.
func (g *GraphStore) GetGraphForProject(ctx context.Context, project string) (Graph, error) {
	if project == "" {
		return Graph{}, fmt.Errorf("docgraph: GetGraphForProject: empty project")
	}

	// 1. Pull every document row for the project. This is our node set for
	//    "known" docs; targets that do not appear here become synthesised
	//    missing nodes below.
	docsByID, docsBySlug, err := g.loadProjectDocs(ctx, project)
	if err != nil {
		return Graph{}, err
	}

	// 2. Pull every outgoing ref for the project (excluding vedox-scheme).
	refs, err := g.loadProjectRefs(ctx, project)
	if err != nil {
		return Graph{}, err
	}

	// 3. Resolve each ref's target to a known doc id (or keep raw & broken),
	//    and count degrees from the resolved edge set. Computing degrees
	//    here instead of pulling from doc_reference_counts keeps them
	//    consistent with the edges we actually emit — the counts table
	//    tracks raw target_path strings, which drift from resolved ids
	//    when the extractor stores relative paths.
	edges := make([]GraphEdge, 0, len(refs))
	missing := make(map[string]struct{})
	for i, r := range refs {
		targetID, broken := resolveTarget(r, docsByID, docsBySlug)
		if broken {
			missing[targetID] = struct{}{}
		}
		edges = append(edges, GraphEdge{
			ID:     fmt.Sprintf("e:%s->%s#%d", r.SourcePath, targetID, i),
			Source: r.SourcePath,
			Target: targetID,
			Kind:   wireKind(r.LinkType),
			Broken: broken,
		})
		if n, ok := docsByID[r.SourcePath]; ok {
			n.DegreeOut++
		}
		if n, ok := docsByID[targetID]; ok {
			n.DegreeIn++
		}
	}

	// 5. Materialise known-doc nodes + synthesised missing-target nodes.
	allNodes := make([]GraphNode, 0, len(docsByID)+len(missing))
	for _, d := range docsByID {
		allNodes = append(allNodes, *d)
	}
	for m := range missing {
		allNodes = append(allNodes, GraphNode{
			ID:      m,
			Project: project,
			Slug:    path.Base(strings.TrimSuffix(m, ".md")),
			Title:   path.Base(strings.TrimSuffix(m, ".md")),
			Type:    "missing",
			Status:  "broken",
		})
	}

	// 6. Apply cap + sort. Deterministic order: degree sum DESC, then mod_time
	//    DESC, then id ASC as a final tiebreaker for stable tests.
	totalNodes := len(allNodes)
	totalEdges := len(edges)
	sort.Slice(allNodes, func(i, j int) bool {
		a, b := allNodes[i], allNodes[j]
		sa, sb := a.DegreeIn+a.DegreeOut, b.DegreeIn+b.DegreeOut
		if sa != sb {
			return sa > sb
		}
		if a.Modified != b.Modified {
			return a.Modified > b.Modified
		}
		return a.ID < b.ID
	})
	truncated := false
	if len(allNodes) > graphNodeCap {
		allNodes = allNodes[:graphNodeCap]
		truncated = true
	}
	// Filter edges whose endpoints are both in the retained node set.
	retained := make(map[string]struct{}, len(allNodes))
	for _, n := range allNodes {
		retained[n.ID] = struct{}{}
	}
	keptEdges := edges[:0]
	for _, e := range edges {
		if _, ok := retained[e.Source]; !ok {
			continue
		}
		if _, ok := retained[e.Target]; !ok {
			continue
		}
		keptEdges = append(keptEdges, e)
	}

	return Graph{
		Nodes:      allNodes,
		Edges:      keptEdges,
		Truncated:  truncated,
		TotalNodes: totalNodes,
		TotalEdges: totalEdges,
	}, nil
}

// loadProjectDocs pulls every documents row for the given project and returns
// two lookup maps: id → *GraphNode (authoritative) and slug → *GraphNode
// (wikilink resolution). The same node pointer is shared between both maps so
// degree updates in attachDegrees are visible via either key.
func (g *GraphStore) loadProjectDocs(ctx context.Context, project string) (map[string]*GraphNode, map[string]*GraphNode, error) {
	rows, err := g.readDB.QueryContext(ctx,
		`SELECT id, slug, COALESCE(title,''), COALESCE(type,''), COALESCE(status,''), mod_time
		   FROM documents
		  WHERE project = ?`,
		project,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("docgraph: loadProjectDocs %q: %w", project, err)
	}
	defer rows.Close()

	byID := make(map[string]*GraphNode)
	bySlug := make(map[string]*GraphNode)
	for rows.Next() {
		n := &GraphNode{Project: project}
		if err := rows.Scan(&n.ID, &n.Slug, &n.Title, &n.Type, &n.Status, &n.Modified); err != nil {
			return nil, nil, fmt.Errorf("scan document row: %w", err)
		}
		byID[n.ID] = n
		if n.Slug != "" {
			// First-writer-wins on slug collisions — extremely rare, and the
			// store enforces (project, slug) uniqueness at write time.
			if _, dup := bySlug[n.Slug]; !dup {
				bySlug[n.Slug] = n
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate documents: %w", err)
	}
	return byID, bySlug, nil
}

// loadProjectRefs pulls every outgoing reference whose source_doc_id lives
// under the given project prefix, excluding vedox-scheme edges.
func (g *GraphStore) loadProjectRefs(ctx context.Context, project string) ([]DocRef, error) {
	rows, err := g.readDB.QueryContext(ctx,
		`SELECT source_doc_id, target_path, link_type, line_num, anchor_text
		   FROM doc_references
		  WHERE source_doc_id LIKE ? ESCAPE '\'
		    AND link_type != ?
		  ORDER BY source_doc_id, line_num, id`,
		escapeLikePrefix(project+"/")+"%",
		string(LinkTypeVedoxScheme),
	)
	if err != nil {
		return nil, fmt.Errorf("docgraph: loadProjectRefs %q: %w", project, err)
	}
	defer rows.Close()
	return scanRefs(rows)
}

// resolveTarget maps a raw ref's target_path to a concrete doc id within the
// project. Returns (resolvedID, broken). When the target resolves to a known
// doc, broken is false and resolvedID is that doc's id. Otherwise broken is
// true and resolvedID is a stable synthesised id (the resolved path if we
// could compute one, else the raw target) so the frontend can still render a
// dangling node.
//
// Resolution order (v1):
//  1. Treat target as a relative md path from the source doc's directory.
//     Accept .md or .mdx extensions; tolerate missing extension.
//  2. Treat target as a slug within the project.
//  3. Fall through to broken with a best-effort synthesised id.
func resolveTarget(r DocRef, byID map[string]*GraphNode, bySlug map[string]*GraphNode) (string, bool) {
	raw := strings.TrimSpace(r.TargetPath)
	if raw == "" {
		return "", true
	}

	// 1. Path-resolved lookup. Only attempt this when the target looks like
	//    a path (contains a slash or ends in a markdown extension) — slug
	//    wikilinks like "ADR Auth" would otherwise collide with path logic.
	if looksLikePath(raw) {
		resolved := path.Clean(path.Join(path.Dir(r.SourcePath), raw))
		if _, ok := byID[resolved]; ok {
			return resolved, false
		}
		// Try with .md appended when the author omitted the extension.
		if !strings.HasSuffix(resolved, ".md") {
			if _, ok := byID[resolved+".md"]; ok {
				return resolved + ".md", false
			}
		}
	}

	// 2. Slug lookup (primary for wikilinks; also covers bare slug targets).
	if n, ok := bySlug[raw]; ok {
		return n.ID, false
	}
	// Also try the raw string with a normalised slug form (lowercase, spaces
	// → hyphens) — mirrors the typical wikilink→slug convention.
	if norm := slugify(raw); norm != raw {
		if n, ok := bySlug[norm]; ok {
			return n.ID, false
		}
	}

	// 3. Broken. Use the path-resolved id when we computed one (keeps the
	//    synthetic node near its expected location in the graph); otherwise
	//    fall back to the raw target string.
	if looksLikePath(raw) {
		return path.Clean(path.Join(path.Dir(r.SourcePath), raw)), true
	}
	return raw, true
}

// looksLikePath heuristically distinguishes a path-shaped target from a
// slug/title-shaped target. True when the string contains a slash or ends
// in a known markdown extension.
func looksLikePath(s string) bool {
	if strings.Contains(s, "/") {
		return true
	}
	low := strings.ToLower(s)
	return strings.HasSuffix(low, ".md") || strings.HasSuffix(low, ".mdx")
}

// slugify is the minimal normalisation used to match wikilink display names
// to canonical slugs. It lowercases and replaces whitespace runs with single
// hyphens. Not a general-purpose slugifier — just enough to bridge typical
// [[Title Case]] ↔ title-case conventions.
func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(s))
	prevSpace := false
	for _, r := range s {
		if r == ' ' || r == '\t' {
			if !prevSpace {
				b.WriteByte('-')
				prevSpace = true
			}
			continue
		}
		prevSpace = false
		b.WriteRune(r)
	}
	return b.String()
}

// wireKind maps the internal LinkType enum to the frontend's canonical
// discriminator. The frontend interface in DocGraph.svelte pins these exact
// strings, so the mapping lives at the backend edge rather than in the UI.
func wireKind(lt LinkType) string {
	switch lt {
	case LinkTypeMD:
		return "mdlink"
	case LinkTypeWikilink:
		return "wikilink"
	case LinkTypeFrontmatter:
		return "frontmatter"
	case LinkTypeVedoxScheme:
		return "vedox_ref"
	default:
		return string(lt)
	}
}

// readDB exposes the underlying read-only *sql.DB handle for internal use.
// It exists so the query helpers above do not need to close over the
// GraphStore receiver repeatedly. Not exported.
var _ = (*sql.DB)(nil)
