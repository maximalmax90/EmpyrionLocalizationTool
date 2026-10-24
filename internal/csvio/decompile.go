package csvio

import (
	"encoding/csv"
	"fmt"
	"io"
	"sort"

	"elt/internal/localize"
)

type DecompileResult struct {
	Table  localize.Table
	Issues []Issue
}

func Decompile(r io.Reader) (DecompileResult, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	cr.LazyQuotes = true

	header, err := cr.Read()
	if err != nil {
		return DecompileResult{}, fmt.Errorf("csvio: reading header: %w", err)
	}
	if len(header) == 0 {
		return DecompileResult{}, fmt.Errorf("csvio: empty header row")
	}

	h := localize.Header{
		KeyColumn: header[0],
		Languages: append([]string(nil), header[1:]...),
	}
	expected := 1 + len(h.Languages)

	var (
		result   DecompileResult
		keyLines = map[string][]int{}
	)
	result.Table.Header = h

	for {
		fields, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return DecompileResult{}, fmt.Errorf("csvio: reading row: %w", err)
		}

		line, _ := cr.FieldPos(0)
		key := fields[0]
		keyLines[key] = append(keyLines[key], line)

		row := localize.Row{
			Key:    key,
			Line:   line,
			Values: make(map[string]string, len(h.Languages)),
		}

		switch {
		case len(fields) == expected:
			for i, lang := range h.Languages {
				row.Values[lang] = fields[i+1]
			}

		case len(fields) < expected:

			for i, lang := range h.Languages {
				if idx := i + 1; idx < len(fields) {
					row.Values[lang] = fields[idx]
				} else {
					row.Values[lang] = ""
				}
			}
			result.Issues = append(result.Issues, Issue{
				Code:     IssueShortRow,
				Severity: SeverityWarn,
				Line:     line,
				Key:      key,
				Detail:   fmt.Sprintf("row has %d field(s), header has %d", len(fields), expected),
			})

		default:

			for i, lang := range h.Languages {
				row.Values[lang] = fields[i+1]
			}
			result.Issues = append(result.Issues, Issue{
				Code:     IssueLongRow,
				Severity: SeverityError,
				Line:     line,
				Key:      key,
				Detail:   fmt.Sprintf("row has %d field(s), header has %d; extra fields were discarded", len(fields), expected),
			})
		}

		result.Table.Rows = append(result.Table.Rows, row)
	}

	for key, lines := range keyLines {
		if len(lines) > 1 {
			result.Issues = append(result.Issues, Issue{
				Code:     IssueDuplicateKey,
				Severity: SeverityWarn,
				Line:     lines[0],
				Key:      key,
				Detail:   fmt.Sprintf("key appears on lines %v", lines),
			})
		}
	}

	sort.Slice(result.Issues, func(i, j int) bool {
		if result.Issues[i].Line != result.Issues[j].Line {
			return result.Issues[i].Line < result.Issues[j].Line
		}
		return result.Issues[i].Code < result.Issues[j].Code
	})

	return result, nil
}
