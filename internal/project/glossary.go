package project

import (
	"fmt"
	"sort"
	"strings"

	"elt/internal/localize"
)

type TermAction string

const (
	TermKeep          TermAction = "keep"
	TermTranslate     TermAction = "translate"
	TermTransliterate TermAction = "transliterate"
)

type TermStatus string

const (
	TermDraft    TermStatus = "draft"
	TermApproved TermStatus = "approved"

	TermRejected TermStatus = "rejected"
)

type GlossaryKeyRef struct {
	FileType string
	Key      string
}

type GlossaryTerm struct {
	Term        string
	Action      TermAction
	Translation string
	Inflect     bool
	Status      TermStatus
	Notes       string

	MatchedKeys []GlossaryKeyRef
}

func normalizeTermKey(term string) string {
	return strings.ToLower(strings.TrimSpace(term))
}

func findTermIndex(terms []GlossaryTerm, term string) int {
	key := normalizeTermKey(term)
	for i, t := range terms {
		if normalizeTermKey(t.Term) == key {
			return i
		}
	}
	return -1
}

func AddGlossaryTerm(p *Project, lang string, term GlossaryTerm) error {
	name := strings.TrimSpace(term.Term)
	if name == "" {
		return fmt.Errorf("project: glossary term text is required")
	}
	if p.Glossary == nil {
		p.Glossary = map[string][]GlossaryTerm{}
	}
	if findTermIndex(p.Glossary[lang], name) >= 0 {
		return fmt.Errorf("project: %q is already a glossary term for %s", name, lang)
	}
	term.Term = name
	if term.Status == "" {
		term.Status = TermDraft
	}
	p.Glossary[lang] = append(p.Glossary[lang], term)
	return nil
}

func UpdateGlossaryTerm(p *Project, lang, termText string, updated GlossaryTerm) error {
	terms := p.Glossary[lang]
	idx := findTermIndex(terms, termText)
	if idx < 0 {
		return fmt.Errorf("project: no glossary term %q for %s", termText, lang)
	}
	newName := strings.TrimSpace(updated.Term)
	if newName == "" {
		return fmt.Errorf("project: glossary term text is required")
	}
	if normalizeTermKey(newName) != normalizeTermKey(termText) {
		if findTermIndex(terms, newName) >= 0 {
			return fmt.Errorf("project: %q is already a glossary term for %s", newName, lang)
		}
	}
	updated.Term = newName
	if updated.Status == "" {
		updated.Status = terms[idx].Status
	}
	updated.MatchedKeys = terms[idx].MatchedKeys
	terms[idx] = updated
	p.Glossary[lang] = terms
	return nil
}

func RemoveGlossaryTerm(p *Project, lang, termText string) error {
	terms := p.Glossary[lang]
	idx := findTermIndex(terms, termText)
	if idx < 0 {
		return fmt.Errorf("project: no glossary term %q for %s", termText, lang)
	}
	p.Glossary[lang] = append(terms[:idx], terms[idx+1:]...)
	return nil
}

func ClearGlossary(p *Project, lang string) (removed int) {
	removed = len(p.Glossary[lang])
	delete(p.Glossary, lang)
	return removed
}

func SetGlossaryTermStatus(p *Project, lang, termText string, status TermStatus) error {
	terms := p.Glossary[lang]
	idx := findTermIndex(terms, termText)
	if idx < 0 {
		return fmt.Errorf("project: no glossary term %q for %s", termText, lang)
	}
	terms[idx].Status = status
	return nil
}

func ImportGlossaryTerms(p *Project, lang string, terms []GlossaryTerm) (added, updated int, errs []string) {
	for i, term := range terms {
		if strings.TrimSpace(term.Term) == "" {
			errs = append(errs, fmt.Sprintf("term %d: term text is required", i))
			continue
		}
		if err := AddGlossaryTerm(p, lang, term); err == nil {
			added++
			continue
		}
		if err := UpdateGlossaryTerm(p, lang, term.Term, term); err != nil {
			errs = append(errs, fmt.Sprintf("term %d (%q): %v", i, term.Term, err))
			continue
		}
		updated++
	}
	return added, updated, errs
}

func GlossaryTerms(p *Project, lang string) []GlossaryTerm {
	terms := append([]GlossaryTerm(nil), p.Glossary[lang]...)
	sort.Slice(terms, func(i, j int) bool {
		return normalizeTermKey(terms[i].Term) < normalizeTermKey(terms[j].Term)
	})
	return terms
}

type GlossaryStatusFilter string

