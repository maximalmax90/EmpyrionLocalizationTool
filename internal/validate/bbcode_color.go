package validate

import "strings"

type stackItem struct {
	tag        string
	needsColor bool
	hasColor   bool
}

var (
	fmtOpen = map[string]bool{
		"[b]": true, "[i]": true, "[u]": true, "[sub]": true, "[sup]": true,
	}
	fmtClose = map[string]bool{
		"[/b]": true, "[/i]": true, "[/u]": true, "[/sub]": true, "[/sup]": true,
	}
)

func isColorOpenToken(tok string) bool {
	return len(tok) == 8 && tok[0] == '[' && tok[7] == ']' && reHex6.MatchString(tok[1:7])
}

func validateBBCodeColorBlocks(text string, strictTagOrder bool) []Issue {
	var issues []Issue

	type tok struct {
		isBB  bool
		value string
	}
	var toks []tok
	last := 0
	for _, loc := range reSupportedBBCodeToken.FindAllStringIndex(text, -1) {
		if loc[0] > last {
			if seg := text[last:loc[0]]; seg != "" {
				toks = append(toks, tok{isBB: false, value: seg})
			}
		}
		toks = append(toks, tok{isBB: true, value: text[loc[0]:loc[1]]})
		last = loc[1]
	}
	if last < len(text) {
		if seg := text[last:]; seg != "" {
			toks = append(toks, tok{isBB: false, value: seg})
		}
	}

	var stack []stackItem

	inC := func() bool {
		for _, it := range stack {
			if it.tag == "c" {
				return true
			}
		}
		return false
	}

	nearestC := func() int {
		for i := len(stack) - 1; i >= 0; i-- {
			if stack[i].tag == "c" {
				return i
			}
		}
		return -1
	}
	nearestTagged := func(tag string) int {
		for i := len(stack) - 1; i >= 0; i-- {
			if stack[i].tag == tag {
				return i
			}
		}
		return -1
	}

	for _, t := range toks {
		if !t.isBB {
			anyNeedsColor := false
			for _, it := range stack {
				if it.tag == "c" && it.needsColor {
					anyNeedsColor = true
					break
				}
			}
			if anyNeedsColor && strings.TrimSpace(t.value) != "" {
				issues = append(issues, Issue{Code: "bb_color_missing_code"})
				if i := nearestC(); i >= 0 {
					stack[i].needsColor = false
				}
			}
			continue
		}

		v := t.value
		switch {
		case v == "[c]":
			stack = append(stack, stackItem{tag: "c", needsColor: true})
			continue

		case v == "[/c]":
			idx := nearestTagged("c")
			if idx < 0 {
				issues = append(issues, Issue{Code: "bb_color_close_without_open", Token: v})
				continue
			}

			allowedDirectColor := len(stack) >= 2 &&
				stack[len(stack)-1].tag == "color" &&
				stack[len(stack)-2].tag == "c" &&
				idx == len(stack)-2

			if strictTagOrder && idx != len(stack)-1 && !allowedDirectColor {
				issues = append(issues, Issue{Code: "bb_tag_order", Token: v})
			}

			if !stack[idx].hasColor {
				issues = append(issues, Issue{Code: "bb_color_missing_code"})
			}

			if strictTagOrder {
				stack = stack[:idx]
			} else {
				for j := len(stack) - 1; j > idx; j-- {
					if stack[j].tag == "color" {
						stack = append(stack[:j], stack[j+1:]...)
					}
				}
				stack = append(stack[:idx], stack[idx+1:]...)
			}
			continue

		case fmtOpen[v]:
			name := strings.Trim(v, "[]")
			stack = append(stack, stackItem{tag: name})
			continue

		case fmtClose[v]:
			closing := strings.Trim(v, "[]/")
			if strictTagOrder {
				if len(stack) == 0 || stack[len(stack)-1].tag != closing {
					issues = append(issues, Issue{Code: "bb_unbalanced", Token: v})
					continue
				}
				stack = stack[:len(stack)-1]
				continue
			}
			idx := nearestTagged(closing)
			if idx < 0 {
				issues = append(issues, Issue{Code: "bb_unbalanced", Token: v})
				continue
			}
			stack = append(stack[:idx], stack[idx+1:]...)
			continue

		case isColorOpenToken(v):
			if !inC() {

				continue
			}
			stack = append(stack, stackItem{tag: "color"})
			if i := nearestC(); i >= 0 {
				stack[i].needsColor = false
				stack[i].hasColor = true
			}
			continue

		case v == "[-]":
			if len(stack) == 0 {
				continue
			}
			idx := nearestTagged("color")
			if idx < 0 {
				continue
			}
			stack = append(stack[:idx], stack[idx+1:]...)
			continue

		default:

		}
	}

	for _, it := range stack {
		switch it.tag {
		case "c":
			if !it.hasColor {
				issues = append(issues, Issue{Code: "bb_color_missing_code"})
			}
			issues = append(issues, Issue{Code: "bb_color_unclosed"})
		case "color":

		}
	}

	return issues
}
