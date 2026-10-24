package localai

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"elt/internal/ai"
	"elt/internal/apiclient"
)

func (m *Manager) Handler(remoteAddr string, insecure bool) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /ai-config", m.handleSetConfig)
	mux.HandleFunc("GET /ai-config", m.handleGetConfig)
	mux.HandleFunc("GET /ai-config/defaults", m.handleGetConfigDefaults)
	mux.HandleFunc("POST /ai-models", m.handleListModels)
	mux.HandleFunc("POST /files/{type}/ai-jobs", m.withClient(remoteAddr, insecure, m.handleStartTranslate))
	mux.HandleFunc("GET /files/{type}/ai-jobs", m.handleListJobs)
	mux.HandleFunc("GET /files/{type}/ai-jobs/{id}", m.handleJobStatus)
	mux.HandleFunc("POST /files/{type}/ai-jobs/{id}/cancel", m.handleCancelJob)
	mux.HandleFunc("POST /files/{type}/glossary-discovery", m.withClient(remoteAddr, insecure, m.handleStartDiscovery))
	return mux
}

var LocalAIPaths = []string{
	"/ai-config",
	"/ai-config/defaults",
	"/ai-models",
}

func IsLocalAIPath(path string) bool {
	for _, p := range LocalAIPaths {
		if path == p {
			return true
		}
	}
	if !strings.HasPrefix(path, "/files/") {
		return false
	}
	rest := strings.TrimPrefix(path, "/files/")
	slash := strings.IndexByte(rest, '/')
	if slash < 0 {
		return false
	}
	tail := rest[slash+1:]
	return tail == "ai-jobs" || strings.HasPrefix(tail, "ai-jobs/") || tail == "glossary-discovery"
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func clientFor(remoteAddr string, insecure bool, r *http.Request) *apiclient.Client {
	token := ""
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		token = strings.TrimPrefix(auth, "Bearer ")
	}

	baseURL, err := apiclient.NormalizeBaseURL(remoteAddr)
	if err != nil {
		baseURL = "http://" + remoteAddr
	}
	client := &apiclient.Client{BaseURL: baseURL, Token: token}
	if insecure {
		client.HTTPClient = apiclient.InsecureHTTPClient()
	}
	return client
}

func (m *Manager) withClient(remoteAddr string, insecure bool, h func(http.ResponseWriter, *http.Request, *apiclient.Client)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h(w, r, clientFor(remoteAddr, insecure, r))
	}
}

func (m *Manager) handleSetConfig(w http.ResponseWriter, r *http.Request) {
	var cfg ai.Config
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, "decoding request body: "+err.Error())
		return
	}
	SeedDefaultPrompts(&cfg)
	if err := SaveConfig(cfg); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (m *Manager) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := LoadConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !Configured(cfg) {
		writeJSON(w, http.StatusOK, map[string]bool{"configured": false})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"configured": true, "url": cfg.URL,
		"default": cfg.Default, "translate": cfg.Translate, "glossary": cfg.Glossary, "refine": cfg.Refine,
	})
}

func (m *Manager) handleGetConfigDefaults(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"translate": DefaultTranslatePromptTemplate,
		"glossary":  DefaultGlossaryDiscoveryPromptTemplate,
	})
}

func (m *Manager) handleListModels(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL   string `json:"url"`
		Token string `json:"token"`
	}
	if r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "decoding request body: "+err.Error())
			return
		}
	}
	url, token := req.URL, req.Token
	if url == "" {
		cfg, err := LoadConfig()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !Configured(cfg) {
			writeError(w, http.StatusBadRequest, "no url supplied and no AI provider configured yet")
			return
		}
		url = cfg.URL
		if token == "" {
			token = cfg.Token
		}
	}
	client := ai.NewClient(ai.Config{URL: url, Token: token}, nil)
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	models, err := client.ListModels(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string][]string{"models": models})
}

func (m *Manager) handleStartTranslate(w http.ResponseWriter, r *http.Request, client *apiclient.Client) {
	ft := r.PathValue("type")
	var req StartTranslateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "decoding request body: "+err.Error())
		return
	}
	if len(req.Langs) == 0 {
		writeError(w, http.StatusBadRequest, `"langs" must list at least one language`)
		return
	}
	cfg, err := LoadConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !Configured(cfg) {
		writeError(w, http.StatusConflict, "no AI provider is configured on this client — POST /ai-config first")
		return
	}
	job, err := m.StartTranslate(client, cfg, ft, req)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (m *Manager) handleStartDiscovery(w http.ResponseWriter, r *http.Request, client *apiclient.Client) {
	ft := r.PathValue("type")
	var req StartDiscoveryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "decoding request body: "+err.Error())
		return
	}
	if req.Lang == "" {
		writeError(w, http.StatusBadRequest, `"lang" is required`)
		return
	}
	cfg, err := LoadConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !Configured(cfg) {
		writeError(w, http.StatusConflict, "no AI provider is configured on this client — POST /ai-config first")
		return
	}
	job, err := m.StartDiscovery(client, cfg, ft, req)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (m *Manager) handleListJobs(w http.ResponseWriter, r *http.Request) {
	ft := r.PathValue("type")
	jobs := m.listJobs(ft)
	resp := make([]JobStatusResponse, 0, len(jobs))
	for _, j := range jobs {
		resp = append(resp, j.snapshot())
	}
	writeJSON(w, http.StatusOK, resp)
}

func (m *Manager) handleJobStatus(w http.ResponseWriter, r *http.Request) {
	snap, ok := m.GetJob(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "no AI job "+r.PathValue("id"))
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

func (m *Manager) handleCancelJob(w http.ResponseWriter, r *http.Request) {
	job, ok := m.lookupJob(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "no AI job "+r.PathValue("id"))
		return
	}
	job.cancel()
	writeJSON(w, http.StatusOK, job.snapshot())
}