const (
	GlossaryFilterDefault  GlossaryStatusFilter = ""
	GlossaryFilterDraft    GlossaryStatusFilter = "draft"
	GlossaryFilterApproved GlossaryStatusFilter = "approved"
	GlossaryFilterRejected GlossaryStatusFilter = "rejected"
	GlossaryFilterAll      GlossaryStatusFilter = "all"
)

func FilterGlossaryTerms(terms []GlossaryTerm, filter GlossaryStatusFilter) []GlossaryTerm {
	out := make([]GlossaryTerm, 0, len(terms))
	switch filter {
	case GlossaryFilterAll:
		return append(out, terms...)
	case GlossaryFilterDraft, GlossaryFilterApproved, GlossaryFilterRejected:
		for _, t := range terms {
			if string(t.Status) == string(filter) {
				out = append(out, t)
			}
		}
	default:
		for _, t := range terms {
			if t.Status != TermRejected {
				out = append(out, t)
			}
		}
	}
	return out
}

func ApprovedGlossaryTerms(p *Project, lang string) []GlossaryTerm {
	all := GlossaryTerms(p, lang)
	out := make([]GlossaryTerm, 0, len(all))
	for _, t := range all {
		if t.Status == TermApproved {
			out = append(out, t)
		}
	}
	return out
}

func GlossaryTermsUsedIn(terms []GlossaryTerm, text string) []GlossaryTerm {
	lower := strings.ToLower(text)
	var used []GlossaryTerm
	for _, t := range terms {
		if strings.Contains(lower, strings.ToLower(t.Term)) {
			used = append(used, t)
		}
	}
	return used
}

type TermUsageItem struct {
	FileType string
	Key      string
	English  string
}

func TermUsage(p *Project, term string) []TermUsageItem {
	lower := strings.ToLower(term)
	var out []TermUsageItem
	for ft, track := range p.Files {
		for _, key := range track.Order {
			rec, ok := track.Keys[key]
			if !ok || rec.Status != KeyActive {
				continue
			}
			if strings.Contains(strings.ToLower(rec.English), lower) {
				out = append(out, TermUsageItem{FileType: string(ft), Key: key, English: rec.English})
			}
		}
	}
	return out
}

func FillGlossaryMatches(p *Project, lang string) (termsUpdated, totalMatches int) {
	terms := p.Glossary[lang]
	if len(terms) == 0 {
		return 0, 0
	}
	lowerTerms := make([]string, len(terms))
	for i, t := range terms {
		lowerTerms[i] = strings.ToLower(t.Term)
	}
	matches := make([][]GlossaryKeyRef, len(terms))

	for ft, track := range p.Files {
		for _, key := range track.Order {
			rec, ok := track.Keys[key]
			if !ok || rec.Status != KeyActive {
				continue
			}
			english := strings.ToLower(rec.English)
			var matchedIdx []int
			for i, lt := range lowerTerms {
				if lt != "" && strings.Contains(english, lt) {
					matchedIdx = append(matchedIdx, i)
				}
			}
			for _, i := range matchedIdx {
				subsumed := false
				for _, j := range matchedIdx {
					if i != j && len(lowerTerms[j]) > len(lowerTerms[i]) && strings.Contains(lowerTerms[j], lowerTerms[i]) {
						subsumed = true
						break
					}
				}
				if !subsumed {
					matches[i] = append(matches[i], GlossaryKeyRef{FileType: string(ft), Key: key})
					totalMatches++
				}
			}
		}
	}

	for i := range terms {
		terms[i].MatchedKeys = matches[i]
	}
	p.Glossary[lang] = terms
	return len(terms), totalMatches
}

func PruneStaleGlossaryMatches(p *Project, lang, termText string) (updated GlossaryTerm, removed int, err error) {
	terms := p.Glossary[lang]
	idx := findTermIndex(terms, termText)
	if idx < 0 {
		return GlossaryTerm{}, 0, fmt.Errorf("project: no glossary term %q for %s", termText, lang)
	}
	kept := make([]GlossaryKeyRef, 0, len(terms[idx].MatchedKeys))
	for _, ref := range terms[idx].MatchedKeys {
		track, ok := p.Files[localize.FileType(ref.FileType)]
		if !ok {
			removed++
			continue
		}
		rec, ok := track.Keys[ref.Key]
		if !ok || rec.Status != KeyActive {
			removed++
			continue
		}
		kept = append(kept, ref)
	}
	terms[idx].MatchedKeys = kept
	p.Glossary[lang] = terms
	return terms[idx], removed, nil
}
