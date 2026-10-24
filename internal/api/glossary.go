package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"elt/internal/project"
)

type glossaryTermRequest struct {
	Term        string `json:"term"`
	Action      string `json:"action"`
	Translation string `json:"translation,omitempty"`
	Inflect     bool   `json:"inflect,omitempty"`
	Status      string `json:"status,omitempty"`
	Notes       string `json:"notes,omitempty"`
}

func (r glossaryTermRequest) toProject() project.GlossaryTerm {
	return project.GlossaryTerm{
		Term: r.Term, Action: project.TermAction(r.Action), Translation: r.Translation,
		Inflect: r.Inflect, Status: project.TermStatus(r.Status), Notes: r.Notes,
	}
}

type glossaryKeyRefResponse struct {
	FileType string `json:"file_type"`
	Key      string `json:"key"`
}

type glossaryTermResponse struct {
	Term        string                   `json:"term"`
	Action      string                   `json:"action"`
	Translation string                   `json:"translation,omitempty"`
	Inflect     bool                     `json:"inflect,omitempty"`
	Status      string                   `json:"status"`
	Notes       string                   `json:"notes,omitempty"`
	MatchedKeys []glossaryKeyRefResponse `json:"matched_keys,omitempty"`
}

func termResponse(t project.GlossaryTerm) glossaryTermResponse {
	matched := make([]glossaryKeyRefResponse, 0, len(t.MatchedKeys))
	for _, ref := range t.MatchedKeys {
		matched = append(matched, glossaryKeyRefResponse{FileType: ref.FileType, Key: ref.Key})
	}
	return glossaryTermResponse{
		Term: t.Term, Action: string(t.Action), Translation: t.Translation,
		Inflect: t.Inflect, Status: string(t.Status), Notes: t.Notes,
		MatchedKeys: matched,
	}
}

