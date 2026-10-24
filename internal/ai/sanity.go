package ai

import (
	"strings"
	"unicode"
)

const IssueScriptMismatch = "ai_script_mismatch"

var languageScripts = map[string][]*unicode.RangeTable{
	"Russian":    {unicode.Cyrillic},
	"Ukrainian":  {unicode.Cyrillic},
	"Belarusian": {unicode.Cyrillic},
	"Bulgarian":  {unicode.Cyrillic},
	"Serbian":    {unicode.Cyrillic},

	"Japanese": {unicode.Hiragana, unicode.Katakana, unicode.Han},
	"Chinese":  {unicode.Han},
	"Korean":   {unicode.Hangul, unicode.Han},

	"Thai":       {unicode.Thai},
	"Arabic":     {unicode.Arabic},
	"Hebrew":     {unicode.Hebrew},
	"Greek":      {unicode.Greek},
	"Hindi":      {unicode.Devanagari},
	"Vietnamese": {unicode.Latin},

	"English":    {unicode.Latin},
	"German":     {unicode.Latin},
	"French":     {unicode.Latin},
	"Spanish":    {unicode.Latin},
	"Italian":    {unicode.Latin},
	"Portuguese": {unicode.Latin},
	"Polish":     {unicode.Latin},
	"Turkish":    {unicode.Latin},
}

var alwaysAllowedTables = []*unicode.RangeTable{
	unicode.Latin, unicode.Number, unicode.Punct, unicode.Symbol, unicode.Space, unicode.Mn,
}

func WordScriptIssues(text, lang string) []string {
	scripts, ok := languageScripts[lang]
	if !ok {
		return nil
	}

	var bad []string
	for _, word := range strings.Fields(text) {
		if hasUnexpectedScript(word, scripts) {
			bad = append(bad, word)
		}
	}
	return bad
}

func hasUnexpectedScript(word string, scripts []*unicode.RangeTable) bool {
	for _, r := range word {
		if unicode.IsOneOf(alwaysAllowedTables, r) {
			continue
		}
		if unicode.IsOneOf(scripts, r) {
			continue
		}
		return true
	}
	return false
}
