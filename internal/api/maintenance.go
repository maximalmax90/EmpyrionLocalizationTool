package api

import (
	"encoding/json"
	"net/http"
)

type rollbackRequest struct {
	Revision string `json:"revision"`
	Message  string `json:"message,omitempty"`
}

func (s *Server) handleRollback(w http.ResponseWriter, r *http.Request) {
	var req rollbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "decoding request body: %v", err)
		return
	}
	if req.Revision == "" {
		writeError(w, http.StatusBadRequest, "\"revision\" is required")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.store.Rollback(req.Revision, req.Message); err != nil {
		writeError(w, http.StatusBadRequest, "%v", err)
		return
	}
	p, err := s.store.Load()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "rolled back but failed to reload state: %v", err)
		return
	}
	s.proj = p

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleGC(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.store.GC(); err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type compactRequest struct {
	Boundary string `json:"boundary"`
	Message  string `json:"message,omitempty"`
}

type compactResponse struct {
	Hash string `json:"hash"`
}

func (s *Server) handleCompact(w http.ResponseWriter, r *http.Request) {
	var req compactRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "decoding request body: %v", err)
		return
	}
	if req.Boundary == "" {
		writeError(w, http.StatusBadRequest, "\"boundary\" is required")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	hash, err := s.store.Compact(req.Boundary, req.Message)
	if err != nil {
		writeError(w, http.StatusBadRequest, "%v", err)
		return
	}
	p, err := s.store.Load()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "compacted but failed to reload state: %v", err)
		return
	}
	s.proj = p

	writeJSON(w, http.StatusOK, compactResponse{Hash: hash})
}
