package localai

import (
	"net/url"

	"elt/internal/apiclient"
)

type keyListItem struct {
	Key         string
	English     string
	Translation struct {
		Text  string
		Stale bool
	}
}

type keysResponse struct {
	Items []keyListItem `json:"items"`
	Total int           `json:"total"`
}

type listKeysRequestBody struct {
	Lang   string   `json:"lang"`
	Status string   `json:"status,omitempty"`
	Limit  int      `json:"limit"`
	Keys   []string `json:"keys,omitempty"`
}

func fetchKeys(client *apiclient.Client, ft, lang string, keys []string) ([]keyListItem, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	body := listKeysRequestBody{Lang: lang, Keys: keys, Limit: len(keys)}
	var resp keysResponse
	if err := client.CallJSON("POST", "/files/"+url.PathEscape(ft)+"/keys", body, &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

func fetchKeysByFilter(client *apiclient.Client, ft, lang string, force bool) ([]keyListItem, error) {
	q := url.Values{}
	q.Set("lang", lang)
	if !force {
		q.Set("status", "needs_work")
	}
	q.Set("limit", "1000000")
	var resp keysResponse
	if err := client.CallJSON("GET", "/files/"+url.PathEscape(ft)+"/keys?"+q.Encode(), nil, &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

func lockOwnerFor(jobID string) string {
	return "ai-job:" + jobID
}

func lockPath(ft, lang, key, owner string) string {
	q := url.Values{"lang": {lang}, "owner": {owner}}
	return "/files/" + url.PathEscape(ft) + "/keys/" + url.PathEscape(key) + "/lock?" + q.Encode()
}

func acquireLock(client *apiclient.Client, ft, lang, key, owner string) error {
	_, err := client.Call("POST", lockPath(ft, lang, key, owner), nil)
	return err
}

func releaseLock(client *apiclient.Client, ft, lang, key, owner string) {

	_, _ = client.Call("DELETE", lockPath(ft, lang, key, owner), nil)
}

type setTranslationRequest struct {
	Text   string `json:"text"`
	Source string `json:"source"`
}

func writeTranslation(client *apiclient.Client, ft, lang, key, owner, text, source string) error {
	q := url.Values{"owner": {owner}}
	path := "/files/" + url.PathEscape(ft) + "/keys/" + url.PathEscape(key) + "/translations/" + url.PathEscape(lang) + "?" + q.Encode()
	return client.CallJSON("PUT", path, setTranslationRequest{Text: text, Source: source}, nil)
}

func fetchGlossaryTerms(client *apiclient.Client, lang, status string) ([]glossaryTermForPrompt, error) {
	q := url.Values{"lang": {lang}}
	if status != "" {
		q.Set("status", status)
	}
	var terms []glossaryTermForPrompt
	if err := client.CallJSON("GET", "/glossary?"+q.Encode(), nil, &terms); err != nil {
		return nil, err
	}
	return terms, nil
}

type addGlossaryTermRequest struct {
	Term   string `json:"term"`
	Action string `json:"action"`
}

func addGlossaryTermDraft(client *apiclient.Client, lang, term string) error {
	q := url.Values{"lang": {lang}}
	return client.CallJSON("POST", "/glossary?"+q.Encode(), addGlossaryTermRequest{Term: term, Action: "keep"}, nil)
}
