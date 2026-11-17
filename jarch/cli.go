package jarch

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const cliUsage = `usage: jplay --fetch [flags] <subcommand> [<id>]

subcommands:
  seasons              parse listseasons.php               -> listseasons.json
  season <season_id>   parse showseason.php?season=<id>    -> season-<id>.json
  game <game_id>       parse showgame.php?game_id=<id>     -> <id>.json
  player <player_id>   parse showplayer.php?player_id=<id> -> player-<id>.json

flags:
  --refresh     ignore any cached HTML and re-fetch from the network
  --out <path>  write JSON to <path> instead of the default
  --stdout      write JSON to stdout only (no file)
  -h, --help    show this help

environment:
  JARCH_DIR          override cache AND output directory (default: cache in
                     the configured jarch cache dir, output in current directory);
                     also disables index-page staleness, so fixture runs stay offline
  JARCH_FETCH_DELAY  seconds to sleep before a network fetch (default: 2)
  JARCH_INDEX_TTL    seconds before a cached seasons/season page is re-fetched
                     (default: 43200; 0 = cache forever)
`

var intRE = regexp.MustCompile(`^[0-9]+$`)

// RunCLI runs the standalone fetcher with args (everything after --fetch).
// cacheDir is the default cache directory. It returns a process exit code.
func RunCLI(args []string, cacheDir string, stdout, stderr io.Writer) int {
	die := func(format string, a ...any) int {
		fmt.Fprintf(stderr, "jarch: "+format+"\n", a...)
		return 1
	}

	var refresh, stdoutOnly bool
	var outPath string
	var pos []string
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--refresh":
			refresh = true
		case a == "--stdout":
			stdoutOnly = true
		case a == "--out":
			if i+1 >= len(args) {
				return die("--out requires a path")
			}
			i++
			outPath = args[i]
		case a == "-h" || a == "--help":
			fmt.Fprint(stdout, cliUsage)
			return 0
		case strings.HasPrefix(a, "--"):
			return die("unknown flag: %s", a)
		default:
			pos = append(pos, a)
		}
	}
	if len(pos) < 1 {
		fmt.Fprint(stderr, cliUsage)
		return 1
	}
	sub, id := pos[0], ""
	if len(pos) > 1 {
		id = pos[1]
	}

	outDir := "."
	c := New(cacheDir)
	if d := os.Getenv("JARCH_DIR"); d != "" {
		c.CacheDir, outDir = d, d
		c.IndexTTL = 0
	}
	c.Refresh = refresh
	c.Warn = stderr

	var v any
	var defaultOut string
	var err error
	switch sub {
	case "seasons":
		if id != "" {
			return die("seasons takes no argument")
		}
		v, err = c.Seasons()
		defaultOut = "listseasons.json"
	case "season":
		if id == "" {
			return die("season requires a <season_id>")
		}
		v, err = c.Season(id)
		defaultOut = "season-" + id + ".json"
	case "game":
		if !intRE.MatchString(id) {
			return die("game requires an integer <game_id>")
		}
		n, _ := strconv.Atoi(id)
		v, err = c.Game(n)
		defaultOut = id + ".json"
	case "player":
		if !intRE.MatchString(id) {
			return die("player requires an integer <player_id>")
		}
		n, _ := strconv.Atoi(id)
		v, err = c.Player(n)
		defaultOut = "player-" + id + ".json"
	default:
		return die("unknown subcommand: %s", sub)
	}
	if err != nil {
		return die("%v", err)
	}

	out, err := Marshal(v)
	if err != nil {
		return die("%v", err)
	}
	if !stdoutOnly {
		if outPath == "" {
			outPath = filepath.Join(outDir, defaultOut)
		}
		if err := os.WriteFile(outPath, out, 0o644); err != nil {
			return die("%v", err)
		}
	}
	stdout.Write(out)
	return 0
}
