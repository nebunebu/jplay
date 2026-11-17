// jplay is a terminal UI for playing Jeopardy! games from J! Archive, with a
// built-in archive fetcher (jarch) also usable standalone via --fetch.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"jplay/config"
	"jplay/jarch"
	"jplay/tui"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "jplay: bad config: %v\n", err)
		os.Exit(1)
	}

	// Standalone fetcher: everything after --fetch belongs to jarch, which
	// has its own flag set (flags may follow positional args).
	if len(os.Args) > 1 && (os.Args[1] == "--fetch" || os.Args[1] == "-fetch") {
		os.Exit(jarch.RunCLI(os.Args[2:], cfg.ResolvedCacheDir(), os.Stdout, os.Stderr))
	}

	stats := flag.Bool("stats", false, "open stats review mode")
	conf := flag.Bool("config", false, "open the settings screen")
	flag.Bool("fetch", false, "standalone archive fetcher (see jplay --fetch --help)")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), `usage:
  jplay [game.json]          play a jarch game file directly
  jplay                      landing screen: browse archive or open file
  jplay --stats              stats review mode
  jplay --config             settings screen
  jplay --fetch <sub> [id]   fetch/parse J! Archive pages to JSON
                             (seasons | season <id> | game <id> | player <id>;
                             see jplay --fetch --help)
`)
	}
	flag.Parse()

	opts := tui.Options{Mode: "play"}
	switch {
	case *stats:
		opts.Mode = "stats"
	case *conf:
		opts.Mode = "config"
	default:
		if flag.NArg() > 0 {
			opts.GamePath = flag.Arg(0)
		}
	}

	p := tea.NewProgram(tui.New(cfg, opts), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "jplay: %v\n", err)
		os.Exit(1)
	}
}
