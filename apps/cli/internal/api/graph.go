package api

import (
	"net/http"
	"strings"

	"github.com/vedox/vedox/internal/docgraph"
)

// handleGraph handles GET /api/graph?project=<name>.
//
// It returns the per-project doc reference graph in the enriched shape the
// frontend DocGraph component expects: flat node objects with project, slug,
// title, type, status, degree_in/out, modified; flat edge objects with source,
// target, kind (mdlink|wikilink|frontmatter|vedox_ref), broken; plus
// truncated / total_nodes / total_edges on the envelope.
//
// The heavy lifting lives in docgraph.GetGraphForProject — this handler is
// just parameter validation, error mapping, and JSON marshalling.
//
// Query parameters:
//
//	project: project name as returned by GET /api/projects. REQUIRED.
//	         An empty or missing project returns 400 VDX-400.
//
// Errors:
//
//	400 VDX-400 — project query parameter is missing.
//	503 VDX-503 — GraphStore is not available (dev-server without a db).
//	500 VDX-500 — database read error.
//
// Unknown projects (project not present in the documents table) return 200
// with an empty graph — this matches how /api/projects/{project}/docs treats
// the same case, and keeps the frontend empty-state logic as the single place
// that renders "no docs yet".
//
// Auth: no token required (consistent with every other GET /api/* endpoint
// at alpha; bootstrap token scope is a GA gate per FIX-ARCH-01 spec).
func (s *Server) handleGraph(w http.ResponseWriter, r *http.Request) {
	project := strings.TrimSpace(r.URL.Query().Get("project"))
	if project == "" {
		writeError(w, http.StatusBadRequest, "VDX-400",
			"project query parameter is required")
		return
	}

	if s.graphStore == nil {
		writeError(w, http.StatusServiceUnavailable, "VDX-503",
			"graph store is not available")
		return
	}

	graph, err := s.graphStore.GetGraphForProject(r.Context(), project)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "VDX-500",
			"failed to read graph data")
		return
	}

	// Always return non-null slices so the frontend can range without a
	// nil guard.
	if graph.Nodes == nil {
		graph.Nodes = []docgraph.GraphNode{}
	}
	if graph.Edges == nil {
		graph.Edges = []docgraph.GraphEdge{}
	}

	writeJSON(w, http.StatusOK, graph)
}
