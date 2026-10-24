package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"

	"elt/internal/store"
)

func runProject(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf(projectUsage)
	}
	verb, rest := args[0], args[1:]

	switch verb {
	case "-h", "--help", "help":
		fmt.Print(projectUsage)
		return nil
	case "status":
		return runProjectStatus(rest)
	case "track":
		return runProjectTrack(rest)
	case "sync":
		return runProjectSync(rest)
	case "export":
		return runProjectExport(rest)
	case "translate":
		return runProjectTranslate(rest)
	case "confirm":
		return runProjectConfirm(rest)
	case "validate":
		return runProjectValidate(rest)
	case "ai-config-set":
		return runProjectAIConfigSet(rest)
	case "ai-config-status":
		return runProjectAIConfigStatus(rest)
	case "ai-config-prompt-get":
		return runProjectAIConfigPromptGet(rest)
	case "ai-translate":
		return runProjectAITranslate(rest)
	case "glossary-discover":
		return runProjectGlossaryDiscover(rest)
	case "glossary-list":
		return runProjectGlossaryList(rest)
	case "glossary-add":
		return runProjectGlossaryAdd(rest)
	case "glossary-update":
		return runProjectGlossaryUpdate(rest)
	case "glossary-remove":
		return runProjectGlossaryRemove(rest)
	case "glossary-approve":
		return runProjectGlossaryApprove(rest)
	case "glossary-unapprove":
		return runProjectGlossaryUnapprove(rest)
	case "glossary-reject":
		return runProjectGlossaryReject(rest)
	case "glossary-usage":
		return runProjectGlossaryUsage(rest)
	case "lock":
		return runProjectLock(rest)
	case "unlock":
		return runProjectUnlock(rest)
	case "lock-status":
		return runProjectLockStatus(rest)
	case "lock-keys":
		return runProjectLockKeys(rest)
	case "glossary-lock":
		return runProjectGlossaryLock(rest)
	case "glossary-unlock":
		return runProjectGlossaryUnlock(rest)
	case "split":
		return runProjectSplit(rest)
	case "batches":
		return runProjectBatches(rest)
	case "get-batch":
		return runProjectGetBatch(rest)
	case "merge":
		return runProjectMerge(rest)
	case "reclaim":
		return runProjectReclaim(rest)
	case "history":
		return runProjectHistory(rest)
	case "commit":
		return runProjectCommit(rest)
	case "remote-set":
		return runProjectRemoteSet(rest)
	case "remote-status":
		return runProjectRemoteStatus(rest)
	case "push":
		return runProjectPush(rest)
	case "clone":
		return runProjectClone(rest)
	case "rollback":
		return runProjectRollback(rest)
	case "gc":
		return runProjectGC(rest)
	case "compact":
		return runProjectCompact(rest)
	case "chain-source-upload":
		return runProjectChainSourceUpload(rest)
	case "chain-roots":
		return runProjectChainRoots(rest)
	case "chain":
		return runProjectChain(rest)
	case "keys":
		return runProjectKeys(rest)
	case "diff":
		return runProjectDiff(rest)
	case "clear":
		return runProjectClear(rest)
	default:
		return fmt.Errorf("elt project: unknown verb %q\n\n%s", verb, projectUsage)
	}
}

