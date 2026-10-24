package main

import (
	"flag"
	"fmt"
	"net/url"
	"strings"
)

func ownerQuery(owner string) string {
	if owner == "" {
		return ""
	}
	return "owner=" + url.QueryEscape(owner)
}

func joinQuery(parts ...string) string {
	var nonEmpty []string
	for _, p := range parts {
		if p != "" {
			nonEmpty = append(nonEmpty, p)
		}
	}
	if len(nonEmpty) == 0 {
		return ""
	}
	return "?" + strings.Join(nonEmpty, "&")
}

func runProjectLock(args []string) error {
	var lang, owner string
	cf, rest, err := parseWithConn("lock", args, func(fs *flag.FlagSet) {
		fs.StringVar(&lang, "lang", "", "target language — a lock is scoped to one key+language pair, not the whole key (see keyLockID's doc)")
		fs.StringVar(&owner, "owner", "", "your identity, if this server has no authenticated user on the connection")
	})
	if err != nil {
		return err
	}
	if len(rest) != 2 || lang == "" {
		return fmt.Errorf("usage: elt project lock <FileType> <key> --lang=Russian [--owner=..]")
	}
	base, cleanup, err := cf.connect()
	if err != nil {
		return err
	}
	defer cleanup()

	path := "/files/" + url.PathEscape(rest[0]) + "/keys/" + url.PathEscape(rest[1]) + "/lock" + joinQuery("lang="+url.QueryEscape(lang), ownerQuery(owner))
	var out any
	if err := apiCallJSON("POST", base, path, nil, &out); err != nil {
		return err
	}
	fmt.Println(prettyJSON(out))
	return nil
}

func runProjectUnlock(args []string) error {
	var lang, owner string
	cf, rest, err := parseWithConn("unlock", args, func(fs *flag.FlagSet) {
		fs.StringVar(&lang, "lang", "", "target language — must match the language --lock was called with")
		fs.StringVar(&owner, "owner", "", "your identity, if this server has no authenticated user on the connection")
	})
	if err != nil {
		return err
	}
	if len(rest) != 2 || lang == "" {
		return fmt.Errorf("usage: elt project unlock <FileType> <key> --lang=Russian [--owner=..]")
	}
	base, cleanup, err := cf.connect()
	if err != nil {
		return err
	}
	defer cleanup()

	path := "/files/" + url.PathEscape(rest[0]) + "/keys/" + url.PathEscape(rest[1]) + "/lock" + joinQuery("lang="+url.QueryEscape(lang), ownerQuery(owner))
	if _, err := apiCall("DELETE", base, path, nil); err != nil {
		return err
	}
	fmt.Println(prettyJSON(map[string]bool{"ok": true}))
	return nil
}

func runProjectLockStatus(args []string) error {
	var lang string
	cf, rest, err := parseWithConn("lock-status", args, func(fs *flag.FlagSet) {
		fs.StringVar(&lang, "lang", "", "target language")
	})
	if err != nil {
		return err
	}
	if len(rest) != 2 || lang == "" {
		return fmt.Errorf("usage: elt project lock-status <FileType> <key> --lang=Russian")
	}
	base, cleanup, err := cf.connect()
	if err != nil {
		return err
	}
	defer cleanup()

	path := "/files/" + url.PathEscape(rest[0]) + "/keys/" + url.PathEscape(rest[1]) + "/lock" + joinQuery("lang="+url.QueryEscape(lang))
	var out any
	if err := apiCallJSON("GET", base, path, nil, &out); err != nil {
		return err
	}
	fmt.Println(prettyJSON(out))
	return nil
}

func runProjectLockKeys(args []string) error {
	var lang, owner string
	cf, rest, err := parseWithConn("lock-keys", args, func(fs *flag.FlagSet) {
		fs.StringVar(&lang, "lang", "", "target language — the whole set is locked for this one language")
		fs.StringVar(&owner, "owner", "", "your identity, if this server has no authenticated user on the connection")
	})
	if err != nil {
		return err
	}
	if len(rest) < 2 || lang == "" {
		return fmt.Errorf("usage: elt project lock-keys <FileType> <key> [<key>...] --lang=Russian [--owner=..]")
	}
	base, cleanup, err := cf.connect()
	if err != nil {
		return err
	}
	defer cleanup()

	req := map[string]any{"lang": lang, "keys": rest[1:]}
	path := "/files/" + url.PathEscape(rest[0]) + "/locks" + joinQuery(ownerQuery(owner))
	var out any
	if err := apiCallJSON("POST", base, path, req, &out); err != nil {
		return err
	}
	fmt.Println(prettyJSON(out))
	return nil
}

func runProjectGlossaryLock(args []string) error {
	var lang, term, owner string
	cf, rest, err := parseWithConn("glossary-lock", args, func(fs *flag.FlagSet) {
		fs.StringVar(&lang, "lang", "", "language")
		fs.StringVar(&term, "term", "", "term text")
		fs.StringVar(&owner, "owner", "", "your identity, if this server has no authenticated user on the connection")
	})
	if err != nil {
		return err
	}
	if len(rest) != 0 || lang == "" || term == "" {
		return fmt.Errorf("usage: elt project glossary-lock --lang=Russian --term=... [--owner=..]")
	}
	base, cleanup, err := cf.connect()
	if err != nil {
		return err
	}
	defer cleanup()

	path := "/glossary/" + url.PathEscape(term) + "/lock" + joinQuery("lang="+url.QueryEscape(lang), ownerQuery(owner))
	var out any
	if err := apiCallJSON("POST", base, path, nil, &out); err != nil {
		return err
	}
	fmt.Println(prettyJSON(out))
	return nil
}

func runProjectGlossaryUnlock(args []string) error {
	var lang, term, owner string
	cf, rest, err := parseWithConn("glossary-unlock", args, func(fs *flag.FlagSet) {
		fs.StringVar(&lang, "lang", "", "language")
		fs.StringVar(&term, "term", "", "term text")
		fs.StringVar(&owner, "owner", "", "your identity, if this server has no authenticated user on the connection")
	})
	if err != nil {
		return err
	}
	if len(rest) != 0 || lang == "" || term == "" {
		return fmt.Errorf("usage: elt project glossary-unlock --lang=Russian --term=... [--owner=..]")
	}
	base, cleanup, err := cf.connect()
	if err != nil {
		return err
	}
	defer cleanup()

	path := "/glossary/" + url.PathEscape(term) + "/lock" + joinQuery("lang="+url.QueryEscape(lang), ownerQuery(owner))
	if _, err := apiCall("DELETE", base, path, nil); err != nil {
		return err
	}
	fmt.Println(prettyJSON(map[string]bool{"ok": true}))
	return nil
}
