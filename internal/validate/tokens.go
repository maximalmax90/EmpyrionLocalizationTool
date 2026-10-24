package validate

import (
	"regexp"
)

const (
	TokenPlaceholder    = "placeholder"
	TokenEngineMarker   = "q_token"
	TokenEscapedNewline = "escaped_newline"
)

var (
	rePlaceholder = regexp.MustCompile(`\{[^{}]+\}`)

	reEngineMarker = regexp.MustCompile(`@[dpqw](?:\d+)?`)

	reEscapedNewline = regexp.MustCompile(`\\n`)
)

func classifyControlToken(tok string) string {
	if rePlaceholder.MatchString(tok) && rePlaceholder.FindString(tok) == tok {
		return TokenPlaceholder
	}
	if reEngineMarker.MatchString(tok) && reEngineMarker.FindString(tok) == tok {
		return TokenEngineMarker
	}
	if tok == `\n` {
		return TokenEscapedNewline
	}
	return ""
}

func controlTokenCounts(text string) map[string]int {
	counts := map[string]int{}
	for _, m := range rePlaceholder.FindAllString(text, -1) {
		counts[m]++
	}
	for _, m := range reEngineMarker.FindAllString(text, -1) {
		counts[m]++
	}
	for _, m := range reEscapedNewline.FindAllString(text, -1) {
		counts[m]++
	}
	return counts
}
