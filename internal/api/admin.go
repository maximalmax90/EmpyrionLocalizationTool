package api

import (
	"encoding/json"
	"net/http"

	"elt/internal/store"
)

type adminUserResponse struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Email   string `json:"email"`
	IsAdmin bool   `json:"is_admin"`
}

func toAdminUserResponse(u store.UserConfig) adminUserResponse {
	return adminUserResponse{ID: u.ID, Name: u.Name, Email: u.Email, IsAdmin: u.IsAdmin}
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	users, err := s.store.ListUsers()
	s.mu.Unlock()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	out := make([]adminUserResponse, 0, len(users))
	for _, u := range users {
		out = append(out, toAdminUserResponse(u))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleListUserNames(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	users, err := s.store.ListUsers()
	s.mu.Unlock()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	names := make([]string, 0, len(users))
	for _, u := range users {
		names = append(names, u.Name)
	}
	writeJSON(w, http.StatusOK, names)
}

type addUserRequest struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	IsAdmin bool   `json:"is_admin"`
}

type addUserResponse struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Email   string `json:"email"`
	IsAdmin bool   `json:"is_admin"`
	Token   string `json:"token"`
}

func (s *Server) handleAddUser(w http.ResponseWriter, r *http.Request) {
	if s.loopbackOnly {
		writeError(w, http.StatusForbidden, "adding users is disabled while this server only listens on localhost — expose it on a real network address first (elt serve --listen 0.0.0.0:<port> or similar)")
		return
	}

	var req addUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "decoding request body: %v", err)
		return
	}

	s.mu.Lock()
	id, token, err := s.store.AddUser(req.Name, req.Email, req.IsAdmin)
	s.mu.Unlock()
	if err != nil {
		writeError(w, http.StatusBadRequest, "%v", err)
		return
	}
	writeJSON(w, http.StatusOK, addUserResponse{ID: id, Name: req.Name, Email: req.Email, IsAdmin: req.IsAdmin, Token: token})
}

func (s *Server) handleRemoveUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.mu.Lock()
	err := s.store.RemoveUser(id)
	s.mu.Unlock()
	if err != nil {
		writeError(w, http.StatusBadRequest, "%v", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type regenerateTokenResponse struct {
	Token string `json:"token"`
}

func (s *Server) handleRegenerateToken(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.mu.Lock()
	token, err := s.store.RegenerateToken(id)
	s.mu.Unlock()
	if err != nil {
		writeError(w, http.StatusNotFound, "%v", err)
		return
	}
	writeJSON(w, http.StatusOK, regenerateTokenResponse{Token: token})
}
