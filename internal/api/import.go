package api

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"elt/internal/csvio"
	"elt/internal/project"
)

type ImportResponse struct {
	Applied                map[string][]string `json:"applied"`
	SkippedUnknownKey      []string            `json:"skipped_unknown_key"`
	SkippedEnglishMismatch []string            `json:"skipped_english_mismatch"`
	Issues                 []csvio.Issue       `json:"issues,omitempty"`
}

func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	ft, ok := fileTypeFromRequest(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown file type %q", r.PathValue("type"))
		return
	}

	q := r.URL.Query()
	rawLangs := q.Get("langs")
	if rawLangs == "" {
		writeError(w, http.StatusBadRequest, "?langs= is required")
		return
	}
	langs := strings.Split(rawLangs, ",")
	matchEnglish := q.Get("match_english") != "false"

	maxBody := s.maxSyncBodyBytes
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body exceeds %d bytes", maxBody)
			return
		}
		writeError(w, http.StatusBadRequest, "reading request body: %v", err)
		return
	}
	result, err := csvio.Decompile(bytes.NewReader(body))
	if err != nil {
		writeError(w, http.StatusBadRequest, "parsing CSV: %v", err)
		return
	}

	force := q.Get("force") == "true"
	if !force {
		for _, iss := range result.Issues {
			if iss.Severity == csvio.SeverityError {
				writeJSON(w, http.StatusConflict, ImportResponse{Issues: result.Issues})
				return
			}
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	track, ok := s.track(ft)
	if !ok {
		writeError(w, http.StatusBadRequest, "%s is not tracked yet; POST /files/%s/track first", ft, ft)
		return
	}

	for _, lang := range langs {
		tracked := false
		for _, l := range track.Languages {
			if l == lang {
				tracked = true
				break
			}
		}
		if !tracked {
			writeError(w, http.StatusBadRequest, "%q is not a tracked language for %s", lang, ft)
			return
		}
	}

	importResult := project.Import(track, result.Table, langs, matchEnglish)

	totalApplied := 0
	for _, keys := range importResult.Applied {
		totalApplied += len(keys)
	}
	msg := fmt.Sprintf("import %s translations for %s from CSV (%d applied, %d unknown key, %d english mismatch)",
		ft, strings.Join(langs, ", "), totalApplied, len(importResult.SkippedUnknownKey), len(importResult.SkippedEnglishMismatch))
	if err := s.commit(s.proj, r, msg); err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}

	writeJSON(w, http.StatusOK, ImportResponse{
		Applied:                importResult.Applied,
		SkippedUnknownKey:      importResult.SkippedUnknownKey,
		SkippedEnglishMismatch: importResult.SkippedEnglishMismatch,
		Issues:                 result.Issues,
	})
}
