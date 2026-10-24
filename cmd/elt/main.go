package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"elt/internal/localize"
)

func main() {
	var err error
	if len(os.Args) < 2 {

		err = runGUI(nil)
	} else {
		switch os.Args[1] {
		case "decompile":
			err = runDecompile(os.Args[2:])
		case "validate":
			err = runValidate(os.Args[2:])
		case "serve":
			err = runServe(os.Args[2:])
		case "gui":
			err = runGUI(os.Args[2:])
		case "project":
			err = runProject(os.Args[2:])
		case "-h", "--help", "help":
			usage()
			return
		default:
			fmt.Fprintf(os.Stderr, "elt: unknown command %q\n\n", os.Args[1])
			usage()
			os.Exit(2)
		}
	}

	if err != nil {

		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintf(os.Stderr, "elt: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `elt - Empyrion Localization Tool (CLI)

Every command below, and every "elt project <verb>" subcommand, accepts
-h/--help for its own detailed help (its full description, usage line,
and complete flag list) — this top-level overview is intentionally just
a map of what exists; run e.g. "elt sync --help" or
"elt project ai-translate-start --help" for the real detail on any one
of them.

A bare "elt" with no arguments at all is exactly "elt gui" — run it and
it works, the everyday local/solo default on every platform.

Usage:
  elt decompile <csv-file>
      Read a game localization CSV and report its structure: header
      languages, row count, and any structural issues found (duplicate
      keys, short/long rows). Standalone utility, no project involved —
      doesn't open or need a project directory at all.

  elt validate --type=<Localization|Dialogues|PDA> <english-text> <translation-text>
      Check one translated string against its English source (BBCode/tag
      balance, placeholder and marker consistency, escaped-newline
      consistency) and print the resulting issues, or "ok" if there are
      none. Standalone utility, no project involved, same as decompile.

  elt serve --data-dir <dir> [--listen host:port] [--no-ui]
      Host a project's HTTP+JSON API (and, unless --no-ui, the same
      browser GUI "elt gui" opens, served from the same address) for
      other clients — team members, the terminal client, another
      machine — to connect to over the network. Opens/initializes <dir>
      as a git-backed project store the first time it's given. Runs
      until stopped (Ctrl+C); every mutation is already durably saved to
      disk as it happens, nothing to flush on exit. --no-ui is the
      headless-server persona: a plain JSON API with no UI assets
      mounted at all, meant to be paired with each participant's own
      "elt gui --addr=" instead of anyone opening this address directly.

  elt gui [--data-dir <dir>] [--listen host:port] [--no-browser]
      Exactly "elt serve", plus it opens the project in your default web
      browser automatically (or just prints the URL with --no-browser).
      This is the ordinary everyday way to use elt locally/solo — the one
      command in this binary meant to keep running rather than do one
      thing and exit.

  elt gui --addr host:port [--listen host:port] [--no-browser]
      A different mode: a thin client with no project of its own,
      proxying every API call through to a real "elt serve"/"elt api" at
      --addr while serving the browser GUI's static assets locally — see
      "elt gui --help" for the full picture. Meant for each participant
      on a team server to run locally instead of opening the server's own
      address in a browser directly.

  elt project <verb> [flags]
      Everything project-shaped (sync/translate/validate/split/merge/AI
      jobs/glossary/locks/history/...) as a scriptable one-shot command —
      run "elt project --help" (or with no verb at all) for the complete
      list of ~50 verbs. Every verb accepts either --addr host:port (talk
      to a running "elt serve") or --data-dir dir (open/init a local
      project and serve it just for that one command, then exit).
`)
}

func parseFileType(s string) (localize.FileType, error) {
	ft := localize.FileType(s)
	if !ft.Valid() {
		return "", fmt.Errorf("invalid --type %q (want Localization, Dialogues, PDA, or LocalizationVanilla)", s)
	}
	return ft, nil
}
