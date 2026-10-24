package validate

import "sort"

type Issue struct {
	Code  string
	Token string
	Count int
}

type Result struct {
	OK     bool
	Text   string
	Issues []Issue
	Fixes  []string
}

func dedupIssues(issues []Issue) []Issue {
	type key struct {
		code  string
		token string
		count int
	}
	seen := make(map[key]bool, len(issues))
	out := make([]Issue, 0, len(issues))
	for _, iss := range issues {
		k := key{iss.Code, iss.Token, iss.Count}
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, iss)
	}
	return out
}

func positiveDiff(a, b map[string]int) map[string]int {
	out := map[string]int{}
	for tok, cnt := range a {
		if d := cnt - b[tok]; d > 0 {
			out[tok] = d
		}
	}
	return out
}

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func stripMarkupTokens(m map[string]int) {
	for tok := range m {
		kind := classifyToken(tok)
		if kind == "angle_tag" || kind == "bb_tag" {
			delete(m, tok)
		}
	}
}
