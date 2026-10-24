package api

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type addLanguageRequest struct {
	Language string `json:"language"`
}

func (s *Server) handleAddLanguage(w http.ResponseWriter, r *http.Request) {
	var req addLanguageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "decoding request body: %v", err)
		return
	}
	if req.Language == "" {
		writeError(w, http.StatusBadRequest, "language is required")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.proj.AddLanguage(req.Language)
	if err := s.commit(s.proj, r, fmt.Sprintf("add language %s", req.Language)); err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleRemoveLanguage(w http.ResponseWriter, r *http.Request) {
	lang := r.PathValue("lang")
	if lang == "" {
		writeError(w, http.StatusBadRequest, "language is required")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.proj.RemoveLanguage(lang)
	if err := s.commit(s.proj, r, fmt.Sprintf("remove language %s", lang)); err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
