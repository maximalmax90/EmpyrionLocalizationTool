package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"elt/internal/localize"
	"elt/internal/project"
)

type KeysResponse struct {
	Items []project.KeyListItem `json:"items"`
	Total int                   `json:"total"`
}

func (s *Server) handleListKeys(w http.ResponseWriter, r *http.Request) {
	ft, ok := fileTypeFromRequest(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown file type %q", r.PathValue("type"))
		return
	}
	q := r.URL.Query()
	lang := q.Get("lang")
	if lang == "" {
		writeError(w, http.StatusBadRequest, "?lang= is required")
		return
	}
	filter := project.KeyFilter(q.Get("status"))
	search := q.Get("search")
	offset, err := intParam(q, "offset", 0)
	if err != nil {
		writeError(w, http.StatusBadRequest, "?offset= must be an integer")
		return
	}
	limit, err := intParam(q, "limit", 100)
	if err != nil {
		writeError(w, http.StatusBadRequest, "?limit= must be an integer")
		return
	}

	var onlyKeys map[string]bool
	if raw := q.Get("keys"); raw != "" {
		onlyKeys = map[string]bool{}
		for _, k := range strings.Split(raw, ",") {
			if k != "" {
				onlyKeys[k] = true
			}
		}
	}

	s.respondListKeys(w, ft, lang, filter, search, onlyKeys, offset, limit)
}

type listKeysRequest struct {
	Lang   string   `json:"lang"`
	Status string   `json:"status"`
	Search string   `json:"search"`
	Offset int      `json:"offset"`
	Limit  int      `json:"limit"`
	Keys   []string `json:"keys"`
}

func (s *Server) handleListKeysPost(w http.ResponseWriter, r *http.Request) {
	ft, ok := fileTypeFromRequest(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown file type %q", r.PathValue("type"))
		return
	}
	var req listKeysRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "decoding request body: %s", err.Error())
		return
	}
	if req.Lang == "" {
		writeError(w, http.StatusBadRequest, `"lang" is required`)
		return
	}
	limit := req.Limit
	if limit == 0 {
		limit = 100
	}
	var onlyKeys map[string]bool
	if len(req.Keys) > 0 {
		onlyKeys = map[string]bool{}
		for _, k := range req.Keys {
			if k != "" {
				onlyKeys[k] = true
			}
		}
	}
	s.respondListKeys(w, ft, req.Lang, project.KeyFilter(req.Status), req.Search, onlyKeys, req.Offset, limit)
}

func (s *Server) respondListKeys(w http.ResponseWriter, ft localize.FileType, lang string, filter project.KeyFilter, search string, onlyKeys map[string]bool, offset, limit int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	track, ok := s.track(ft)
	if !ok {
		writeError(w, http.StatusBadRequest, "%s is not tracked", ft)
		return
	}

	items, total := project.ListKeys(track, lang, filter, search, onlyKeys, offset, limit)
	writeJSON(w, http.StatusOK, KeysResponse{Items: items, Total: total})
}

func intParam(q map[string][]string, name string, def int) (int, error) {
	raw := q[name]
	if len(raw) == 0 || raw[0] == "" {
		return def, nil
	}
	return strconv.Atoi(raw[0])
}

func (s *Server) handleDiff(w http.ResponseWriter, r *http.Request) {
	ft, ok := fileTypeFromRequest(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown file type %q", r.PathValue("type"))
		return
	}
	from := r.URL.Query().Get("from")
	if from == "" {
		writeError(w, http.StatusBadRequest, "?from= is required")
		return
	}
	to := r.URL.Query().Get("to")
	lang := r.URL.Query().Get("lang")

	s.mu.Lock()
	defer s.mu.Unlock()

	fromProj, err := s.store.LoadAt(from)
	if err != nil {
		writeError(w, http.StatusBadRequest, "%v", err)
		return
	}
	fromTrack := fromProj.Files[ft]

	var toTrack *project.FileTrack
	if to == "" {
		toTrack, ok = s.track(ft)
		if !ok {
			writeError(w, http.StatusBadRequest, "%s is not tracked", ft)
			return
		}
	} else {
		toProj, err := s.store.LoadAt(to)
		if err != nil {
			writeError(w, http.StatusBadRequest, "%v", err)
			return
		}
		toTrack = toProj.Files[ft]
	}

	writeJSON(w, http.StatusOK, project.DiffFileTracks(fromTrack, toTrack, lang))
}

type clearTranslationsRequest struct {
	Lang string   `json:"lang"`
	Keys []string `json:"keys"`
}

type ClearTranslationsResponse struct {
	Cleared []string          `json:"cleared"`
	Errors  map[string]string `json:"errors,omitempty"`
}

func (s *Server) handleClearTranslations(w http.ResponseWriter, r *http.Request) {
	ft, ok := fileTypeFromRequest(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown file type %q", r.PathValue("type"))
		return
	}
	var req clearTranslationsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "decoding request body: %v", err)
		return
	}
	if req.Lang == "" {
		writeError(w, http.StatusBadRequest, "\"lang\" is required")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	track, ok := s.track(ft)
	if !ok {
		writeError(w, http.StatusBadRequest, "%s is not tracked", ft)
		return
	}

	resp := ClearTranslationsResponse{Cleared: make([]string, 0, len(req.Keys))}
	for _, key := range req.Keys {
		if err := project.ClearTranslation(track, key, req.Lang); err != nil {
			if resp.Errors == nil {
				resp.Errors = map[string]string{}
			}
			resp.Errors[key] = err.Error()
			continue
		}
		resp.Cleared = append(resp.Cleared, key)
	}

	if len(resp.Cleared) > 0 {
		if err := s.store.Save(s.proj); err != nil {
			writeError(w, http.StatusInternalServerError, "%v", err)
			return
		}
	}
	writeJSON(w, http.StatusOK, resp)
}
