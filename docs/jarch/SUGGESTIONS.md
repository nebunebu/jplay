# Suggestions

Non-blocking observations and open questions raised while writing `SPEC.md`.
None of these are required to start implementing; they are decisions worth
making deliberately.

## Sample-data corrections (`6789.json`)

The fixture predates the finalised schema and violates it in a few places.
Fix these so the fixture is a valid canonical example:

1. **`face_value` → `value`.** Clues `394874`, `394866`, and `394922` still use
   `face_value`; the schema uses `value` everywhere. Remove `face_value`.
2. **Inconsistent `wager` presence.** Non-DD clues should *all* carry
   `"wager": null`; several omit the key entirely (e.g. `394868`, `394879`).
   Emit it uniformly.
3. **DJ Daily Double `394911`** has `value: 2000` and no `wager`. That `2000`
   is the wager (`DD: $2,000` in the HTML). Under the schema it should be
   `value: 600` (the Double Jeopardy row-3 board value, derived from its five
   non-DD row siblings) and `wager: 2000`.
4. **DJ Daily Double `394918`** has `value: 1000` and no `wager`. It needs
   `value` = its row's board value and `wager` = the `DD: $…` amount from the
   page. (Re-fetch/parse to recover the wager.)
5. **`host_comment` vs `post_response_comment`.** Clue `394922` labels
   `(Alex: Yes, ambergris.)` as `post_response_comment`, but Final Jeopardy
   labels the structurally-identical `(Alex: …)` as `host_comment`. See below —
   this split is a judgment call, not something in the markup.

## `host_comment` / `post_response_comment` are not machine-separable

In the current J! Archive markup both a "clue" host remark and a
"post-response" host remark render as an identical `(Host: …)` parenthetical in
the same position (immediately before `<em class="correct_response">`). There
is no structural signal distinguishing them. Recommended resolutions, pick one:

- **Collapse to a single field** (`host_comment`, an array or single string of
  all host parentheticals in the cell). Simplest and fully mechanical; drop the
  post/response distinction.
- **Keep both fields but treat `post_response_comment` as a manual/curated
  field**, populated in a human review pass. The parser fills `host_comment`
  automatically and leaves `post_response_comment: null` for later.

The spec currently documents the second option. If exactness of the split is
not important to consumers, the first is less work and has no ambiguity.

## `combined_coryat` placement

`combined_coryat` currently sits at the top level, a sibling of
`coryat_scores`. Consider nesting the two together, e.g.:

```json
"coryat": {
  "per_contestant": [ … ],
  "combined": 18800
}
```

This keeps the Coryat data in one place. Kept flat in the current spec to match
the existing fixture; change both together if you adopt this.

## `scores` shape: object vs array

Round `scores` is an object keyed by snapshot (`after_clue_14`, `end_of_round`,
`final`). Commercial-break keys are dynamic (`after_clue_<N>`), which is a
little awkward for consumers that want to iterate snapshots in order. An array
of labelled snapshots would be more uniform:

```json
"scores": [
  { "label": "commercial_break", "after_clue": 14, "standings": [ … ] },
  { "label": "end_of_round", "after_clue": null, "standings": [ … ] }
]
```

Object form is kept in the spec to match the fixture; the array form is worth
considering if snapshot ordering/labelling matters.

## `href` absolute-URL alternative

The spec stores hrefs verbatim (site-relative) and documents a `BASE_URL`
constant for on-demand resolution. If downstream consumers would rather not
carry the base URL, an alternative is to store fully-qualified absolute URLs
(`https://j-archive.com/showgame.php?game_id=6789`) at parse time. Trade-off:
absolute URLs are self-contained but bind the data to the current scheme/host
and are no longer usable directly as cache filenames.

## `score_remarks` under "Final scores"

The "Final scores" table carries a `td.score_remarks` row that is empty in this
fixture but can contain notes in other games. It is currently not captured
anywhere (the Coryat R/W remarks are captured separately into `coryat_scores`).
If final-score remarks ever carry content worth keeping, add an optional
`remarks` field to the `final` snapshot.

## Nickname ↔ full-name mapping

Clue/score data use short nicknames (`Carolyn`), while `contestants[].name`
holds full names (`Carolyn Meek`). Consumers must map between them. Consider
emitting the nickname explicitly on each contestant (e.g.
`"nickname": "Carolyn"`, taken from the `score_player_nickname` cells) so the
join is data, not a first-name heuristic — nicknames can collide or differ from
the first name.

## Tournament round vocabulary

`tournament_context.round` is derived from a keyword list
(`quarterfinal`, `semifinal`, `final`, `tiebreaker`, …). This list will need
extending as new tournament formats appear (`play-in`, `wild card`, `group`,
etc.). Consider logging notes where a tournament-shaped comment yields no round
match, so the vocabulary can be grown from real misses rather than guessed up
front.

## Empty / unrevealed clues

Some archived games have missing or unrevealed clues (blank board cells,
timed-out categories). The spec assumes a full 30-clue board. Decide how to
represent an absent clue (omit vs emit with `play_order: null` and empty
`responses`) and state it, so parsers don't silently drop cells.
