# jplay — open issues

## Bugs

*(all fixed 2026-07-04)*

- ~~**Wager not deducted on Daily Double loss** / **Lost wagers not deducted from score**~~ — root cause: passing a Daily Double (Esc or empty submit) deducted nothing unless `pass_penalty` was on. A committed DD wager is now forfeited on a pass regardless of that knob (`tui/play.go resolve`); the pass hints in the clue modal warn about it. Final Jeopardy was never affected — it has no pass, and an incorrect/blank answer always deducted the wager.
- ~~**Scoring misfire on clue with mid-clue remarks**~~ — root cause was in **jarch**, not jplay: `parse_rcell` turned every parenthesized contestant remark into a response, defaulting to "incorrect" when the speaker had no right/wrong ruling (e.g. Caleb's quip on a triple stumper in game 9488 got him docked $600). Responses are now derived from the authoritative right/wrong `<td>` rulings, with the *last* paren line per contestant attached as `given_response` (earlier ones are host-prompted partial answers or DD banter). Verified against all 7 cached games; `jarch/testdata/7154.json` fixture regenerated.
- ~~**Opponent scores silently zero when a nickname isn't the first name**~~ — fixed 2026-11-17: the loader keyed contestants by first name, so a score-table nickname like "Matt" for "Matthew" (or "Mike S." / "Mike J.") never matched and that opponent's simulated score stayed at $0. jarch now emits `contestants[].nickname` and the loader joins on it (first-name fallback only for older JSON).

## In-game UX

- ~~**Show clue text when answer is revealed**~~ — done 2026-07-04: reveal modal repeats the clue text (dim) under the category header
- ~~**Announce when a category is run**~~ — done 2026-07-04: "★ You ran the category!" on the reveal that completes a column of 5 correct answers (manual `o` override can complete a run too); strict rule — all five cells must exist and be answered correctly
- ~~**Special treatment for triple stumpers**~~ — done 2026-07-04: reveal shows a TRIPLE STUMPER badge; gold "you got what no contestant did!" when you get it
- *(low priority)* **Surfacing the previous clue result mid-game** — exact affordance TBD; needs design before implementation
- *(low priority)* **Answer entry timer** — configurable on/off in KDL config

## Final Jeopardy

- ~~**Show opponent scores on the wager screen**~~ — done 2026-11-17: "Going into Final" standings (hidden when contestant scores are turned off in config) plus a one-line wagering hint — runaway cap, cover-the-double minimum, tie, or deficit

## End screen & stats

- ~~**Show contestant correct/incorrect counts alongside yours on end screen**~~ — done 2026-07-04: R/W column in the rank table (contestant counts tallied from archive responses incl. FJ)
- ~~**Per-game stats drill-down**~~ — done 2026-07-04: `d` on the end screen or Enter on a Recent row opens the full clue-by-clue record of that play-through (DB got `GameClues`; `SaveGame` now returns the row id)
- ~~**Landing page "Recent" list shows finish position**~~ — done 2026-07-04: "2nd of 4" on the landing list and a Finish column in the stats Recent tab (rank computed against saved contestant finals)

## Archive browser

- ~~**Episode list indicates played/cached status**~~ — done 2026-11-17: each episode shows `played` / `cached` / `remote`. Played games are matched by J! Archive game id (new `games.archive_id` column, migrated in place); rows saved before the migration fall back to matching the `J#<show>` label
- ~~**jarch as a separate tool**~~ — done 2026-11-16: jarch is built in (`jarch/` package); standalone use is `jplay --fetch`

## Session & profile management

- **Save and resume in-progress games**
- **Multiple player profiles with isolated stats**

## XDG

- ~~follow xdg standard~~ — done: config in `$XDG_CONFIG_HOME/jplay/config.kdl`, history in `$XDG_DATA_HOME/jplay/history.db`, archive cache in `$XDG_CACHE_HOME/jarch/` (via `github.com/adrg/xdg`)
