package api

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
)

type listenConfigResponse struct {
	Address         string `json:"address"`
	RestartRequired bool   `json:"restart_required"`
}

func (s *Server) handleGetListenConfig(w http.ResponseWriter, r *http.Request) {
	if s.listenConfig.Get == nil {
		writeError(w, http.StatusNotImplemented, "this server was not started in a way that supports reading/changing its listen address (no --data-dir/elt gui or elt serve config to persist it in)")
		return
	}
	addr, err := s.listenConfig.Get()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	writeJSON(w, http.StatusOK, listenConfigResponse{Address: addr})
}

type setListenConfigRequest struct {
	Address string `json:"address"`
}

func (s *Server) handleSetListenConfig(w http.ResponseWriter, r *http.Request) {
	if s.listenConfig.Set == nil {
		writeError(w, http.StatusNotImplemented, "this server was not started in a way that supports reading/changing its listen address (no --data-dir/elt gui or elt serve config to persist it in)")
		return
	}
	var req setListenConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "decoding request body: %v", err)
		return
	}
	addr := strings.TrimSpace(req.Address)
	if err := validateListenAddr(addr); err != nil {
		writeError(w, http.StatusBadRequest, "%v", err)
		return
	}
	if err := s.listenConfig.Set(addr); err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	writeJSON(w, http.StatusOK, listenConfigResponse{Address: addr, RestartRequired: true})
}

func validateListenAddr(addr string) error {
	if addr == "" {
		return fmt.Errorf("an address is required (e.g. 127.0.0.1:8080 or 0.0.0.0:8080)")
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("%q is not a valid host:port address: %w", addr, err)
	}
	if host != "" && net.ParseIP(host) == nil && host != "localhost" {
		return fmt.Errorf("%q is not a valid host — use an IP address (e.g. 127.0.0.1, 0.0.0.0) or leave it empty for all interfaces", host)
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 0 || n > 65535 {
		return fmt.Errorf("%q is not a valid port — must be a number from 0 to 65535", port)
	}
	return nil
}
