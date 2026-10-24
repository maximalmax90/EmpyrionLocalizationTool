package content

import (
	"bufio"
	"io"
	"regexp"
	"strconv"
	"strings"
)

var (
	reDialogueStart = regexp.MustCompile(`\{\s*\+.*Dialogue\s+Name\s*:\s*(.+)$`)
	reKeyValue      = regexp.MustCompile(`^\s*([A-Za-z0-9_]+)\s*:\s*(.+?)\s*$`)
)

var skipTokens = []string{"/*", "Execute", "Variable", "If", "Functions", "#"}

type DialogueTextRef struct {
	Role  string
	Index int
	Value string
}

type DialogueEdge struct {
	Role   string
	Index  int
	Target string
}

type DialogueNode struct {
	Texts      []DialogueTextRef
	OptionText map[int]string
	Edges      []DialogueEdge
}

type DialogueGraph map[string]*DialogueNode

func ParseDialoguesECF(r io.Reader) (DialogueGraph, error) {
	graph := DialogueGraph{}
	cur := ""

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	for scanner.Scan() {
		line := scanner.Text()

		if m := reDialogueStart.FindStringSubmatch(line); m != nil {
			dlg := strings.TrimSpace(m[1])
			if dlg != "" {
				cur = dlg
				if _, ok := graph[dlg]; !ok {
					graph[dlg] = &DialogueNode{OptionText: map[int]string{}}
				}
			}
			continue
		}

		if cur == "" {
			continue
		}
		if strings.TrimSpace(line) == "}" {
			continue
		}
		if containsAny(line, skipTokens) {
			continue
		}

		m2 := reKeyValue.FindStringSubmatch(line)
		if m2 == nil {
			continue
		}
		name := m2[1]
		value := strings.TrimSpace(m2[2])
		if value == "" || value == "End" {
			continue
		}

		first := strings.TrimSpace(strings.SplitN(value, ",", 2)[0])

		node := graph[cur]
		switch {
		case name == "NPCName" || name == "Output":
			node.Texts = append(node.Texts, DialogueTextRef{Role: name, Index: -1, Value: first})

		case strings.HasPrefix(name, "Option_"):
			idx := indexSuffix(name, "Option_")
			node.Texts = append(node.Texts, DialogueTextRef{Role: "Option", Index: idx, Value: first})
			if idx >= 0 {
				node.OptionText[idx] = first
			}

		case strings.HasPrefix(name, "Next_"):
			idx := indexSuffix(name, "Next_")
			node.Edges = append(node.Edges, DialogueEdge{Role: "Next", Index: idx, Target: first})

		case strings.HasPrefix(name, "OptionNext_"):
			idx := indexSuffix(name, "OptionNext_")
			node.Edges = append(node.Edges, DialogueEdge{Role: "OptionNext", Index: idx, Target: first})
		}
	}

	return graph, scanner.Err()
}

func containsAny(s string, tokens []string) bool {
	for _, tok := range tokens {
		if strings.Contains(s, tok) {
			return true
		}
	}
	return false
}

func indexSuffix(name, prefix string) int {
	n, err := strconv.Atoi(strings.TrimPrefix(name, prefix))
	if err != nil {
		return -1
	}
	return n
}
