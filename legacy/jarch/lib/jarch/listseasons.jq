# Parse listseasons.php. Input: none (uses --rawfile html).
# Season rows are single-line <tr> elements inside div#content.
include "common";

[$html
 | captures("<tr><td><a href=\"(?<href>showseason\\.php\\?season=[^\"]+)\">(?<title>.*?)</a></td>"
     + "<td class=\"left_padded\">(?<dates>[^<]*)</td>"
     + "<td[^>]*>\\((?<n>[0-9]+) games? archived\\)</td>")
 | .[]
 | (.dates | trim | split(" to ")) as $d
 | { title: (.title | strip_tags | unescape | trim),
     start_date: $d[0],
     end_date: $d[1],
     games_archived: (.n | tonumber),
     href: .href }
]
