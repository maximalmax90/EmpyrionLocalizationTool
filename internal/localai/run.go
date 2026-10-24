package localai

import (
	"context"
	"net/http"
	"time"

	"elt/internal/ai"
	"elt/internal/apiclient"
)

type StartTranslateRequest struct {
	Langs         []string `json:"langs"`
	Force         bool     `json:"force"`
	Keys          []string `json:"keys,omitempty"`
	DiscoverTerms bool     `json:"discover_terms,omitempty"`
	Priority      bool     `json:"priority,omitempty"`
}

func (m *Manager) StartTranslate(client *apiclient.Client, cfg ai.Config, ft string, req StartTranslateRequest) (JobStatusResponse, error) {
	keySet := map[string]bool{}
	var order []string
	addKeys := func(items []keyListItem) {
		for _, it := range items {
			if !keySet[it.Key] {
				keySet[it.Key] = true
				order = append(order, it.Key)
			}
		}
	}
	if len(req.Keys) > 0 {

		items, err := fetchKeys(client, ft, req.Langs[0], req.Keys)
		if err != nil {
			return JobStatusResponse{}, err
		}
		addKeys(items)
	} else {
		for _, lang := range req.Langs {
			items, err := fetchKeysByFilter(client, ft, lang, req.Force)
			if err != nil {
				return JobStatusResponse{}, err
			}
			addKeys(items)
		}
	}

	httpClient := &http.Client{}
	clients := make(map[string]*ai.Client, len(req.Langs))
	approvedByLang := make(map[string][]glossaryTermForPrompt, len(req.Langs))
	knownByLang := make(map[string]string, len(req.Langs))
	for _, lang := range req.Langs {
		langCfg := cfg
		translatePrompt := langCfg.Resolve(ai.TaskTranslate).SystemPrompt
		if translatePrompt == "" {
			translatePrompt = DefaultTranslatePromptTemplate
		}
		tc := ai.TaskConfig{}
		if langCfg.Translate != nil {
			tc = *langCfg.Translate
		}
		tc.SystemPrompt = RenderPromptTemplate(translatePrompt, lang)
		langCfg.Translate = &tc

		if req.DiscoverTerms {
			glossaryPrompt := langCfg.Resolve(ai.TaskGlossary).SystemPrompt
			if glossaryPrompt == "" {
				glossaryPrompt = DefaultGlossaryDiscoveryPromptTemplate
			}
			gc := ai.TaskConfig{}
			if langCfg.Glossary != nil {
				gc = *langCfg.Glossary
			}
			gc.SystemPrompt = RenderPromptTemplate(glossaryPrompt, lang)
			langCfg.Glossary = &gc
			known, err := fetchGlossaryTerms(client, lang, "all")
			if err != nil {
				return JobStatusResponse{}, err
			}
			knownByLang[lang] = KnownTermsBlock(known)
		}

		clients[lang] = ai.NewClient(langCfg, httpClient)
		approved, err := fetchGlossaryTerms(client, lang, "approved")
		if err != nil {
			return JobStatusResponse{}, err
		}
		approvedByLang[lang] = approved
	}

	ctx, cancel := context.WithCancel(context.Background())
	job := &Job{
		ID: newJobID(), Kind: JobKindTranslate, FileType: ft, Langs: req.Langs, Priority: req.Priority,
		ctx: ctx, cancel: cancel,
		status: JobRunning, total: len(order) * len(req.Langs), startedAt: time.Now(),
	}
	m.addJob(job)

	go m.runTranslateJob(job, client, ft, req.Langs, order, clients, req.Force, approvedByLang, req.DiscoverTerms, knownByLang)

	return job.snapshot(), nil
}

func (m *Manager) runTranslateJob(job *Job, client *apiclient.Client, ft string, langs, keys []string, clients map[string]*ai.Client, force bool, approvedByLang map[string][]glossaryTermForPrompt, discoverTerms bool, knownByLang map[string]string) {
	for _, key := range keys {
		for _, lang := range langs {
			if job.cancelled() {
				job.finish(JobCancelled)
				return
			}
			if cancelled := m.processKey(job, client, ft, lang, key, clients[lang], force, approvedByLang[lang], discoverTerms, knownByLang[lang]); cancelled {
				job.finish(JobCancelled)
				return
			}
		}
	}
	job.finish(JobDone)
}

func (m *Manager) processKey(job *Job, client *apiclient.Client, ft, lang, key string, aiClient *ai.Client, force bool, approvedTerms []glossaryTermForPrompt, discoverTerms bool, knownBlock string) (cancelled bool) {
	owner := lockOwnerFor(job.ID)
	if err := acquireLock(client, ft, lang, key, owner); err != nil {
		job.recordSkipped(key, "locked by someone else")
		return false
	}
	defer releaseLock(client, ft, lang, key, owner)

	english, needsWork, reason, err := precheckKey(client, ft, lang, key, force)
	if err != nil {
		job.recordError(key, err)
		return false
	}
	if !needsWork {
		job.recordSkipped(key, reason)
		return false
	}

	messages := []ai.Message{}
	if used := TermsUsedIn(approvedTerms, english); len(used) > 0 {
		messages = append(messages, ai.Message{Role: "system", Content: GlossaryPromptBlock(used, lang)})
	}
	messages = append(messages, ai.Message{Role: "user", Content: english})

	result, err := m.queue.submit(job.ctx, job.Priority, aiClient, ai.TaskTranslate, messages)
	if err != nil {
		if job.cancelled() {
			return true
		}
		job.recordError(key, err)
		return false
	}

	if discoverTerms {
		if added, derr := m.discoverTermsForKey(job.ctx, client, aiClient, lang, english, knownBlock, job.Priority); derr != nil {
			job.recordDiscoveryError(key, derr)
		} else {
			job.recordDiscovered(added)
		}
	}

	currentEnglish, stillNeedsWork, reason, err := precheckKey(client, ft, lang, key, force)
	if err != nil {
		job.recordError(key, err)
		return false
	}
	if !stillNeedsWork {
		job.recordSkipped(key, reason)
		return false
	}
	if currentEnglish != english {
		job.recordSkipped(key, "source text changed")
		return false
	}

	if err := writeTranslation(client, ft, lang, key, owner, result.Text, "machine"); err != nil {
		job.recordError(key, err)
		return false
	}
	job.recordDone(key)
	return false
}

func precheckKey(client *apiclient.Client, ft, lang, key string, force bool) (english string, needsWork bool, reason string, err error) {
	items, err := fetchKeys(client, ft, lang, []string{key})
	if err != nil {
		return "", false, "", err
	}
	if len(items) == 0 {
		return "", false, "key no longer active", nil
	}
	item := items[0]

	if !force && item.Translation.Text != "" && !item.Translation.Stale {
		return item.English, false, "already translated", nil
	}
	return item.English, true, "", nil
}
