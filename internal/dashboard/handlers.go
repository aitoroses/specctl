package dashboard

import (
	"encoding/json"
	"net/http"
)

func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	state := s.state
	s.mu.RUnlock()

	if state == nil {
		writeJSON(w, OverviewResponse{Charters: []CharterSummary{}})
		return
	}
	writeJSON(w, state.overview)
}

func (s *Server) handleCharters(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	state := s.state
	s.mu.RUnlock()

	if state == nil {
		writeJSON(w, []CharterSummary{})
		return
	}
	writeJSON(w, state.charters)
}

func (s *Server) handleSpec(w http.ResponseWriter, r *http.Request) {
	charter := r.PathValue("charter")
	slug := r.PathValue("slug")
	key := charter + ":" + slug

	s.mu.RLock()
	state := s.state
	s.mu.RUnlock()

	if state == nil {
		http.Error(w, `{"error":"not_ready"}`, http.StatusServiceUnavailable)
		return
	}

	detail, ok := state.specs[key]
	if !ok {
		http.Error(w, `{"error":"not_found"}`, http.StatusNotFound)
		return
	}
	writeJSON(w, detail)
}

func (s *Server) handleGraph(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	state := s.state
	s.mu.RUnlock()

	if state == nil {
		writeJSON(w, GraphResponse{Nodes: []GraphNode{}, Edges: []GraphEdge{}})
		return
	}
	writeJSON(w, state.graph)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, `{"error":"encode_error"}`, http.StatusInternalServerError)
	}
}
