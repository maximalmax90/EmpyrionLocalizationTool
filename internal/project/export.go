package project

import "elt/internal/localize"

func Export(ft *FileTrack) localize.Table {
	header := localize.Header{
		KeyColumn: "KEY",
		Languages: append([]string{"English"}, localize.KnownLanguages...),
	}

	table := localize.Table{Header: header}
	for _, key := range ft.Order {
		rec, ok := ft.Keys[key]
		if !ok || rec.Status != KeyActive {
			continue
		}
		values := make(map[string]string, len(header.Languages))
		values["English"] = rec.English
		for _, lang := range ft.Languages {
			values[lang] = rec.Translations[lang].Text
		}
		table.Rows = append(table.Rows, localize.Row{Key: key, Values: values})
	}
	return table
}

func ExportMerged(primary, fallback *FileTrack) localize.Table {
	table := Export(primary)
	if fallback == nil {
		return table
	}
	covered := make(map[string]bool, len(table.Rows))
	for _, row := range table.Rows {
		covered[row.Key] = true
	}
	for _, key := range fallback.Order {
		if covered[key] {
			continue
		}
		rec, ok := fallback.Keys[key]
		if !ok || rec.Status != KeyActive {
			continue
		}
		values := make(map[string]string, len(table.Header.Languages))
		values["English"] = rec.English
		for _, lang := range fallback.Languages {
			values[lang] = rec.Translations[lang].Text
		}
		table.Rows = append(table.Rows, localize.Row{Key: key, Values: values})
	}
	return table
}
