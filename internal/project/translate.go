package project

import (
	"fmt"

	"elt/internal/validate"
)

func SetTranslation(ft *FileTrack, key, lang, text, source string) error {
	rec, ok := ft.Keys[key]
	if !ok {
		return fmt.Errorf("project: unknown key %q", key)
	}
	if rec.Translations == nil {
		rec.Translations = map[string]Translation{}
	}
	rec.Translations[lang] = Translation{Text: text, Source: source, NeedsReview: rec.Translations[lang].NeedsReview}
	return nil
}

func SetAITranslation(ft *FileTrack, key, lang, text string, policy validate.Policy, extraIssues []string) error {
	rec, ok := ft.Keys[key]
	if !ok {
		return fmt.Errorf("project: unknown key %q", key)
	}

	result := validate.ValidateTextPair(rec.English, text, policy)
	var issues []string
	for _, iss := range result.Issues {
		issues = append(issues, iss.Code)
	}
	issues = append(issues, extraIssues...)

	if rec.Translations == nil {
		rec.Translations = map[string]Translation{}
	}
	rec.Translations[lang] = Translation{Text: result.Text, Source: "machine", Issues: issues, NeedsReview: rec.Translations[lang].NeedsReview}
	return nil
}

func ConfirmTranslation(ft *FileTrack, key, lang string) error {
	rec, ok := ft.Keys[key]
	if !ok {
		return fmt.Errorf("project: unknown key %q", key)
	}
	tr, ok := rec.Translations[lang]
	if !ok || tr.Text == "" {
		return fmt.Errorf("project: key %q has no %s translation to confirm", key, lang)
	}
	tr.Source = "manual"
	rec.Translations[lang] = tr
	return nil
}

func SetNeedsReview(ft *FileTrack, key, lang string, needsReview bool) error {
	rec, ok := ft.Keys[key]
	if !ok {
		return fmt.Errorf("project: unknown key %q", key)
	}
	tr, ok := rec.Translations[lang]
	if !ok || tr.Text == "" {
		return fmt.Errorf("project: key %q has no %s translation to flag", key, lang)
	}
	tr.NeedsReview = needsReview
	rec.Translations[lang] = tr
	return nil
}

func Validate(ft *FileTrack, lang string, policy validate.Policy) map[string][]string {
	changed := map[string][]string{}
	for key, rec := range ft.Keys {
		if rec.Status != KeyActive {
			continue
		}
		tr, ok := rec.Translations[lang]
		if !ok || tr.Text == "" {
			continue
		}

		result := validate.ValidateTextPair(rec.English, tr.Text, policy)
		tr.Text = result.Text
		tr.Issues = nil
		for _, iss := range result.Issues {
			tr.Issues = append(tr.Issues, iss.Code)
		}
		rec.Translations[lang] = tr

		if len(tr.Issues) > 0 {
			changed[key] = tr.Issues
		}
	}
	return changed
}
