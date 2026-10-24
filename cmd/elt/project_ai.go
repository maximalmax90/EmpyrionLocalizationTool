package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"elt/internal/ai"
	"elt/internal/apiclient"
	"elt/internal/localai"
)

func splitKeysFlag(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func runProjectAIConfigSet(args []string) error {
	fs := flag.NewFlagSet("ai-config-set", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), verbHelp["ai-config-set"])
		fmt.Fprintln(fs.Output(), "\nFlags:")
		fs.PrintDefaults()
	}
	urlF := fs.String("url", "", "OpenAI-compatible base URL, e.g. http://localhost:11434/v1")
	tokenF := fs.String("ai-token", "", "AI provider's bearer token (empty for a provider with no auth layer of its own)")
	modelF := fs.String("model", "", "default model name, e.g. qwen2.5:14b-instruct")
	effortF := fs.String("reasoning-effort", "", `default "reasoning_effort" (none|low|medium|high|max), or empty to omit the field`)
	translatePromptFileF := fs.String("translate-prompt-file", "", "path to a file with the TaskTranslate system prompt (a {lang} token, if present, is substituted with the job's target language); omitted = reset to the built-in default")
	glossaryPromptFileF := fs.String("glossary-prompt-file", "", "path to a file with the TaskGlossary (term-discovery) system prompt; same {lang} substitution and omitted-means-reset rule as --translate-prompt-file")
	if err := fs.Parse(reorderArgs(args)); err != nil {
		return err
	}
	if len(fs.Args()) != 0 || *urlF == "" || *modelF == "" {
		return fmt.Errorf("usage: elt project ai-config-set --url=... --model=... [--ai-token=..] [--reasoning-effort=..] [--translate-prompt-file=..] [--glossary-prompt-file=..]")
	}

	cfg := ai.Config{
		URL: *urlF, Token: *tokenF,
		Default: ai.TaskConfig{Model: *modelF, ReasoningEffort: *effortF},
	}
	if *translatePromptFileF != "" {
		text, err := os.ReadFile(*translatePromptFileF)
		if err != nil {
			return fmt.Errorf("reading --translate-prompt-file: %w", err)
		}
		cfg.Translate = &ai.TaskConfig{SystemPrompt: string(text)}
	}
	if *glossaryPromptFileF != "" {
		text, err := os.ReadFile(*glossaryPromptFileF)
		if err != nil {
			return fmt.Errorf("reading --glossary-prompt-file: %w", err)
		}
		cfg.Glossary = &ai.TaskConfig{SystemPrompt: string(text)}
	}
	localai.SeedDefaultPrompts(&cfg)
	if err := localai.SaveConfig(cfg); err != nil {
		return err
	}
	path, _ := os.UserConfigDir()
	fmt.Printf("AI config saved to %s/elt/ai.json\n", path)
	return nil
}

func runProjectAIConfigStatus(args []string) error {
	fs := flag.NewFlagSet("ai-config-status", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), verbHelp["ai-config-status"])
		fmt.Fprintln(fs.Output(), "\nFlags:")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := localai.LoadConfig()
	if err != nil {
		return err
	}
	if !localai.Configured(cfg) {
		fmt.Println(`{"configured":false}`)
		return nil
	}
	cfg.Token = ""
	fmt.Println(prettyJSON(map[string]any{
		"configured": true, "url": cfg.URL,
		"default": cfg.Default, "translate": cfg.Translate, "glossary": cfg.Glossary, "refine": cfg.Refine,
	}))
	return nil
}

func runProjectAIConfigPromptGet(args []string) error {
	fs := flag.NewFlagSet("ai-config-prompt-get", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), verbHelp["ai-config-prompt-get"])
		fmt.Fprintln(fs.Output(), "\nFlags:")
		fs.PrintDefaults()
	}
	useDefaults := fs.Bool("defaults", false, "print this build's compiled-in default instead of the currently configured prompt")
	if err := fs.Parse(reorderArgs(args)); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) != 1 || (rest[0] != "translate" && rest[0] != "glossary") {
		return fmt.Errorf("usage: elt project ai-config-prompt-get <translate|glossary> [--defaults]")
	}
	task := rest[0]

	if *useDefaults {
		if task == "translate" {
			fmt.Println(localai.DefaultTranslatePromptTemplate)
		} else {
			fmt.Println(localai.DefaultGlossaryDiscoveryPromptTemplate)
		}
		return nil
	}

	cfg, err := localai.LoadConfig()
	if err != nil {
		return err
	}
	if !localai.Configured(cfg) {
		return fmt.Errorf("no AI provider is configured on this machine yet — pass --defaults to see the built-in default, or run ai-config-set first")
	}
	switch task {
	case "translate":
		if cfg.Translate != nil {
			fmt.Println(cfg.Translate.SystemPrompt)
		}
	case "glossary":
		if cfg.Glossary != nil {
			fmt.Println(cfg.Glossary.SystemPrompt)
		}
	}
	return nil
}

