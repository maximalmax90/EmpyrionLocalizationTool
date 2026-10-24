package project

import (
	"sort"

	"elt/internal/localize"
)

type SyncResult struct {
	Added     []string
	Changed   []string
	Removed   []string
	Unchanged int
}

func Sync(ft *FileTrack, table localize.Table) SyncResult {

	res := SyncResult{Added: make([]string, 0), Changed: make([]string, 0), Removed: make([]string, 0)}
	seen := make(map[string]bool, len(table.Rows))

	for _, row := range table.Rows {
		english := row.Value("English")
		seen[row.Key] = true
		existing, ok := ft.Keys[row.Key]
		if !ok {
			rec := &KeyRecord{
				English:      english,
				Status:       KeyActive,
				Translations: make(map[string]Translation, len(ft.Languages)),
			}
			for _, lang := range ft.Languages {
				rec.Translations[lang] = Translation{}
			}
			ft.Keys[row.Key] = rec
			ft.Order = append(ft.Order, row.Key)
			res.Added = append(res.Added, row.Key)
			continue
		}

		existing.Status = KeyActive

		if existing.English != english {
			existing.English = english
			for lang, tr := range existing.Translations {
				if tr.Text != "" {
					tr.Stale = true
					existing.Translations[lang] = tr
				}
			}
			res.Changed = append(res.Changed, row.Key)
		} else {
			res.Unchanged++
		}
	}

	for key, rec := range ft.Keys {
		if !seen[key] && rec.Status == KeyActive {
			rec.Status = KeyRemoved
			res.Removed = append(res.Removed, key)
		}
	}

	sort.Strings(res.Added)
	sort.Strings(res.Changed)
	sort.Strings(res.Removed)
	return res
}
