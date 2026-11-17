# jplay — open issues

## Open

### In-game UX
- *(low priority)* **Surface the previous clue result mid-game** — exact affordance TBD; needs design before implementation.
- *(low priority)* **Answer entry timer** — configurable on/off in the KDL config.

### Session & profile management
- **Save and resume in-progress games** (abandoned games are currently not recorded).
- **Multiple player profiles with isolated stats.**

Fetcher work is tracked in [`docs/jarch/TODO.md`](docs/jarch/TODO.md).

## Resolved

Kept as a short record; design reasoning lives in [`docs/jarch/SUGGESTIONS.md`](docs/jarch/SUGGESTIONS.md) and change history in [`docs/jarch/AUDIT.md`](docs/jarch/AUDIT.md).

**Bugs**
- Passing a Daily Double (Esc or empty submit) now forfeits the committed wager regardless of `pass_penalty` (`tui/play.go resolve`). Final Jeopardy was never affected.
- Mid-clue contestant remarks were scored as wrong responses (root cause in jarch's `parse_rcell`); responses are now derived from the authoritative right/wrong rulings, with the last parenthetical per contestant as `given_response`.
- Opponent scores stayed at $0 when a score-table nickname differed from the first name ("Matt" for "Matthew"); jarch now emits `contestants[].nickname` and the loader joins on it.

**In-game UX**
- Reveal modal repeats the clue text; "★ You ran the category!" on a completed column; TRIPLE STUMPER badge (and a gold note when you get it).
- Final Jeopardy wager screen shows "Going into Final" standings (hidden when contestant scores are off) and a one-line wagering hint.

**End screen & stats**
- R/W counts for contestants in the end-screen rank table; per-game clue-by-clue drill-down (`d` / Enter on a Recent row); finish position ("2nd of 4") on the landing list and stats Recent tab.

**Archive browser & infrastructure**
- Episodes are badged `played` / `cached` / `remote` (matched by `games.archive_id`, migrated in place; older rows fall back to the `J#<show>` label).
- jarch is built in (`jarch/` package; standalone use is `jplay --fetch`).
- XDG paths: config in `$XDG_CONFIG_HOME/jplay/config.kdl`, history in `$XDG_DATA_HOME/jplay/history.db`, archive cache in `$XDG_CACHE_HOME/jarch/`.
