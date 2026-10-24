package api

import (
	"encoding/json"
	"net/http"

	"elt/internal/store"
)

type setRemoteRequest struct {
	URL      string `json:"url"`
	Username string `json:"username,omitempty"`
	Token    string `json:"token,omitempty"`
	Push     string `json:"push,omitempty"`
}

type remoteStatusResponse struct {
	Configured bool   `json:"configured"`
	URL        string `json:"url,omitempty"`
	Username   string `json:"username,omitempty"`
	Push       string `json:"push,omitempty"`
}

func (s *Server) handleSetRemote(w http.ResponseWriter, r *http.Request) {
	var req setRemoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "decoding request body: %v", err)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	err := s.store.SetRemote(store.RemoteConfig{
		URL: req.URL, Username: req.Username, Token: req.Token, Push: req.Push,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, "%v", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleGetRemote(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	cfg, ok, err := s.store.Remote()
	s.mu.Unlock()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if !ok {
		writeJSON(w, http.StatusOK, remoteStatusResponse{Configured: false})
		return
	}
	writeJSON(w, http.StatusOK, remoteStatusResponse{
		Configured: true, URL: cfg.URL, Username: cfg.Username, Push: cfg.Push,
	})
}

func (s *Server) handlePush(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	err := s.store.Push()
	s.mu.Unlock()
	if err != nil {
		writeError(w, http.StatusBadRequest, "%v", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
