package content

import (
	"regexp"
	"sort"
	"strings"
)

var reKeyName = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func IsKeyName(s string) bool {
	s = strings.TrimSpace(s)
	return s != "" && reKeyName.MatchString(s)
}

func ExtractPDAMessageKeys(value string) []string {
	var keys []string
	for _, part := range strings.Split(value, "|") {
		part = strings.TrimSpace(part)
		if part == "" || strings.Contains(part, ";") {
			continue
		}
		if IsKeyName(part) {
			keys = append(keys, part)
		}
	}
	return keys
}

func addKey(v string, out *[]string) {
	v = strings.TrimSpace(v)
	if IsKeyName(v) {
		*out = append(*out, v)
	}
}

func pdaChapterKeys(ch Chapter) []string {
	var out []string
	addKey(ch.ChapterTitle, &out)
	addKey(ch.Description, &out)
	addKey(ch.SkipMessage, &out)
	addKey(ch.Group, &out)
	out = append(out, ExtractPDAMessageKeys(ch.StartMessage)...)
	out = append(out, ExtractPDAMessageKeys(ch.CompletedMessage)...)

	for _, task := range ch.Tasks {
		addKey(task.TaskTitle, &out)
		out = append(out, ExtractPDAMessageKeys(task.StartMessage)...)
		for _, action := range task.Actions {
			addKey(action.ActionTitle, &out)
			addKey(action.Description, &out)
			out = append(out, ExtractPDAMessageKeys(action.CompletedMessage)...)
		}
	}

	for _, rc := range ch.RewardedChapters {
		addKey(rc, &out)
	}
	return out
}

func pdaChapterLinks(ch Chapter) []string {
	var out []string
	if IsKeyName(ch.ActivateChapterOnCompletion) {
		out = append(out, strings.TrimSpace(ch.ActivateChapterOnCompletion))
	}
	for _, rc := range ch.RewardedChapters {
		if IsKeyName(rc) {
			out = append(out, strings.TrimSpace(rc))
		}
	}
	return out
}

func pdaTitleMap(chapters []Chapter) map[string]Chapter {
	byTitle := make(map[string]Chapter, len(chapters))
	for _, ch := range chapters {
		title := strings.TrimSpace(ch.ChapterTitle)
		if title != "" && IsKeyName(title) {
			byTitle[title] = ch
		}
	}
	return byTitle
}

func PDAChainRoots(chapters []Chapter) []string {
	byTitle := pdaTitleMap(chapters)
	roots := make([]string, 0, len(byTitle))
	for title := range byTitle {
		roots = append(roots, title)
	}
	sort.Strings(roots)
	return roots
}

func PDAChain(chapters []Chapter, root string) []string {
	byTitle := pdaTitleMap(chapters)
	return walkPDAChain(byTitle, root)
}

func walkPDAChain(byTitle map[string]Chapter, root string) []string {
	seen := map[string]bool{}
	var ordered []string

	onPath := map[string]bool{}
	var walk func(title string)
	walk = func(title string) {
		if onPath[title] {
			return
		}
		ch, ok := byTitle[title]
		if !ok {
			return
		}
		onPath[title] = true
		defer delete(onPath, title)

		for _, key := range pdaChapterKeys(ch) {
			if !seen[key] {
				seen[key] = true
				ordered = append(ordered, key)
			}
		}
		for _, next := range pdaChapterLinks(ch) {
			if _, ok := byTitle[next]; ok {
				walk(next)
			}
		}
	}

	walk(root)
	return ordered
}

func BuildPDAChains(chapters []Chapter) map[string][]string {
	byTitle := pdaTitleMap(chapters)
	roots := make([]string, 0, len(byTitle))
	for title := range byTitle {
		roots = append(roots, title)
	}
	sort.Strings(roots)

	chains := make(map[string][]string, len(roots))
	for _, root := range roots {
		chains[root] = walkPDAChain(byTitle, root)
	}
	return chains
}

func DialogueChainRoots(graph DialogueGraph) []string {
	roots := make([]string, 0, len(graph))
	for name := range graph {
		if strings.TrimSpace(name) != "" {
			roots = append(roots, name)
		}
	}
	sort.Strings(roots)
	return roots
}

func DialogueChain(graph DialogueGraph, root string) []string {
	return walkDialogueChain(graph, root)
}

func walkDialogueChain(graph DialogueGraph, root string) []string {
	seen := map[string]bool{}
	var ordered []string

	onPath := map[string]bool{}
	var walk func(name string)
	walk = func(name string) {
		if onPath[name] {
			return
		}
		node, ok := graph[name]
		if !ok {
			return
		}
		onPath[name] = true
		defer delete(onPath, name)

		for _, t := range node.Texts {
			if IsKeyName(t.Value) {
				v := strings.TrimSpace(t.Value)
				if !seen[v] {
					seen[v] = true
					ordered = append(ordered, v)
				}
			}
		}

		for _, e := range node.Edges {
			if e.Role != "Next" && e.Role != "OptionNext" {
				continue
			}
			target := strings.TrimSpace(strings.SplitN(e.Target, ",", 2)[0])
			if target != "" {
				walk(target)
			}
		}
	}

	walk(root)
	return ordered
}

func BuildDialogueChains(graph DialogueGraph) map[string][]string {
	roots := DialogueChainRoots(graph)
	chains := make(map[string][]string, len(roots))
	for _, root := range roots {
		chains[root] = walkDialogueChain(graph, root)
	}
	return chains
}
