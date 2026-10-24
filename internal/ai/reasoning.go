package ai

import (
	"regexp"
	"strings"
)

var thinkBlock = regexp.MustCompile(`(?is)<think(?:ing)?>.*?</think(?:ing)?>\s*`)

var unterminatedThink = regexp.MustCompile(`(?is)<think(?:ing)?>.*$`)

func StripReasoning(text string) (visible, reasoning string) {
	var removed []string
	stripped := thinkBlock.ReplaceAllStringFunc(text, func(m string) string {
		removed = append(removed, m)
		return ""
	})
	stripped = unterminatedThink.ReplaceAllStringFunc(stripped, func(m string) string {
		removed = append(removed, m)
		return ""
	})
	return strings.TrimSpace(stripped), strings.Join(removed, "\n")
}