const projectUsage = `elt project <verb> [flags]

Verbs:
  status                                          project + per-file summary
  track <FileType> --langs=Russian,German         start tracking a file type
  sync <FileType> <csv-file> [--force]            upload+reconcile a source CSV
  export <FileType> [--out file.csv] [--rev id]   download the compiled CSV,
                                                   optionally as of a past commit
  translate <FileType> <key> <lang> <text>        set one key's translation
  confirm <FileType> <key> <lang>                 mark an AI draft reviewed (machine -> manual)
                                                   without changing its text
  validate <FileType> --lang=Russian              re-check translations, commit
  ai-config-set --url=... [--ai-token=..] [--model=..] [--reasoning-effort=..]
                [--translate-prompt-file=f] [--glossary-prompt-file=f]
                                                   configure THIS MACHINE's own AI provider
                                                   (~/.config/elt/ai.json, no --addr/--data-dir
                                                   — never project state, same per-machine
                                                   config "elt gui --addr=" uses); omitting a
                                                   prompt file resets that prompt to the
                                                   built-in default (see ai-config-prompt-get)
  ai-config-status                                show this machine's configured AI provider
                                                   (never the token)
  ai-config-prompt-get <translate|glossary> [--defaults]
                                                   print one task's raw prompt text to
                                                   stdout, ready to redirect to a file,
                                                   edit, and pass back via ai-config-set's
                                                   --translate-prompt-file/--glossary-prompt-file
                                                   (--defaults prints the built-in default
                                                   instead of what's configured on this machine)
  ai-translate <FileType> --langs=Russian[,German,...] [--force]
               [--keys=K1,K2,...] [--discover-terms] [--priority]
                                                   run an AI-translate pass to completion, in
                                                   this CLI process itself, using this
                                                   machine's own ai-config-set — works over
                                                   either --addr or --data-dir (see connect.go);
                                                   scope defaults to every key that needs work
                                                   (or every key, with --force), or exactly
                                                   --keys if given (e.g. the output of
                                                   glossary-usage, to retranslate only what a
                                                   glossary change affects) — each key is
                                                   queued once per language listed in --langs,
                                                   not bundled together, so a --priority run
                                                   can jump in between a key's own languages,
                                                   not just between different keys;
                                                   --discover-terms also proposes new draft
                                                   glossary terms for every key actually
                                                   translated (one extra AI call per key);
                                                   --priority jumps this run's AI calls ahead
                                                   of any other ai-translate/glossary-discover
                                                   process on this machine — never interrupts
                                                   a call already in flight. Blocks and prints
                                                   progress until done; no separate status/
                                                   cancel/list verb — Ctrl+C cancels.
  glossary-discover <FileType> --lang=Russian [--keys=K1,K2,...] [--priority]
                                                   ai-translate's standalone counterpart:
                                                   propose new draft glossary terms for every
                                                   active key (or --keys); never auto-approves
                                                   anything — review with glossary-list/
                                                   -approve after. Same local-AI/--addr-or-
                                                   --data-dir/blocks-until-done shape as
                                                   ai-translate above.
  glossary-list --lang=Russian [--status=draft|approved|rejected|all]
                                                   list glossary terms (default: draft +
                                                   approved, rejected hidden — pass
                                                   --status to see one status or all)
  glossary-add --lang=Russian --term=... [--action=keep|translate|transliterate]
               [--translation=...] [--inflect] [--notes=...]
                                                   add a new glossary term (starts draft)
  glossary-update --lang=Russian --term=... [same flags as glossary-add]
                                                   replace an existing term's fields
  glossary-remove --lang=Russian --term=...       delete a glossary term
  glossary-approve --lang=Russian --term=...      mark a term approved (only approved
                                                   terms are ever injected into AI prompts)
  glossary-unapprove --lang=Russian --term=...    revert a term to draft (also un-rejects)
  glossary-reject --lang=Russian --term=...       blacklist a term (e.g. a bad
                                                   glossary-discover candidate) so it's
                                                   never resuggested or re-addable; hidden
                                                   from the default glossary-list
  glossary-usage --term=...                       keys whose English text contains
                                                   this term (not language-scoped)
  lock <FileType> <key> --lang=Russian [--owner=..]
                                                   take (or heartbeat-renew) an advisory
                                                   editing lock on one key+language pair
                                                   (locks never span every language a key
                                                   has, so independent translators on
                                                   different languages never block each
                                                   other)
  unlock <FileType> <key> --lang=Russian [--owner=..]
                                                   release it
  lock-status <FileType> <key> --lang=Russian     check who (if anyone) holds it
  lock-keys <FileType> <key> [<key>...] --lang=Russian [--owner=..]
                                                   lock a whole set atomically for one
                                                   language (a group or chain worked as
                                                   one unit) — all or none, never a
                                                   partial lock
  glossary-lock --lang=Russian --term=... [--owner=..]
                                                   same, for one glossary term
  glossary-unlock --lang=Russian --term=... [--owner=..]
  split <FileType> --lang=Russian --n=2           check out N disjoint batches
  batches <FileType>                              list open batches
  get-batch <FileType> <batch-id> [--out file]    download one batch's content
  merge <FileType> <batch-id> <translations-file> apply a completed batch
  reclaim <FileType> <batch-id>                   abandon an open batch
  history [--limit N]                             show commit history
  commit <message>                                checkpoint pending changes
  remote-set --url=... [--username=..] [--remote-token=..] [--push=manual|on-commit]
                                                   configure the backup/team remote
  remote-status                                   show the configured remote (never the token)
  push                                             push history to the configured remote
  clone --url=... [--username=..] [--token=..] <dest-dir>
                                                   bootstrap a new local project from a
                                                   remote (no --addr/--data-dir; this IS
                                                   the data-dir being created)
  rollback --rev=<commit> [--message=..]          restore a past revision as the new
                                                   current state (a new forward commit,
                                                   history is never rewritten)
  gc                                               compact git storage (safe, no content change)
  compact --boundary=<commit> [--message=..]      squash history up to and including boundary
                                                   into one new root commit, replaying later
                                                   commits on top unchanged (content is never
                                                   altered — see internal/store.Compact's doc).
                                                   Admin-only on a multi-user server; permanently
                                                   discards everything before boundary — unlike
                                                   rollback above, this genuinely rewrites history
  chain-source-upload <Dialogues|PDA> <source-file>
                                                   upload Dialogues.ecf/PDA.yaml as the
                                                   project's chain source (admin-only);
                                                   chain-roots/chain below 409 until this
                                                   has been done at least once
  chain-roots <Dialogues|PDA>                     list chain roots (dialogue names /
                                                   chapter titles) from the uploaded
                                                   chain source
  chain <Dialogues|PDA> <root>                    keys reachable from one chain root
                                                   (navigation only — parsed fresh from
                                                   the uploaded source on every call, same
                                                   as a TUI/GUI would do)
  keys <FileType> --lang=Russian [--status=..] [--search=..] [--offset=N] [--limit=N]
                                                   list/filter/page through keys
  diff <FileType> --from=<rev> [--to=<rev>]       what changed between two revisions
                                                   (default --to is the current state)
  clear <FileType> --lang=Russian <key> [<key>...] reset one or more translations back
                                                   to untranslated (e.g. after reviewing
                                                   a diff and deciding a stale one no
                                                   longer applies)

Every verb also takes --addr host:port (talk to a running "elt serve"; also
accepts a full scheme://host[:port] for a TLS-terminated server, plus
--insecure to skip certificate verification against elt's own self-signed
certificate, which has no real CA behind it), or --data-dir dir (open/init
a local project and serve it for this one command).

Flags and positional arguments may appear in any order, but flags must use
the --name=value form (not --name value); use "--" to stop flag parsing if
a positional argument itself needs to start with "-".
`

