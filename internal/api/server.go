package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"elt/internal/localize"
	"elt/internal/project"
	"elt/internal/store"
)

const DefaultMaxSyncBodyBytes = 32 * 1024 * 1024

type Server struct {
	mu               sync.Mutex
	store            *store.Store
	proj             *project.Project
	maxSyncBodyBytes int64

	loopbackOnly bool

	listenConfig ListenConfigHooks

	aiJobsMu sync.Mutex
	aiJobs   map[string]*aiJob

	locks lockRegistry

	aiQueue aiQueue
}

func NewServer(st *store.Store) (*Server, error) {
	p, err := st.Load()
	if err != nil {
		return nil, err
	}
	return &Server{store: st, proj: p, maxSyncBodyBytes: DefaultMaxSyncBodyBytes}, nil
}

func (s *Server) SetMaxSyncBodyBytes(n int64) {
	s.maxSyncBodyBytes = n
}

func (s *Server) SetLoopbackOnly(v bool) {
	s.loopbackOnly = v
}

type ListenConfigHooks struct {
	Get func() (string, error)
	Set func(addr string) error
}

func (s *Server) SetListenConfigHooks(hooks ListenConfigHooks) {
	s.listenConfig = hooks
}

func (s *Server) Handler(fallback http.Handler) http.Handler {
	mux := http.NewServeMux()

	auth := s.requireToken

	projectAdmin := s.requireProjectAdmin

	mux.HandleFunc("GET /status", auth(s.handleStatus))
	mux.HandleFunc("GET /me", auth(s.handleMe))
	mux.HandleFunc("GET /users", auth(s.handleListUserNames))

	mux.HandleFunc("POST /languages", projectAdmin(s.handleAddLanguage))
	mux.HandleFunc("DELETE /languages/{lang}", projectAdmin(s.handleRemoveLanguage))

	mux.HandleFunc("POST /files/{type}/track", projectAdmin(s.handleTrackFile))
	mux.HandleFunc("POST /files/{type}/sync", projectAdmin(s.handleSync))
	mux.HandleFunc("POST /files/{type}/import", projectAdmin(s.handleImport))
	mux.HandleFunc("GET /files/{type}/export", auth(s.handleExport))

	mux.HandleFunc("GET /files/{type}/keys", auth(s.handleListKeys))
	mux.HandleFunc("POST /files/{type}/keys", auth(s.handleListKeysPost))
	mux.HandleFunc("GET /files/{type}/diff", auth(s.handleDiff))
	mux.HandleFunc("PUT /files/{type}/keys/{key}/translations/{lang}", auth(s.handleSetTranslation))
	mux.HandleFunc("POST /files/{type}/keys/{key}/translations/{lang}/confirm", auth(s.handleConfirmTranslation))
	mux.HandleFunc("POST /files/{type}/keys/{key}/translations/{lang}/needs-review", auth(s.handleSetNeedsReview))
	mux.HandleFunc("POST /files/{type}/translations/clear", auth(s.handleClearTranslations))
	mux.HandleFunc("POST /files/{type}/validate", auth(s.handleValidate))

	mux.HandleFunc("GET /files/{type}/keys/{key}/lock", auth(s.handleGetKeyLock))
	mux.HandleFunc("POST /files/{type}/keys/{key}/lock", auth(s.handleLockKey))
	mux.HandleFunc("DELETE /files/{type}/keys/{key}/lock", auth(s.handleUnlockKey))
	mux.HandleFunc("POST /files/{type}/locks", auth(s.handleLockKeys))
	mux.HandleFunc("GET /files/{type}/locks", auth(s.handleListKeyLocks))

	mux.HandleFunc("POST /ai-config", projectAdmin(s.handleSetAIConfig))
	mux.HandleFunc("GET /ai-config", auth(s.handleGetAIConfig))
	mux.HandleFunc("GET /ai-config/defaults", auth(s.handleGetAIConfigDefaults))
	mux.HandleFunc("POST /ai-models", projectAdmin(s.handleListAIModels))
	mux.HandleFunc("POST /files/{type}/ai-jobs", auth(s.handleStartAITranslate))
	mux.HandleFunc("GET /files/{type}/ai-jobs", auth(s.handleListAIJobs))
	mux.HandleFunc("GET /files/{type}/ai-jobs/{id}", auth(s.handleAIJobStatus))
	mux.HandleFunc("POST /files/{type}/ai-jobs/{id}/cancel", auth(s.handleCancelAIJob))
	mux.HandleFunc("POST /files/{type}/glossary-discovery", auth(s.handleStartGlossaryDiscovery))

	mux.HandleFunc("GET /glossary", auth(s.handleListGlossaryTerms))
	mux.HandleFunc("POST /glossary", auth(s.handleAddGlossaryTerm))

	mux.HandleFunc("POST /glossary/import", projectAdmin(s.handleImportGlossaryTerms))

	mux.HandleFunc("DELETE /glossary", projectAdmin(s.handleClearGlossary))
	mux.HandleFunc("PUT /glossary/{term}", auth(s.handleUpdateGlossaryTerm))
	mux.HandleFunc("DELETE /glossary/{term}", auth(s.handleRemoveGlossaryTerm))
	mux.HandleFunc("POST /glossary/{term}/approve", auth(s.handleApproveGlossaryTerm))
	mux.HandleFunc("POST /glossary/{term}/unapprove", auth(s.handleUnapproveGlossaryTerm))
	mux.HandleFunc("POST /glossary/{term}/reject", auth(s.handleRejectGlossaryTerm))
	mux.HandleFunc("GET /glossary/{term}/usage", auth(s.handleGlossaryTermUsage))

	mux.HandleFunc("POST /glossary/fill", auth(s.handleFillGlossaryMatches))
	mux.HandleFunc("POST /glossary/{term}/prune-keys", auth(s.handlePruneGlossaryMatches))
	mux.HandleFunc("POST /glossary/{term}/lock", auth(s.handleLockTerm))
	mux.HandleFunc("DELETE /glossary/{term}/lock", auth(s.handleUnlockTerm))
	mux.HandleFunc("GET /glossary/{term}/lock", auth(s.handleGetTermLock))
	mux.HandleFunc("GET /glossary/locks", auth(s.handleListGlossaryLocks))

	mux.HandleFunc("POST /files/{type}/batches", projectAdmin(s.handleSplit))
	mux.HandleFunc("GET /files/{type}/batches", auth(s.handleListBatches))
	mux.HandleFunc("GET /files/{type}/batches/{id}", auth(s.handleGetBatch))
	mux.HandleFunc("POST /files/{type}/batches/{id}/merge", projectAdmin(s.handleMergeBatch))
	mux.HandleFunc("POST /files/{type}/batches/{id}/assign", auth(s.handleAssignBatch))
	mux.HandleFunc("POST /files/{type}/batches/{id}/reclaim", projectAdmin(s.handleReclaimBatch))

	mux.HandleFunc("POST /commit", auth(s.handleCommit))
	mux.HandleFunc("GET /history", auth(s.handleHistory))

	mux.HandleFunc("POST /remote", projectAdmin(s.handleSetRemote))
	mux.HandleFunc("GET /remote", projectAdmin(s.handleGetRemote))
	mux.HandleFunc("POST /push", projectAdmin(s.handlePush))

	mux.HandleFunc("POST /rollback", projectAdmin(s.handleRollback))

	mux.HandleFunc("POST /gc", projectAdmin(s.handleGC))
	mux.HandleFunc("POST /compact", projectAdmin(s.handleCompact))

	mux.HandleFunc("GET /export/repository", projectAdmin(s.handleExportRepository))

	mux.HandleFunc("POST /files/{type}/chains/source", projectAdmin(s.handleUploadChainSource))
	mux.HandleFunc("GET /files/{type}/chains/roots", auth(s.handleChainRoots))
	mux.HandleFunc("GET /files/{type}/chains/{root}", auth(s.handleChain))
	mux.HandleFunc("GET /files/{type}/chains/{root}/tree", auth(s.handleChainTree))

	mux.HandleFunc("GET /admin/users", projectAdmin(s.handleListUsers))
	mux.HandleFunc("POST /admin/users", projectAdmin(s.handleAddUser))
	mux.HandleFunc("DELETE /admin/users/{id}", projectAdmin(s.handleRemoveUser))
	mux.HandleFunc("POST /admin/users/{id}/regenerate-token", projectAdmin(s.handleRegenerateToken))
	mux.HandleFunc("GET /admin/listen", projectAdmin(s.handleGetListenConfig))
	mux.HandleFunc("POST /admin/listen", projectAdmin(s.handleSetListenConfig))
	mux.HandleFunc("GET /admin/tls", projectAdmin(s.handleGetTLSConfig))
	mux.HandleFunc("POST /admin/tls", projectAdmin(s.handleSetTLSConfig))
	mux.HandleFunc("POST /admin/tls/generate-self-signed", projectAdmin(s.handleGenerateSelfSignedCert))

	mux.HandleFunc("/setup/", s.handleSetupNotFound)

	if fallback != nil {

		mux.Handle("/", fallback)
	}
	return mux
}

func (s *Server) handleSetupNotFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotFound, "no onboarding in progress")
}

func fileTypeFromRequest(r *http.Request) (localize.FileType, bool) {
	ft := localize.FileType(r.PathValue("type"))
	return ft, ft.Valid()
}

func (s *Server) track(ft localize.FileType) (*project.FileTrack, bool) {
	t, ok := s.proj.Files[ft]
	return t, ok
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
