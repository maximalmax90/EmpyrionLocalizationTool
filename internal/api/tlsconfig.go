package api

import (
	"encoding/json"
	"net/http"

	"elt/internal/store"
)

type tlsConfigResponse struct {
	Enabled         bool   `json:"enabled"`
	Cert            string `json:"cert,omitempty"`
	Key             string `json:"key,omitempty"`
	RestartRequired bool   `json:"restart_required,omitempty"`
}

func (s *Server) handleGetTLSConfig(w http.ResponseWriter, r *http.Request) {
	deployment, _, err := s.store.Deployment()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	resp := tlsConfigResponse{}
	if deployment.TLS != nil {
		resp.Enabled = deployment.TLS.Enabled
		resp.Cert = deployment.TLS.Cert
		resp.Key = deployment.TLS.Key
	}
	writeJSON(w, http.StatusOK, resp)
}

type setTLSConfigRequest struct {
	Enabled bool   `json:"enabled"`
	Cert    string `json:"cert"`
	Key     string `json:"key"`
}

func (s *Server) handleSetTLSConfig(w http.ResponseWriter, r *http.Request) {
	var req setTLSConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "decoding request body: %v", err)
		return
	}
	if (req.Cert == "") != (req.Key == "") {
		writeError(w, http.StatusBadRequest, "provide both a certificate and key path, or leave both empty to use a generated self-signed certificate")
		return
	}

	deployment, _, err := s.store.Deployment()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	deployment.TLS = &store.TLSConfig{Enabled: req.Enabled, Cert: req.Cert, Key: req.Key}
	if err := s.store.SetDeployment(deployment); err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	writeJSON(w, http.StatusOK, tlsConfigResponse{Enabled: req.Enabled, Cert: req.Cert, Key: req.Key, RestartRequired: true})
}

type generateSelfSignedCertResponse struct {
	Cert string `json:"cert"`
	Key  string `json:"key"`
}

func (s *Server) handleGenerateSelfSignedCert(w http.ResponseWriter, r *http.Request) {
	certPath, keyPath, err := s.store.EnsureSelfSignedCert()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	writeJSON(w, http.StatusOK, generateSelfSignedCertResponse{Cert: certPath, Key: keyPath})
}
