# jarch

`jarch` is a site-specific pipeline tool that fetches pages from
[J! Archive](https://j-archive.com), caches them, and parses them into JSON.
It is a set of shell scripts composing small, well-understood tools —
`curl`, `pup`, and `jq` — rather than a large program. See `spec/SPEC.md`
for the full specification and output schemas.

Every page goes through the same pipeline:

```
fetch (curl)  ->  cache (plaintext .html)  ->  parse (pup + jq)  ->  output (.json)
```

Parsing always reads from the cache, never directly from the network, so a
fetch and a parse are independently re-runnable and parsing can be developed
offline against the fixtures in `spec/`.

> J! Archive's disclaimer prohibits scraping. `jarch` is for personal,
> low-volume, incremental use only: it fetches only what it is asked for,
> cache-first, rate-limited, with a descriptive User-Agent.

## Usage

```
jarch seasons                 # listseasons.php               -> listseasons.json
jarch season <season_id>      # showseason.php?season=<id>    -> season-<id>.json
jarch game <game_id>          # showgame.php?game_id=<id>     -> <id>.json
jarch player <player_id>      # showplayer.php?player_id=<id> -> player-<id>.json
```

`<season_id>` is an opaque string (`42`, `jm`, `pcj`); game and player ids are
integers. Results are printed to stdout and written to the conventional output
file in the current directory; cached HTML lives in `$XDG_CACHE_HOME/jarch/`
(`~/.cache/jarch/` by default).

Flags:

- `--refresh` — ignore any cached HTML and re-fetch from the network.
- `--out <path>` — write JSON to `<path>` instead of the default.
- `--stdout` — write JSON to stdout only (no file).

Environment:

- `JARCH_DIR` — override for both the cache and output directory (unset by
  default; used by the spec fixtures). When unset, cache goes to
  `$XDG_CACHE_HOME/jarch/` (`~/.cache/jarch/` if `XDG_CACHE_HOME` is unset)
  and output to the current directory. Cache filenames mirror the site's URL
  path and query, e.g. `showgame.php?game_id=6789`.
- `JARCH_FETCH_DELAY` — seconds slept before each network fetch (default: 2).
- `JARCH_INDEX_TTL` — seconds before a cached *index* page (`seasons`,
  `season`) is considered stale and re-fetched on a cache hit (default:
  43200, i.e. 12h). These pages gain new episodes/seasons as the show airs;
  `game` and `player` pages are immutable and always cache-forever. Set to
  `0` to cache index pages forever too (the old behaviour). Ignored when
  `JARCH_DIR` is set, so fixture reproduction stays offline.

hrefs in the output are stored verbatim (site-relative); resolve on demand
against `BASE_URL = https://j-archive.com/`.

## Development

The nix flake provides both a devshell (`nix develop`, or `direnv allow`) with
`curl`/`pup`/`jq` and the `jarch` package itself (`nix build .#jarch`).

Layout:

- `bin/jarch` — the CLI: argument handling, cache-first fetch, dispatch.
- `lib/jarch/*.jq` — one jq parser per page type, plus `common.jq` (entity
  decoding, note parsing, the shared games-table row parser).
- `spec/` — the specification and cached-page fixtures with expected JSON
  (`spec/6789.json` is the canonical game fixture; the tool reproduces it
  byte-for-byte via `JARCH_DIR=spec jarch game 6789 --stdout`).

Parsers use pup's `json{}` tree for structured parts and jq regexes over the
raw cached HTML for mixed-content cells (clue text, revealed-answer cells),
where inline-markup order matters and each cell is a single line in the
source.

## Settled judgment calls

Decisions taken from `spec/SUGGESTIONS.md`:

- **`host_comment` vs `post_response_comment`** — both fields are kept. The
  parser mechanically fills `host_comment` with every host parenthetical
  (joined with newlines); `post_response_comment` is always `null` and is a
  manual/curated field, as the split is not machine-determinable.
- **`combined_coryat`** stays at the top level; **round `scores`** stays an
  object keyed by snapshot — both match the fixture.
- **hrefs** stay site-relative; no `nickname` field is emitted on contestants.
- **Final-scores `score_remarks`** are not captured.
- **Unrevealed / absent clues** — board cells with no clue in the source are
  omitted from `clues`; a clue that is present but has no order number gets
  `play_order: null` and `responses: []`.
- **Tournament round vocabulary** — when a tournament-shaped note yields no
  recognised round keyword, a warning is printed to stderr so the vocabulary
  can be grown from real misses.