func reorderArgs(args []string) []string {
	var flags, positional []string
	for i, a := range args {
		if a == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if strings.HasPrefix(a, "-") {
			flags = append(flags, a)
		} else {
			positional = append(positional, a)
		}
	}
	return append(flags, positional...)
}

func parseWithConn(name string, args []string, extra func(fs *flag.FlagSet)) (*connFlags, []string, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)

	if help, ok := verbHelp[name]; ok {
		fs.Usage = func() {
			fmt.Fprint(fs.Output(), help)
			fmt.Fprintln(fs.Output(), "\nFlags:")
			fs.PrintDefaults()
		}
	}
	cf := addConnFlags(fs)
	if extra != nil {
		extra(fs)
	}
	if err := fs.Parse(reorderArgs(args)); err != nil {
		return nil, nil, err
	}
	return cf, fs.Args(), nil
}

func runProjectStatus(args []string) error {
	cf, _, err := parseWithConn("status", args, nil)
	if err != nil {
		return err
	}
	base, cleanup, err := cf.connect()
	if err != nil {
		return err
	}
	defer cleanup()

	var out any
	if err := apiCallJSON("GET", base, "/status", nil, &out); err != nil {
		return err
	}
	fmt.Println(prettyJSON(out))
	return nil
}

func runProjectTrack(args []string) error {
	var langs string
	cf, rest, err := parseWithConn("track", args, func(fs *flag.FlagSet) {
		fs.StringVar(&langs, "langs", "", "comma-separated target languages, e.g. Russian,German")
	})
	if err != nil {
		return err
	}
	if len(rest) != 1 {
		return fmt.Errorf("usage: elt project track <FileType> --langs=Russian,German")
	}
	base, cleanup, err := cf.connect()
	if err != nil {
		return err
	}
	defer cleanup()

	path := fmt.Sprintf("/files/%s/track?langs=%s", url.PathEscape(rest[0]), url.QueryEscape(langs))
	var out any
	if err := apiCallJSON("POST", base, path, nil, &out); err != nil {
		return err
	}
	fmt.Println(prettyJSON(out))
	return nil
}

