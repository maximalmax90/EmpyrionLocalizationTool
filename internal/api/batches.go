package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"elt/internal/project"
)

type BatchSummary struct {
	ID         string    `json:"id"`
	Lang       string    `json:"lang"`
	CreatedAt  time.Time `json:"created_at"`
	KeyCount   int       `json:"key_count"`
	AssignedTo string    `json:"assigned_to,omitempty"`
}

func summarize(b *project.Batch) BatchSummary {
	return BatchSummary{ID: b.ID, Lang: b.Lang, CreatedAt: b.CreatedAt, KeyCount: len(b.Keys), AssignedTo: b.AssignedTo}
}

func (s *Server) handleSplit(w http.ResponseWriter, r *http.Request) {
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
	n, err := strconv.Atoi(r.URL.Query().Get("n"))
	if err != nil || n < 1 {
		writeError(w, http.StatusBadRequest, "?n= must be a positive integer")
		return
	}
	onlyNeedsWork := r.URL.Query().Get("only_needs_work") != "false"

	s.mu.Lock()
	defer s.mu.Unlock()

	track, ok := s.track(ft)
	if !ok {
		writeError(w, http.StatusBadRequest, "%s is not tracked", ft)
		return
	}

	batches, err := project.Split(track, lang, n, onlyNeedsWork)
	if err != nil {
		writeError(w, http.StatusBadRequest, "%v", err)
		return
	}

	if len(batches) > 0 {
		msg := fmt.Sprintf("split %s (%s) into %d batch(es)", ft, lang, len(batches))
		if err := s.commit(s.proj, r, msg); err != nil {
			writeError(w, http.StatusInternalServerError, "%v", err)
			return
		}
	}

	summaries := make([]BatchSummary, 0, len(batches))
	for _, b := range batches {
		summaries = append(summaries, summarize(b))
	}
	writeJSON(w, http.StatusOK, summaries)
}

func (s *Server) handleListBatches(w http.ResponseWriter, r *http.Request) {
	ft, ok := fileTypeFromRequest(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown file type %q", r.PathValue("type"))
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	track, ok := s.track(ft)
	if !ok {
		writeError(w, http.StatusBadRequest, "%s is not tracked", ft)
		return
	}
	summaries := make([]BatchSummary, 0, len(track.OpenBatches))
	for _, b := range track.OpenBatches {
		summaries = append(summaries, summarize(b))
	}
	writeJSON(w, http.StatusOK, summaries)
}

func (s *Server) handleGetBatch(w http.ResponseWriter, r *http.Request) {
	ft, ok := fileTypeFromRequest(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown file type %q", r.PathValue("type"))
		return
	}
	id := r.PathValue("id")

	s.mu.Lock()
	track, ok := s.track(ft)
	if !ok {
		s.mu.Unlock()
		writeError(w, http.StatusBadRequest, "%s is not tracked", ft)
		return
	}
	batch, ok := track.OpenBatches[id]
	if !ok {
		s.mu.Unlock()
		writeError(w, http.StatusNotFound, "batch %q is not open", id)
		return
	}
	keys := batch.Keys
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="batch-%s.json"`, id))
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(keys)
}

type mergeBatchRequest map[string]string

func (s *Server) handleMergeBatch(w http.ResponseWriter, r *http.Request) {
	ft, ok := fileTypeFromRequest(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown file type %q", r.PathValue("type"))
		return
	}
	id := r.PathValue("id")

	var req mergeBatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "decoding request body: %v", err)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	track, ok := s.track(ft)
	if !ok {
		writeError(w, http.StatusBadRequest, "%s is not tracked", ft)
		return
	}

	res, err := project.MergeBatch(track, id, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "%v", err)
		return
	}

	msg := fmt.Sprintf("merge batch %s (%d applied, %d skipped)", id, len(res.Applied), len(res.Skipped))
	if err := s.commit(s.proj, r, msg); err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

type assignBatchRequest struct {
	AssignedTo string `json:"assigned_to"`
}

func (s *Server) handleAssignBatch(w http.ResponseWriter, r *http.Request) {
	ft, ok := fileTypeFromRequest(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown file type %q", r.PathValue("type"))
		return
	}
	id := r.PathValue("id")

	var req assignBatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "decoding request body: %v", err)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	track, ok := s.track(ft)
	if !ok {
		writeError(w, http.StatusBadRequest, "%s is not tracked", ft)
		return
	}
	if user, authed := RequestUser(r); authed && !user.IsAdmin {
		batch, exists := track.OpenBatches[id]
		if !exists {
			writeError(w, http.StatusNotFound, "batch %q is not open", id)
			return
		}
		if req.AssignedTo != "" || batch.AssignedTo != user.Name {
			writeError(w, http.StatusForbidden, "only a Maintainer can assign a batch — you can only give up your own assignment")
			return
		}
	}
	if err := project.AssignBatch(track, id, req.AssignedTo); err != nil {
		writeError(w, http.StatusNotFound, "%v", err)
		return
	}
	msg := fmt.Sprintf("assign batch %s to %q", id, req.AssignedTo)
	if req.AssignedTo == "" {
		msg = fmt.Sprintf("unassign batch %s", id)
	}
	if err := s.commit(s.proj, r, msg); err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	writeJSON(w, http.StatusOK, summarize(track.OpenBatches[id]))
}

func (s *Server) handleReclaimBatch(w http.ResponseWriter, r *http.Request) {
	ft, ok := fileTypeFromRequest(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown file type %q", r.PathValue("type"))
		return
	}
	id := r.PathValue("id")

	s.mu.Lock()
	defer s.mu.Unlock()

	track, ok := s.track(ft)
	if !ok {
		writeError(w, http.StatusBadRequest, "%s is not tracked", ft)
		return
	}
	if err := project.Reclaim(track, id); err != nil {
		writeError(w, http.StatusNotFound, "%v", err)
		return
	}
	if err := s.commit(s.proj, r, fmt.Sprintf("reclaim batch %s", id)); err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
