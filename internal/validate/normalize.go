package validate

import (
	"regexp"
	"strings"
)

var reMultiSpace = regexp.MustCompile(`[ \t]{2,}`)

func NormalizeTranslation(english, translation string) (string, []string) {
	var fixes []string

	tr := translation
	if trimmed := strings.TrimSpace(tr); trimmed != tr {
		fixes = append(fixes, "strip")
		tr = trimmed
	}

	engHasEscaped := strings.Contains(english, `\n`)
	engHasReal := strings.Contains(english, "\n")

	switch {
	case engHasEscaped:

		if strings.Contains(tr, "\n") {
			tr = strings.ReplaceAll(tr, "\r\n", "\n")
			tr = strings.ReplaceAll(tr, "\r", "\n")
			tr = strings.ReplaceAll(tr, "\n", `\n`)
			fixes = append(fixes, "convert_newlines_to_escaped")
		}
		if strings.Contains(tr, "\r") {
			tr = strings.ReplaceAll(tr, "\r\n", "\n")
			tr = strings.ReplaceAll(tr, "\r", "\n")
			fixes = append(fixes, "normalize_crlf")
		}

	case engHasReal:

		if strings.Contains(tr, `\n`) && !strings.Contains(tr, "\n") {
			tr = strings.ReplaceAll(tr, `\n`, "\n")
			fixes = append(fixes, "convert_escaped_to_newlines")
		}
		if strings.Contains(tr, "\r") {
			tr = strings.ReplaceAll(tr, "\r\n", "\n")
			tr = strings.ReplaceAll(tr, "\r", "\n")
			fixes = append(fixes, "normalize_crlf")
		}

	default:

		if strings.Contains(tr, "\n") || strings.Contains(tr, "\r") {
			tr = strings.ReplaceAll(tr, "\r\n", " ")
			tr = strings.ReplaceAll(tr, "\r", " ")
			tr = strings.ReplaceAll(tr, "\n", " ")
			fixes = append(fixes, "remove_newlines")
		}
	}

	if !strings.Contains(english, "  ") {
		if collapsed := reMultiSpace.ReplaceAllString(tr, " "); collapsed != tr {
			tr = collapsed
			fixes = append(fixes, "collapse_spaces")
		}
	}

	return tr, fixes
}
