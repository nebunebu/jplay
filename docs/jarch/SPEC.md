# jarch

`jarch` is a site-specific pipeline tool to fetch data from
[J! Archive](https://j-archive.com), parse it, and transform it into JSON.

It is intended to be light: a set of shell scripts that compose small,
well-understood tools rather than a large program.

## Dependencies

- `curl` — fetch pages over HTTP
- `pup` — HTML parser / CSS-selector extractor
- `jq` — JSON construction and transformation

These dependencies must be added to the nix flake, and the `jarch` package must
be added to the devshell.

## Pipeline shape

Each page type is handled by the same three-stage pipeline:

```
fetch (curl)  ->  cache (plaintext .html)  ->  parse (pup + jq)  ->  output (.json)
```

- **fetch** downloads the raw page.
- **cache** writes the raw HTML to a plaintext file on disk (see
  [Fetching & caching](#fetching--caching)). Parsing always reads from the cache,
  never directly from the network.
- **parse** runs `pup` to select the relevant nodes and `jq` to assemble the
  output document.
- **output** writes a `.json` file next to (or derived from) the cache file.

Intermediate HTML is kept on disk (not just piped) so that a fetch and a parse
are independently re-runnable and so parsing can be developed offline against
fixtures.

## Fetching & caching

J! Archive's disclaimer prohibits scraping. `jarch` is intended for personal,
low-volume, incremental use only. Fetch behaviour is therefore conservative:

- **Cache to plaintext files.** Every fetched page is written verbatim to a
  local cache file. The cache filename mirrors the site's URL path and query,
  e.g. the URL `showgame.php?game_id=6789` is cached as the file
  `showgame.php?game_id=6789`. (The sample fixtures in this repo follow exactly
  this convention.)
- **Cache-first.** Before fetching, `jarch` checks for an existing cache file.
  If present, it is used as-is unless the caller forces a refresh
  (`--refresh`). This makes re-parsing free and avoids re-hitting the site.
- **Polite fetching.** Network fetches should be rate-limited (a small delay
  between requests) and send a descriptive User-Agent. Bulk crawling is out of
  scope; `jarch` fetches only what it is asked for.

## href resolution policy

**hrefs are stored verbatim, exactly as they appear in the source HTML — i.e.
site-relative** (`showgame.php?game_id=6789`, `showplayer.php?player_id=5162`).

Rationale: relative hrefs match the source, keep output stable if the site's
scheme/host changes, and double as cache keys (the href *is* the cache
filename). Callers that need an absolute URL resolve on demand against the
documented base:

```
BASE_URL = https://j-archive.com/
```

(An absolute-href alternative is noted in `SUGGESTIONS.md`.)

## CLI interface

`jarch` is invoked as a single command with subcommands. Every subcommand
follows the fetch → cache → parse → output pipeline above and prints the
resulting JSON to stdout (and/or writes it to the conventional output path).

```
jarch seasons                 # parse listseasons.php  -> list of seasons
jarch season <season_id>      # parse showseason.php?season=<id> -> list of games
jarch game <game_id>          # parse showgame.php?game_id=<id>  -> full episode
jarch player <player_id>      # parse showplayer.php?player_id=<id> -> player + games
```

Arguments:

- `<season_id>` is an **opaque string**, not necessarily numeric. Regular
  seasons are integers (`42`), but tournaments and specials use alphanumeric
  ids (`jm`, `cwcpi`, `pcj`). Pass it through unmodified.
- `<game_id>` and `<player_id>` are integers.

Global flags:

- `--refresh` — ignore any cached HTML and re-fetch from the network.
- `--out <path>` — write JSON to `<path>` instead of the default.
- `--stdout` — write JSON to stdout only (no file).

Default output paths:

| Subcommand | Default output file |
| --- | --- |
| `seasons` | `listseasons.json` |
| `season <id>` | `season-<id>.json` |
| `game <id>` | `<id>.json` |
| `player <id>` | `player-<id>.json` |

## Page Types

### List Seasons

Input: `listseasons.php` — a table of seasons.

```html
<div id="content">
<table>
    <tr><td><a href="showseason.php?season=42">Season 42</a></td><td class="left_padded">2025-09-08 to 2026-07-24</td><td align="right" class="left_padded">(215 games archived)</td><td align="right" class="left_padded"></td></tr>
    <tr><td><a href="showseason.php?season=cwcpi">Audio-only games</a></td><td class="left_padded">2024-01-12 to 2024-02-02</td><td align="right" class="left_padded">(3 games archived)</td></tr>
  <tr><td><a href="showseason.php?season=jm"><i>Jeopardy! Masters</i></a></td><td class="left_padded">2023-05-08 to 2023-05-24</td><td align="right" class="left_padded">(56 games archived)</td></tr>
<table>
```

Parsing rules:

- One object per `<tr>`.
- `title` — the text of the season link. Strip inner markup such as `<i>` (so
  `<i>Jeopardy! Masters</i>` → `Jeopardy! Masters`).
- `href` — the link's `href`, stored verbatim.
- `start_date` / `end_date` — split the `"<start> to <end>"` date cell on `to`.
- `games_archived` — the integer inside `(N games archived)`.

Expected output:

```json
[
  {
    "title": "Season 42",
    "start_date": "2025-09-08",
    "end_date": "2026-07-24",
    "games_archived": 215,
    "href": "showseason.php?season=42"
  },
  {
    "title": "Audio-only games",
    "start_date": "2024-01-12",
    "end_date": "2024-02-02",
    "games_archived": 3,
    "href": "showseason.php?season=cwcpi"
  },
  {
    "title": "Jeopardy! Masters",
    "start_date": "2023-05-08",
    "end_date": "2023-05-24",
    "games_archived": 56,
    "href": "showseason.php?season=jm"
  }
]
```

### Seasons

Input: `showseason.php?season=<id>` — a table of the games in one season.

```html
<div id="content">
<h2 class="season">Season 42</h2>
<table>
  <tr>
    <td align="left" valign="top" style="width:140px">
      <a href="showgame.php?game_id=9489" title="Taped 2026-06-02">&#35;9600, aired&#160;2026-07-03</a>
    </td>
    <td valign="top">
      Caleb Groen vs. Derek Vance vs. Amber Born
    </td>
    <td valign="top" class="left_padded">
    </td>
  </tr>
  <tr>
    <td align="left" valign="top" style="width:140px">
      <a href="showgame.php?game_id=9486" title="Taped 2026-06-02">&#35;9597, aired&#160;2026-06-30</a>
    </td>
    <td valign="top">
      Adam Remsen vs. Richard Nguyen vs. Mandy Moreno
    </td>
    <td valign="top" class="left_padded">
      Adam Remsen game 13.
    </td>
  </tr>
```

Each `<tr>` is one game and produces one object with these fields:

- `episode_number` — the `#NNNN` in the first cell (strip the `#`). `&#35;` is
  `#` and `&#160;` is a non-breaking space; decode entities before parsing.
- `air_date` — the `aired YYYY-MM-DD` date in the first cell.
- `taped_date` — the `YYYY-MM-DD` from the link's `title="Taped …"` attribute.
- `href` / `game_id` — the link href (verbatim) and the `game_id` query value.
- `contestants` — the second cell, split on `" vs. "` into a list of names.
- `note` — the raw text of the third cell (`left_padded`), or `null` if empty.
  Inner markup (`<i>`) is stripped to plain text.
- `returning_champion` — see [note parsing](#note-parsing) below.
- `tournament_context` — see [note parsing](#note-parsing) below.

#### note parsing

The third cell (`note`) is free text but follows a small set of conventions.
Two structured fields are derived from it. **The same rules apply to the
game-page `game_comments` field** (see Episodes) and to the games table on
player pages.

**`returning_champion`** — parsed from a `"<Name> game <N>."` pattern where
`<Name>` also appears in that row's `contestants` list:

- Pattern: `^(?<name>.+?) game (?<n>\d+)\.` at the start of the note.
- `name` must match one of the `contestants` (this disambiguates it from other
  prose). If it matches, emit `{ "name": <name>, "game_number": <n> }`.
- Otherwise `returning_champion` is `null`.

Example: note `"Adam Remsen game 13."` with Adam Remsen among the contestants →

```json
{ "returning_champion": { "name": "Adam Remsen", "game_number": 13 },
  "note": "Adam Remsen game 13." }
```

**`tournament_context`** — parsed from tournament-style notes such as
`"2026 Primetime Celebrity Jeopardy! All-Stars semifinal game 3."`:

- `round` — the first recognised round keyword found in the note, lower-cased.
  Recognised keywords (extend as encountered):
  `quarterfinal`, `semifinal`, `final`, `finals`, `tiebreaker`,
  `wild card`, `wildcard`.
- `game_number` — the integer following `game` (`game 3` → `3`), else `null`.
- `tournament` — the note with the trailing `"<round> game <N>."` /
  `"<round> game."` / `"<round>."` clause removed, trimmed of a trailing
  `game`/period.
- If no round keyword is present, `tournament_context` is `null`.

Example →

```json
{ "tournament": "2026 Primetime Celebrity Jeopardy! All-Stars",
  "round": "semifinal",
  "game_number": 3 }
```

When neither pattern matches, both `returning_champion` and
`tournament_context` are `null` and only `note` carries the text.

Full expected output for a regular-season game with a returning champion:

```json
{
  "episode_number": 9597,
  "taped_date": "2026-06-02",
  "air_date": "2026-06-30",
  "contestants": ["Adam Remsen", "Richard Nguyen", "Mandy Moreno"],
  "href": "showgame.php?game_id=9486",
  "game_id": 9486,
  "returning_champion": { "name": "Adam Remsen", "game_number": 13 },
  "note": "Adam Remsen game 13.",
  "tournament_context": null
}
```

### Episodes

Input: `showgame.php?game_id=<id>` (e.g. `showgame.php?game_id=6789`)
Output: `<id>.json` (e.g. `6789.json`)

The full worked example is the `6789.json` fixture in this repo. This section
specifies the schema and how each field is parsed. (See `SUGGESTIONS.md` for
sample-data corrections needed to make that fixture fully conform.)

#### Top-level fields

```json
{
  "game_id": 6789,
  "show_number": 1834,
  "air_date": "1992-07-16",
  "tape_date": "1992-02-25",
  "game_comments": "1992 Seniors Tournament final game 1.",
  "tournament_context": { "tournament": "1992 Seniors Tournament", "round": "final", "game_number": 1 },
  "navigation": { … },
  "contestants": [ … ],
  "rounds": { "jeopardy": {…}, "double_jeopardy": {…}, "final_jeopardy": {…} },
  "coryat_scores": [ … ],
  "combined_coryat": 18800
}
```

- `game_id` — from the requested URL.
- `show_number` — the `#NNNN` in `<div id="game_title"><h1>Show #1834 - …</h1>`.
- `air_date` — the human date in the title, normalised to `YYYY-MM-DD`.
- `tape_date` — from the `Game tape date: YYYY-MM-DD` line near the foot of the
  page (`<h6>`).
- `game_comments` — text of `<div id="game_comments">`, or `null` if empty.
- `tournament_context` — derived from `game_comments` using the exact
  [note-parsing](#note-parsing) rules from the Seasons section.

#### navigation

Parsed from the `[<< previous game]` / `[next game >>]` links in
`<table id="contestants_table">`:

```json
"navigation": {
  "previous_game": { "game_id": 6788, "href": "showgame.php?game_id=6788" },
  "next_game":     { "game_id": 6790, "href": "showgame.php?game_id=6790" }
}
```

If a link is absent (first or last archived game), that side is `null`
(`"previous_game": null`). Keep the keys; only the value is nulled.

#### contestants

Parsed from the `<p class="contestants">` paragraphs. Each is
`<a href="showplayer.php?player_id=ID">Full Name</a>, <description>`:

```json
{
  "player_id": 13521,
  "name": "Carolyn Meek",
  "description": "a high school English teacher from Houston, Texas",
  "href": "showplayer.php?player_id=13521"
}
```

- `name` — the link text.
- `player_id` / `href` — from the `showplayer.php?player_id=` link (verbatim
  href; `player_id` is the query value). This link is the join key to
  [player pages](#player-pages).
- `description` — the paragraph text after the link, with the leading `, `
  stripped. `null` if absent.

Note: the short nicknames used throughout the clue/score data (`Carolyn`,
`Leonard`, `Hansford`) correspond to these contestants by first name / the
`score_player_nickname` cells; `contestants[].name` holds the full name.

#### rounds

`rounds` has three keys: `jeopardy`, `double_jeopardy`, `final_jeopardy`.
The first two share the same shape (`categories`, `clues`, `scores`); Final
Jeopardy has its own shape.

##### categories

From each round's six `<td class="category">` cells:

```json
{ "column": 1, "name": "BODIES OF WATER", "comments": null }
```

- `column` — 1–6, left to right.
- `name` — `td.category_name` text.
- `comments` — `td.category_comments` text, or `null` if empty.

##### clues

One object per board cell. The DOM id `clue_J_<col>_<row>` (J / DJ) encodes the
board position; the revealed cell is the sibling id with the `_r` suffix.

```json
{
  "clue_id": 394874,
  "board_position": { "column": 1, "row": 1 },
  "play_order": 12,
  "value": 100,
  "is_daily_double": false,
  "wager": null,
  "text": "Named for the Atlas Mountains, it's the second-largest body of water in the world",
  "correct_response": "the Atlantic Ocean",
  "host_comment": null,
  "post_response_comment": null,
  "triple_stumper": false,
  "responses": [ { "contestant": "Hansford", "result": "correct" } ]
}
```

Field parsing:

- `clue_id` — the `clue_id=` value in the `suggestcorrection.php?clue_id=…`
  link inside `td.clue_order_number`.
- `board_position` — `column`/`row` from the `clue_J_<col>_<row>` id.
- `play_order` — the integer shown in `td.clue_order_number` (the order the
  clue was actually played). `null` if the clue was not revealed.
- **`value`** — the dollar **board value** of the cell, always an integer.
  - For a normal clue it is the `td.clue_value` amount (`$100` → `100`).
  - For a **Daily Double** the board value is *not* shown (the cell shows
    `DD: $<wager>`). **Derive `value` from the clue's row**: the five non-DD
    clues in the same row all share the row's value. Do **not** hardcode a
    fixed value ladder — J! Archive values doubled in Nov 2001, so the ladder is
    era-dependent; deriving from siblings is era-proof.
  - This field replaces the old `face_value`. `face_value` must not be emitted.
- **`wager`** — integer amount risked on a Daily Double (from `DD: $<n>` in
  `td.clue_value_daily_double`). `null` for every non-DD clue.
- `is_daily_double` — `true` iff the header shows
  `td.clue_value_daily_double`.
- `text` — the clue text from the `td#clue_J_<col>_<row>` cell.
- `correct_response` — the `<em class="correct_response">` text in the `_r`
  cell (strip inner `<i>` etc.).
- `host_comment` / `post_response_comment` — see
  [host comments](#host-comments) below.
- `triple_stumper` — `true` iff the `_r` cell contains a
  `<td class="wrong">Triple Stumper</td>` element; else `false`.
- `responses` — see [responses](#responses) below.

##### responses

The `_r` cell contains contestant attempts in two places:

1. **Parenthetical given responses** before the reveal:
   `(Hansford: Uh, what is the Pacific Ocean?)` — a wrong/attempted response,
   with the contestant nickname before the colon and the spoken response after.
2. **Result tables** after the reveal: a `<td class="right">Name</td>` marks a
   correct responder; `<td class="wrong">Name</td>` marks an incorrect one.

Combine them into `responses`, in play order:

- Correct responder → `{ "contestant": "<name>", "result": "correct" }`.
- Incorrect responder → `{ "contestant": "<name>", "result": "incorrect",
  "given_response": "<what they said>" }` where `given_response` comes from the
  matching parenthetical.
- A correct response may also carry a `given_response` when the archive records
  the exact phrasing (e.g. `"What's The [*] Show?"`).

##### host comments

Host remarks appear in the `_r` cell as a parenthetical whose speaker is the
**host** (e.g. `(Alex: …)`), distinct from contestant parentheticals whose
speaker is a contestant nickname. Detect a host parenthetical by matching the
host name (the text before the first colon) against the show's host rather than
against a contestant nickname.

There are two fields:

- `host_comment` — a host remark tied to the clue itself (a quote, hint, or
  read-aloud), e.g. Final Jeopardy's `(Alex: I am big! It's the pictures that
  got small.)`.
- `post_response_comment` — a host remark adjudicating or confirming *after* a
  response, e.g. `(Alex: Yes, ambergris.)`.

**Known limitation:** in the current markup both appear as identical
`(Host: …)` parentheticals in the same position (before `<em
class="correct_response">`), so which field a given comment belongs to is **not
mechanically determinable** — it is a semantic judgment. The parser should:

1. Extract every host parenthetical from the `_r` cell.
2. Populate `host_comment` by default; `post_response_comment` is a best-effort
   / manual classification and may require a human review pass.

Both default to `null` when no host parenthetical is present. See
`SUGGESTIONS.md` for the recommended long-term resolution (collapse to one
field, or accept the manual pass).

##### scores (per round)

Each round carries a `scores` object holding the score snapshots J! Archive
publishes for that round. Snapshot arrays list one `{ "contestant", "score" }`
per player (scores may be negative).

- **Jeopardy round** — one entry per commercial break plus the round end:
  ```json
  "scores": {
    "after_clue_14": [
      { "contestant": "Hansford", "score": 600 },
      { "contestant": "Leonard", "score": 2000 },
      { "contestant": "Carolyn", "score": 0 }
    ],
    "end_of_round": [
      { "contestant": "Hansford", "score": 2200 },
      { "contestant": "Leonard", "score": 4100 },
      { "contestant": "Carolyn", "score": -400 }
    ]
  }
  ```
  Commercial-break snapshots are keyed `after_clue_<N>` where `<N>` is taken
  from the `<h3>Scores at the first commercial break (after clue N):</h3>`
  heading. A round may have more than one break; emit one key per break.
- **Double Jeopardy round** — `end_of_round` only (from `<h3>Scores at the end
  of the Double Jeopardy! Round:</h3>`).
- Source: each `<h3>` heading is followed by a table whose first row is
  `td.score_player_nickname` (names) and second row is
  `td.score_positive` / `td.score_negative` (amounts). Parse `$2,200` → `2200`,
  `-$400` → `-400`.

> Note on the fixture: the Double Jeopardy end-of-round score table also has a
> trailing `td.score_remarks` row, which is empty here. It corresponds to the
> "Final scores" remarks and is generally blank for round tables.

##### final_jeopardy

```json
"final_jeopardy": {
  "category": "FILMS OF THE '50s",
  "category_comments": null,
  "text": "One of the 1st lines in this William Holden film is \"The poor dope. He always wanted a pool.\"",
  "correct_response": "Sunset Boulevard",
  "host_comment": "I am big! It's the pictures that got small.",
  "responses": [
    { "contestant": "Carolyn", "result": "correct", "given_response": "What is Sunset Blvd.?", "wager": 1000 },
    { "contestant": "Leonard", "result": "correct", "given_response": "What is Sunset Blvd?", "wager": 4500 },
    { "contestant": "Hansford", "result": "correct", "given_response": "What is Sunset Blvd.?", "wager": 2000 }
  ],
  "scores": {
    "final": [
      { "contestant": "Hansford", "score": 10200 },
      { "contestant": "Leonard", "score": 11200 },
      { "contestant": "Carolyn", "score": 2000 }
    ]
  }
}
```

- `category` / `category_comments` — as for a round category.
- `text` — the `td#clue_FJ` clue text.
- `correct_response` — the `<em class="correct_response">` in `td#clue_FJ_r`.
- `host_comment` — the host parenthetical in `td#clue_FJ_r` (see
  [host comments](#host-comments)); the surrounding `(Alex: …)` wrapper is
  stripped.
- `responses` — one per contestant. The `clue_FJ_r` table pairs each
  `td.right`/`td.wrong` nickname with the written response and, on the next
  row, the `$` wager. `result` is `correct` for `td.right`, `incorrect` for
  `td.wrong`. `wager` is the integer amount.
- `scores.final` — the `<h3>Final scores:</h3>` table.

#### coryat_scores / combined_coryat

From the `<h3>Coryat scores:</h3>` table. Each column has a
`score_player_nickname`, a `score_positive`/`score_negative` amount, and a
`score_remarks` cell such as `27 R (including 1 DD), 5 W (including 2 DDs)`:

```json
"coryat_scores": [
  { "contestant": "Hansford", "score": 8200, "correct": 22, "incorrect": 3,
    "daily_doubles_correct": null, "daily_doubles_incorrect": null },
  { "contestant": "Leonard", "score": 9600, "correct": 27, "incorrect": 5,
    "daily_doubles_correct": 1, "daily_doubles_incorrect": 2 }
]
```

- `score` — the Coryat amount.
- `correct` / `incorrect` — the `N R` / `N W` counts in `score_remarks`.
- `daily_doubles_correct` / `daily_doubles_incorrect` — the `(including N DD…)`
  counts inside the R / W clauses respectively; `null` when the clause is
  absent.
- `combined_coryat` — the amount in `<h3>Combined Coryat: $18,800</h3>`, at the
  top level. (Placement is discussed in `SUGGESTIONS.md`.)

## Player Pages

Input: `showplayer.php?player_id=<id>` (e.g. `showplayer.php?player_id=5162`)
Output: `player-<id>.json`

A player page has a bio block and a table of the player's archived games. The
games table is **structurally identical to the Seasons games table**, so it is
parsed with the same row parser (including [note parsing](#note-parsing)).

```html
<p class="player_occupation_and_origin">An administrative assistant from Seattle, Washington...</p>
<p class="player_full_name">Emily Thorsley</p>
…
<tr><td colspan="2" class="top_padded">Emily appeared in the following 2 archived games:</td></tr>
<tr><table>
  <tr>
    <td align="left" valign="top" style="width:140px">
      <a href="showgame.php?game_id=2672" title="Taped 2008-07-15">&#35;5516, aired&#160;2008-09-08</a>
    </td>
    <td valign="top">Emily Thorsley vs. Greg Lindsay vs. Kurt Davies</td>
    <td valign="top" class="left_padded">First game of Season 25. New title graphics. New theme...</td>
  </tr>
  <tr>
    <td align="left" valign="top" style="width:140px">
      <a href="showgame.php?game_id=2581" title="Taped 2008-04-23">&#35;5515, aired&#160;2008-07-25</a>
    </td>
    <td valign="top">Mark Wales vs. Matt Heimer vs. Emily Thorsley</td>
    <td valign="top" class="left_padded">Mark Wales game 6.
Last game of Season 24.</td>
  </tr>
</table></tr>
```

Expected output:

```json
{
  "player_id": 5162,
  "name": "Emily Thorsley",
  "occupation_and_origin": "An administrative assistant from Seattle, Washington...",
  "photo": "players/EmilyThorsleySeason25.jpg",
  "hometown_howdy": "Hey, Seattle.  I'm Emily Thorsley.  Grab a grande soy no-whip white mocha and watch me on Jeopardy! [Season 24]\n\nHey, Seattle. I'm defending champion Emily Thorsley. Watch me go for win number two on Jeopardy! [Season 25]",
  "career_summary": "Season 24 1-time champion: $29,800 + $1,000.",
  "stats_href": "showplayerstats.php?player_id=5162",
  "games": [
    {
      "episode_number": 5516,
      "taped_date": "2008-07-15",
      "air_date": "2008-09-08",
      "contestants": ["Emily Thorsley", "Greg Lindsay", "Kurt Davies"],
      "href": "showgame.php?game_id=2672",
      "game_id": 2672,
      "note": "First game of Season 25. New title graphics. New theme...",
      "returning_champion": null,
      "tournament_context": null
    },
    {
      "episode_number": 5515,
      "taped_date": "2008-04-23",
      "air_date": "2008-07-25",
      "contestants": ["Mark Wales", "Matt Heimer", "Emily Thorsley"],
      "href": "showgame.php?game_id=2581",
      "game_id": 2581,
      "note": "Mark Wales game 6.\nLast game of Season 24.",
      "returning_champion": { "name": "Mark Wales", "game_number": 6 },
      "tournament_context": null
    }
  ]
}
```

Bio parsing:

- `player_id` — from the requested URL.
- `name` — `p.player_full_name` text.
- `occupation_and_origin` — `p.player_occupation_and_origin` text, or `null`.
- `photo` — the `src` of `img.playerpicture`, verbatim; `null` if absent.
- `hometown_howdy` — `p.player_hometown_howdy` text; `<br />` becomes newline;
  inner markup (`<i>Jeopardy!</i>`) is flattened to text. `null` if absent.
- `career_summary` — the `p.text` summary line, or `null`.
- `stats_href` — the `showplayerstats.php?player_id=…` link, verbatim.

Games parsing:

- Reuse the Seasons row parser for every `<tr>` in the games table.
- `returning_champion` uses the same note pattern; the champion name is
  cross-checked against the row's `contestants`, so on the player's own losing
  game it correctly attributes the champion to the *other* player (e.g. Mark
  Wales, not Emily).
- The linkage between a game page's `contestants[].player_id` and this page is
  the shared `showplayer.php?player_id=` href: a contestant on a game page can
  be resolved to their player page, and a player page lists back the games they
  appeared in.
