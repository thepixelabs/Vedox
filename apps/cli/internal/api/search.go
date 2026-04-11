package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/vedox/vedox/internal/db"
)

// searchResponse is the JSON shape for a single search result.
// It mirrors db.SearchResult but is a separate type so the API response
// contract is decoupled from the DB schema.
type searchResponse struct {
	ID      string  `json:"id"`
	Project string  `json:"project"`
	Title   string  `json:"title"`
	Type    string  `json:"type"`
	Status  string  `json:"status"`
	Snippet string  `json:"snippet"`
	Score   float64 `json:"score"`
}

// handleSearch runs a BM25 FTS5 query scoped to the given project.
//
// Query parameter: ?q=<search terms>
//
// An empty or missing ?q returns an empty result set rather than an error —
// the frontend uses this to clear the search results pane gracefully.
//
// The db.Store.Search method sanitises the FTS5 query internally (tokens are
// double-quoted and joined with implicit AND) so we pass the raw query value
// without further modification.
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	project := chi.URLParam(r, "project")
	q := r.URL.Query().Get("q")

	// Empty query is a valid no-op — return an empty list rather than 400.
	if q == "" {
		writeJSON(w, http.StatusOK, []searchResponse{})
		return
	}

	filters := db.SearchFilters{
		Project: project,
		Type:    r.URL.Query().Get("type"),
		Status:  r.URL.Query().Get("status"),
		Tag:     r.URL.Query().Get("tag"),
	}
	// Allow ?project= to override the route param when the handler is mounted
	// on a non-project-scoped route (future-proofing).
	if p := r.URL.Query().Get("project"); p != "" {
		filters.Project = p
	}

	results, err := s.db.Search(r.Context(), q, filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "VDX-000",
			"search failed; check server logs")
		return
	}

	out := make([]searchResponse, 0, len(results))
	for _, sr := range results {
		out = append(out, searchResponse{
			ID:      sr.ID,
			Project: sr.Project,
			Title:   sr.Title,
			Type:    sr.Type,
			Status:  sr.Status,
			Snippet: sr.Snippet,
			Score:   sr.Score,
		})
	}
	writeJSON(w, http.StatusOK, out)
}