func runProjectSync(args []string) error {
	var force bool
	cf, rest, err := parseWithConn("sync", args, func(fs *flag.FlagSet) {
		fs.BoolVar(&force, "force", false, "sync even if error-severity structural issues were found")
	})
	if err != nil {
		return err
	}
	if len(rest) != 2 {
		return fmt.Errorf("usage: elt project sync <FileType> <csv-file> [--force]")
	}
	base, cleanup, err := cf.connect()
	if err != nil {
		return err
	}
	defer cleanup()

	body, err := os.ReadFile(rest[1])
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/files/%s/sync", url.PathEscape(rest[0]))
	if force {
		path += "?force=true"
	}
	data, err := apiCall("POST", base, path, body)
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func runProjectExport(args []string) error {
	var out, rev string
	cf, rest, err := parseWithConn("export", args, func(fs *flag.FlagSet) {
		fs.StringVar(&out, "out", "", "write the CSV here instead of stdout")
		fs.StringVar(&rev, "rev", "", "export as of this commit/revision (see 'elt project history') instead of the current state")
	})
	if err != nil {
		return err
	}
	if len(rest) != 1 {
		return fmt.Errorf("usage: elt project export <FileType> [--out file.csv] [--rev <commit>]")
	}
	base, cleanup, err := cf.connect()
	if err != nil {
		return err
	}
	defer cleanup()

	path := fmt.Sprintf("/files/%s/export", url.PathEscape(rest[0]))
	if rev != "" {
		path += "?rev=" + url.QueryEscape(rev)
	}
	data, err := apiCall("GET", base, path, nil)
	if err != nil {
		return err
	}
	if out == "" {
		fmt.Print(string(data))
		return nil
	}
	return os.WriteFile(out, data, 0o644)
}

func runProjectTranslate(args []string) error {
	var source, owner string
	cf, rest, err := parseWithConn("translate", args, func(fs *flag.FlagSet) {
		fs.StringVar(&source, "source", "manual", `"manual" or "machine"`)
		fs.StringVar(&owner, "owner", "", "your identity, if this key is locked and this server has no authenticated user on the connection")
	})
	if err != nil {
		return err
	}
	if len(rest) != 4 {
		return fmt.Errorf("usage: elt project translate <FileType> <key> <lang> <text> [--owner=..]")
	}
	base, cleanup, err := cf.connect()
	if err != nil {
		return err
	}
	defer cleanup()

	path := fmt.Sprintf("/files/%s/keys/%s/translations/%s", url.PathEscape(rest[0]), url.PathEscape(rest[1]), url.PathEscape(rest[2])) + joinQuery(ownerQuery(owner))
	req := map[string]string{"text": rest[3], "source": source}
	var respOut any
	if err := apiCallJSON("PUT", base, path, req, &respOut); err != nil {
		return err
	}
	fmt.Println(prettyJSON(respOut))
	return nil
}

func runProjectConfirm(args []string) error {
	var owner string
	cf, rest, err := parseWithConn("confirm", args, func(fs *flag.FlagSet) {
		fs.StringVar(&owner, "owner", "", "your identity, if this key is locked and this server has no authenticated user on the connection")
	})
	if err != nil {
		return err
	}
	if len(rest) != 3 {
		return fmt.Errorf("usage: elt project confirm <FileType> <key> <lang> [--owner=..]")
	}
	base, cleanup, err := cf.connect()
	if err != nil {
		return err
	}
	defer cleanup()

	path := fmt.Sprintf("/files/%s/keys/%s/translations/%s/confirm", url.PathEscape(rest[0]), url.PathEscape(rest[1]), url.PathEscape(rest[2])) + joinQuery(ownerQuery(owner))
	var out any
	if err := apiCallJSON("POST", base, path, nil, &out); err != nil {
		return err
	}
	fmt.Println(prettyJSON(out))
	return nil
}

func runProjectValidate(args []string) error {
	var lang string
	cf, rest, err := parseWithConn("validate", args, func(fs *flag.FlagSet) {
		fs.StringVar(&lang, "lang", "", "language to validate")
	})
	if err != nil {
		return err
	}
	if len(rest) != 1 || lang == "" {
		return fmt.Errorf("usage: elt project validate <FileType> --lang=Russian")
	}
	base, cleanup, err := cf.connect()
	if err != nil {
		return err
	}
	defer cleanup()

	path := fmt.Sprintf("/files/%s/validate?lang=%s", url.PathEscape(rest[0]), url.QueryEscape(lang))
	var out any
	if err := apiCallJSON("POST", base, path, nil, &out); err != nil {
		return err
	}
	fmt.Println(prettyJSON(out))
	return nil
}

func runProjectSplit(args []string) error {
	var lang string
	var n int
	var onlyNeedsWork bool
	cf, rest, err := parseWithConn("split", args, func(fs *flag.FlagSet) {
		fs.StringVar(&lang, "lang", "", "language to split")
		fs.IntVar(&n, "n", 2, "number of batches")
		fs.BoolVar(&onlyNeedsWork, "only-needs-work", true, "only include untranslated/stale/failing keys")
	})
	if err != nil {
		return err
	}
	if len(rest) != 1 || lang == "" {
		return fmt.Errorf("usage: elt project split <FileType> --lang=Russian --n=2")
	}
	base, cleanup, err := cf.connect()
	if err != nil {
		return err
	}
	defer cleanup()

	path := fmt.Sprintf("/files/%s/batches?lang=%s&n=%d&only_needs_work=%t", url.PathEscape(rest[0]), url.QueryEscape(lang), n, onlyNeedsWork)
	var out any
	if err := apiCallJSON("POST", base, path, nil, &out); err != nil {
		return err
	}
	fmt.Println(prettyJSON(out))
	return nil
}

func runProjectBatches(args []string) error {
	cf, rest, err := parseWithConn("batches", args, nil)
	if err != nil {
		return err
	}
	if len(rest) != 1 {
		return fmt.Errorf("usage: elt project batches <FileType>")
	}
	base, cleanup, err := cf.connect()
	if err != nil {
		return err
	}
	defer cleanup()

	var out any
	if err := apiCallJSON("GET", base, fmt.Sprintf("/files/%s/batches", url.PathEscape(rest[0])), nil, &out); err != nil {
		return err
	}
	fmt.Println(prettyJSON(out))
	return nil
}

func runProjectGetBatch(args []string) error {
	var out string
	cf, rest, err := parseWithConn("get-batch", args, func(fs *flag.FlagSet) {
		fs.StringVar(&out, "out", "", "write the batch here instead of stdout")
	})
	if err != nil {
		return err
	}
	if len(rest) != 2 {
		return fmt.Errorf("usage: elt project get-batch <FileType> <batch-id> [--out file.json]")
	}
	base, cleanup, err := cf.connect()
	if err != nil {
		return err
	}
	defer cleanup()

	data, err := apiCall("GET", base, fmt.Sprintf("/files/%s/batches/%s", url.PathEscape(rest[0]), url.PathEscape(rest[1])), nil)
	if err != nil {
		return err
	}
	if out == "" {
		fmt.Println(string(data))
		return nil
	}
	return os.WriteFile(out, data, 0o644)
}

func runProjectMerge(args []string) error {
	cf, rest, err := parseWithConn("merge", args, nil)
	if err != nil {
		return err
	}
	if len(rest) != 3 {
		return fmt.Errorf(`usage: elt project merge <FileType> <batch-id> <translations.json>

translations.json must be a JSON object of exactly the keys the batch was
cut with, mapping each key to its translated text: {"Key1": "text", ...}`)
	}
	base, cleanup, err := cf.connect()
	if err != nil {
		return err
	}
	defer cleanup()

	body, err := os.ReadFile(rest[2])
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/files/%s/batches/%s/merge", url.PathEscape(rest[0]), url.PathEscape(rest[1]))
	data, err := apiCall("POST", base, path, body)
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func runProjectReclaim(args []string) error {
	cf, rest, err := parseWithConn("reclaim", args, nil)
	if err != nil {
		return err
	}
	if len(rest) != 2 {
		return fmt.Errorf("usage: elt project reclaim <FileType> <batch-id>")
	}
	base, cleanup, err := cf.connect()
	if err != nil {
		return err
	}
	defer cleanup()

	path := fmt.Sprintf("/files/%s/batches/%s/reclaim", url.PathEscape(rest[0]), url.PathEscape(rest[1]))
	if _, err := apiCall("POST", base, path, nil); err != nil {
		return err
	}
	fmt.Println("ok")
	return nil
}

func runProjectHistory(args []string) error {
	var limit int
	cf, _, err := parseWithConn("history", args, func(fs *flag.FlagSet) {
		fs.IntVar(&limit, "limit", 0, "max commits to show (0 = all)")
	})
	if err != nil {
		return err
	}
	base, cleanup, err := cf.connect()
	if err != nil {
		return err
	}
	defer cleanup()

	path := "/history"
	if limit > 0 {
		path = fmt.Sprintf("/history?limit=%d", limit)
	}
	var out any
	if err := apiCallJSON("GET", base, path, nil, &out); err != nil {
		return err
	}
	fmt.Println(prettyJSON(out))
	return nil
}

func runProjectCommit(args []string) error {
	cf, rest, err := parseWithConn("commit", args, nil)
	if err != nil {
		return err
	}
	if len(rest) != 1 {
		return fmt.Errorf("usage: elt project commit <message>")
	}
	base, cleanup, err := cf.connect()
	if err != nil {
		return err
	}
	defer cleanup()

	var out any
	if err := apiCallJSON("POST", base, "/commit", map[string]string{"message": rest[0]}, &out); err != nil {
		return err
	}
	fmt.Println(prettyJSON(out))
	return nil
}

func remoteFlags(fs *flag.FlagSet) (url, username, token, push *string) {
	url = fs.String("url", "", "git remote URL (GitHub, GitLab, any self-hosted git-over-HTTPS server)")
	username = fs.String("username", "", `HTTP basic-auth username sent with --token (defaults to "token")`)
	token = fs.String("token", "", "personal access token")
	push = fs.String("push", "", `"manual" (default) or "on-commit" to push automatically after every commit`)
	return
}

func remoteSetFlags(fs *flag.FlagSet) (url, username, token, push *string) {
	url = fs.String("url", "", "git remote URL (GitHub, GitLab, any self-hosted git-over-HTTPS server)")
	username = fs.String("username", "", `HTTP basic-auth username sent with --remote-token (defaults to "token")`)
	token = fs.String("remote-token", "", "personal access token for the git remote (distinct from --token, which authenticates this CLI invocation to an --addr elt server)")
	push = fs.String("push", "", `"manual" (default) or "on-commit" to push automatically after every commit`)
	return
}

func runProjectRemoteSet(args []string) error {
	var urlF, usernameF, tokenF, pushF *string
	cf, rest, err := parseWithConn("remote-set", args, func(fs *flag.FlagSet) {
		urlF, usernameF, tokenF, pushF = remoteSetFlags(fs)
	})
	if err != nil {
		return err
	}
	if len(rest) != 0 || *urlF == "" {
		return fmt.Errorf("usage: elt project remote-set --url=... [--username=..] [--remote-token=..] [--push=manual|on-commit]")
	}
	base, cleanup, err := cf.connect()
	if err != nil {
		return err
	}
	defer cleanup()

	req := map[string]string{"url": *urlF, "username": *usernameF, "token": *tokenF, "push": *pushF}
	var out any
	if err := apiCallJSON("POST", base, "/remote", req, &out); err != nil {
		return err
	}
	fmt.Println(prettyJSON(out))
	return nil
}

func runProjectRemoteStatus(args []string) error {
	cf, _, err := parseWithConn("remote-status", args, nil)
	if err != nil {
		return err
	}
	base, cleanup, err := cf.connect()
	if err != nil {
		return err
	}
	defer cleanup()

	var out any
	if err := apiCallJSON("GET", base, "/remote", nil, &out); err != nil {
		return err
	}
	fmt.Println(prettyJSON(out))
	return nil
}

func runProjectPush(args []string) error {
	cf, _, err := parseWithConn("push", args, nil)
	if err != nil {
		return err
	}
	base, cleanup, err := cf.connect()
	if err != nil {
		return err
	}
	defer cleanup()

	var out any
	if err := apiCallJSON("POST", base, "/push", nil, &out); err != nil {
		return err
	}
	fmt.Println(prettyJSON(out))
	return nil
}

func runProjectClone(args []string) error {
	fs := flag.NewFlagSet("clone", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), verbHelp["clone"])
		fmt.Fprintln(fs.Output(), "\nFlags:")
		fs.PrintDefaults()
	}
	urlF, usernameF, tokenF, _ := remoteFlags(fs)
	if err := fs.Parse(reorderArgs(args)); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) != 1 || *urlF == "" {
		return fmt.Errorf("usage: elt project clone --url=... [--username=..] [--token=..] <dest-dir>")
	}

	s, err := store.CloneFrom(rest[0], store.RemoteConfig{URL: *urlF, Username: *usernameF, Token: *tokenF})
	if err != nil {
		return err
	}
	p, err := s.Load()
	if err != nil {
		return err
	}
	fmt.Printf("cloned %q into %s\n", p.Name, s.Dir())
	return nil
}

