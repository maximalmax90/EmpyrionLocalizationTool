package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"elt/internal/ai"
	"elt/internal/localize"
	"elt/internal/project"
)

func knownTermsBlock(known []project.GlossaryTerm) string {
	if len(known) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Already-known terms — do not propose any of these again:\n")
	for _, t := range known {
		fmt.Fprintf(&b, "- %s\n", t.Term)
	}
	return strings.TrimRight(b.String(), "\n")
}

func parseCandidateTerms(raw string) ([]string, error) {
	start := strings.Index(raw, "[")
	end := strings.LastIndex(raw, "]")
	if start == -1 || end == -1 || end < start {
		return nil, fmt.Errorf("ai: glossary discovery response did not contain a JSON array: %q", truncateForError(raw))
	}

	var candidates []string
	if err := json.Unmarshal([]byte(raw[start:end+1]), &candidates); err != nil {
		return nil, fmt.Errorf("ai: parsing glossary discovery response as a JSON string array: %w", err)
	}

	out := make([]string, 0, len(candidates))
	seen := map[string]bool{}
	for _, c := range candidates {
		c = strings.TrimSpace(c)
		key := strings.ToLower(c)
		if c == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, c)
	}
	return out, nil
}

func truncateForError(s string) string {
	const max = 200
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

type startGlossaryDiscoveryRequest struct {
	Lang string   `json:"lang"`
	Keys []string `json:"keys,omitempty"`

	Priority bool `json:"priority,omitempty"`
}

func (s *Server) handleStartGlossaryDiscovery(w http.ResponseWriter, r *http.Request) {
	ft, ok := fileTypeFromRequest(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown file type %q", r.PathValue("type"))
		return
	}
	var req startGlossaryDiscoveryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "decoding request body: %v", err)
		return
	}
	if req.Lang == "" {
		writeError(w, http.StatusBadRequest, "\"lang\" is required")
		return
	}

	aiCfg, ok, err := s.store.AIConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if !ok {
		writeError(w, http.StatusConflict, "no AI provider is configured for this project — POST /ai-config first")
		return
	}

	glossaryPrompt := aiCfg.Resolve(ai.TaskGlossary).SystemPrompt
	if glossaryPrompt == "" {
		glossaryPrompt = defaultGlossaryDiscoveryPromptTemplate
	}
	aiCfg.Default.SystemPrompt = renderPromptTemplate(glossaryPrompt, req.Lang)

	s.mu.Lock()
	track, ok := s.track(ft)
	if !ok {
		s.mu.Unlock()
		writeError(w, http.StatusBadRequest, "%s is not tracked", ft)
		return
	}
	keys := resolveJobKeys(track, req.Lang, project.FilterAll, req.Keys)

	known := project.GlossaryTerms(s.proj, req.Lang)
	s.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	job := &aiJob{
		ID: newAIJobID(), Kind: aiJobKindDiscover, FileType: ft, Langs: []string{req.Lang}, Priority: req.Priority,
		ctx: ctx, cancel: cancel,
		status: aiJobRunning, total: len(keys), startedAt: time.Now(),
	}
	s.aiJobsMu.Lock()
	if s.aiJobs == nil {
		s.aiJobs = map[string]*aiJob{}
	}
	s.aiJobs[job.ID] = job
	s.aiJobsMu.Unlock()

	client := ai.NewClient(aiCfg, nil)
	go s.runGlossaryDiscoveryJob(job, ft, req.Lang, keys, client, known)

	writeJSON(w, http.StatusOK, job.snapshot())
}

func (s *Server) runGlossaryDiscoveryJob(job *aiJob, ft localize.FileType, lang string, keys []string, client *ai.Client, known []project.GlossaryTerm) {
	knownBlock := knownTermsBlock(known)

	for _, key := range keys {
		if job.ctx.Err() != nil {
			job.finish(aiJobCancelled)
			return
		}

		s.mu.Lock()
		track, ok := s.track(ft)
		var english string
		var active bool
		if ok {
			if rec, recOK := track.Keys[key]; recOK && rec.Status == project.KeyActive {
				english, active = rec.English, true
			}
		}
		s.mu.Unlock()
		if !active {
			job.recordSkipped(key, "key no longer active")
			continue
		}

		added, err := s.discoverTermsForKey(job.ctx, client, lang, english, knownBlock, job.Priority)
		if err != nil {
			if job.ctx.Err() != nil {
				job.finish(aiJobCancelled)
				return
			}
			job.recordError(key, err)
			continue
		}
		job.recordDiscovered(added)
		job.recordDone(key)
	}
	job.finish(aiJobDone)
}

func (s *Server) discoverTermsForKey(ctx context.Context, client *ai.Client, lang, english, knownBlock string, priority bool) (added int, err error) {
	var messages []ai.Message
	if knownBlock != "" {
		messages = append(messages, ai.Message{Role: "system", Content: knownBlock})
	}
	messages = append(messages, ai.Message{Role: "user", Content: english})

	result, err := s.aiQueue.submit(ctx, priority, client, ai.TaskGlossary, messages)
	if err != nil {
		return 0, err
	}
	candidates, err := parseCandidateTerms(result.Text)
	if err != nil {
		return 0, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range candidates {

		if err := project.AddGlossaryTerm(s.proj, lang, project.GlossaryTerm{Term: c, Action: project.TermKeep}); err == nil {
			added++
		}
	}
	if added == 0 {
		return 0, nil
	}
	if err := s.store.Save(s.proj); err != nil {
		return added, err
	}
	return added, nil
}
