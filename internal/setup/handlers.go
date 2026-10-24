package setup

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"elt/internal/csvio"
	"elt/internal/localize"
	"elt/internal/store"
)

const maxUploadBodyBytes = 32 * 1024 * 1024

func (s *Session) Handler(fallback http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /setup/state", s.handleState)
	mux.HandleFunc("GET /setup/browse", s.handleBrowse)
	mux.HandleFunc("POST /setup/directory", s.handleDirectory)
	mux.HandleFunc("POST /setup/open-existing", s.handleOpenExisting)
	mux.HandleFunc("POST /setup/local", s.handleLocal)
	mux.HandleFunc("POST /setup/remote", s.handleRemote)
	mux.HandleFunc("POST /setup/upload/{type}", s.handleUpload)
	mux.HandleFunc("POST /setup/upload/{type}/skip", s.handleUploadSkip)
	mux.HandleFunc("POST /setup/languages", s.handleLanguages)
	if fallback != nil {
		mux.Handle("/", fallback)
	}
	return mux
}

type stateResponse struct {
	Step                string   `json:"step"`
	Directory           string   `json:"directory,omitempty"`
	DirectoryStatus     string   `json:"directory_status,omitempty"`
	NextUpload          string   `json:"next_upload,omitempty"`
	KnownLanguages      []string `json:"known_languages"`
	SuggestedAuthorName string   `json:"suggested_author_name,omitempty"`
	Warning             string   `json:"warning,omitempty"`
}

func (s *Session) handleState(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	resp := stateResponse{
		Step:                s.step(),
		Directory:           s.dir,
		NextUpload:          string(s.nextUpload()),
		KnownLanguages:      s.knownLanguagesFor(),
		SuggestedAuthorName: suggestedAuthorName(),
		Warning:             s.warning,
	}
	if s.dir != "" {
		resp.DirectoryStatus = string(s.dirStatus)
	}
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, resp)
}

type browseEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type browseResponse struct {
	Path         string        `json:"path"`
	Parent       string        `json:"parent,omitempty"`
	Entries      []browseEntry `json:"entries"`
	Truncated    bool          `json:"truncated,omitempty"`
	IsProjectDir bool          `json:"is_project_dir"`

	Roots []string `json:"roots,omitempty"`
}

func availableRootsForResponse() []string {
	if runtime.GOOS != "windows" {
		return nil
	}
	return availableRoots(driveRootExists)
}

func driveRootExists(root string) bool {
	info, err := os.Stat(root)
	return err == nil && info.IsDir()
}

