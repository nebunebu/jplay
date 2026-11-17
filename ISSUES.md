# jplay — open issues

## Bugs

*(all fixed 2026-07-04)*

- ~~**Wager not deducted on Daily Double loss** / **Lost wagers not deducted from score**~~ — root cause: passing a Daily Double (Esc or empty submit) deducted nothing unless `pass_penalty` was on. A committed DD wager is now forfeited on a pass regardless of that knob (`tui/play.go resolve`); the pass hints in the clue modal warn about it. Final Jeopardy was never affected — it has no pass, and an incorrect/blank answer always deducted the wager.
- ~~**Scoring misfire on clue with mid-clue remarks**~~ — root cause was in **jarch**, not jplay: `parse_rcell` turned every parenthesized contestant remark into a response, defaulting to "incorrect" when the speaker had no right/wrong ruling (e.g. Caleb's quip on a triple stumper in game 9488 got him docked $600). Responses are now derived from the authoritative right/wrong `<td>` rulings, with the *last* paren line per contestant attached as `given_response` (earlier ones are host-prompted partial answers or DD banter). Verified against all 7 cached games; `jarch/7154.json` fixture regenerated.

## In-game UX

- ~~**Show clue text when answer is revealed**~~ — done 2026-07-04: reveal modal repeats the clue text (dim) under the category header
- ~~**Announce when a category is run**~~ — done 2026-07-04: "★ You ran the category!" on the reveal that completes a column of 5 correct answers (manual `o` override can complete a run too); strict rule — all five cells must exist and be answered correctly
- ~~**Special treatment for triple stumpers**~~ — done 2026-07-04: reveal shows a TRIPLE STUMPER badge; gold "you got what no contestant did!" when you get it
- *(low priority)* **Surfacing the previous clue result mid-game** — exact affordance TBD; needs design before implementation
- *(low priority)* **Answer entry timer** — configurable on/off in KDL config

## Final Jeopardy

- **Show opponent scores on the wager screen** — optional: add wagering strategy hints

## End screen & stats

- ~~**Show contestant correct/incorrect counts alongside yours on end screen**~~ — done 2026-07-04: R/W column in the rank table (contestant counts tallied from archive responses incl. FJ)
- ~~**Per-game stats drill-down**~~ — done 2026-07-04: `d` on the end screen or Enter on a Recent row opens the full clue-by-clue record of that play-through (DB got `GameClues`; `SaveGame` now returns the row id)
- ~~**Landing page "Recent" list shows finish position**~~ — done 2026-07-04: "2nd of 4" on the landing list and a Finish column in the stats Recent tab (rank computed against saved contestant finals)

## Archive browser

- **Episode list indicates played/cached status**

## Session & profile management

- **Save and resume in-progress games**
- **Multiple player profiles with isolated stats**

## XDG

- follow xdg standard
- config in $XDG_CONFIG/jplay/
- database in `~/.local/` or wherever it goes
