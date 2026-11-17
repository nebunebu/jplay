# Parse showgame.php?game_id=<id>.
# Input: pup 'div#content json{}' on stdin; --rawfile html; --arg game_id.
#
# Structured parts (headers, score tables, categories, contestants) come from
# the pup json tree; mixed-content cells (clue text, revealed "_r" cells) are
# sliced out of the raw HTML, where each cell is a single line, so inline
# markup order is preserved.
include "common";

# Clue-facing text: entities decoded, tags flattened, space runs collapsed.
def ctext: htext | gsub(" +"; " ");

def month_num:
  { January: "01", February: "02", March: "03", April: "04",
    May: "05", June: "06", July: "07", August: "08",
    September: "09", October: "10", November: "11", December: "12" }[.];

# "Thursday, July 16, 1992" -> "1992-07-16"
def human_date:
  capture("(?<mon>[A-Z][a-z]+) (?<d>[0-9]+), (?<y>[0-9]{4})")
  | "\(.y)-\(.mon | month_num)-\(if (.d | length) == 1 then "0" + .d else .d end)";

# --- score tables -------------------------------------------------------------

# One {contestant, score} per column of a score table node.
def score_table:
  . as $t
  | ($t | nodes(.class? == "score_player_nickname") | map(.text | unescape)) as $names
  | ($t | nodes(.class? == "score_positive" or .class? == "score_negative")
        | map(.text | unescape | money)) as $scores
  | [range(0; $names | length) | { contestant: $names[.], score: $scores[.] }];