func runProjectRollback(args []string) error {
	var rev, message string
	cf, rest, err := parseWithConn("rollback", args, func(fs *flag.FlagSet) {
		fs.StringVar(&rev, "rev", "", "commit/revision to restore as the new current state (see 'elt project history')")
		fs.StringVar(&message, "message", "", "commit message for the rollback (default: mentions the resolved revision)")
	})
	if err != nil {
		return err
	}
	if len(rest) != 0 || rev == "" {
		return fmt.Errorf("usage: elt project rollback --rev=<commit> [--message=...]")
	}
	base, cleanup, err := cf.connect()
	if err != nil {
		return err
	}
	defer cleanup()

	req := map[string]string{"revision": rev, "message": message}
	var out any
	if err := apiCallJSON("POST", base, "/rollback", req, &out); err != nil {
		return err
	}
	fmt.Println(prettyJSON(out))
	return nil
}

func runProjectGC(args []string) error {
	cf, _, err := parseWithConn("gc", args, nil)
	if err != nil {
		return err
	}
	base, cleanup, err := cf.connect()
	if err != nil {
		return err
	}
	defer cleanup()

	var out any
	if err := apiCallJSON("POST", base, "/gc", nil, &out); err != nil {
		return err
	}
	fmt.Println(prettyJSON(out))
	return nil
}

