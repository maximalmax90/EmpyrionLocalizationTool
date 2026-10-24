package content

import (
	"fmt"
	"strings"
)

type ChainTreeNode struct {
	Label    string           `json:"label"`
	Key      string           `json:"key,omitempty"`
	Kind     string           `json:"kind"`
	Children []*ChainTreeNode `json:"children,omitempty"`
}

const DefaultChainTreeMaxNodes = 4000

func contains(path []string, name string) bool {
	for _, p := range path {
		if p == name {
			return true
		}
	}
	return false
}

func DialogueChainTree(graph DialogueGraph, root string, maxNodes int) (tree *ChainTreeNode, capped bool) {
	if maxNodes <= 0 {
		maxNodes = DefaultChainTreeMaxNodes
	}
	b := &dialogueTreeBuilder{graph: graph, max: maxNodes}
	b.spend()
	return b.build(root, nil), b.capped
}

type dialogueTreeBuilder struct {
	graph  DialogueGraph
	max    int
	count  int
	capped bool
}

func (b *dialogueTreeBuilder) spend() bool {
	if b.count >= b.max {
		b.capped = true
		return false
	}
	b.count++
	return true
}

func (b *dialogueTreeBuilder) build(dlgName string, path []string) *ChainTreeNode {
	node := &ChainTreeNode{Label: dlgName, Kind: "dialogue"}
	d, ok := b.graph[dlgName]
	if !ok {
		return node
	}
	path = append(path, dlgName)

	optionChildren := map[int]*ChainTreeNode{}
	for _, t := range d.Texts {
		v := strings.TrimSpace(t.Value)
		if !IsKeyName(v) {
			continue
		}
		if !b.spend() {
			return node
		}
		child := &ChainTreeNode{Label: v, Key: v, Kind: strings.ToLower(t.Role)}
		node.Children = append(node.Children, child)
		if t.Role == "Option" {
			optionChildren[t.Index] = child
		}
	}

	for _, e := range d.Edges {
		if e.Role != "OptionNext" {
			continue
		}
		opt := optionChildren[e.Index]
		if opt == nil {
			if !b.spend() {
				return node
			}
			opt = &ChainTreeNode{Label: fmt.Sprintf("Option %d", e.Index), Kind: "option"}
			node.Children = append(node.Children, opt)
			optionChildren[e.Index] = opt
		}
		target := strings.TrimSpace(strings.SplitN(e.Target, ",", 2)[0])
		if target == "" {
			continue
		}
		if contains(path, target) {
			if !b.spend() {
				return node
			}
			opt.Children = append(opt.Children, &ChainTreeNode{Label: target, Kind: "cycle"})
			continue
		}
		if !b.spend() {
			return node
		}
		opt.Children = append(opt.Children, b.build(target, path))
	}

	for _, e := range d.Edges {
		if e.Role != "Next" {
			continue
		}
		target := strings.TrimSpace(strings.SplitN(e.Target, ",", 2)[0])
		if target == "" {
			continue
		}
		if contains(path, target) {
			if !b.spend() {
				return node
			}
			node.Children = append(node.Children, &ChainTreeNode{Label: target, Kind: "cycle"})
			continue
		}
		if !b.spend() {
			return node
		}
		node.Children = append(node.Children, b.build(target, path))
	}

	return node
}

func PDAChainTree(chapters []Chapter, root string, maxNodes int) (tree *ChainTreeNode, capped bool) {
	if maxNodes <= 0 {
		maxNodes = DefaultChainTreeMaxNodes
	}
	b := &pdaTreeBuilder{byTitle: pdaTitleMap(chapters), max: maxNodes}
	b.spend()
	return b.build(root, nil), b.capped
}

type pdaTreeBuilder struct {
	byTitle map[string]Chapter
	max     int
	count   int
	capped  bool
}

func (b *pdaTreeBuilder) spend() bool {
	if b.count >= b.max {
		b.capped = true
		return false
	}
	b.count++
	return true
}

func (b *pdaTreeBuilder) addLeaf(node *ChainTreeNode, value, kind string) bool {
	v := strings.TrimSpace(value)
	if !IsKeyName(v) {
		return true
	}
	if !b.spend() {
		return false
	}
	node.Children = append(node.Children, &ChainTreeNode{Label: v, Key: v, Kind: kind})
	return true
}

func (b *pdaTreeBuilder) addLeaves(node *ChainTreeNode, values []string, kind string) bool {
	for _, v := range values {
		if !b.addLeaf(node, v, kind) {
			return false
		}
	}
	return true
}

func (b *pdaTreeBuilder) build(title string, path []string) *ChainTreeNode {
	node := &ChainTreeNode{Label: title, Kind: "chapter"}
	ch, ok := b.byTitle[title]
	if !ok {
		return node
	}
	path = append(path, title)

	if !b.addLeaf(node, ch.ChapterTitle, "chapter_title") {
		return node
	}
	if !b.addLeaf(node, ch.Description, "description") {
		return node
	}
	if !b.addLeaf(node, ch.SkipMessage, "skip_message") {
		return node
	}
	if !b.addLeaf(node, ch.Group, "group") {
		return node
	}
	if !b.addLeaves(node, ExtractPDAMessageKeys(ch.StartMessage), "start_message") {
		return node
	}
	if !b.addLeaves(node, ExtractPDAMessageKeys(ch.CompletedMessage), "completed_message") {
		return node
	}

	for _, task := range ch.Tasks {
		if !b.addLeaf(node, task.TaskTitle, "task_title") {
			return node
		}
		if !b.addLeaves(node, ExtractPDAMessageKeys(task.StartMessage), "start_message") {
			return node
		}
		for _, action := range task.Actions {
			if !b.addLeaf(node, action.ActionTitle, "action_title") {
				return node
			}
			if !b.addLeaf(node, action.Description, "description") {
				return node
			}
			if !b.addLeaves(node, ExtractPDAMessageKeys(action.CompletedMessage), "completed_message") {
				return node
			}
		}
	}

	var linkTargets []string
	if IsKeyName(ch.ActivateChapterOnCompletion) {
		linkTargets = append(linkTargets, strings.TrimSpace(ch.ActivateChapterOnCompletion))
	}
	for _, rc := range ch.RewardedChapters {
		if IsKeyName(rc) {
			linkTargets = append(linkTargets, strings.TrimSpace(rc))
		}
	}
	for _, target := range linkTargets {
		if contains(path, target) {
			if !b.spend() {
				return node
			}
			node.Children = append(node.Children, &ChainTreeNode{Label: target, Kind: "cycle"})
			continue
		}
		if _, ok := b.byTitle[target]; !ok {

			if !b.spend() {
				return node
			}
			node.Children = append(node.Children, &ChainTreeNode{Label: target, Kind: "chapter"})
			continue
		}
		if !b.spend() {
			return node
		}
		node.Children = append(node.Children, b.build(target, path))
	}

	return node
}
