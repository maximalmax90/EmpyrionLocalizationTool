package api

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"elt/internal/csvio"
	"elt/internal/localize"
	"elt/internal/project"
)

func (s *Server) handleTrackFile(w http.ResponseWriter, r *http.Request) {
	ft, ok := fileTypeFromRequest(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown file type %q", r.PathValue("type"))
		return
	}
	var langs []string
	if raw := r.URL.Query().Get("langs"); raw != "" {
		langs = strings.Split(raw, ",")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	_, alreadyTracked := s.proj.Files[ft]
	track := s.proj.TrackFile(ft, langs)
	if !alreadyTracked {
		if err := s.commit(s.proj, r, fmt.Sprintf("track %s (%s)", ft, strings.Join(langs, ", "))); err != nil {
			writeError(w, http.StatusInternalServerError, "%v", err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"file_type": ft, "languages": track.Languages})
}

type SyncResponse struct {
	Added     []string      `json:"added"`
	Changed   []string      `json:"changed"`
	Removed   []string      `json:"removed"`
	Unchanged int           `json:"unchanged"`
	Issues    []csvio.Issue `json:"issues,omitempty"`

	ChainSourceCleared bool `json:"chain_source_cleared,omitempty"`
}

func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	ft, ok := fileTypeFromRequest(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown file type %q", r.PathValue("type"))
		return
	}

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

	force := r.URL.Query().Get("force") == "true"
	if !force {
		for _, iss := range result.Issues {
			if iss.Severity == csvio.SeverityError {
				writeJSON(w, http.StatusConflict, SyncResponse{Issues: result.Issues})
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

	syncResult := project.Sync(track, result.Table)

	changedAnything := len(syncResult.Added) > 0 || len(syncResult.Changed) > 0 || len(syncResult.Removed) > 0
	chainSourceCleared := ft.SupportsChains() && changedAnything && project.HasChainSource(s.proj, ft)
	if chainSourceCleared {
		project.ClearChainSource(s.proj, ft)
	}

	msg := fmt.Sprintf("sync %s: +%d ~%d -%d", ft, len(syncResult.Added), len(syncResult.Changed), len(syncResult.Removed))
	if chainSourceCleared {
		msg += " (chain source reset — re-upload needed)"
	}
	if err := s.commit(s.proj, r, msg); err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}

	writeJSON(w, http.StatusOK, SyncResponse{
		Added:              syncResult.Added,
		Changed:            syncResult.Changed,
		Removed:            syncResult.Removed,
		Unchanged:          syncResult.Unchanged,
		Issues:             result.Issues,
		ChainSourceCleared: chainSourceCleared,
	})
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	ft, ok := fileTypeFromRequest(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown file type %q", r.PathValue("type"))
		return
	}
	rev := r.URL.Query().Get("rev")

	var mergeFrom localize.FileType
	if raw := r.URL.Query().Get("merge_from"); raw != "" {
		mergeFrom = localize.FileType(raw)
		if !mergeFrom.Valid() {
			writeError(w, http.StatusBadRequest, "unknown merge_from file type %q", raw)
			return
		}
		if rev != "" {
			writeError(w, http.StatusBadRequest, "merge_from is not supported together with rev")
			return
		}
	}

	s.mu.Lock()
	var track, fallback *project.FileTrack
	if rev == "" {
		track, ok = s.track(ft)
	} else {
		var historical *project.Project
		historical, err := s.store.LoadAt(rev)
		if err != nil {
			s.mu.Unlock()
			writeError(w, http.StatusBadRequest, "%v", err)
			return
		}
		track, ok = historical.Files[ft]
	}
	if !ok {
		s.mu.Unlock()
		writeError(w, http.StatusNotFound, "%s is not tracked%s", ft, revSuffix(rev))
		return
	}
	if mergeFrom != "" {
		fallback, ok = s.track(mergeFrom)
		if !ok {
			s.mu.Unlock()
			writeError(w, http.StatusBadRequest, "%s is not tracked; cannot merge_from it", mergeFrom)
			return
		}
	}
	var table localize.Table
	if fallback != nil {
		table = project.ExportMerged(track, fallback)
	} else {
		table = project.Export(track)
	}
	s.mu.Unlock()

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, ft.ExportFilename()))
	w.Header().Set("Cache-Control", "no-store")
	_ = csvio.Compile(w, table)
}

func revSuffix(rev string) string {
	if rev == "" {
		return ""
	}
	return fmt.Sprintf(" at revision %s", rev)
}
