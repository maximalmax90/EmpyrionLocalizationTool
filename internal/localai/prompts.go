package localai

import (
	"fmt"
	"strings"

	"elt/internal/ai"
)

const promptLangPlaceholder = "{lang}"

func RenderPromptTemplate(tmpl, lang string) string {
	return strings.ReplaceAll(tmpl, promptLangPlaceholder, lang)
}

const DefaultTranslatePromptTemplate = "" +
	"You are translating video game localization text from English into {lang}. " +
	"Output only the {lang} translation of the user's message — no explanation, " +
	"preamble, or extra commentary.\n\n" +
	"Preserve these exactly as they appear in the source, character for " +
	"character — translate only the surrounding text:\n" +
	"- BBCode tags (e.g. [b] [/b] [c][RRGGBB]...[-][/c]) and HTML-like tags " +
	"(e.g. <b> <color=#RRGGBB>)\n" +
	"- {placeholder} tokens\n" +
	"- Engine marker sequences such as @q0, @w2, @p9 — keep the exact Latin " +
	"letters and digits; never substitute look-alike characters from another " +
	"alphabet\n" +
	"- Escape sequences like \\n\n\n" +
	"Text wrapped in angle or square brackets that is NOT one of the tags " +
	"above (e.g. \"< SomeFile.ext >\", \"[ CharacterName ]\") is ordinary " +
	"decorative text, not markup — handle its content the same way you would " +
	"any other text, without altering the brackets themselves.\n\n" +
	"Do not invent an expansion for an acronym or abbreviation you don't " +
	"recognize — leave it exactly as written.\n\n" +
	"Produce natural, fluent {lang} that preserves meaning and tone (this is " +
	"sci-fi/space-survival game text). Avoid literal word-for-word order " +
	"where it breaks natural {lang} grammar. Keep capitalization where it " +
	"matters (proper names, acronyms) — don't change case without reason.\n\n" +
	"Before answering, silently check: does every tag, placeholder, and " +
	"marker sequence in your translation match the source exactly, in both " +
	"content and count? If anything differs, fix it. Then output only the " +
	"final translated text."

const DefaultGlossaryDiscoveryPromptTemplate = "" +
	"You are reviewing video game localization text (English source, to be " +
	"translated into {lang}) to find candidate glossary terms — invented or " +
	"fictional proper nouns specific to this game's world: faction names, " +
	"item/weapon/ship/vehicle names, place names, character names, and " +
	"similarly invented in-universe terminology.\n\n" +
	"Do NOT flag: ordinary English words, common technical or sci-fi " +
	"vocabulary that isn't specific to this game, generic game-UI text " +
	"(\"Inventory\", \"Health\", \"Settings\", \"Continue\"), or a term already " +
	"listed below as already known.\n\n" +
	"Decomposition rule: when a candidate is a multi-word phrase built from " +
	"an invented proper noun plus a common descriptor (e.g. \"<Faction> " +
	"Trooper\", \"<Item> Tank\", \"<Faction> Commander\"), propose BOTH the " +
	"full phrase AND the invented proper noun alone as separate candidates, " +
	"since the bare name is reused across many other lines this exact " +
	"phrase doesn't appear in. Do not do this for phrases where the " +
	"descriptor itself is not a common noun likely to recur with other " +
	"proper nouns.\n\n" +
	"Respond with nothing but a JSON array of strings — the exact substrings " +
	"from the source text that are candidate terms, copied verbatim (a " +
	"candidate must appear as-is somewhere in the source text, including as " +
	"a substring inside a longer phrase). If there are none, respond with " +
	"an empty array: []. No explanation, no markdown formatting, no extra " +
	"text before or after the array."

func SeedDefaultPrompts(cfg *ai.Config) {
	if cfg.Translate == nil {
		cfg.Translate = &ai.TaskConfig{}
	}
	if cfg.Translate.SystemPrompt == "" {
		cfg.Translate.SystemPrompt = DefaultTranslatePromptTemplate
	}
	if cfg.Glossary == nil {
		cfg.Glossary = &ai.TaskConfig{}
	}
	if cfg.Glossary.SystemPrompt == "" {
		cfg.Glossary.SystemPrompt = DefaultGlossaryDiscoveryPromptTemplate
	}
}

type glossaryTermForPrompt struct {
	Term        string `json:"term"`
	Action      string `json:"action"`
	Translation string `json:"translation"`
	Inflect     bool   `json:"inflect"`
	Notes       string `json:"notes"`
}

func GlossaryPromptBlock(used []glossaryTermForPrompt, lang string) string {
	var b strings.Builder
	b.WriteString("Glossary — this text contains the following known terms. Use exactly ")
	b.WriteString("this handling for each one, adapting only what's explicitly allowed:\n")
	for _, t := range used {
		switch t.Action {
		case "keep":
			fmt.Fprintf(&b, "- %q: keep exactly as written in English, do not translate it.\n", t.Term)
		case "transliterate":
			fmt.Fprintf(&b, "- %q: always render as %q (a fixed transliteration, not a fresh translation).\n", t.Term, t.Translation)
		case "translate":
			if t.Inflect {
				fmt.Fprintf(&b, "- %q: translate as %q, grammatically inflected/declined as %s grammar requires in context.\n", t.Term, t.Translation, lang)
			} else {
				fmt.Fprintf(&b, "- %q: translate as %q exactly, in this form, without inflecting or declining it.\n", t.Term, t.Translation)
			}
		}
		if t.Notes != "" {
			fmt.Fprintf(&b, "  Note: %s\n", t.Notes)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func TermsUsedIn(terms []glossaryTermForPrompt, english string) []glossaryTermForPrompt {
	lower := strings.ToLower(english)
	var used []glossaryTermForPrompt
	for _, t := range terms {
		if strings.Contains(lower, strings.ToLower(t.Term)) {
			used = append(used, t)
		}
	}
	return used
}

func KnownTermsBlock(known []glossaryTermForPrompt) string {
	if len(known) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Already-known terms — do not propose any of these again:\n")
	for _, t := range known {
		fmt.Fprintf(&b, "- %s\n", t.Term)
	}
	return strings.TrimRight(b.String(), "\n")
}
