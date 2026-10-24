package api

import (
	"context"
	"net/http"
	"strings"

	"elt/internal/project"
	"elt/internal/store"
)

type contextKey int

const userContextKey contextKey = iota

func RequestUser(r *http.Request) (store.UserConfig, bool) {
	u, ok := r.Context().Value(userContextKey).(store.UserConfig)
	return u, ok
}

func (s *Server) requireToken(next http.HandlerFunc) http.HandlerFunc {
	return s.requireTokenImpl(false, next)
}

func (s *Server) requireProjectAdmin(next http.HandlerFunc) http.HandlerFunc {
	return s.requireTokenImpl(true, next)
}

func (s *Server) requireTokenImpl(adminOnly bool, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		users, err := s.store.ListUsers()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "%v", err)
			return
		}
		if len(users) == 0 {
			next(w, r)
			return
		}

		token := bearerToken(r)
		if token == "" {
			writeError(w, http.StatusUnauthorized, "an Authorization: Bearer token is required")
			return
		}
		user, ok, err := s.store.Authenticate(token)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "%v", err)
			return
		}
		if !ok {
			writeError(w, http.StatusUnauthorized, "invalid token")
			return
		}
		if adminOnly && !user.IsAdmin {
			writeError(w, http.StatusForbidden, "this action requires an admin token")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), userContextKey, user)))
	}
}

func bearerToken(r *http.Request) string {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, prefix) {
		return ""
	}
	return strings.TrimPrefix(h, prefix)
}

func (s *Server) commit(p *project.Project, r *http.Request, message string) error {
	if user, ok := RequestUser(r); ok {
		return s.store.SaveAndCommitAs(p, message, store.AuthorConfig{Name: user.Name, Email: user.Email})
	}
	return s.store.SaveAndCommit(p, message)
}

func (s *Server) commitOnly(r *http.Request, message string) error {
	if user, ok := RequestUser(r); ok {
		return s.store.CommitAs(message, store.AuthorConfig{Name: user.Name, Email: user.Email})
	}
	return s.store.Commit(message)
}

type meResponse struct {
	Authenticated bool   `json:"authenticated"`
	Name          string `json:"name,omitempty"`

	IsAdmin bool `json:"is_admin,omitempty"`
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	user, ok := RequestUser(r)
	if !ok {
		writeJSON(w, http.StatusOK, meResponse{Authenticated: false})
		return
	}
	writeJSON(w, http.StatusOK, meResponse{Authenticated: true, Name: user.Name, IsAdmin: user.IsAdmin})
}
