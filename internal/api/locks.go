package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"elt/internal/localize"
)

const (
	humanLockTTL = 5 * time.Minute
	aiLockTTL    = 5 * time.Minute
)

type resourceLock struct {
	Owner     string
	ExpiresAt time.Time
}

type lockRegistry struct {
	mu    sync.Mutex
	locks map[string]resourceLock
}

func keyLockID(ft localize.FileType, lang, key string) string {
	return "key:" + string(ft) + ":" + lang + ":" + key
}

func termLockID(lang, term string) string {
	return "term:" + lang + ":" + strings.ToLower(strings.TrimSpace(term))
}

func (r *lockRegistry) heldBy(id string) (owner string, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	lock, exists := r.locks[id]
	if !exists || time.Now().After(lock.ExpiresAt) {
		return "", false
	}
	return lock.Owner, true
}

func (r *lockRegistry) tryLock(id, owner string, ttl time.Duration) (ok bool, heldBy string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.locks == nil {
		r.locks = map[string]resourceLock{}
	}
	now := time.Now()
	if existing, exists := r.locks[id]; exists && !now.After(existing.ExpiresAt) && existing.Owner != owner {
		return false, existing.Owner
	}
	r.locks[id] = resourceLock{Owner: owner, ExpiresAt: now.Add(ttl)}
	return true, ""
}

func (r *lockRegistry) tryLockAll(ids []string, owner string, ttl time.Duration) (ok bool, conflicts []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.locks == nil {
		r.locks = map[string]resourceLock{}
	}
	now := time.Now()
	for _, id := range ids {
		if existing, exists := r.locks[id]; exists && !now.After(existing.ExpiresAt) && existing.Owner != owner {
			conflicts = append(conflicts, id)
		}
	}
	if len(conflicts) > 0 {
		return false, conflicts
	}
	expires := now.Add(ttl)
	for _, id := range ids {
		r.locks[id] = resourceLock{Owner: owner, ExpiresAt: expires}
	}
	return true, nil
}

func (r *lockRegistry) unlock(id, owner string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, exists := r.locks[id]
	if !exists || existing.Owner != owner {
		return false
	}
	delete(r.locks, id)
	return true
}

func (r *lockRegistry) listByPrefix(prefix string) map[string]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	out := map[string]string{}
	for id, lock := range r.locks {
		if !strings.HasPrefix(id, prefix) || now.After(lock.ExpiresAt) {
			continue
		}
		out[strings.TrimPrefix(id, prefix)] = lock.Owner
	}
	return out
}

func lockOwner(r *http.Request) (string, error) {
	if user, ok := RequestUser(r); ok {
		return user.Name, nil
	}
	owner := r.URL.Query().Get("owner")
	if owner == "" {
		return "", fmt.Errorf("?owner= is required (no authenticated user on this request)")
	}
	return owner, nil
}

func (s *Server) checkLock(id string, r *http.Request) error {
	heldBy, ok := s.locks.heldBy(id)
	if !ok {
		return nil
	}
	owner, err := lockOwner(r)
	if err != nil || owner != heldBy {
		return fmt.Errorf("currently locked by %s", heldBy)
	}
	return nil
}

type lockResponse struct {
	Locked bool   `json:"locked"`
	Owner  string `json:"owner,omitempty"`
}

func (s *Server) handleLockKey(w http.ResponseWriter, r *http.Request) {
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
	owner, err := lockOwner(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "%v", err)
		return
	}
	key := r.PathValue("key")

	ok2, heldBy := s.locks.tryLock(keyLockID(ft, lang, key), owner, humanLockTTL)
	if !ok2 {
		writeError(w, http.StatusConflict, "%s is currently locked by %s", key, heldBy)
		return
	}
	writeJSON(w, http.StatusOK, lockResponse{Locked: true, Owner: owner})
}

