package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"elt/internal/ai"
)

type setAIConfigRequest struct {
	URL       string         `json:"url"`
	Token     string         `json:"token,omitempty"`
	Default   ai.TaskConfig  `json:"default"`
	Translate *ai.TaskConfig `json:"translate,omitempty"`
	Glossary  *ai.TaskConfig `json:"glossary,omitempty"`
	Refine    *ai.TaskConfig `json:"refine,omitempty"`
}

type aiConfigStatusResponse struct {
	Configured bool           `json:"configured"`
	URL        string         `json:"url,omitempty"`
	Default    ai.TaskConfig  `json:"default,omitempty"`
	Translate  *ai.TaskConfig `json:"translate,omitempty"`
	Glossary   *ai.TaskConfig `json:"glossary,omitempty"`
	Refine     *ai.TaskConfig `json:"refine,omitempty"`
}

func (s *Server) handleSetAIConfig(w http.ResponseWriter, r *http.Request) {
	var req setAIConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "decoding request body: %v", err)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	cfg := ai.Config{
		URL: req.URL, Token: req.Token,
		Default: req.Default, Translate: req.Translate, Glossary: req.Glossary, Refine: req.Refine,
	}
	seedDefaultPrompts(&cfg)

	if err := s.store.SetAIConfig(cfg); err != nil {
		writeError(w, http.StatusBadRequest, "%v", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type aiConfigDefaultsResponse struct {
	Translate string `json:"translate"`
	Glossary  string `json:"glossary"`
}

func (s *Server) handleGetAIConfigDefaults(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, aiConfigDefaultsResponse{
		Translate: defaultTranslatePromptTemplate,
		Glossary:  defaultGlossaryDiscoveryPromptTemplate,
	})
}

type listAIModelsRequest struct {
	URL   string `json:"url,omitempty"`
	Token string `json:"token,omitempty"`
}

type listAIModelsResponse struct {
	Models []string `json:"models"`
}

func (s *Server) handleListAIModels(w http.ResponseWriter, r *http.Request) {
	var req listAIModelsRequest
	if r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "decoding request body: %v", err)
			return
		}
	}

	url, token := req.URL, req.Token
	if url == "" {
		s.mu.Lock()
		cfg, ok, err := s.store.AIConfig()
		s.mu.Unlock()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "%v", err)
			return
		}
		if !ok {
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
		writeError(w, http.StatusBadGateway, "%v", err)
		return
	}
	writeJSON(w, http.StatusOK, listAIModelsResponse{Models: models})
}

func (s *Server) handleGetAIConfig(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	cfg, ok, err := s.store.AIConfig()
	s.mu.Unlock()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if !ok {
		writeJSON(w, http.StatusOK, aiConfigStatusResponse{Configured: false})
		return
	}
	writeJSON(w, http.StatusOK, aiConfigStatusResponse{
		Configured: true, URL: cfg.URL,
		Default: cfg.Default, Translate: cfg.Translate, Glossary: cfg.Glossary, Refine: cfg.Refine,
	})
}
