package project

import (
	"sort"

	"elt/internal/localize"
)

type ImportResult struct {
	Applied                map[string][]string
	SkippedUnknownKey      []string
	SkippedEnglishMismatch []string
}

func Import(ft *FileTrack, table localize.Table, langs []string, matchEnglish bool) ImportResult {
	res := ImportResult{
		Applied:                make(map[string][]string, len(langs)),
		SkippedUnknownKey:      make([]string, 0),
		SkippedEnglishMismatch: make([]string, 0),
	}
	for _, lang := range langs {
		res.Applied[lang] = make([]string, 0)
	}

	for _, row := range table.Rows {
		rec, ok := ft.Keys[row.Key]
		if !ok || rec.Status != KeyActive {
			res.SkippedUnknownKey = append(res.SkippedUnknownKey, row.Key)
			continue
		}

		rowEnglish := row.Value("English")
		if matchEnglish && rowEnglish != rec.English {
			res.SkippedEnglishMismatch = append(res.SkippedEnglishMismatch, row.Key)
			continue
		}

		for _, lang := range langs {
			text := row.Value(lang)
			if text == "" {
				continue
			}
			if rec.Translations == nil {
				rec.Translations = map[string]Translation{}
			}
			rec.Translations[lang] = Translation{
				Text:        text,
				Source:      "manual",
				Stale:       !matchEnglish && rowEnglish != rec.English,
				NeedsReview: rec.Translations[lang].NeedsReview,
			}
			res.Applied[lang] = append(res.Applied[lang], row.Key)
		}
	}

	for _, lang := range langs {
		sort.Strings(res.Applied[lang])
	}
	sort.Strings(res.SkippedUnknownKey)
	sort.Strings(res.SkippedEnglishMismatch)
	return res
}
