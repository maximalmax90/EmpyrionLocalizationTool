package main

import (
	"flag"
	"fmt"
	"net/url"
)

func addGlossaryTermFlags(fs *flag.FlagSet) (term, action, translation, notes *string, inflect *bool) {
	term = fs.String("term", "", "term text")
	action = fs.String("action", "keep", "keep|translate|transliterate")
	translation = fs.String("translation", "", "target-language text (unused for --action=keep)")
	inflect = fs.Bool("inflect", false, "allow grammatical inflection/declension when the term is used in context")
	notes = fs.String("notes", "", "free-text notes")
	return
}

type glossaryTermBody struct {
	Term        string `json:"term"`
	Action      string `json:"action"`
	Translation string `json:"translation,omitempty"`
	Inflect     bool   `json:"inflect,omitempty"`
	Notes       string `json:"notes,omitempty"`
}

func runProjectGlossaryList(args []string) error {
	var lang, status string
	cf, rest, err := parseWithConn("glossary-list", args, func(fs *flag.FlagSet) {
		fs.StringVar(&lang, "lang", "", "language")
		fs.StringVar(&status, "status", "", "draft|approved|rejected|all — omit for draft+approved (rejected hidden)")
	})
	if err != nil {
		return err
	}
	if len(rest) != 0 || lang == "" {
		return fmt.Errorf("usage: elt project glossary-list --lang=Russian [--status=draft|approved|rejected|all]")
	}
	base, cleanup, err := cf.connect()
	if err != nil {
		return err
	}
	defer cleanup()

	path := "/glossary?lang=" + url.QueryEscape(lang)
	if status != "" {
		path += "&status=" + url.QueryEscape(status)
	}
	var out any
	if err := apiCallJSON("GET", base, path, nil, &out); err != nil {
		return err
	}
	fmt.Println(prettyJSON(out))
	return nil
}

func runProjectGlossaryAdd(args []string) error {
	var lang string
	var term, action, translation, notes *string
	var inflect *bool
	cf, rest, err := parseWithConn("glossary-add", args, func(fs *flag.FlagSet) {
		fs.StringVar(&lang, "lang", "", "language")
		term, action, translation, notes, inflect = addGlossaryTermFlags(fs)
	})
	if err != nil {
		return err
	}
	if len(rest) != 0 || lang == "" || *term == "" {
		return fmt.Errorf("usage: elt project glossary-add --lang=Russian --term=... [--action=keep|translate|transliterate] [--translation=...] [--inflect] [--notes=...]")
	}
	base, cleanup, err := cf.connect()
	if err != nil {
		return err
	}
	defer cleanup()

	body := glossaryTermBody{Term: *term, Action: *action, Translation: *translation, Inflect: *inflect, Notes: *notes}
	var out any
	if err := apiCallJSON("POST", base, "/glossary?lang="+url.QueryEscape(lang), body, &out); err != nil {
		return err
	}
	fmt.Println(prettyJSON(out))
	return nil
}

func runProjectGlossaryUpdate(args []string) error {
	var lang string
	var term, action, translation, notes *string
	var inflect *bool
	var owner string
	cf, rest, err := parseWithConn("glossary-update", args, func(fs *flag.FlagSet) {
		fs.StringVar(&lang, "lang", "", "language")
		term, action, translation, notes, inflect = addGlossaryTermFlags(fs)
		fs.StringVar(&owner, "owner", "", "your identity, if this term is locked and this server has no authenticated user on the connection")
	})
	if err != nil {
		return err
	}
	if len(rest) != 0 || lang == "" || *term == "" {
		return fmt.Errorf("usage: elt project glossary-update --lang=Russian --term=... [--action=..] [--translation=..] [--inflect] [--notes=..] [--owner=..]")
	}
	base, cleanup, err := cf.connect()
	if err != nil {
		return err
	}
	defer cleanup()

	body := glossaryTermBody{Term: *term, Action: *action, Translation: *translation, Inflect: *inflect, Notes: *notes}
	path := "/glossary/" + url.PathEscape(*term) + joinQuery("lang="+url.QueryEscape(lang), ownerQuery(owner))
	var out any
	if err := apiCallJSON("PUT", base, path, body, &out); err != nil {
		return err
	}
	fmt.Println(prettyJSON(out))
	return nil
}

func runProjectGlossaryRemove(args []string) error {
	var lang, term, owner string
	cf, rest, err := parseWithConn("glossary-remove", args, func(fs *flag.FlagSet) {
		fs.StringVar(&lang, "lang", "", "language")
		fs.StringVar(&term, "term", "", "term text")
		fs.StringVar(&owner, "owner", "", "your identity, if this term is locked and this server has no authenticated user on the connection")
	})
	if err != nil {
		return err
	}
	if len(rest) != 0 || lang == "" || term == "" {
		return fmt.Errorf("usage: elt project glossary-remove --lang=Russian --term=... [--owner=..]")
	}
	base, cleanup, err := cf.connect()
	if err != nil {
		return err
	}
	defer cleanup()

	path := "/glossary/" + url.PathEscape(term) + joinQuery("lang="+url.QueryEscape(lang), ownerQuery(owner))
	if _, err := apiCall("DELETE", base, path, nil); err != nil {
		return err
	}
	fmt.Println(prettyJSON(map[string]bool{"ok": true}))
	return nil
}

func runProjectGlossarySetStatus(verb, action string, args []string) error {
	var lang, term string
	cf, rest, err := parseWithConn(verb, args, func(fs *flag.FlagSet) {
		fs.StringVar(&lang, "lang", "", "language")
		fs.StringVar(&term, "term", "", "term text")
	})
	if err != nil {
		return err
	}
	if len(rest) != 0 || lang == "" || term == "" {
		return fmt.Errorf("usage: elt project %s --lang=Russian --term=...", verb)
	}
	base, cleanup, err := cf.connect()
	if err != nil {
		return err
	}
	defer cleanup()

	path := "/glossary/" + url.PathEscape(term) + "/" + action + "?lang=" + url.QueryEscape(lang)
	var out any
	if err := apiCallJSON("POST", base, path, nil, &out); err != nil {
		return err
	}
	fmt.Println(prettyJSON(out))
	return nil
}

func runProjectGlossaryApprove(args []string) error {
	return runProjectGlossarySetStatus("glossary-approve", "approve", args)
}

func runProjectGlossaryUnapprove(args []string) error {
	return runProjectGlossarySetStatus("glossary-unapprove", "unapprove", args)
}

func runProjectGlossaryReject(args []string) error {
	return runProjectGlossarySetStatus("glossary-reject", "reject", args)
}

func runProjectGlossaryUsage(args []string) error {
	var term string
	cf, rest, err := parseWithConn("glossary-usage", args, func(fs *flag.FlagSet) {
		fs.StringVar(&term, "term", "", "term text")
	})
	if err != nil {
		return err
	}
	if len(rest) != 0 || term == "" {
		return fmt.Errorf("usage: elt project glossary-usage --term=...")
	}
	base, cleanup, err := cf.connect()
	if err != nil {
		return err
	}
	defer cleanup()

	var out any
	if err := apiCallJSON("GET", base, "/glossary/"+url.PathEscape(term)+"/usage", nil, &out); err != nil {
		return err
	}
	fmt.Println(prettyJSON(out))
	return nil
}