func (s *Server) handleUnlockKey(w http.ResponseWriter, r *http.Request) {
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
	owner, err := lockOwner(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "%v", err)
		return
	}
	s.locks.unlock(keyLockID(ft, lang, r.PathValue("key")), owner)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleGetKeyLock(w http.ResponseWriter, r *http.Request) {
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
	owner, held := s.locks.heldBy(keyLockID(ft, lang, r.PathValue("key")))
	writeJSON(w, http.StatusOK, lockResponse{Locked: held, Owner: owner})
}

type keyLocksResponse struct {
	Locks map[string]string `json:"locks"`
}

func (s *Server) handleListKeyLocks(w http.ResponseWriter, r *http.Request) {
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
	writeJSON(w, http.StatusOK, keyLocksResponse{Locks: s.locks.listByPrefix(keyLockID(ft, lang, ""))})
}

type lockKeysRequest struct {
	Lang string   `json:"lang"`
	Keys []string `json:"keys"`
}

type lockKeysResponse struct {
	Locked    bool     `json:"locked"`
	Conflicts []string `json:"conflicts,omitempty"`
}

func (s *Server) handleLockKeys(w http.ResponseWriter, r *http.Request) {
	ft, ok := fileTypeFromRequest(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown file type %q", r.PathValue("type"))
		return
	}
	var req lockKeysRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "decoding request body: %v", err)
		return
	}
	if len(req.Keys) == 0 {
		writeError(w, http.StatusBadRequest, "\"keys\" must be non-empty")
		return
	}
	if req.Lang == "" {
		writeError(w, http.StatusBadRequest, "\"lang\" is required")
		return
	}
	owner, err := lockOwner(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "%v", err)
		return
	}

	ids := make([]string, len(req.Keys))
	for i, k := range req.Keys {
		ids[i] = keyLockID(ft, req.Lang, k)
	}
	ok2, conflicts := s.locks.tryLockAll(ids, owner, humanLockTTL)
	if !ok2 {
		bare := make([]string, len(conflicts))
		prefix := "key:" + string(ft) + ":" + req.Lang + ":"
		for i, c := range conflicts {
			bare[i] = strings.TrimPrefix(c, prefix)
		}
		writeJSON(w, http.StatusConflict, lockKeysResponse{Locked: false, Conflicts: bare})
		return
	}
	writeJSON(w, http.StatusOK, lockKeysResponse{Locked: true})
}

func (s *Server) handleLockTerm(w http.ResponseWriter, r *http.Request) {
	lang := r.URL.Query().Get("lang")
	if lang == "" {
		writeError(w, http.StatusBadRequest, "?lang= is required")
		return
	}
	owner, err := lockOwner(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "%v", err)
		return
	}
	term := r.PathValue("term")

	ok, heldBy := s.locks.tryLock(termLockID(lang, term), owner, humanLockTTL)
	if !ok {
		writeError(w, http.StatusConflict, "%s is currently locked by %s", term, heldBy)
		return
	}
	writeJSON(w, http.StatusOK, lockResponse{Locked: true, Owner: owner})
}

func (s *Server) handleUnlockTerm(w http.ResponseWriter, r *http.Request) {
	lang := r.URL.Query().Get("lang")
	if lang == "" {
		writeError(w, http.StatusBadRequest, "?lang= is required")
		return
	}
	owner, err := lockOwner(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "%v", err)
		return
	}
	s.locks.unlock(termLockID(lang, r.PathValue("term")), owner)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleGetTermLock(w http.ResponseWriter, r *http.Request) {
	lang := r.URL.Query().Get("lang")
	if lang == "" {
		writeError(w, http.StatusBadRequest, "?lang= is required")
		return
	}
	owner, held := s.locks.heldBy(termLockID(lang, r.PathValue("term")))
	writeJSON(w, http.StatusOK, lockResponse{Locked: held, Owner: owner})
}

func (s *Server) handleListGlossaryLocks(w http.ResponseWriter, r *http.Request) {
	lang := r.URL.Query().Get("lang")
	if lang == "" {
		writeError(w, http.StatusBadRequest, "?lang= is required")
		return
	}
	writeJSON(w, http.StatusOK, keyLocksResponse{Locks: s.locks.listByPrefix(termLockID(lang, ""))})
}
