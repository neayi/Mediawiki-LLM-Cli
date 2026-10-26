// Command mediawiki reads and edits pages of a MediaWiki wiki from the command
// line, using a bot password stored on the local machine.
package main

import (
	"fmt"
	"os"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

const usage = `mediawiki - read and edit MediaWiki pages from the command line

Usage:
  mediawiki login  <wiki-url> [--username NAME] [--no-browser]
  mediawiki logout <wiki-url>
  mediawiki read   <page-url>
  mediawiki write  <page-url> <file> [--content-model MODEL] [--summary TEXT]
  mediawiki upload <file-page-url> <local-file> [--description FILE] [--summary TEXT] [--ignore-warnings]
  mediawiki rename <page-url> <new-page-url> [--reason TEXT] [--no-redirect]
  mediawiki delete <page-url> [--reason TEXT]
  mediawiki version

A file argument of "-" reads from standard input.

Examples:
  mediawiki login https://tripleperformance.fr
  mediawiki read https://tripleperformance.fr/wiki/Une_page
  mediawiki write https://tripleperformance.fr/wiki/Une_page unepage.wiki
  mediawiki write --content-model=json https://tripleperformance.fr/wiki/Une_page data.json
  mediawiki upload https://tripleperformance.fr/wiki/File:Une_image.jpg image.jpg --description description.wiki
  mediawiki rename https://tripleperformance.fr/wiki/Une_page https://tripleperformance.fr/wiki/Une_autre_page
  mediawiki delete https://tripleperformance.fr/wiki/Une_page
`

// command describes a subcommand: its positional arguments and options.
type command struct {
	nargs      int
	valueFlags []string
	boolFlags  []string
	run        func(*args) error
}

var commands = map[string]command{
	"login":  {1, []string{"username"}, []string{"no-browser"}, cmdLogin},
	"logout": {1, nil, nil, cmdLogout},
	"read":   {1, nil, nil, cmdRead},
	"write":  {2, []string{"content-model", "summary"}, nil, cmdWrite},
	"upload": {2, []string{"description", "summary"}, []string{"ignore-warnings"}, cmdUpload},
	"rename": {2, []string{"reason"}, []string{"no-redirect"}, cmdRename},
	"delete": {1, []string{"reason"}, nil, cmdDelete},
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	name := os.Args[1]
	switch name {
	case "-h", "--help", "help":
		fmt.Print(usage)
		return
	case "version", "--version":
		fmt.Println("mediawiki", version)
		return
	}

	cmd, ok := commands[name]
	if !ok {
		fmt.Fprintf(os.Stderr, "mediawiki: unknown command %q\n\n%s", name, usage)
		os.Exit(2)
	}
	for _, a := range os.Args[2:] {
		if a == "-h" || a == "--help" {
			fmt.Print(usage)
			return
		}
	}
	a, err := parseArgs(os.Args[2:], cmd.valueFlags, cmd.boolFlags)
	if err == nil && len(a.pos) != cmd.nargs {
		err = fmt.Errorf("%s expects %d argument(s), got %d", name, cmd.nargs, len(a.pos))
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "mediawiki: %v\n\n%s", err, usage)
		os.Exit(2)
	}
	if err := cmd.run(a); err != nil {
		fmt.Fprintf(os.Stderr, "mediawiki %s: %v\n", name, err)
		os.Exit(1)
	}
}