# Pair each <h3> heading in a round div with the table that follows it.
def h3_tables:
  (.children // []) as $ch
  | [range(0; $ch | length) as $i
     | $ch[$i]
     | select(.tag == "h3")
     | { heading: rtext, table: $ch[$i + 1] }
     | select(.table.tag? == "table")];

# --- revealed-cell ("_r") parsing ---------------------------------------------

# $rcell: raw inner HTML of the _r cell; $nicknames: contestant nicknames.
# Returns {correct_response, host_comment, responses, triple_stumper}.
def parse_rcell($nicknames):
  . as $s
  | ($s | split("<em class=\"correct_response\"")[0]) as $pre
  | ($pre | split("<br />")
     | map(strip_tags | unescape | trim)
     | map(capture("^\\((?<speaker>[^:]+): ?(?<said>.*)\\)$") // empty)) as $parens
  | ($parens | map(select(.speaker as $n | $nicknames | index($n)))) as $cparens
  | ($parens | map(select(.speaker as $n | $nicknames | index($n) | not))) as $hparens
  | ($s | captures("<td class=\"(?<cls>right|wrong)\">(?<name>[^<]*)</td>")
        | map(select(.name != "Triple Stumper"))) as $tds
  | { correct_response:
        ((($s | capture("<em class=\"correct_response\">(?<r>.*?)</em>")) // null)
         | if . == null then null else (.r | ctext) end),
      host_comment:
        ($hparens | map(.said | gsub(" +"; " ")) | join("\n") | nullify),
      triple_stumper: ($s | test(">Triple Stumper<")),
      # The right/wrong <td> rulings are the authoritative response list; a
      # contestant paren line with no ruling is mid-clue banter, not a
      # response (e.g. a quip on a triple stumper), and must not be scored.
      # When a contestant has several paren lines (DD "make it a true Daily
      # Double" then the answer), the last one is the actual response.
      responses:
        ($tds | map(. as $t
          | ($cparens | map(select(.speaker == $t.name)) | last) as $p
          | { contestant: $t.name,
              result: (if $t.cls == "right" then "correct" else "incorrect" end) }
            + (if $p == null then {}
               else { given_response: ($p.said | gsub(" +"; " ")) } end)))
    };

# --- board clues ----------------------------------------------------------------

# Build one clue from its board cell; emits nothing if the cell is empty.
def build_clue($prefix; $col; $row; $rdiv; $html; $nicknames):
  "clue_\($prefix)_\($col)_\($row)" as $id
  | ($html | [match("<td id=\"\($id)\" class=\"clue_text\">(?<t>.*)</td>")] | first) as $textm
  | if $textm == null then empty
    else
      ($html | [match("<td id=\"\($id)_r\"[^>]*>(?<t>.*)</td>")] | first
       | if . == null then null else (.captures[0].string) end) as $rcell
      | ($rcell | if . == null
          then { correct_response: null, host_comment: null, triple_stumper: false, responses: [] }
          else parse_rcell($nicknames) end) as $r
      | ($rdiv | nodes(.tag? == "tr"
                       and ((.children // []) | any(.id? == $id + "_stuck")))
               | first) as $hdr
      | ($hdr | nodes(.class? == "clue_value") | first
              | if . == null then null else (.text | unescape | money) end) as $board_value
      | ($hdr | nodes(.class? == "clue_value_daily_double") | first
              | if . == null then null
                else (.text | unescape | capture("DD: \\$(?<n>[0-9,]+)").n | money) end) as $wager
      | ($hdr | nodes(.tag? == "a") | first) as $order_a
      | { clue_id:
            (($order_a.href? // "" | capture("clue_id=(?<n>[0-9]+)") // null)
             | if . == null then null else (.n | tonumber) end),
          board_position: { column: $col, row: $row },
          play_order:
            (($order_a.text? // null) | if . == null then null else tonumber end),
          value: $board_value,          # DDs filled in from row siblings below
          is_daily_double: ($wager != null),
          wager: $wager,
          text: ($textm.captures[0].string | ctext),
          correct_response: $r.correct_response,
          host_comment: $r.host_comment,
          post_response_comment: null,  # manual/curated field; see SPEC "host comments"
          triple_stumper: $r.triple_stumper,
          responses: $r.responses }
    end;

# A Daily Double cell hides the board value; recover it from the row's
# non-DD siblings (era-proof: no hardcoded value ladder).
def fill_dd_values:
  . as $clues
  | map(if .is_daily_double
        then .board_position.row as $row
             | .value = ($clues
                         | map(select(.board_position.row == $row
                                      and (.is_daily_double | not)
                                      and .value != null))
                         | first | .value? // null)
        else . end);

def build_round($prefix; $div_id; $content; $html; $nicknames):
  ($content | nodes(.id? == $div_id) | first) as $rdiv
  | { categories:
        (($rdiv | nodes(.class? == "category_name")) as $names
         | ($rdiv | nodes(.class? == "category_comments")) as $comments
         | [range(0; $names | length)
            | { column: (. + 1),
                name: ($names[.] | rtext),
                comments: ($comments[.] | rtext | nullify) }]),
      clues:
        ([range(1; 6) as $row | range(1; 7) as $col
          | build_clue($prefix; $col; $row; $rdiv; $html; $nicknames)]
         | fill_dd_values),
      scores:
        ($rdiv | h3_tables
         | map(if (.heading | test("commercial break"))
               then { key: "after_clue_\(.heading | capture("after clue (?<n>[0-9]+)").n)",
                      value: (.table | score_table) }
               elif (.heading | test("end of the")) then
                    { key: "end_of_round", value: (.table | score_table) }
               else empty end)
         | from_entries) };

# --- assemble -------------------------------------------------------------------

.[0] as $content
| ($content | nodes(.class? == "score_player_nickname")
            | map(.text | unescape) | unique) as $nicknames
| ($content | nodes(.id? == "game_title") | first
            | nodes(.tag? == "h1") | first | .text | unescape) as $title
| (($html | capture("<div id=\"game_comments\">(?<c>.*?)</div>")).c
   | htext | nullify) as $game_comments
| ($content | nodes(.id? == "contestants_table") | first
            | nodes(.tag? == "a" and (.href? // "" | test("showgame")))) as $nav
| ($content | nodes(.id? == "final_jeopardy_round") | first) as $fjdiv
| ($html | [match("<td id=\"clue_FJ_r\"[^>]*>(?<t>.*)</td>")] | first
         | if . == null then null else .captures[0].string end) as $fjr
| ($fjdiv | h3_tables) as $fj_tables
| ($content | nodes(.tag? == "p" and .class? == "contestants")) as $cps
| { game_id: ($game_id | tonumber),
    show_number: ($title | capture("#(?<n>[0-9]+)").n | tonumber),
    air_date: ($title | human_date),
    tape_date:
      (($html | capture("Game tape date: (?<d>[0-9]{4}-[0-9]{2}-[0-9]{2})") // null)
       | if . == null then null else .d end),
    game_comments: $game_comments,
    tournament_context: ($game_comments | parse_note([]) | .tournament_context),
    navigation:
      { previous_game:
          ($nav | map(select(rtext | test("previous game"))) | first
           | if . == null then null
             else { game_id: (.href | capture("game_id=(?<n>[0-9]+)").n | tonumber),
                    href: .href } end),
        next_game:
          ($nav | map(select(rtext | test("next game"))) | first
           | if . == null then null
             else { game_id: (.href | capture("game_id=(?<n>[0-9]+)").n | tonumber),
                    href: .href } end) },
    contestants:
      ($cps | map(
        (.children[0]) as $a
        | { player_id: ($a.href | capture("player_id=(?<n>[0-9]+)").n | tonumber),
            name: ($a | rtext),
            description: (.text // "" | unescape | sub("^,\\s*"; "") | trim | nullify),
            href: $a.href })),
    rounds:
      { jeopardy: build_round("J"; "jeopardy_round"; $content; $html; $nicknames),
        double_jeopardy: build_round("DJ"; "double_jeopardy_round"; $content; $html; $nicknames),
        final_jeopardy:
          { category: ($fjdiv | nodes(.class? == "category_name") | first | rtext),
            category_comments:
              ($fjdiv | nodes(.class? == "category_comments") | first | rtext | nullify),
            text:
              (($html | capture("<td id=\"clue_FJ\" class=\"clue_text\">(?<t>.*?)</td>")).t | ctext),
            correct_response:
              (($fjr | capture("<em class=\"correct_response\">(?<r>.*?)</em>")).r | ctext),
            host_comment:
              ($fjr | split("<table")[0] | strip_tags | unescape | trim
               | (capture("^\\((?<speaker>[^:]+): ?(?<said>.*)\\)$") // null)
               | if . == null then null else (.said | gsub(" +"; " ")) end),
            responses:
              ($fjr | captures("<td class=\"(?<cls>right|wrong)\">(?<name>[^<]*)</td>"
                               + "<td rowspan=\"2\"[^>]*>(?<gr>.*?)</td></tr><tr><td>\\$(?<w>[0-9,]+)</td>")
               | map({ contestant: (.name | unescape),
                       result: (if .cls == "right" then "correct" else "incorrect" end),
                       given_response: (.gr | ctext),
                       wager: (.w | money) })),
            scores:
              { final:
                  ($fj_tables | map(select(.heading | test("Final scores")))
                   | first | .table | score_table) } } },
    coryat_scores:
      (($fj_tables | map(select(.heading | test("Coryat scores"))) | first | .table) as $ct
       | ($ct | score_table) as $base
       | ($ct | nodes(.class? == "score_remarks") | map(rtext)) as $remarks
       | [range(0; $base | length) as $i
         | $base[$i]
         | ($remarks[$i] // "") as $rem
         | . + { correct:
                   ((($rem | capture("(?<n>[0-9]+) R") // null)
                     | if . == null then null else (.n | tonumber) end)),
                 incorrect:
                   ((($rem | capture("(?<n>[0-9]+) W") // null)
                     | if . == null then null else (.n | tonumber) end)),
                 daily_doubles_correct:
                   ((($rem | capture("R\\s*\\(including (?<n>[0-9]+) DD") // null)
                     | if . == null then null else (.n | tonumber) end)),
                 daily_doubles_incorrect:
                   ((($rem | capture("W\\s*\\(including (?<n>[0-9]+) DD") // null)
                     | if . == null then null else (.n | tonumber) end)) }]),
    combined_coryat:
      (($html | capture("Combined Coryat</a>: \\$(?<n>[0-9,]+)") // null)
       | if . == null then null else (.n | money) end)
  }
