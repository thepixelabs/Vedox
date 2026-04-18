package api

import (
	"net/http"
	"strings"
)

// graphNode is the Cytoscape-compatible node shape returned by GET /api/graph.
// The `data` wrapper is required by Cytoscape.js — it ignores top-level keys.
type graphNode struct {
	Data graphNodeData `json:"data"`
}

type graphNodeData struct {
	// ID is the workspace-relative slash path of the document (e.g. "docs/adr/001.md").
	ID string `json:"id"`
	// Label is the human-readable display name derived from the file name.
	Label string `json:"label"`
}

// graphEdge is the Cytoscape-compatible edge shape. source/target refer to
// graphNodeData.ID values.
type graphEdge struct {
	Data graphEdgeData `json:"data"`
}

type graphEdgeData struct {
	// ID is a stable synthetic identifier for the edge: "<source>::<target>".
	ID string `json:"id"`
	// Source is the workspace-relative path of the document that contains the link.
	Source string `json:"source"`
	// Target is the raw target path as stored in doc_references.target_path.
	Target string `json:"target"`
	// LinkType is the syntactic origin of this edge (md-link, wikilink, etc.).
	LinkType string `json:"linkType"`
}

// graphResponse is the top-level payload for GET /api/graph.
type graphResponse struct {
	Nodes []graphNode `json:"nodes"`
	Edges []graphEdge `json:"edges"`
}

// handleGraph handles GET /api/graph?project=<project>.
//
// It reads all outgoing doc references for every document in the project from
// the GraphStore and assembles them into a Cytoscape-compatible {nodes, edges}
// payload. Nodes are deduplicated — a document appears once as a source node
// and zero or more times as an implicit target. Targets that do not correspond
// to a known source are still emitted as stub nodes so the graph remains
// coherent for the frontend (broken links are visible as dangling edges).
//
// Query parameters:
//
//	project: project name as returned by GET /api/projects. Required.
//
// Errors:
//
//	400 VDX-400 — project parameter is empty.
//	503 VDX-503 — GraphStore is not available (nil — dev-server mode without a db).
//	500 VDX-500 — database read error.
//
// Auth: no token required (consistent with all other GET endpoints at alpha;
// bootstrap token scope is a GA gate per FIX-ARCH-01 spec).
func (s *Server) handleGraph(w http.ResponseWriter, r *http.Request) {
	project := strings.TrimSpace(r.URL.Query().Get("project"))
	if project == "" {
		writeError(w, http.StatusBadRequest, "VDX-400", "project parameter is required")
		return
	}

	if s.graphStore == nil {
		writeError(w, http.StatusServiceUnavailable, "VDX-503", "graph store is not available")
		return
	}

	ctx := r.Context()

	// Collect all references for every document that belongs to this project.
	// The graph store indexes by workspace-relative doc ID; all docs belonging
	// to a project share the prefix "<project>/".
	prefix := project + "/"
	refs, err := s.graphStore.GetAllRefsForPrefix(ctx, prefix)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "VDX-500", "failed to read graph data")
		return
	}

	// Deduplicate nodes. We track by ID to avoid emitting the same node twice
	// when a document has multiple outgoing edges.
	seen := make(map[string]struct{})
	var nodes []graphNode
	var edges []graphEdge

	addNode := func(id string) {
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		nodes = append(nodes, graphNode{
			Data: graphNodeData{
				ID:    id,
				Label: labelFromID(id),
			},
		})
	}

	for _, ref := range refs {
		addNode(ref.SourcePath)
		// Emit the target as a stub node when it is not already known. This
		// keeps the graph coherent for the Cytoscape frontend even when the
		// target does not exist in the index (broken links remain visible).
		addNode(ref.TargetPath)
		edges = append(edges, graphEdge{
			Data: graphEdgeData{
				ID:       ref.SourcePath + "::" + ref.TargetPath,
				Source:   ref.SourcePath,
				Target:   ref.TargetPath,
				LinkType: string(ref.LinkType),
			},
		})
	}

	// Always return non-null slices so the frontend can range without a nil check.
	if nodes == nil {
		nodes = []graphNode{}
	}
	if edges == nil {
		edges = []graphEdge{}
	}

	writeJSON(w, http.StatusOK, graphResponse{
		Nodes: nodes,
		Edges: edges,
	})
}

// labelFromID derives a short human-readable label from a workspace-relative
// doc path. It uses the base file name without extension.
// "docs/adr/001-init.md" → "001-init"
// "my-doc.md"            → "my-doc"
func labelFromID(id string) string {
	// Take the last path segment.
	base := id
	if i := strings.LastIndex(id, "/"); i >= 0 {
		base = id[i+1:]
	}
	// Strip the file extension.
	if i := strings.LastIndex(base, "."); i > 0 {
		base = base[:i]
	}
	return base
}