func availableRoots(exists func(string) bool) []string {
	var roots []string
	for c := 'A'; c <= 'Z'; c++ {
		root := string(c) + `:\`
		if exists(root) {
			roots = append(roots, root)
		}
	}
	return roots
}

const maxBrowseEntries = 300

func (s *Session) handleBrowse(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "%v", err)
			return
		}
		path = home
	}
	path = filepath.Clean(path)
	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))

	info, err := os.Stat(path)
	if err != nil {
		writeError(w, http.StatusBadRequest, "%v", err)
		return
	}
	if !info.IsDir() {
		writeError(w, http.StatusBadRequest, "%s is not a directory", path)
		return
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		writeError(w, http.StatusBadRequest, "%v", err)
		return
	}
	out := make([]browseEntry, 0, len(entries))
	truncated := false
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(e.Name()), query) {
			continue
		}
		if len(out) >= maxBrowseEntries {
			truncated = true
			break
		}
		out = append(out, browseEntry{Name: e.Name(), Path: filepath.Join(path, e.Name())})
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })

	isProject, err := store.LooksLikeProjectDir(path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}

	parent := filepath.Dir(path)
	if parent == path {
		parent = ""
	}
	writeJSON(w, http.StatusOK, browseResponse{Path: path, Parent: parent, Entries: out, Truncated: truncated, IsProjectDir: isProject, Roots: availableRootsForResponse()})
}

type directoryRequest struct {
	Path   string `json:"path"`
	Create bool   `json:"create"`
}

func (s *Session) handleDirectory(w http.ResponseWriter, r *http.Request) {
	var req directoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "decoding request body: %v", err)
		return
	}
	if req.Path == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}
	status, err := s.chooseDirectory(filepath.Clean(req.Path), req.Create)
	if err != nil {
		writeError(w, http.StatusBadRequest, "%v", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": string(status)})
}

func (s *Session) handleOpenExisting(w http.ResponseWriter, r *http.Request) {
	if err := s.openExisting(); err != nil {
		writeError(w, http.StatusBadRequest, "%v", err)
		return
	}

	<-s.HandedOff()
	writeJSON(w, http.StatusOK, map[string]string{"step": "done"})
}

type localRequest struct {
	ProjectName string `json:"project_name"`
	AuthorName  string `json:"author_name"`
	AuthorEmail string `json:"author_email"`
}

func (s *Session) handleLocal(w http.ResponseWriter, r *http.Request) {
	var req localRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "decoding request body: %v", err)
		return
	}
	if err := s.openLocal(req.ProjectName, authorInput{Name: req.AuthorName, Email: req.AuthorEmail}); err != nil {
		writeError(w, http.StatusBadRequest, "%v", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"step": s.currentStep()})
}

type remoteRequest struct {
	ProjectName string `json:"project_name"`
	URL         string `json:"url"`
	Username    string `json:"username"`
	Token       string `json:"token"`
	Push        string `json:"push"`
	AuthorName  string `json:"author_name"`
	AuthorEmail string `json:"author_email"`
}

type remoteResponse struct {
	Step           string `json:"step"`
	JoinedExisting bool   `json:"joined_existing"`
}

func (s *Session) handleRemote(w http.ResponseWriter, r *http.Request) {
	var req remoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "decoding request body: %v", err)
		return
	}
	if req.URL == "" {
		writeError(w, http.StatusBadRequest, "url is required")
		return
	}
	cfg := store.RemoteConfig{URL: req.URL, Username: req.Username, Token: req.Token, Push: req.Push}

	result, err := s.openRemote(req.ProjectName, cfg, authorInput{Name: req.AuthorName, Email: req.AuthorEmail})
	if err != nil {
		writeError(w, http.StatusBadRequest, "%v", err)
		return
	}
	if result.JoinedExisting {

		<-s.HandedOff()
	}
	writeJSON(w, http.StatusOK, remoteResponse{Step: s.currentStep(), JoinedExisting: result.JoinedExisting})
}

func fileTypeFromRequest(r *http.Request) (localize.FileType, bool) {
	ft := localize.FileType(r.PathValue("type"))
	return ft, ft.Valid()
}

type uploadResponse struct {
	Step           string        `json:"step,omitempty"`
	Issues         []csvio.Issue `json:"issues,omitempty"`
	LanguagesFound []string      `json:"languages_found,omitempty"`
}

func (s *Session) handleUpload(w http.ResponseWriter, r *http.Request) {
	ft, ok := fileTypeFromRequest(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown file type %q", r.PathValue("type"))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body exceeds %d bytes", maxUploadBodyBytes)
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
				writeJSON(w, http.StatusConflict, uploadResponse{Issues: result.Issues})
				return
			}
		}
	}

	if err := s.uploadFile(ft, result.Table, result.Issues); err != nil {
		writeError(w, http.StatusBadRequest, "%v", err)
		return
	}
	writeJSON(w, http.StatusOK, uploadResponse{
		Step:           s.currentStep(),
		Issues:         result.Issues,
		LanguagesFound: filterTargetLanguages(result.Table.Header.Languages),
	})
}

func (s *Session) handleUploadSkip(w http.ResponseWriter, r *http.Request) {
	ft, ok := fileTypeFromRequest(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown file type %q", r.PathValue("type"))
		return
	}
	if err := s.skipFile(ft); err != nil {
		writeError(w, http.StatusBadRequest, "%v", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"step": s.currentStep()})
}

type languagesRequest struct {
	Languages []string `json:"languages"`
}

func (s *Session) handleLanguages(w http.ResponseWriter, r *http.Request) {
	var req languagesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "decoding request body: %v", err)
		return
	}
	if err := s.finishLanguages(req.Languages); err != nil {
		writeError(w, http.StatusBadRequest, "%v", err)
		return
	}

	<-s.HandedOff()
	writeJSON(w, http.StatusOK, map[string]string{"step": "done"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type errorResponse struct {
	Error string `json:"error"`
}

func writeError(w http.ResponseWriter, status int, format string, args ...any) {
	writeJSON(w, status, errorResponse{Error: fmt.Sprintf(format, args...)})
}
