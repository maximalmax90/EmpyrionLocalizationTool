package localai

import (
	"context"
	"time"

	"elt/internal/ai"
	"elt/internal/apiclient"
)

type StartDiscoveryRequest struct {
	Lang     string   `json:"lang"`
	Keys     []string `json:"keys,omitempty"`
	Priority bool     `json:"priority,omitempty"`
}

func (m *Manager) StartDiscovery(client *apiclient.Client, cfg ai.Config, ft string, req StartDiscoveryRequest) (JobStatusResponse, error) {
	var items []keyListItem
	var err error
	if len(req.Keys) > 0 {
		items, err = fetchKeys(client, ft, req.Lang, req.Keys)
	} else {
		items, err = fetchKeysByFilter(client, ft, req.Lang, true)
	}
	if err != nil {
		return JobStatusResponse{}, err
	}
	keys := make([]string, len(items))
	for i, it := range items {
		keys[i] = it.Key
	}

	known, err := fetchGlossaryTerms(client, req.Lang, "all")
	if err != nil {
		return JobStatusResponse{}, err
	}

	glossaryPrompt := cfg.Resolve(ai.TaskGlossary).SystemPrompt
	if glossaryPrompt == "" {
		glossaryPrompt = DefaultGlossaryDiscoveryPromptTemplate
	}
	cfg.Default.SystemPrompt = RenderPromptTemplate(glossaryPrompt, req.Lang)
	client2 := ai.NewClient(cfg, nil)

	ctx, cancel := context.WithCancel(context.Background())
	job := &Job{
		ID: newJobID(), Kind: JobKindDiscover, FileType: ft, Langs: []string{req.Lang}, Priority: req.Priority,
		ctx: ctx, cancel: cancel,
		status: JobRunning, total: len(keys), startedAt: time.Now(),
	}
	m.addJob(job)

	go m.runDiscoveryJob(job, client, req.Lang, items, client2, known)

	return job.snapshot(), nil
}

func (m *Manager) runDiscoveryJob(job *Job, client *apiclient.Client, lang string, items []keyListItem, aiClient *ai.Client, known []glossaryTermForPrompt) {
	knownBlock := KnownTermsBlock(known)

	for _, item := range items {
		if job.cancelled() {
			job.finish(JobCancelled)
			return
		}
		added, err := m.discoverTermsForKey(job.ctx, client, aiClient, lang, item.English, knownBlock, job.Priority)
		if err != nil {
			if job.cancelled() {
				job.finish(JobCancelled)
				return
			}
			job.recordError(item.Key, err)
			continue
		}
		job.recordDiscovered(added)
		job.recordDone(item.Key)
	}
	job.finish(JobDone)
}

func (m *Manager) discoverTermsForKey(ctx context.Context, client *apiclient.Client, aiClient *ai.Client, lang, english, knownBlock string, priority bool) (added int, err error) {
	var messages []ai.Message
	if knownBlock != "" {
		messages = append(messages, ai.Message{Role: "system", Content: knownBlock})
	}
	messages = append(messages, ai.Message{Role: "user", Content: english})

	result, err := m.queue.submit(ctx, priority, aiClient, ai.TaskGlossary, messages)
	if err != nil {
		return 0, err
	}
	candidates, err := ParseCandidateTerms(result.Text)
	if err != nil {
		return 0, err
	}

	for _, c := range candidates {
		if err := addGlossaryTermDraft(client, lang, c); err == nil {
			added++
		}
	}
	return added, nil
}