func clientFromConnFlags(cf *connFlags) (*apiclient.Client, func(), error) {
	base, cleanup, err := cf.connect()
	if err != nil {
		return nil, nil, err
	}
	return &apiclient.Client{BaseURL: base, Token: currentToken, HTTPClient: currentHTTPClient}, cleanup, nil
}

func runProjectAITranslate(args []string) error {
	var langsF, keysF string
	var force, discoverTerms, priority bool
	cf, rest, err := parseWithConn("ai-translate", args, func(fs *flag.FlagSet) {
		fs.StringVar(&langsF, "langs", "", "comma-separated languages to translate (each key is queued once per language, not bundled — see internal/localai's StartTranslate doc)")
		fs.BoolVar(&force, "force", false, "retranslate every key, even ones that already have a good translation (ignored if --keys is given)")
		fs.StringVar(&keysF, "keys", "", "comma-separated key list to scope the run to instead of the default filter (e.g. from glossary-usage)")
		fs.BoolVar(&discoverTerms, "discover-terms", false, "also run glossary term discovery for every key actually translated (costs one extra AI call per key)")
		fs.BoolVar(&priority, "priority", false, "jump this job's AI calls ahead of any ordinary job's still-queued calls; never interrupts a call already in flight")
	})
	if err != nil {
		return err
	}
	langs := splitKeysFlag(langsF)
	if len(rest) != 1 || len(langs) == 0 {
		return fmt.Errorf("usage: elt project ai-translate <FileType> --langs=Russian[,German,...] [--force] [--keys=K1,K2,...] [--discover-terms] [--priority] (--addr=host:port or --data-dir=dir)")
	}
	fileType := rest[0]

	cfg, err := localai.LoadConfig()
	if err != nil {
		return err
	}
	if !localai.Configured(cfg) {
		return fmt.Errorf("no AI provider is configured on this machine yet — run ai-config-set first")
	}

	client, cleanup, err := clientFromConnFlags(cf)
	if err != nil {
		return err
	}
	defer cleanup()

	m := localai.NewManager()
	req := localai.StartTranslateRequest{Langs: langs, Force: force, Keys: splitKeysFlag(keysF), DiscoverTerms: discoverTerms, Priority: priority}
	job, err := m.StartTranslate(client, cfg, fileType, req)
	if err != nil {
		return err
	}
	return watchLocalJob(m, job.ID)
}

func runProjectGlossaryDiscover(args []string) error {
	var lang, keysF string
	var priority bool
	cf, rest, err := parseWithConn("glossary-discover", args, func(fs *flag.FlagSet) {
		fs.StringVar(&lang, "lang", "", "language whose glossary to add discovered terms to")
		fs.StringVar(&keysF, "keys", "", "comma-separated key list to scope the run to instead of every active key")
		fs.BoolVar(&priority, "priority", false, "jump this job's AI calls ahead of any ordinary job's still-queued calls — see ai-translate --priority's doc")
	})
	if err != nil {
		return err
	}
	if len(rest) != 1 || lang == "" {
		return fmt.Errorf("usage: elt project glossary-discover <FileType> --lang=Russian [--keys=K1,K2,...] [--priority] (--addr=host:port or --data-dir=dir)")
	}
	fileType := rest[0]

	cfg, err := localai.LoadConfig()
	if err != nil {
		return err
	}
	if !localai.Configured(cfg) {
		return fmt.Errorf("no AI provider is configured on this machine yet — run ai-config-set first")
	}

	client, cleanup, err := clientFromConnFlags(cf)
	if err != nil {
		return err
	}
	defer cleanup()

	m := localai.NewManager()
	req := localai.StartDiscoveryRequest{Lang: lang, Keys: splitKeysFlag(keysF), Priority: priority}
	job, err := m.StartDiscovery(client, cfg, fileType, req)
	if err != nil {
		return err
	}
	return watchLocalJob(m, job.ID)
}

func watchLocalJob(m *localai.Manager, id string) error {
	for {
		status, ok := m.GetJob(id)
		if !ok {
			return fmt.Errorf("internal error: job %s vanished from its own manager", id)
		}
		discoverySuffix := ""
		if status.Discovered != 0 || status.DiscoveryErrors != 0 {
			discoverySuffix = fmt.Sprintf(", %d terms discovered (%d discovery errors)", status.Discovered, status.DiscoveryErrors)
		}
		fmt.Fprintf(os.Stderr, "\r%s: %d/%d done, %d skipped, %d errored%s...", status.Status, status.Done, status.Total, status.Skipped, status.Errored, discoverySuffix)
		if status.Status != "running" {
			fmt.Fprintln(os.Stderr)
			fmt.Println(prettyJSON(status))
			return nil
		}
		time.Sleep(1 * time.Second)
	}
}
