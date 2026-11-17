# Parse showplayer.php?player_id=<id>.
# Input: pup 'div#content json{}' on stdin; --rawfile html; --arg player_id.
include "common";

.[0] as $content
| { player_id: ($player_id | tonumber),
    name:
      (($html | capture("<p class=\"player_full_name\">(?<t>.*?)</p>")).t | htext),
    occupation_and_origin:
      ((($html | capture("<p class=\"player_occupation_and_origin\">(?<t>.*?)</p>"; "m")) // null)
       | if . == null then null else (.t | htext | nullify) end),
    photo:
      ($content | nodes(.class? == "playerpicture") | first | .src? // null),
    hometown_howdy:
      ((($html | capture("<p class=\"player_hometown_howdy\">(?<t>.*?)</p>"; "m")) // null)
       | if . == null then null else (.t | nl_norm | brs_to_nl | strip_tags | unescape | trim | nullify) end),
    career_summary:
      ((($html | capture("<p class=\"text\">(?<t>.*?)</p>"; "m")) // null)
       | if . == null then null else (.t | htext | nullify) end),
    stats_href:
      (($html | [match("showplayerstats\\.php\\?player_id=[0-9]+")] | first | .string?) // null),
    games: game_rows($html)
  }