func (s *Server) handleListGlossaryTerms(w http.ResponseWriter, r *http.Request) {
	lang := r.URL.Query().Get("lang")
	if lang == "" {
		writeError(w, http.StatusBadRequest, "?lang= is required")
		return
	}
	filter := project.GlossaryStatusFilter(r.URL.Query().Get("status"))

	s.mu.Lock()
	defer s.mu.Unlock()

	terms := project.FilterGlossaryTerms(project.GlossaryTerms(s.proj, lang), filter)
	resp := make([]glossaryTermResponse, 0, len(terms))
	for _, t := range terms {
		resp = append(resp, termResponse(t))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleAddGlossaryTerm(w http.ResponseWriter, r *http.Request) {
	lang := r.URL.Query().Get("lang")
	if lang == "" {
		writeError(w, http.StatusBadRequest, "?lang= is required")
		return
	}
	var req glossaryTermRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "decoding request body: %v", err)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := project.AddGlossaryTerm(s.proj, lang, req.toProject()); err != nil {
		writeError(w, http.StatusBadRequest, "%v", err)
		return
	}
	if err := s.store.Save(s.proj); err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleUpdateGlossaryTerm(w http.ResponseWriter, r *http.Request) {
	lang := r.URL.Query().Get("lang")
	if lang == "" {
		writeError(w, http.StatusBadRequest, "?lang= is required")
		return
	}
	term := r.PathValue("term")
	var req glossaryTermRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "decoding request body: %v", err)
		return
	}

	if err := s.checkLock(termLockID(lang, term), r); err != nil {
		writeError(w, http.StatusConflict, "%v", err)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := project.UpdateGlossaryTerm(s.proj, lang, term, req.toProject()); err != nil {
		writeError(w, http.StatusNotFound, "%v", err)
		return
	}
	if err := s.store.Save(s.proj); err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleRemoveGlossaryTerm(w http.ResponseWriter, r *http.Request) {
	lang := r.URL.Query().Get("lang")
	if lang == "" {
		writeError(w, http.StatusBadRequest, "?lang= is required")
		return
	}
	term := r.PathValue("term")

	if err := s.checkLock(termLockID(lang, term), r); err != nil {
		writeError(w, http.StatusConflict, "%v", err)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := project.RemoveGlossaryTerm(s.proj, lang, term); err != nil {
		writeError(w, http.StatusNotFound, "%v", err)
		return
	}
	if err := s.store.Save(s.proj); err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) setGlossaryTermStatus(w http.ResponseWriter, r *http.Request, status project.TermStatus) {
	lang := r.URL.Query().Get("lang")
	if lang == "" {
		writeError(w, http.StatusBadRequest, "?lang= is required")
		return
	}
	term := r.PathValue("term")

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := project.SetGlossaryTermStatus(s.proj, lang, term, status); err != nil {
		writeError(w, http.StatusNotFound, "%v", err)
		return
	}
	if err := s.store.Save(s.proj); err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleApproveGlossaryTerm(w http.ResponseWriter, r *http.Request) {
	s.setGlossaryTermStatus(w, r, project.TermApproved)
}

func (s *Server) handleUnapproveGlossaryTerm(w http.ResponseWriter, r *http.Request) {
	s.setGlossaryTermStatus(w, r, project.TermDraft)
}

func (s *Server) handleRejectGlossaryTerm(w http.ResponseWriter, r *http.Request) {
	s.setGlossaryTermStatus(w, r, project.TermRejected)
}

type glossaryImportRequest struct {
	Language string                `json:"language"`
	Terms    []glossaryTermRequest `json:"terms"`
}

type glossaryImportResponse struct {
	Added   int      `json:"added"`
	Updated int      `json:"updated"`
	Errors  []string `json:"errors,omitempty"`
}

func (s *Server) handleImportGlossaryTerms(w http.ResponseWriter, r *http.Request) {
	var req glossaryImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "decoding request body: %v", err)
		return
	}
	if req.Language == "" {
		writeError(w, http.StatusBadRequest, `"language" is required in the request body`)
		return
	}
	terms := make([]project.GlossaryTerm, len(req.Terms))
	for i, t := range req.Terms {
		terms[i] = t.toProject()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	added, updated, errs := project.ImportGlossaryTerms(s.proj, req.Language, terms)
	if added > 0 || updated > 0 {
		if err := s.store.Save(s.proj); err != nil {
			writeError(w, http.StatusInternalServerError, "%v", err)
			return
		}
	}
	writeJSON(w, http.StatusOK, glossaryImportResponse{Added: added, Updated: updated, Errors: errs})
}

type glossaryClearResponse struct {
	Removed int `json:"removed"`
}

func (s *Server) handleClearGlossary(w http.ResponseWriter, r *http.Request) {
	lang := r.URL.Query().Get("lang")
	if lang == "" {
		writeError(w, http.StatusBadRequest, "?lang= is required")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.commit(s.proj, r, fmt.Sprintf("checkpoint before clearing %s glossary", lang)); err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}

	removed := project.ClearGlossary(s.proj, lang)
	if err := s.commit(s.proj, r, fmt.Sprintf("clear %s glossary (%d term(s) removed)", lang, removed)); err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	writeJSON(w, http.StatusOK, glossaryClearResponse{Removed: removed})
}

type termUsageResponse struct {
	FileType string `json:"file_type"`
	Key      string `json:"key"`
	English  string `json:"english"`
}

func (s *Server) handleGlossaryTermUsage(w http.ResponseWriter, r *http.Request) {
	term := r.PathValue("term")
	if term == "" {
		writeError(w, http.StatusBadRequest, "term is required")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	usage := project.TermUsage(s.proj, term)
	resp := make([]termUsageResponse, 0, len(usage))
	for _, u := range usage {
		resp = append(resp, termUsageResponse{FileType: u.FileType, Key: u.Key, English: u.English})
	}
	writeJSON(w, http.StatusOK, resp)
}

type glossaryFillResponse struct {
	TermsUpdated int `json:"terms_updated"`
	TotalMatches int `json:"total_matches"`
}

func (s *Server) handleFillGlossaryMatches(w http.ResponseWriter, r *http.Request) {
	lang := r.URL.Query().Get("lang")
	if lang == "" {
		writeError(w, http.StatusBadRequest, "?lang= is required")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	termsUpdated, totalMatches := project.FillGlossaryMatches(s.proj, lang)
	if err := s.store.Save(s.proj); err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	writeJSON(w, http.StatusOK, glossaryFillResponse{TermsUpdated: termsUpdated, TotalMatches: totalMatches})
}

type glossaryPruneResponse struct {
	Removed int                  `json:"removed"`
	Term    glossaryTermResponse `json:"term"`
}

func (s *Server) handlePruneGlossaryMatches(w http.ResponseWriter, r *http.Request) {
	lang := r.URL.Query().Get("lang")
	if lang == "" {
		writeError(w, http.StatusBadRequest, "?lang= is required")
		return
	}
	term := r.PathValue("term")

	s.mu.Lock()
	defer s.mu.Unlock()

	updated, removed, err := project.PruneStaleGlossaryMatches(s.proj, lang, term)
	if err != nil {
		writeError(w, http.StatusNotFound, "%v", err)
		return
	}
	if removed > 0 {
		if err := s.store.Save(s.proj); err != nil {
			writeError(w, http.StatusInternalServerError, "%v", err)
			return
		}
	}
	writeJSON(w, http.StatusOK, glossaryPruneResponse{Removed: removed, Term: termResponse(updated)})
}
