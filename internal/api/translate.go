package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"elt/internal/project"
	"elt/internal/validate"
)

type setTranslationRequest struct {
	Text   string `json:"text"`
	Source string `json:"source"`
}

func (s *Server) handleSetTranslation(w http.ResponseWriter, r *http.Request) {
	ft, ok := fileTypeFromRequest(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown file type %q", r.PathValue("type"))
		return
	}
	key := r.PathValue("key")
	lang := r.PathValue("lang")

	var req setTranslationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "decoding request body: %v", err)
		return
	}
	if req.Source == "" {
		req.Source = "manual"
	}

	if err := s.checkLock(keyLockID(ft, lang, key), r); err != nil {
		writeError(w, http.StatusConflict, "%v", err)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	track, ok := s.track(ft)
	if !ok {
		writeError(w, http.StatusBadRequest, "%s is not tracked", ft)
		return
	}
	if err := project.SetTranslation(track, key, lang, req.Text, req.Source); err != nil {
		writeError(w, http.StatusNotFound, "%v", err)
		return
	}
	if err := s.store.Save(s.proj); err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}

	writeJSON(w, http.StatusOK, track.Keys[key].Translations[lang])
}

func (s *Server) handleConfirmTranslation(w http.ResponseWriter, r *http.Request) {
	ft, ok := fileTypeFromRequest(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown file type %q", r.PathValue("type"))
		return
	}
	key := r.PathValue("key")
	lang := r.PathValue("lang")

	if err := s.checkLock(keyLockID(ft, lang, key), r); err != nil {
		writeError(w, http.StatusConflict, "%v", err)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	track, ok := s.track(ft)
	if !ok {
		writeError(w, http.StatusBadRequest, "%s is not tracked", ft)
		return
	}
	if err := project.ConfirmTranslation(track, key, lang); err != nil {
		writeError(w, http.StatusNotFound, "%v", err)
		return
	}
	if err := s.store.Save(s.proj); err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}

	writeJSON(w, http.StatusOK, track.Keys[key].Translations[lang])
}

type setNeedsReviewRequest struct {
	NeedsReview bool `json:"needs_review"`
}

func (s *Server) handleSetNeedsReview(w http.ResponseWriter, r *http.Request) {
	ft, ok := fileTypeFromRequest(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown file type %q", r.PathValue("type"))
		return
	}
	key := r.PathValue("key")
	lang := r.PathValue("lang")

	var req setNeedsReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "decoding request body: %v", err)
		return
	}

	if err := s.checkLock(keyLockID(ft, lang, key), r); err != nil {
		writeError(w, http.StatusConflict, "%v", err)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	track, ok := s.track(ft)
	if !ok {
		writeError(w, http.StatusBadRequest, "%s is not tracked", ft)
		return
	}
	if err := project.SetNeedsReview(track, key, lang, req.NeedsReview); err != nil {
		writeError(w, http.StatusNotFound, "%v", err)
		return
	}
	if err := s.store.Save(s.proj); err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}

	writeJSON(w, http.StatusOK, track.Keys[key].Translations[lang])
}

type ValidateResponse struct {
	Issues map[string][]string `json:"issues"`
}

func (s *Server) handleValidate(w http.ResponseWriter, r *http.Request) {
	ft, ok := fileTypeFromRequest(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown file type %q", r.PathValue("type"))
		return
	}
	lang := r.URL.Query().Get("lang")
	if lang == "" {
		writeError(w, http.StatusBadRequest, "?lang= is required")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	track, ok := s.track(ft)
	if !ok {
		writeError(w, http.StatusBadRequest, "%s is not tracked", ft)
		return
	}

	policy := validate.DefaultPolicy(ft)
	issues := project.Validate(track, lang, policy)

	msg := fmt.Sprintf("validate %s (%s): %d issue(s)", ft, lang, len(issues))
	if err := s.commit(s.proj, r, msg); err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}

	writeJSON(w, http.StatusOK, ValidateResponse{Issues: issues})
}
