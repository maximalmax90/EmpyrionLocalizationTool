package validate

import "elt/internal/localize"

type Policy struct {
	FileType       localize.FileType
	StrictTagOrder bool
	IgnoreMismatch bool
	CheckTags      bool
	CheckNewlines  bool
	Normalize      bool
	SkipForAI      bool
}

func DefaultPolicy(ft localize.FileType) Policy {
	return Policy{
		FileType:       ft,
		StrictTagOrder: true,
		CheckTags:      true,
		CheckNewlines:  true,
		Normalize:      true,
	}
}

func ValidateTextPair(english, translation string, policy Policy) Result {
	text := translation
	var fixes []string
	if policy.Normalize {
		text, fixes = NormalizeTranslation(english, text)
	}

	if policy.SkipForAI {
		return Result{OK: true, Text: text, Fixes: fixes}
	}

	srcForValidation := english
	textForValidation := text
	if policy.CheckTags {
		srcForValidation = normalizeMarkupForValidation(english, policy.FileType)
		textForValidation = normalizeMarkupForValidation(text, policy.FileType)
	}

	var preIssues []Issue
	if policy.CheckTags && MarkupPolicyForFileType(policy.FileType).AllowBB {
		seen := map[string]bool{}
		for _, raw := range malformedSupportedBBCodeTokens(text) {
			if seen[raw] {
				continue
			}
			seen[raw] = true
			preIssues = append(preIssues, Issue{Code: "invalid_bbcode_tag", Token: raw})
		}
	}

	ok, issues := ValidateTranslation(srcForValidation, textForValidation, Options{
		FileType:       policy.FileType,
		StrictTagOrder: policy.StrictTagOrder,
		IgnoreMismatch: policy.IgnoreMismatch,
		CheckTags:      policy.CheckTags,
		CheckNewlines:  policy.CheckNewlines,
	})

	all := dedupIssues(append(preIssues, issues...))
	return Result{OK: ok && len(all) == 0, Text: text, Issues: all, Fixes: fixes}
}
