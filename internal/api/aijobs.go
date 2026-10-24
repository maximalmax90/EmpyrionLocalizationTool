package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"elt/internal/ai"
	"elt/internal/localize"
	"elt/internal/project"
	"elt/internal/validate"
)

type aiJobStatus string

const (
	aiJobRunning   aiJobStatus = "running"
	aiJobDone      aiJobStatus = "done"
	aiJobCancelled aiJobStatus = "cancelled"
)

const aiJobRecentCap = 20

type aiJobKind string

const (
	aiJobKindTranslate aiJobKind = "translate"
	aiJobKindDiscover  aiJobKind = "discover"
)

type aiJob struct {
	ID       string
	Kind     aiJobKind
	FileType localize.FileType

	Langs []string

	Priority bool
	ctx      context.Context
	cancel   context.CancelFunc

	mu      sync.Mutex
	status  aiJobStatus
	total   int
	done    int
	skipped int
	errored int

	discovered      int
	discoveryErrors int
	recent          []string
	lastError       string
	lastSkipped     string
	startedAt       time.Time
	finishedAt      time.Time
}

func glossaryPromptBlock(used []project.GlossaryTerm, lang string) string {
	var b strings.Builder
	b.WriteString("Glossary — this text contains the following known terms. Use exactly ")
	b.WriteString("this handling for each one, adapting only what's explicitly allowed:\n")
	for _, t := range used {
		switch t.Action {
		case project.TermKeep:
			fmt.Fprintf(&b, "- %q: keep exactly as written in English, do not translate it.\n", t.Term)
		case project.TermTransliterate:
			fmt.Fprintf(&b, "- %q: always render as %q (a fixed transliteration, not a fresh translation).\n", t.Term, t.Translation)
		case project.TermTranslate:
			if t.Inflect {
				fmt.Fprintf(&b, "- %q: translate as %q, grammatically inflected/declined as %s grammar requires in context.\n", t.Term, t.Translation, lang)
			} else {
				fmt.Fprintf(&b, "- %q: translate as %q exactly, in this form, without inflecting or declining it.\n", t.Term, t.Translation)
			}
		}
		if t.Notes != "" {
			fmt.Fprintf(&b, "  Note: %s\n", t.Notes)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func newAIJobID() string {
	var buf [8]byte
	_, _ = rand.Read(buf[:])
	return hex.EncodeToString(buf[:])
}

func (j *aiJob) recordDone(key string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.done++
	j.pushRecent(key)
}

func (j *aiJob) recordSkipped(key, reason string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.skipped++
	j.lastSkipped = fmt.Sprintf("%s: %s", key, reason)
}

func (j *aiJob) recordError(key string, err error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.errored++
	j.lastError = fmt.Sprintf("%s: %v", key, err)
}

func (j *aiJob) recordDiscovered(n int) {
	if n == 0 {
		return
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	j.discovered += n
}

func (j *aiJob) recordDiscoveryError(key string, err error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.discoveryErrors++
	j.lastError = fmt.Sprintf("%s (discovery): %v", key, err)
}

func (j *aiJob) pushRecent(key string) {
	j.recent = append(j.recent, key)
	if len(j.recent) > aiJobRecentCap {
		j.recent = j.recent[len(j.recent)-aiJobRecentCap:]
	}
}

func (j *aiJob) finish(status aiJobStatus) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.status != aiJobRunning {
		return
	}
	j.status = status
	j.finishedAt = time.Now()
}

type AIJobStatusResponse struct {
	ID       string   `json:"id"`
	Kind     string   `json:"kind"`
	FileType string   `json:"file_type"`
	Langs    []string `json:"langs"`
	Priority bool     `json:"priority"`
	Status   string   `json:"status"`
	Total    int      `json:"total"`
	Done     int      `json:"done"`
	Skipped  int      `json:"skipped"`
	Errored  int      `json:"errored"`

	Discovered      int      `json:"discovered"`
	DiscoveryErrors int      `json:"discovery_errors"`
	Recent          []string `json:"recent"`
	LastError       string   `json:"last_error,omitempty"`

	LastSkipped string     `json:"last_skipped,omitempty"`
	StartedAt   time.Time  `json:"started_at"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
}

func (j *aiJob) snapshot() AIJobStatusResponse {
	j.mu.Lock()
	defer j.mu.Unlock()
	resp := AIJobStatusResponse{
		ID:              j.ID,
		Kind:            string(j.Kind),
		FileType:        string(j.FileType),
		Langs:           append(make([]string, 0, len(j.Langs)), j.Langs...),
		Priority:        j.Priority,
		Status:          string(j.status),
		Total:           j.total,
		Done:            j.done,
		Skipped:         j.skipped,
		Errored:         j.errored,
		Discovered:      j.discovered,
		DiscoveryErrors: j.discoveryErrors,
		Recent:          append([]string(nil), j.recent...),
		LastError:       j.lastError,
		LastSkipped:     j.lastSkipped,
		StartedAt:       j.startedAt,
	}
	if !j.finishedAt.IsZero() {
		t := j.finishedAt
		resp.FinishedAt = &t
	}
	return resp
}

type startAIJobRequest struct {
	Langs []string `json:"langs"`

	Force bool `json:"force"`

	Keys []string `json:"keys,omitempty"`

	DiscoverTerms bool `json:"discover_terms,omitempty"`

	Priority bool `json:"priority,omitempty"`
}

func resolveJobKeys(track *project.FileTrack, lang string, filter project.KeyFilter, explicit []string) []string {
	if len(explicit) > 0 {
		keys := make([]string, 0, len(explicit))
		for _, k := range explicit {
			if rec, ok := track.Keys[k]; ok && rec.Status == project.KeyActive {
				keys = append(keys, k)
			}
		}
		return keys
	}
	items, _ := project.ListKeys(track, lang, filter, "", nil, 0, 0)
	keys := make([]string, 0, len(items))
	for _, it := range items {
		keys = append(keys, it.Key)
	}
	return keys
}

func (s *Server) handleStartAITranslate(w http.ResponseWriter, r *http.Request) {
	ft, ok := fileTypeFromRequest(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown file type %q", r.PathValue("type"))
		return
	}
	var req startAIJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "decoding request body: %v", err)
		return
	}
	if len(req.Langs) == 0 {
		writeError(w, http.StatusBadRequest, "\"langs\" must list at least one language")
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

	s.mu.Lock()
	track, ok := s.track(ft)
	if !ok {
		s.mu.Unlock()
		writeError(w, http.StatusBadRequest, "%s is not tracked", ft)
		return
	}
	filter := project.FilterNeedsWork
	if req.Force {
		filter = project.FilterAll
	}

	keySet := map[string]bool{}
	if len(req.Keys) > 0 {
		for _, k := range resolveJobKeys(track, "", filter, req.Keys) {
			keySet[k] = true
		}
	} else {
		for _, lang := range req.Langs {
			for _, k := range resolveJobKeys(track, lang, filter, nil) {
				keySet[k] = true
			}
		}
	}
	keys := make([]string, 0, len(keySet))
	for _, k := range track.Order {
		if keySet[k] {
			keys = append(keys, k)
		}
	}

	httpClient := &http.Client{}
	clients := make(map[string]*ai.Client, len(req.Langs))
	approvedTermsByLang := make(map[string][]project.GlossaryTerm, len(req.Langs))
	knownTermsBlockByLang := make(map[string]string, len(req.Langs))
	for _, lang := range req.Langs {

		cfg := aiCfg
		translatePrompt := cfg.Resolve(ai.TaskTranslate).SystemPrompt
		if translatePrompt == "" {
			translatePrompt = defaultTranslatePromptTemplate
		}
		tc := ai.TaskConfig{}
		if cfg.Translate != nil {
			tc = *cfg.Translate
		}
		tc.SystemPrompt = renderPromptTemplate(translatePrompt, lang)
		cfg.Translate = &tc

		if req.DiscoverTerms {
			glossaryPrompt := cfg.Resolve(ai.TaskGlossary).SystemPrompt
			if glossaryPrompt == "" {
				glossaryPrompt = defaultGlossaryDiscoveryPromptTemplate
			}
			gc := ai.TaskConfig{}
			if cfg.Glossary != nil {
				gc = *cfg.Glossary
			}
			gc.SystemPrompt = renderPromptTemplate(glossaryPrompt, lang)
			cfg.Glossary = &gc

			knownTermsBlockByLang[lang] = knownTermsBlock(project.GlossaryTerms(s.proj, lang))
		}

		clients[lang] = ai.NewClient(cfg, httpClient)
		approvedTermsByLang[lang] = project.ApprovedGlossaryTerms(s.proj, lang)
	}
	s.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	job := &aiJob{
		ID: newAIJobID(), Kind: aiJobKindTranslate, FileType: ft, Langs: req.Langs, Priority: req.Priority,
		ctx: ctx, cancel: cancel,
		status: aiJobRunning, total: len(keys) * len(req.Langs), startedAt: time.Now(),
	}
	s.aiJobsMu.Lock()
	if s.aiJobs == nil {
		s.aiJobs = map[string]*aiJob{}
	}
	s.aiJobs[job.ID] = job
	s.aiJobsMu.Unlock()

	go s.runAITranslateJob(job, ft, req.Langs, keys, clients, req.Force, approvedTermsByLang, req.DiscoverTerms, knownTermsBlockByLang)

	writeJSON(w, http.StatusOK, job.snapshot())
}

func (s *Server) runAITranslateJob(job *aiJob, ft localize.FileType, langs []string, keys []string, clients map[string]*ai.Client, force bool, approvedTermsByLang map[string][]project.GlossaryTerm, discoverTerms bool, knownTermsBlockByLang map[string]string) {
	policy := validate.DefaultPolicy(ft)

	for _, key := range keys {
		for _, lang := range langs {
			if job.ctx.Err() != nil {
				job.finish(aiJobCancelled)
				return
			}
			cancelled := s.processAIJobKey(
				job, ft, lang, key, clients[lang], force, approvedTermsByLang[lang], policy, discoverTerms, knownTermsBlockByLang[lang],
			)
			if cancelled {
				job.finish(aiJobCancelled)
				return
			}
		}
	}
	job.finish(aiJobDone)
}

func (s *Server) processAIJobKey(job *aiJob, ft localize.FileType, lang, key string, client *ai.Client, force bool, approvedTerms []project.GlossaryTerm, policy validate.Policy, discoverTerms bool, knownTermsBlockText string) (cancelled bool) {
	aiOwner := "ai-job:" + job.ID
	lockID := keyLockID(ft, lang, key)
	if ok, heldBy := s.locks.tryLock(lockID, aiOwner, aiLockTTL); !ok {

		job.recordSkipped(key, "locked by "+heldBy)
		return false
	}
	defer s.locks.unlock(lockID, aiOwner)

	s.mu.Lock()
	english, proceed, reason := s.aiJobKeyPrecheck(ft, key, lang, force)
	s.mu.Unlock()
	if !proceed {
		job.recordSkipped(key, reason)
		return false
	}

	messages := []ai.Message{}
	if used := project.GlossaryTermsUsedIn(approvedTerms, english); len(used) > 0 {
		messages = append(messages, ai.Message{Role: "system", Content: glossaryPromptBlock(used, lang)})
	}
	messages = append(messages, ai.Message{Role: "user", Content: english})

	result, err := s.aiQueue.submit(job.ctx, job.Priority, client, ai.TaskTranslate, messages)
	if err != nil {
		if job.ctx.Err() != nil {
			return true
		}
		job.recordError(key, err)
		return false
	}

	if discoverTerms {

		if added, derr := s.discoverTermsForKey(job.ctx, client, lang, english, knownTermsBlockText, job.Priority); derr != nil {
			job.recordDiscoveryError(key, derr)
		} else {
			job.recordDiscovered(added)
		}
	}

	var extraIssues []string
	if bad := ai.WordScriptIssues(result.Text, lang); len(bad) > 0 {
		extraIssues = append(extraIssues, ai.IssueScriptMismatch)
	}

	s.mu.Lock()
	_, proceed, reason = s.aiJobKeyPrecheck(ft, key, lang, force)
	if !proceed {
		s.mu.Unlock()
		job.recordSkipped(key, reason)
		return false
	}
	track, _ := s.track(ft)
	if track.Keys[key].English != english {

		s.mu.Unlock()
		job.recordSkipped(key, "source text changed")
		return false
	}
	setErr := project.SetAITranslation(track, key, lang, result.Text, policy, extraIssues)
	var saveErr error
	if setErr == nil {
		saveErr = s.store.Save(s.proj)
	}
	s.mu.Unlock()

	if setErr != nil {
		job.recordError(key, setErr)
		return false
	}
	if saveErr != nil {
		job.recordError(key, saveErr)
		return false
	}
	job.recordDone(key)
	return false
}

func (s *Server) aiJobKeyPrecheck(ft localize.FileType, key, lang string, force bool) (english string, needsWork bool, reason string) {
	track, ok := s.track(ft)
	if !ok {
		return "", false, "file type not tracked"
	}
	rec, ok := track.Keys[key]
	if !ok || rec.Status != project.KeyActive {
		return "", false, "key no longer active"
	}
	if !force {
		if tr, ok := rec.Translations[lang]; ok && tr.Text != "" && !tr.Stale {
			return rec.English, false, "already translated"
		}
	}
	return rec.English, true, ""
}

func (s *Server) handleAIJobStatus(w http.ResponseWriter, r *http.Request) {
	job, ok := s.lookupAIJob(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "no AI job %q", r.PathValue("id"))
		return
	}
	writeJSON(w, http.StatusOK, job.snapshot())
}

func (s *Server) handleCancelAIJob(w http.ResponseWriter, r *http.Request) {
	job, ok := s.lookupAIJob(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "no AI job %q", r.PathValue("id"))
		return
	}
	job.cancel()
	writeJSON(w, http.StatusOK, job.snapshot())
}

func (s *Server) handleListAIJobs(w http.ResponseWriter, r *http.Request) {
	ft, ok := fileTypeFromRequest(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown file type %q", r.PathValue("type"))
		return
	}

	s.aiJobsMu.Lock()
	jobs := make([]*aiJob, 0, len(s.aiJobs))
	for _, j := range s.aiJobs {
		if j.FileType == ft {
			jobs = append(jobs, j)
		}
	}
	s.aiJobsMu.Unlock()

	sort.Slice(jobs, func(i, k int) bool { return jobs[i].startedAt.Before(jobs[k].startedAt) })

	resp := make([]AIJobStatusResponse, 0, len(jobs))
	for _, j := range jobs {
		resp = append(resp, j.snapshot())
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) lookupAIJob(id string) (*aiJob, bool) {
	s.aiJobsMu.Lock()
	defer s.aiJobsMu.Unlock()
	j, ok := s.aiJobs[id]
	return j, ok
}
