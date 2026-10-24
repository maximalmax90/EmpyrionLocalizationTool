package project

import (
	"fmt"
	"sort"
	"strings"
)

type KeyFilter string

const (
	FilterAll          KeyFilter = ""
	FilterUntranslated KeyFilter = "untranslated"
	FilterStale        KeyFilter = "stale"
	FilterNeedsReview  KeyFilter = "needs_review"
	FilterNeedsWork    KeyFilter = "needs_work"

	FilterNeedsConfirmation KeyFilter = "needs_confirmation"

	FilterOK KeyFilter = "ok"

	FilterFlagged KeyFilter = "flagged"
)

type KeyListItem struct {
	Key         string
	English     string
	Translation Translation
}

func matchesFilter(tr Translation, filter KeyFilter) bool {
	switch filter {
	case FilterAll:
		return true
	case FilterUntranslated:
		return tr.Text == ""
	case FilterStale:
		return tr.Stale
	case FilterNeedsReview:
		return len(tr.Issues) > 0
	case FilterNeedsWork:
		return tr.NeedsWork()
	case FilterNeedsConfirmation:
		return !tr.NeedsWork() && tr.Source == "machine"
	case FilterOK:
		return !tr.NeedsWork() && tr.Source != "machine"
	case FilterFlagged:
		return tr.NeedsReview
	default:
		return true
	}
}

func matchesSearch(key, english, translation, search string) bool {
	if search == "" {
		return true
	}
	search = strings.ToLower(search)
	return strings.Contains(strings.ToLower(key), search) ||
		strings.Contains(strings.ToLower(english), search) ||
		strings.Contains(strings.ToLower(translation), search)
}

func ListKeys(ft *FileTrack, lang string, filter KeyFilter, search string, onlyKeys map[string]bool, offset, limit int) (items []KeyListItem, total int) {

	matched := make([]KeyListItem, 0, len(ft.Order))
	for _, key := range ft.Order {
		rec, ok := ft.Keys[key]
		if !ok || rec.Status != KeyActive {
			continue
		}
		if onlyKeys != nil && !onlyKeys[key] {
			continue
		}
		tr := rec.Translations[lang]
		if !matchesSearch(key, rec.English, tr.Text, search) {
			continue
		}
		if !matchesFilter(tr, filter) {
			continue
		}
		matched = append(matched, KeyListItem{Key: key, English: rec.English, Translation: tr})
	}

	total = len(matched)
	if offset < 0 {
		offset = 0
	}
	if offset > len(matched) {
		offset = len(matched)
	}
	end := len(matched)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}
	return matched[offset:end], total
}

type KeyDiff struct {
	Key            string
	Kind           string
	OldEnglish     string
	NewEnglish     string
	OldTranslation string
	NewTranslation string
}

const (
	DiffAdded   = "added"
	DiffChanged = "changed"
	DiffRemoved = "removed"
)

type diffEntry struct {
	english     string
	translation string
}

func DiffFileTracks(from, to *FileTrack, lang string) []KeyDiff {
	entryFor := func(rec *KeyRecord) diffEntry {
		e := diffEntry{english: rec.English}
		if lang != "" {
			e.translation = rec.Translations[lang].Text
		}
		return e
	}

	fromActive := map[string]diffEntry{}
	if from != nil {
		for k, rec := range from.Keys {
			if rec.Status == KeyActive {
				fromActive[k] = entryFor(rec)
			}
		}
	}
	toActive := map[string]diffEntry{}
	if to != nil {
		for k, rec := range to.Keys {
			if rec.Status == KeyActive {
				toActive[k] = entryFor(rec)
			}
		}
	}

	out := make([]KeyDiff, 0, len(toActive))
	for k, newE := range toActive {
		oldE, existed := fromActive[k]
		switch {
		case !existed:
			out = append(out, KeyDiff{Key: k, Kind: DiffAdded, NewEnglish: newE.english, NewTranslation: newE.translation})
		case oldE.english != newE.english || oldE.translation != newE.translation:
			out = append(out, KeyDiff{
				Key: k, Kind: DiffChanged,
				OldEnglish: oldE.english, NewEnglish: newE.english,
				OldTranslation: oldE.translation, NewTranslation: newE.translation,
			})
		}
	}
	for k, oldE := range fromActive {
		if _, stillActive := toActive[k]; !stillActive {
			out = append(out, KeyDiff{Key: k, Kind: DiffRemoved, OldEnglish: oldE.english, OldTranslation: oldE.translation})
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

func ClearTranslation(ft *FileTrack, key, lang string) error {
	rec, ok := ft.Keys[key]
	if !ok {
		return fmt.Errorf("project: unknown key %q", key)
	}
	if rec.Translations == nil {
		rec.Translations = map[string]Translation{}
	}
	rec.Translations[lang] = Translation{}
	return nil
}
