package jarch

// Shared text helpers and parsers, ported from the original lib/jarch/common.jq.
//
// The original ran on jq's Oniguruma regexes, where \s is Unicode-aware. Go's
// \s is ASCII-only, so patterns that used \s use ws (Unicode White_Space)
// instead.

import (
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// ws is a regex character-class body matching Unicode White_Space, which is
// what Oniguruma's \s matches.
const ws = `\s\v\x{85}\pZ`

func re(s string) *regexp.Regexp { return regexp.MustCompile(s) }

var (
	trimLeftRE  = re(`^[` + ws + `]+`)
	trimRightRE = re(`[` + ws + `]+$`)
	numEntityRE = re(`&#([0-9]+);`)
	tagRE       = re(`<[^>]*>`)
	crlfRE      = re(`\r\n?`)
	brRE        = re(`<br ?/?>\n?`)
	moneyRE     = re(`[$,]`)
	spacesRE    = re(` +`)
	wsRunRE     = re(`[` + ws + `]+`)
)

func trim(s string) string {
	return trimRightRE.ReplaceAllString(trimLeftRE.ReplaceAllString(s, ""), "")
}

// unescape decodes the HTML entities that appear in J! Archive markup (and
// that pup re-escaped in its output). NBSP is normalised to a plain space.
func unescape(s string) string {
	s = numEntityRE.ReplaceAllStringFunc(s, func(m string) string {
		n, err := strconv.Atoi(m[2 : len(m)-1])
		if err != nil {
			return m
		}
		return string(rune(n))
	})
	s = strings.ReplaceAll(s, `&quot;`, `"`)
	s = strings.ReplaceAll(s, `&apos;`, `'`)
	s = strings.ReplaceAll(s, `&lt;`, `<`)
	s = strings.ReplaceAll(s, `&gt;`, `>`)
	s = strings.ReplaceAll(s, `&nbsp;`, " ")
	s = strings.ReplaceAll(s, `&amp;`, `&`)
	s = strings.ReplaceAll(s, " ", " ")
	return s
}

func stripTags(s string) string { return tagRE.ReplaceAllString(s, "") }
func nlNorm(s string) string    { return crlfRE.ReplaceAllString(s, "\n") }
func brsToNL(s string) string   { return brRE.ReplaceAllString(s, "\n") }

// htext flattens an HTML fragment to plain text.
func htext(s string) string { return trim(unescape(stripTags(brsToNL(nlNorm(s))))) }

// ctext is clue-facing text: htext with space runs collapsed.
func ctext(s string) string { return spacesRE.ReplaceAllString(htext(s), " ") }

func squeeze(s string) string { return spacesRE.ReplaceAllString(s, " ") }

func money(s string) int {
	n, _ := strconv.Atoi(moneyRE.ReplaceAllString(s, ""))
	return n
}

func atoi(s string) int { n, _ := strconv.Atoi(s); return n }

// nullify maps "" to nil.
func nullify(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func intp(n int) *int { return &n }

// capture returns the named groups of the first match of r in s, or nil.
func capture(r *regexp.Regexp, s string) map[string]string {
	m := r.FindStringSubmatch(s)
	if m == nil {
		return nil
	}
	out := map[string]string{}
	for i, name := range r.SubexpNames() {
		if name != "" {
			out[name] = m[i]
		}
	}
	return out
}

// captures returns the named groups of every match of r in s.
func captures(r *regexp.Regexp, s string) []map[string]string {
	var out []map[string]string
	names := r.SubexpNames()
	for _, m := range r.FindAllStringSubmatch(s, -1) {
		c := map[string]string{}
		for i, name := range names {
			if name != "" {
				c[name] = m[i]
			}
		}
		out = append(out, c)
	}
	return out
}

// --- note parsing (SPEC "note parsing") -------------------------------------

const roundRE = `quarterfinal|semifinal|finals|final|tiebreaker|wild card|wildcard`

var (
	championRE      = re(`^(?P<name>.+?) game (?P<n>[0-9]+)\.`)
	roundKeywordRE  = re(`(?i)` + roundRE)
	tournamentTail  = re(`[` + ws + `]*(?i)(` + roundRE + `)([` + ws + `]+game([` + ws + `]+[0-9]+)?)?\.?[` + ws + `]*$`)
	gameSuffixRE    = re(`[` + ws + `]*(game)?[` + ws + `]*\.?[` + ws + `]*$`)
	roundGameNumRE  = re(`(?i)(` + roundRE + `)[` + ws + `]+game[` + ws + `]+(?P<n>[0-9]+)`)
	anyGameNumRE    = re(`(?i)game[` + ws + `]+(?P<n>[0-9]+)`)
	tournamentishRE = re(`(?i)tournament|championship|masters`)
)

type ReturningChampion struct {
	Name       string `json:"name"`
	GameNumber int    `json:"game_number"`
}

type TournamentContext struct {
	Tournament *string `json:"tournament"`
	Round      string  `json:"round"`
	GameNumber *int    `json:"game_number"`
}

// parseNote extracts the returning champion and tournament context from an
// episode note. warn receives a message for tournament-shaped notes with no
// recognised round keyword, so the vocabulary can be grown from real misses.
func parseNote(note *string, contestants []string, warn io.Writer) (*ReturningChampion, *TournamentContext) {
	n := ""
	if note != nil {
		n = *note
	}
	var rc *ReturningChampion
	if c := capture(championRE, n); c != nil {
		for _, name := range contestants {
			if name == c["name"] {
				rc = &ReturningChampion{Name: c["name"], GameNumber: atoi(c["n"])}
				break
			}
		}
	}

	var tc *TournamentContext
	if loc := roundKeywordRE.FindStringIndex(n); loc != nil {
		t := tournamentTail.ReplaceAllString(n, "") // anchored at $: at most one match
		if t == n {
			t = n[:loc[0]]
		}
		t = trim(gameSuffixRE.ReplaceAllString(t, ""))
		tc = &TournamentContext{
			Tournament: nullify(t),
			Round:      strings.ToLower(n[loc[0]:loc[1]]),
		}
		if c := capture(roundGameNumRE, n); c != nil {
			tc.GameNumber = intp(atoi(c["n"]))
		} else if c := capture(anyGameNumRE, n); c != nil {
			tc.GameNumber = intp(atoi(c["n"]))
		}
	}

	if tc == nil && rc == nil && tournamentishRE.MatchString(n) && warn != nil {
		fmt.Fprintf(warn, "jarch: no round keyword recognised in note: %s\n", n)
	}
	return rc, tc
}

// --- games-table rows (Season + Player pages) -------------------------------

// GameRow is one row of a games table (showseason.php / showplayer.php).
type GameRow struct {
	EpisodeNumber     int                `json:"episode_number"`
	TapedDate         string             `json:"taped_date"`
	AirDate           string             `json:"air_date"`
	Contestants       []string           `json:"contestants"`
	Href              string             `json:"href"`
	GameID            int                `json:"game_id"`
	Note              *string            `json:"note"`
	ReturningChampion *ReturningChampion `json:"returning_champion"`
	TournamentContext *TournamentContext `json:"tournament_context"`
}

var (
	gameRowRE = re(`(?s)<td align="left" valign="top" style="width:140px">[` + ws + `]*` +
		`<a href="(?P<href>showgame\.php\?game_id=[0-9]+)" title="Taped (?P<taped>[0-9-]+)">(?P<ep>.*?)</a>[` + ws + `]*</td>[` + ws + `]*` +
		`<td valign="top">[` + ws + `]*(?P<contestants>.*?)[` + ws + `]*</td>[` + ws + `]*` +
		`<td valign="top" class="left_padded">(?P<note>.*?)</td>`)
	episodeNumRE = re(`#(?P<n>[0-9]+)`)
	airedRE      = re(`aired (?P<d>[0-9]{4}-[0-9]{2}-[0-9]{2})`)
	gameIDRE     = re(`game_id=(?P<n>[0-9]+)`)
)

// gameRows parses the games table rows out of raw page HTML. Rows missing an
// episode number or air date are skipped (as the jq original did).
func gameRows(html string, warn io.Writer) []GameRow {
	rows := []GameRow{}
	for _, c := range captures(gameRowRE, html) {
		ep := unescape(c["ep"])
		epn := capture(episodeNumRE, ep)
		aired := capture(airedRE, ep)
		if epn == nil || aired == nil {
			continue
		}
		names := strings.Split(trim(unescape(stripTags(c["contestants"]))), " vs. ")
		note := nullify(trim(unescape(stripTags(nlNorm(c["note"])))))
		rc, tc := parseNote(note, names, warn)
		rows = append(rows, GameRow{
			EpisodeNumber:     atoi(epn["n"]),
			TapedDate:         c["taped"],
			AirDate:           aired["d"],
			Contestants:       names,
			Href:              c["href"],
			GameID:            atoi(capture(gameIDRE, c["href"])["n"]),
			Note:              note,
			ReturningChampion: rc,
			TournamentContext: tc,
		})
	}
	return rows
}
