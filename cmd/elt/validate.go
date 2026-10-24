package main

import (
	"flag"
	"fmt"

	"elt/internal/validate"
)

const validateHelp = `elt validate --type=<Localization|Dialogues|PDA> <english-text> <translation-text>

Check one translated string against its English source using the same
validation engine (internal/validate) every project verb/GUI/TUI
"Validate" action uses: BBCode "[c]"-color-block and HTML/angle-tag
balance, {placeholder}/@marker/escaped-newline consistency against the
English source, and the file-type-specific policy flags that come with
it. --type picks which policy applies (Localization/PDA use BBCode
"[...]" tags; Dialogues uses HTML-like "<...>" tags) — defaults to
Localization. Prints "ok" if the pair is clean, or one line per issue
found. Standalone utility: no project directory is opened or needed.
`

func runValidate(args []string) error {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), validateHelp)
		fmt.Fprintln(fs.Output(), "\nFlags:")
		fs.PrintDefaults()
	}
	fileType := fs.String("type", "Localization", "file type: Localization, Dialogues, or PDA")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) != 2 {
		return fmt.Errorf("usage: elt validate --type=<FileType> <english-text> <translation-text>")
	}

	ft, err := parseFileType(*fileType)
	if err != nil {
		return err
	}

	result := validate.ValidateTextPair(rest[0], rest[1], validate.DefaultPolicy(ft))
	if result.OK {
		fmt.Println("ok")
		return nil
	}

	fmt.Printf("%d issue(s):\n", len(result.Issues))
	for _, iss := range result.Issues {
		switch {
		case iss.Token != "" && iss.Count > 0:
			fmt.Printf("  %s: %s x%d\n", iss.Code, iss.Token, iss.Count)
		case iss.Token != "":
			fmt.Printf("  %s: %s\n", iss.Code, iss.Token)
		default:
			fmt.Printf("  %s\n", iss.Code)
		}
	}
	return nil
}
