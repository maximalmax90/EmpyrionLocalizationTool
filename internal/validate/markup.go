package validate

import (
	"regexp"
	"strings"

	"elt/internal/localize"
)

var supportedBBCodeNames = map[string]bool{
	"b": true, "i": true, "u": true, "url": true,
	"br": true, "c": true, "sub": true, "sup": true,
}

var (
	reAnyBBCodeToken = regexp.MustCompile(`\[[^\[\]]+\]`)

	reSupportedBBCodeToken = regexp.MustCompile(bbAlt)

	reHex6 = regexp.MustCompile(`^[0-9A-Fa-f]{6}$`)
)

const bbAlt = `\[(?:c|/c|b|/b|i|/i|u|/u|sub|/sub|sup|/sup|/url|url=[^\[\]\n]+|[0-9A-Fa-f]{6}|-)\]`

type MarkupPolicy struct {
	AllowBB    bool
	AllowAngle bool
}

func MarkupPolicyForFileType(ft localize.FileType) MarkupPolicy {
	switch ft {
	case localize.FileTypeLocalization, localize.FileTypePDA, localize.FileTypeLocalizationVanilla:
		return MarkupPolicy{AllowBB: true, AllowAngle: false}
	case localize.FileTypeDialogues:
		return MarkupPolicy{AllowBB: false, AllowAngle: true}
	default:
		return MarkupPolicy{AllowBB: true, AllowAngle: true}
	}
}

func normalizeSupportedBBCodeToken(raw string) (string, bool) {
	if !strings.HasPrefix(raw, "[") || !strings.HasSuffix(raw, "]") {
		return "", false
	}
	inner := strings.TrimSpace(raw[1 : len(raw)-1])
	if inner == "" {
		return "", false
	}
	if reHex6.MatchString(inner) {
		return "[" + inner + "]", true
	}
	if inner == "-" {
		return "[-]", true
	}

	closing := strings.HasPrefix(inner, "/")
	if closing {
		inner = strings.TrimSpace(strings.TrimPrefix(inner, "/"))
	}

	if idx := strings.Index(inner, "="); idx >= 0 {
		name := strings.ToLower(strings.TrimSpace(inner[:idx]))
		arg := strings.TrimSpace(inner[idx+1:])
		if closing {
			return "", false
		}
		if name == "url" && arg != "" {
			return "[url=" + arg + "]", true
		}
		return "", false
	}

	name := strings.ToLower(strings.TrimSpace(inner))
	if supportedBBCodeNames[name] {
		if closing {
			return "[/" + name + "]", true
		}
		return "[" + name + "]", true
	}
	return "", false
}

func normalizeMarkupForValidation(text string, ft localize.FileType) string {
	if !MarkupPolicyForFileType(ft).AllowBB {
		return text
	}
	return reAnyBBCodeToken.ReplaceAllStringFunc(text, func(raw string) string {
		if norm, ok := normalizeSupportedBBCodeToken(raw); ok {
			return norm
		}
		return raw
	})
}

func malformedSupportedBBCodeTokens(text string) []string {
	var bad []string
	for _, raw := range reAnyBBCodeToken.FindAllString(text, -1) {
		if norm, ok := normalizeSupportedBBCodeToken(raw); ok && norm != raw {
			bad = append(bad, raw)
		}
	}
	return bad
}
