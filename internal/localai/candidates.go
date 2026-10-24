package localai

import (
	"encoding/json"
	"fmt"
	"strings"
)

func ParseCandidateTerms(raw string) ([]string, error) {
	start := strings.Index(raw, "[")
	end := strings.LastIndex(raw, "]")
	if start == -1 || end == -1 || end < start {
		return nil, fmt.Errorf("ai: glossary discovery response did not contain a JSON array: %q", truncateForError(raw))
	}

	var candidates []string
	if err := json.Unmarshal([]byte(raw[start:end+1]), &candidates); err != nil {
		return nil, fmt.Errorf("ai: parsing glossary discovery response as a JSON string array: %w", err)
	}

	out := make([]string, 0, len(candidates))
	seen := map[string]bool{}
	for _, c := range candidates {
		c = strings.TrimSpace(c)
		key := strings.ToLower(c)
		if c == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, c)
	}
	return out, nil
}

func truncateForError(s string) string {
	const max = 200
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
