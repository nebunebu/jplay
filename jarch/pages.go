package jarch

// Parsers for the index-style pages: listseasons.php, showseason.php and
// showplayer.php (ported from listseasons.jq, season.jq, player.jq).

import (
	"io"
	"strings"
)

// Season is one row of listseasons.php.
type Season struct {
	Title         string  `json:"title"`
	StartDate     string  `json:"start_date"`
	EndDate       *string `json:"end_date"`
	GamesArchived int     `json:"games_archived"`
	Href          string  `json:"href"`
}

var (
	seasonRowRE = re(`<tr><td><a href="(?P<href>showseason\.php\?season=[^"]+)">(?P<title>.*?)</a></td>` +
		`<td class="left_padded">(?P<dates>[^<]*)</td>` +
		`<td[^>]*>\((?P<n>[0-9]+) games? archived\)</td>`)
	seasonIDRE = re(`season=([^&]+)`)
)

// ID extracts the opaque season id (e.g. "42", "pcj") from the href.
func (s Season) ID() string {
	if m := seasonIDRE.FindStringSubmatch(s.Href); m != nil {
		return m[1]
	}
	return ""
}

func parseSeasons(html string) []Season {
	out := []Season{}
	for _, c := range captures(seasonRowRE, html) {
		d := strings.Split(trim(c["dates"]), " to ")
		s := Season{
			Title:         trim(unescape(stripTags(c["title"]))),
			StartDate:     d[0],
			GamesArchived: atoi(c["n"]),
			Href:          c["href"],
		}
		if len(d) > 1 {
			s.EndDate = &d[1]
		}
		out = append(out, s)
	}
	return out
}

func parseSeason(html string, warn io.Writer) []GameRow { return gameRows(html, warn) }

// Player is a parsed showplayer.php page.
type Player struct {
	PlayerID            int       `json:"player_id"`
	Name                string    `json:"name"`
	OccupationAndOrigin *string   `json:"occupation_and_origin"`
	Photo               *string   `json:"photo"`
	HometownHowdy       *string   `json:"hometown_howdy"`
	CareerSummary       *string   `json:"career_summary"`
	StatsHref           *string   `json:"stats_href"`
	Games               []GameRow `json:"games"`
}

var (
	playerNameRE  = re(`<p class="player_full_name">(?P<t>.*?)</p>`)
	playerOccRE   = re(`(?s)<p class="player_occupation_and_origin">(?P<t>.*?)</p>`)
	playerHowdyRE = re(`(?s)<p class="player_hometown_howdy">(?P<t>.*?)</p>`)
	playerTextRE  = re(`(?s)<p class="text">(?P<t>.*?)</p>`)
	playerStatsRE = re(`showplayerstats\.php\?player_id=[0-9]+`)
)

func parsePlayer(id int, html string, warn io.Writer) *Player {
	content := parseContent(html)
	p := &Player{PlayerID: id, Games: gameRows(html, warn)}
	if c := capture(playerNameRE, html); c != nil {
		p.Name = htext(c["t"])
	}
	if c := capture(playerOccRE, html); c != nil {
		p.OccupationAndOrigin = nullify(htext(c["t"]))
	}
	if pic := content.first(byClass("playerpicture")); pic != nil {
		if src := pic.attr("src"); src != "" {
			p.Photo = &src
		}
	}
	if c := capture(playerHowdyRE, html); c != nil {
		p.HometownHowdy = nullify(htext(c["t"]))
	}
	if c := capture(playerTextRE, html); c != nil {
		p.CareerSummary = nullify(htext(c["t"]))
	}
	if m := playerStatsRE.FindString(html); m != "" {
		p.StatsHref = &m
	}
	return p
}