func runProjectCompact(args []string) error {
	var boundary, message string
	cf, rest, err := parseWithConn("compact", args, func(fs *flag.FlagSet) {
		fs.StringVar(&boundary, "boundary", "", "commit/revision to squash history up to and including (see 'elt project history')")
		fs.StringVar(&message, "message", "", "message for the new squashed root commit (default: mentions the resolved boundary)")
	})
	if err != nil {
		return err
	}
	if len(rest) != 0 || boundary == "" {
		return fmt.Errorf("usage: elt project compact --boundary=<commit> [--message=...]")
	}
	base, cleanup, err := cf.connect()
	if err != nil {
		return err
	}
	defer cleanup()

	req := map[string]string{"boundary": boundary, "message": message}
	var out any
	if err := apiCallJSON("POST", base, "/compact", req, &out); err != nil {
		return err
	}
	fmt.Println(prettyJSON(out))
	return nil
}

func runProjectChainSourceUpload(args []string) error {
	cf, rest, err := parseWithConn("chain-source-upload", args, nil)
	if err != nil {
		return err
	}
	if len(rest) != 2 {
		return fmt.Errorf("usage: elt project chain-source-upload <Dialogues|PDA> <source-file>")
	}
	ft, err := parseFileType(rest[0])
	if err != nil {
		return err
	}
	base, cleanup, err := cf.connect()
	if err != nil {
		return err
	}
	defer cleanup()

	body, err := os.ReadFile(rest[1])
	if err != nil {
		return err
	}
	data, err := apiCall("POST", base, fmt.Sprintf("/files/%s/chains/source", url.PathEscape(string(ft))), body)
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func runProjectChainRoots(args []string) error {
	cf, rest, err := parseWithConn("chain-roots", args, nil)
	if err != nil {
		return err
	}
	if len(rest) != 1 {
		return fmt.Errorf("usage: elt project chain-roots <Dialogues|PDA>")
	}
	ft, err := parseFileType(rest[0])
	if err != nil {
		return err
	}
	base, cleanup, err := cf.connect()
	if err != nil {
		return err
	}
	defer cleanup()

	data, err := apiCall("GET", base, fmt.Sprintf("/files/%s/chains/roots", url.PathEscape(string(ft))), nil)
	if err != nil {
		return err
	}
	var roots []string
	if err := json.Unmarshal(data, &roots); err != nil {
		return err
	}
	for _, r := range roots {
		fmt.Println(r)
	}
	return nil
}

func runProjectChain(args []string) error {
	cf, rest, err := parseWithConn("chain", args, nil)
	if err != nil {
		return err
	}
	if len(rest) != 2 {
		return fmt.Errorf("usage: elt project chain <Dialogues|PDA> <root>")
	}
	ft, err := parseFileType(rest[0])
	if err != nil {
		return err
	}
	base, cleanup, err := cf.connect()
	if err != nil {
		return err
	}
	defer cleanup()

	path := fmt.Sprintf("/files/%s/chains/%s", url.PathEscape(string(ft)), url.PathEscape(rest[1]))
	data, err := apiCall("GET", base, path, nil)
	if err != nil {
		return err
	}
	var keys []string
	if err := json.Unmarshal(data, &keys); err != nil {
		return err
	}
	for _, k := range keys {
		fmt.Println(k)
	}
	return nil
}

func runProjectKeys(args []string) error {
	var lang, status, search string
	var offset, limit int
	cf, rest, err := parseWithConn("keys", args, func(fs *flag.FlagSet) {
		fs.StringVar(&lang, "lang", "", "language to list")
		fs.StringVar(&status, "status", "", "untranslated|stale|needs_review|needs_work|ok (default: all)")
		fs.StringVar(&search, "search", "", "case-insensitive substring over key/English")
		fs.IntVar(&offset, "offset", 0, "page offset")
		fs.IntVar(&limit, "limit", 100, "page size")
	})
	if err != nil {
		return err
	}
	if len(rest) != 1 || lang == "" {
		return fmt.Errorf("usage: elt project keys <FileType> --lang=Russian [--status=..] [--search=..] [--offset=N] [--limit=N]")
	}
	base, cleanup, err := cf.connect()
	if err != nil {
		return err
	}
	defer cleanup()

	path := fmt.Sprintf("/files/%s/keys?lang=%s&status=%s&search=%s&offset=%d&limit=%d",
		url.PathEscape(rest[0]), url.QueryEscape(lang), url.QueryEscape(status), url.QueryEscape(search), offset, limit)
	var out any
	if err := apiCallJSON("GET", base, path, nil, &out); err != nil {
		return err
	}
	fmt.Println(prettyJSON(out))
	return nil
}

func runProjectDiff(args []string) error {
	var from, to string
	cf, rest, err := parseWithConn("diff", args, func(fs *flag.FlagSet) {
		fs.StringVar(&from, "from", "", "revision to diff from (see 'elt project history')")
		fs.StringVar(&to, "to", "", "revision to diff to (default: current state)")
	})
	if err != nil {
		return err
	}
	if len(rest) != 1 || from == "" {
		return fmt.Errorf("usage: elt project diff <FileType> --from=<rev> [--to=<rev>]")
	}
	base, cleanup, err := cf.connect()
	if err != nil {
		return err
	}
	defer cleanup()

	path := fmt.Sprintf("/files/%s/diff?from=%s", url.PathEscape(rest[0]), url.QueryEscape(from))
	if to != "" {
		path += "&to=" + url.QueryEscape(to)
	}
	var out any
	if err := apiCallJSON("GET", base, path, nil, &out); err != nil {
		return err
	}
	fmt.Println(prettyJSON(out))
	return nil
}

func runProjectClear(args []string) error {
	var lang string
	cf, rest, err := parseWithConn("clear", args, func(fs *flag.FlagSet) {
		fs.StringVar(&lang, "lang", "", "language to clear")
	})
	if err != nil {
		return err
	}
	if len(rest) < 2 || lang == "" {
		return fmt.Errorf("usage: elt project clear <FileType> --lang=Russian <key> [<key>...]")
	}
	base, cleanup, err := cf.connect()
	if err != nil {
		return err
	}
	defer cleanup()

	req := map[string]any{"lang": lang, "keys": rest[1:]}
	var out any
	if err := apiCallJSON("POST", base, fmt.Sprintf("/files/%s/translations/clear", url.PathEscape(rest[0])), req, &out); err != nil {
		return err
	}
	fmt.Println(prettyJSON(out))
	return nil
}
