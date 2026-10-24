package main

import (
	"flag"
	"fmt"
	"os"

	"elt/internal/csvio"
)

const decompileHelp = `elt decompile <csv-file>

Read a game localization CSV (Localization.csv/Dialogues.csv/PDA.csv,
whichever shape) and report its structure: how many language columns
the header has, how many rows, and any structural issues found along
the way (a duplicate key, a row shorter than the header — plausibly an
intentionally omitted trailing language, warning severity — or a row
longer than the header, meaning data was actually discarded, most
likely broken CSV quoting rather than an intentional omission, error
severity). Standalone utility: no project directory is opened or
needed, this just parses the one file given and prints what it found.
`

func runDecompile(args []string) error {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help" || args[0] == "help") {
		fmt.Fprint(os.Stderr, decompileHelp)
		return flag.ErrHelp
	}
	if len(args) != 1 {
		return fmt.Errorf("usage: elt decompile <csv-file>")
	}
	path := args[0]

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	result, err := csvio.Decompile(f)
	if err != nil {
		return fmt.Errorf("decompiling %s: %w", path, err)
	}

	fmt.Printf("%s: %d language column(s), %d row(s)\n", path, len(result.Table.Header.Languages), len(result.Table.Rows))
	if len(result.Issues) == 0 {
		fmt.Println("no structural issues found")
		return nil
	}

	fmt.Printf("%d issue(s) found:\n", len(result.Issues))
	for _, iss := range result.Issues {
		fmt.Printf("  [%s] line %d, key %q: %s\n", iss.Severity, iss.Line, iss.Key, iss.Detail)
	}
	return nil
}
