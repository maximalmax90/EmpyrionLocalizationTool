package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"elt/internal/store"
)

type commitRequest struct {
	Message string `json:"message"`
}

func (s *Server) handleCommit(w http.ResponseWriter, r *http.Request) {
	var req commitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "decoding request body: %v", err)
		return
	}
	if req.Message == "" {
		req.Message = "manual checkpoint"
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.commitOnly(r, req.Message); err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}

	hist, err := s.store.History(1)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	writeJSON(w, http.StatusOK, hist)
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			writeError(w, http.StatusBadRequest, "?limit= must be a non-negative integer")
			return
		}
		limit = n
	}

	hist, err := s.storeHistory(limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	writeJSON(w, http.StatusOK, hist)
}

func (s *Server) storeHistory(limit int) ([]store.CommitInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.store.History(limit)
}
