package validate

import (
	"regexp"
	"strings"

	"elt/internal/localize"
)

const angleAlt = `(?i:<#[0-9A-Fa-f]{6}>` +
	`|<\s*/\s*(?:b|i|u|br|a|color|size)\s*>` +
	`|<\s*(?:b|i|u|br)\s*>` +
	`|<\s*(?:a|color|size)\b[^<>]*=[^<>]*>)`

var (
	reAngle     = regexp.MustCompile(angleAlt)
	reAngleFull = regexp.MustCompile(`^` + angleAlt + `$`)

	reBBFull = regexp.MustCompile(`^` + bbAlt + `$`)

	reShortHexHTML      = regexp.MustCompile(`(?i)<#[0-9A-Fa-f]{6}>`)
	reInvalidEmptyColor = regexp.MustCompile(`(?i)<\s*color\s*=\s*>`)
	reColorOpen         = regexp.MustCompile(`(?i)<\s*color\b[^>]*>`)
	reColorClose        = regexp.MustCompile(`(?i)<\s*/\s*color\s*>`)

	reAngleTrailingSpace = regexp.MustCompile(`\s+>$`)
)

func bracketImbalance(text, open, close string) int {
	return strings.Count(text, open) - strings.Count(text, close)
}

func basicBalanceChecks(english, translation string, checkCurly, checkAngle bool) []Issue {
	var issues []Issue
	if checkCurly && bracketImbalance(translation, "{", "}") != bracketImbalance(english, "{", "}") {
		issues = append(issues, Issue{Code: "unbalanced_curly"})
	}
	if checkAngle && bracketImbalance(translation, "<", ">") != bracketImbalance(english, "<", ">") {
		issues = append(issues, Issue{Code: "unbalanced_angle"})
	}
	return issues
}

func findInvalidAngleTagIssues(text string) []Issue {
	var issues []Issue
	for _, m := range reInvalidEmptyColor.FindAllString(text, -1) {
		issues = append(issues, Issue{Code: "invalid_empty_color_value", Token: m})
	}
	return issues
}

func tokensCounter(text string) map[string]int {
	counts := map[string]int{}
	for _, m := range reAngle.FindAllString(text, -1) {
		counts[reAngleTrailingSpace.ReplaceAllString(m, ">")]++
	}
	for _, m := range reSupportedBBCodeToken.FindAllString(text, -1) {
		counts[m]++
	}
	for tok, n := range controlTokenCounts(text) {
		counts[tok] += n
	}
	return counts
}

func classifyToken(tok string) string {
	if kind := classifyControlToken(tok); kind != "" {
		return kind
	}
	if reAngleFull.MatchString(tok) {
		return "angle_tag"
	}
	if reBBFull.MatchString(tok) {
		return "bb_tag"
	}
	return "token"
}

type Options struct {
	FileType       localize.FileType
	StrictTagOrder bool
	IgnoreMismatch bool
	CheckTags      bool
	CheckNewlines  bool
}

func ValidateTranslation(english, translation string, opts Options) (bool, []Issue) {
	policy := MarkupPolicyForFileType(opts.FileType)
	allowBB := policy.AllowBB && opts.CheckTags
	allowAngle := policy.AllowAngle && opts.CheckTags

	var issues []Issue
	issues = append(issues, basicBalanceChecks(english, translation, true, allowAngle)...)

	if allowAngle {
		issues = append(issues, findInvalidAngleTagIssues(translation)...)

		openColor := len(reColorOpen.FindAllString(translation, -1)) +
			len(reShortHexHTML.FindAllString(translation, -1)) -
			len(reColorClose.FindAllString(translation, -1))
		if openColor > 0 {

			issues = append(issues, Issue{Code: "missing_angle_tag", Token: "</color>", Count: openColor})
		}
	}

	if allowBB && (opts.FileType == localize.FileTypeLocalization || opts.FileType == localize.FileTypePDA || opts.FileType == localize.FileTypeLocalizationVanilla) {
		issues = append(issues, validateBBCodeColorBlocks(translation, opts.StrictTagOrder)...)
	}

	var engT, trT map[string]int
	if opts.IgnoreMismatch {
		engT, trT = map[string]int{}, map[string]int{}
	} else {
		engT = tokensCounter(english)
		trT = tokensCounter(translation)
	}

	if !opts.CheckTags {
		stripMarkupTokens(engT)
		stripMarkupTokens(trT)
	}
	if !opts.CheckNewlines {
		delete(engT, `\n`)
		delete(trT, `\n`)
	}

	missing := positiveDiff(engT, trT)
	extra := positiveDiff(trT, engT)

	for _, tok := range sortedKeys(missing) {
		kind := classifyToken(tok)
		if (kind == "angle_tag" && !allowAngle) || (kind == "bb_tag" && !allowBB) {
			continue
		}
		issues = append(issues, Issue{Code: "missing_" + kind, Token: tok, Count: missing[tok]})
	}
	for _, tok := range sortedKeys(extra) {
		kind := classifyToken(tok)
		if (kind == "angle_tag" && !allowAngle) || (kind == "bb_tag" && !allowBB) {
			continue
		}
		issues = append(issues, Issue{Code: "extra_" + kind, Token: tok, Count: extra[tok]})
	}

	if opts.CheckNewlines && !opts.IgnoreMismatch && !strings.Contains(english, "\n") && strings.Contains(translation, "\n") {
		issues = append(issues, Issue{Code: "unexpected_real_newlines"})
	}

	return len(issues) == 0, issues
}
