# Shared helpers for jarch parsers.
#
# Two kinds of input are used throughout:
#   - pup `json{}` node trees (objects with .tag/.text/.children/attributes)
#   - raw HTML strings sliced out of the cached page with regexes (used where
#     text order around inline markup matters, since pup's json{} loses it)

def trim: sub("^[\\s ]+"; "") | sub("[\\s ]+$"; "");

# Decode the HTML entities that appear in J! Archive markup (and that pup
# re-escapes in its json/text output). NBSP is normalised to a plain space.
def unescape:
  gsub("&#(?<n>[0-9]+);"; ([.n | tonumber] | implode))
  | gsub("&quot;"; "\"")
  | gsub("&apos;"; "'")
  | gsub("&lt;"; "<")
  | gsub("&gt;"; ">")
  | gsub("&nbsp;"; " ")
  | gsub("&amp;"; "&")
  | gsub(" "; " ");

def strip_tags: gsub("<[^>]*>"; "");

def nl_norm: gsub("\r\n?"; "\n");

# <br /> (and the newline pup/J!A puts after it) becomes a single newline.
def brs_to_nl: gsub("<br ?/?>\n?"; "\n");

# Flatten an HTML fragment to plain text.
def htext: nl_norm | brs_to_nl | strip_tags | unescape | trim;

def money: gsub("[$,]"; "") | tonumber;

def nullify: if . == null or . == "" then null else . end;

# Flatten all text found under a pup json node, in tree order.
# Only safe when inline markup either wraps the whole text or trails it.
def rtext:
  [.. | objects | .text? // empty] | join(" ") | unescape | gsub("\\s+"; " ") | trim;

# All descendant nodes (including self) matching a condition, document order.
def nodes(f): [.. | objects | select(f)];

# All matches of a regex as objects of named captures.
def captures($re; $flags):
  [match($re; $flags) | .captures | map(select(.name != null) | {(.name): .string}) | add];
def captures($re): captures($re; "g");

# --- note parsing (SPEC "note parsing") -------------------------------------

def round_re: "quarterfinal|semifinal|finals|final|tiebreaker|wild card|wildcard";

# Input: a trimmed note string (or null). Output:
#   { returning_champion: {name, game_number} | null,
#     tournament_context: {tournament, round, game_number} | null }
def parse_note($contestants):
  (. // "") as $note
  | {
      returning_champion:
        ((($note | capture("^(?<name>.+?) game (?<n>[0-9]+)\\.")) // null) as $rc
         | if $rc != null and ($contestants | index($rc.name) != null)
           then { name: $rc.name, game_number: ($rc.n | tonumber) }
           else null
           end),
      tournament_context:
        ((($note | [match("(?i)" + round_re)] | first) // null) as $m
         | if $m == null then null
           else
             { tournament:
                 (($note
                   | sub("\\s*(?i)(" + round_re + ")(\\s+game(\\s+[0-9]+)?)?\\.?\\s*$"; "")) as $t
                  | (if $t == $note then $note[0:$m.offset] else $t end)
                  | sub("\\s*(game)?\\s*\\.?\\s*$"; "")
                  | trim | nullify),
               round: ($m.string | ascii_downcase),
               game_number:
                 ((($note | capture("(?i)(" + round_re + ")\\s+game\\s+(?<n>[0-9]+)")) // null
                   | if . != null then (.n | tonumber) else null end)
                  // (($note | capture("(?i)game\\s+(?<n>[0-9]+)")) // null
                      | if . != null then (.n | tonumber) else null end))
             }
           end)
    }
  # Grow the round vocabulary from real misses: flag tournament-shaped notes
  # that yielded no round keyword.
  | if .tournament_context == null and .returning_champion == null
       and ($note | test("(?i)tournament|championship|masters"))
    then ("jarch: no round keyword recognised in note: \($note)" | stderr | empty), .
    else .
    end;

# --- games-table rows (Seasons + Player pages) -------------------------------

# Parse the games table rows out of raw page HTML. The row layout is identical
# on showseason.php and showplayer.php.
def game_rows($html):
  [$html
   | captures("<td align=\"left\" valign=\"top\" style=\"width:140px\">\\s*"
       + "<a href=\"(?<href>showgame\\.php\\?game_id=[0-9]+)\" title=\"Taped (?<taped>[0-9-]+)\">(?<ep>.*?)</a>\\s*</td>\\s*"
       + "<td valign=\"top\">\\s*(?<contestants>.*?)\\s*</td>\\s*"
       + "<td valign=\"top\" class=\"left_padded\">(?<note>.*?)</td>"; "gm")
   | .[]
   | (.ep | unescape) as $ep
   | (.contestants | strip_tags | unescape | trim | split(" vs. ")) as $names
   | (.note | nl_norm | strip_tags | unescape | trim | nullify) as $note
   | ($note | parse_note($names)) as $parsed
   | { episode_number: ($ep | capture("#(?<n>[0-9]+)").n | tonumber),
       taped_date: .taped,
       air_date: ($ep | capture("aired (?<d>[0-9]{4}-[0-9]{2}-[0-9]{2})").d),
       contestants: $names,
       href: .href,
       game_id: (.href | capture("game_id=(?<n>[0-9]+)").n | tonumber),
       note: $note,
       returning_champion: $parsed.returning_champion,
       tournament_context: $parsed.tournament_context }
  ];
