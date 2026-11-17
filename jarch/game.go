package jarch

// Parser for showgame.php (ported from game.jq).
//
// Structured parts (headers, score tables, categories, contestants) come from
// the DOM; mixed-content cells (clue text, revealed "_r" cells) are sliced out
// of the raw HTML, where each cell is a single line, so inline-markup order is
// preserved.

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

type Game struct {
	GameID            int                `json:"game_id"`
	ShowNumber        int                `json:"show_number"`
	AirDate           string             `json:"air_date"`
	TapeDate          *string            `json:"tape_date"`
	GameComments      *string            `json:"game_comments"`
	TournamentContext *TournamentContext `json:"tournament_context"`
	Navigation        Navigation         `json:"navigation"`
	Contestants       []Contestant       `json:"contestants"`
	Rounds            Rounds             `json:"rounds"`
	CoryatScores      []CoryatScore      `json:"coryat_scores"`
	CombinedCoryat    *int               `json:"combined_coryat"`
}

type Navigation struct {
	PreviousGame *NavLink `json:"previous_game"`
	NextGame     *NavLink `json:"next_game"`
}

type NavLink struct {
	GameID int    `json:"game_id"`
	Href   string `json:"href"`
}

type Contestant struct {
	PlayerID int    `json:"player_id"`
	Name     string `json:"name"`
	// Nickname is the short name used in score tables and clue responses
	// ("Matt" for "Matthew Amodio"); null if it cannot be determined.
	Nickname    *string `json:"nickname"`
	Description *string `json:"description"`
	Href        string  `json:"href"`
}

type Rounds struct {
	Jeopardy       Round `json:"jeopardy"`
	DoubleJeopardy Round `json:"double_jeopardy"`
	FinalJeopardy  Final `json:"final_jeopardy"`
}

type Round struct {
	Categories []Category `json:"categories"`
	Clues      []Clue     `json:"clues"`
	Scores     Snapshots  `json:"scores"`
}

type Category struct {
	Column   int     `json:"column"`
	Name     string  `json:"name"`
	Comments *string `json:"comments"`
}

type Position struct {
	Column int `json:"column"`
	Row    int `json:"row"`
}

type Clue struct {
	ClueID              *int       `json:"clue_id"`
	BoardPosition       Position   `json:"board_position"`
	PlayOrder           *int       `json:"play_order"`
	Value               *int       `json:"value"`
	IsDailyDouble       bool       `json:"is_daily_double"`
	Wager               *int       `json:"wager"`
	Text                string     `json:"text"`
	CorrectResponse     *string    `json:"correct_response"`
	HostComment         *string    `json:"host_comment"`
	PostResponseComment *string    `json:"post_response_comment"` // manual/curated; always null
	TripleStumper       bool       `json:"triple_stumper"`
	Responses           []Response `json:"responses"`
}

type Response struct {
	Contestant    string  `json:"contestant"`
	Result        string  `json:"result"`
	GivenResponse *string `json:"given_response,omitempty"`
}

type Final struct {
	Category         string          `json:"category"`
	CategoryComments *string         `json:"category_comments"`
	Text             *string         `json:"text"`
	CorrectResponse  *string         `json:"correct_response"`
	HostComment      *string         `json:"host_comment"`
	Responses        []FinalResponse `json:"responses"`
	Scores           FinalScores     `json:"scores"`
}

type FinalResponse struct {
	Contestant    string `json:"contestant"`
	Result        string `json:"result"`
	GivenResponse string `json:"given_response"`
	Wager         int    `json:"wager"`
}

type FinalScores struct {
	Final []Score `json:"final"`
}

type Score struct {
	Contestant string `json:"contestant"`
	Score      *int   `json:"score"`
}

type CoryatScore struct {
	Contestant            string `json:"contestant"`
	Score                 *int   `json:"score"`
	Correct               *int   `json:"correct"`
	Incorrect             *int   `json:"incorrect"`
	DailyDoublesCorrect   *int   `json:"daily_doubles_correct"`
	DailyDoublesIncorrect *int   `json:"daily_doubles_incorrect"`
}

// Snapshots is an ordered map of score snapshots ("after_clue_14",
// "end_of_round") to per-contestant scores.
type Snapshots []Snapshot

type Snapshot struct {
	Key    string
	Scores []Score
}

func (s Snapshots) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, kv := range s {
		if i > 0 {
			b.WriteByte(',')
		}
		k, err := marshal(kv.Key, "")
		if err != nil {
			return nil, err
		}
		v, err := marshal(kv.Scores, "")
		if err != nil {
			return nil, err
		}
		b.Write(bytes.TrimSpace(k))
		b.WriteByte(':')
		b.Write(bytes.TrimSpace(v))
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

func (s *Snapshots) set(key string, v []Score) {
	for i := range *s {
		if (*s)[i].Key == key {
			(*s)[i].Scores = v
			return
		}
	}
	*s = append(*s, Snapshot{key, v})
}

// --- helpers -------------------------------------------------------------------

var months = map[string]string{
	"January": "01", "February": "02", "March": "03", "April": "04",
	"May": "05", "June": "06", "July": "07", "August": "08",
	"September": "09", "October": "10", "November": "11", "December": "12",
}

var humanDateRE = re(`(?P<mon>[A-Z][a-z]+) (?P<d>[0-9]+), (?P<y>[0-9]{4})`)

// humanDate turns "Thursday, July 16, 1992" into "1992-07-16".
func humanDate(s string) string {
	c := capture(humanDateRE, s)
	if c == nil {
		return ""
	}
	d := c["d"]
	if len(d) == 1 {
		d = "0" + d
	}
	return c["y"] + "-" + months[c["mon"]] + "-" + d
}

// scoreTable returns one {contestant, score} per column of a score table.
func scoreTable(t *node) []Score {
	var names []string
	for _, n := range t.all(byClass("score_player_nickname")) {
		names = append(names, unescape(n.text))
	}
	var scores []int
	for _, n := range t.all(func(x *node) bool {
		c := x.attr("class")
		return c == "score_positive" || c == "score_negative"
	}) {
		scores = append(scores, money(unescape(n.text)))
	}
	out := []Score{}
	for i, name := range names {
		s := Score{Contestant: name}
		if i < len(scores) {
			s.Score = intp(scores[i])
		}
		out = append(out, s)
	}
	return out
}

type headedTable struct {
	heading string
	table   *node
}

// h3Tables pairs each <h3> child of a round div with the table that follows.
func h3Tables(div *node) []headedTable {
	var out []headedTable
	if div == nil {
		return out
	}
	ch := div.children
	for i, c := range ch {
		if c.tag() == "h3" && i+1 < len(ch) && ch[i+1].tag() == "table" {
			out = append(out, headedTable{c.rtext(), ch[i+1]})
		}
	}
	return out
}

// --- revealed-cell ("_r") parsing ------------------------------------------------

var (
	parenRE       = re(`^\((?P<speaker>[^:]+): ?(?P<said>.*)\)$`)
	rulingRE      = re(`<td class="(?P<cls>right|wrong)">(?P<name>[^<]*)</td>`)
	correctRespRE = re(`<em class="correct_response">(?P<r>.*?)</em>`)
)

type rcell struct {
	correct   *string
	host      *string
	stumper   bool
	responses []Response
}

func parseRCell(s string, nicknames []string) rcell {
	pre := strings.SplitN(s, `<em class="correct_response"`, 2)[0]
	type paren struct{ speaker, said string }
	var cparens []paren
	var hostLines []string
	for _, line := range strings.Split(pre, "<br />") {
		c := capture(parenRE, trim(unescape(stripTags(line))))
		if c == nil {
			continue
		}
		if slices.Contains(nicknames, c["speaker"]) {
			cparens = append(cparens, paren{c["speaker"], c["said"]})
		} else {
			hostLines = append(hostLines, squeeze(c["said"]))
		}
	}

	r := rcell{
		host:      nullify(strings.Join(hostLines, "\n")),
		stumper:   strings.Contains(s, ">Triple Stumper<"),
		responses: []Response{},
	}
	if c := capture(correctRespRE, s); c != nil {
		r.correct = ptr(ctext(c["r"]))
	}
	// The right/wrong rulings are the authoritative response list; a
	// contestant paren line with no ruling is mid-clue banter. When a
	// contestant has several paren lines (DD "make it a true Daily Double"
	// then the answer), the last one is the actual response.
	for _, t := range captures(rulingRE, s) {
		if t["name"] == "Triple Stumper" {
			continue
		}
		resp := Response{Contestant: t["name"], Result: result(t["cls"])}
		for _, p := range cparens {
			if p.speaker == t["name"] {
				resp.GivenResponse = ptr(squeeze(p.said))
			}
		}
		r.responses = append(r.responses, resp)
	}
	return r
}

func result(cls string) string {
	if cls == "right" {
		return "correct"
	}
	return "incorrect"
}

func ptr[T any](v T) *T { return &v }

// --- board clues ------------------------------------------------------------------

var (
	clueIDRE  = re(`clue_id=(?P<n>[0-9]+)`)
	ddWagerRE = re(`DD: \$(?P<n>[0-9,]+)`)
)

func cellRE(format string, id string) *regexp.Regexp {
	return re(fmt.Sprintf(format, regexp.QuoteMeta(id)))
}

// buildClue builds one clue from its board cell; ok is false if the cell is empty.
func buildClue(prefix string, col, row int, rdiv *node, html string, nicknames []string) (Clue, bool) {
	id := fmt.Sprintf("clue_%s_%d_%d", prefix, col, row)
	tm := cellRE(`<td id="%s" class="clue_text">(.*)</td>`, id).FindStringSubmatch(html)
	if tm == nil {
		return Clue{}, false
	}
	r := rcell{responses: []Response{}}
	if rm := cellRE(`<td id="%s_r"[^>]*>(.*)</td>`, id).FindStringSubmatch(html); rm != nil {
		r = parseRCell(rm[1], nicknames)
	}

	hdr := rdiv.first(func(x *node) bool {
		if x.tag() != "tr" {
			return false
		}
		for _, c := range x.children {
			if c.attr("id") == id+"_stuck" {
				return true
			}
		}
		return false
	})
	c := Clue{
		BoardPosition:   Position{col, row},
		Text:            ctext(tm[1]),
		CorrectResponse: r.correct,
		HostComment:     r.host,
		TripleStumper:   r.stumper,
		Responses:       r.responses,
	}
	if v := hdr.first(byClass("clue_value")); v != nil {
		c.Value = intp(money(unescape(v.text)))
	}
	if dd := hdr.first(byClass("clue_value_daily_double")); dd != nil {
		if m := capture(ddWagerRE, unescape(dd.text)); m != nil {
			c.Wager = intp(money(m["n"]))
			c.IsDailyDouble = true
		}
	}
	if a := hdr.first(byTag("a")); a != nil {
		if m := capture(clueIDRE, a.attr("href")); m != nil {
			c.ClueID = intp(atoi(m["n"]))
		}
		if a.hasText {
			if n, err := strconv.Atoi(a.text); err == nil {
				c.PlayOrder = &n
			}
		}
	}
	return c, true
}

// fillDDValues recovers a Daily Double's hidden board value from the row's
// non-DD siblings (era-proof: no hardcoded value ladder).
func fillDDValues(clues []Clue) {
	for i := range clues {
		if !clues[i].IsDailyDouble {
			continue
		}
		clues[i].Value = nil
		for _, o := range clues {
			if o.BoardPosition.Row == clues[i].BoardPosition.Row && !o.IsDailyDouble && o.Value != nil {
				clues[i].Value = o.Value
				break
			}
		}
	}
}

var afterClueRE = re(`after clue (?P<n>[0-9]+)`)

func buildRound(prefix, divID string, content *node, html string, nicknames []string) Round {
	rdiv := content.first(byID(divID))
	r := Round{Categories: []Category{}, Clues: []Clue{}, Scores: Snapshots{}}

	names := rdiv.all(byClass("category_name"))
	comments := rdiv.all(byClass("category_comments"))
	for i, n := range names {
		cat := Category{Column: i + 1, Name: n.rtext()}
		if i < len(comments) {
			cat.Comments = nullify(comments[i].rtext())
		}
		r.Categories = append(r.Categories, cat)
	}

	for row := 1; row <= 5; row++ {
		for col := 1; col <= 6; col++ {
			if c, ok := buildClue(prefix, col, row, rdiv, html, nicknames); ok {
				r.Clues = append(r.Clues, c)
			}
		}
	}
	fillDDValues(r.Clues)

	for _, ht := range h3Tables(rdiv) {
		switch {
		case strings.Contains(ht.heading, "commercial break"):
			if m := capture(afterClueRE, ht.heading); m != nil {
				r.Scores.set("after_clue_"+m["n"], scoreTable(ht.table))
			}
		case strings.Contains(ht.heading, "end of the"):
			r.Scores.set("end_of_round", scoreTable(ht.table))
		}
	}
	return r
}

// assignNicknames pairs contestants with score-table nicknames: first by
// exact first name, then by prefix ("Matt" / "Matthew"), then by position
// among the leftovers — contestants are listed in the reverse of the
// score-table (podium) order.
func assignNicknames(cs []Contestant, nicknames []string) {
	taken := map[string]bool{}
	assign := func(match func(c Contestant, nick string) bool) {
		for i := range cs {
			if cs[i].Nickname != nil {
				continue
			}
			var hits []string
			for _, n := range nicknames {
				if !taken[n] && match(cs[i], n) {
					hits = append(hits, n)
				}
			}
			if len(hits) == 1 {
				cs[i].Nickname = ptr(hits[0])
				taken[hits[0]] = true
			}
		}
	}
	first := func(c Contestant) string {
		if f := strings.Fields(c.Name); len(f) > 0 {
			return f[0]
		}
		return ""
	}
	assign(func(c Contestant, n string) bool { return first(c) == n })
	assign(func(c Contestant, n string) bool {
		f := first(c)
		return f != "" && (strings.HasPrefix(f, n) || strings.HasPrefix(n, f) || strings.HasPrefix(c.Name, n))
	})

	var freeC []int
	for i := range cs {
		if cs[i].Nickname == nil {
			freeC = append(freeC, i)
		}
	}
	var freeN []string
	for _, n := range slices.Backward(nicknames) {
		if !taken[n] {
			freeN = append(freeN, n)
		}
	}
	if len(freeC) == len(freeN) {
		for k, i := range freeC {
			cs[i].Nickname = ptr(freeN[k])
		}
	}
}

// --- assemble ---------------------------------------------------------------------

var (
	showNumRE        = re(`#(?P<n>[0-9]+)`)
	tapeDateRE       = re(`Game tape date: (?P<d>[0-9]{4}-[0-9]{2}-[0-9]{2})`)
	gameCommentsRE   = re(`<div id="game_comments">(?P<c>.*?)</div>`)
	playerIDRE       = re(`player_id=(?P<n>[0-9]+)`)
	leadingCommaRE   = re(`^,[` + ws + `]*`)
	fjTextRE         = re(`<td id="clue_FJ" class="clue_text">(?P<t>.*?)</td>`)
	fjRCellRE        = re(`<td id="clue_FJ_r"[^>]*>(.*)</td>`)
	fjResponseRE     = re(`<td class="(?P<cls>right|wrong)">(?P<name>[^<]*)</td><td rowspan="2"[^>]*>(?P<gr>.*?)</td></tr><tr><td>\$(?P<w>[0-9,]+)</td>`)
	coryatCorrectRE  = re(`(?P<n>[0-9]+) R`)
	coryatWrongRE    = re(`(?P<n>[0-9]+) W`)
	coryatDDRightRE  = re(`R[` + ws + `]*\(including (?P<n>[0-9]+) DD`)
	coryatDDWrongRE  = re(`W[` + ws + `]*\(including (?P<n>[0-9]+) DD`)
	combinedCoryatRE = re(`Combined Coryat</a>: \$(?P<n>[0-9,]+)`)
)

func capInt(r *regexp.Regexp, s string) *int {
	if c := capture(r, s); c != nil {
		return intp(atoi(c["n"]))
	}
	return nil
}

func parseGame(id int, html string, warn io.Writer) (*Game, error) {
	content := parseContent(html)
	if content == nil {
		return nil, fmt.Errorf("game %d: no div#content in page", id)
	}

	var nicknames []string // distinct, in page (podium) order
	for _, n := range content.all(byClass("score_player_nickname")) {
		if nick := unescape(n.text); !slices.Contains(nicknames, nick) {
			nicknames = append(nicknames, nick)
		}
	}

	title := ""
	if h1 := content.first(byID("game_title")).first(byTag("h1")); h1 != nil {
		title = unescape(h1.text)
	}

	g := &Game{GameID: id, AirDate: humanDate(title), Contestants: []Contestant{}}
	if c := capture(showNumRE, title); c != nil {
		g.ShowNumber = atoi(c["n"])
	}
	if c := capture(tapeDateRE, html); c != nil {
		g.TapeDate = ptr(c["d"])
	}
	if c := capture(gameCommentsRE, html); c != nil {
		g.GameComments = nullify(htext(c["c"]))
	}
	_, g.TournamentContext = parseNote(g.GameComments, nil, warn)

	nav := content.first(byID("contestants_table")).all(func(x *node) bool {
		return x.tag() == "a" && strings.Contains(x.attr("href"), "showgame")
	})
	navLink := func(label string) *NavLink {
		for _, a := range nav {
			if strings.Contains(a.rtext(), label) {
				return &NavLink{GameID: atoi(capture(gameIDRE, a.attr("href"))["n"]), Href: a.attr("href")}
			}
		}
		return nil
	}
	g.Navigation = Navigation{PreviousGame: navLink("previous game"), NextGame: navLink("next game")}

	for _, p := range content.all(func(x *node) bool { return x.tag() == "p" && x.attr("class") == "contestants" }) {
		if len(p.children) == 0 {
			continue
		}
		a := p.children[0]
		g.Contestants = append(g.Contestants, Contestant{
			PlayerID:    atoi(capture(playerIDRE, a.attr("href"))["n"]),
			Name:        a.rtext(),
			Description: nullify(trim(leadingCommaRE.ReplaceAllString(unescape(p.text), ""))),
			Href:        a.attr("href"),
		})
	}

	assignNicknames(g.Contestants, nicknames)

	g.Rounds.Jeopardy = buildRound("J", "jeopardy_round", content, html, nicknames)
	g.Rounds.DoubleJeopardy = buildRound("DJ", "double_jeopardy_round", content, html, nicknames)

	fjdiv := content.first(byID("final_jeopardy_round"))
	fjTables := h3Tables(fjdiv)
	tableFor := func(heading string) *node {
		for _, ht := range fjTables {
			if strings.Contains(ht.heading, heading) {
				return ht.table
			}
		}
		return nil
	}
	fj := Final{
		Category:  fjdiv.first(byClass("category_name")).rtext(),
		Responses: []FinalResponse{},
		Scores:    FinalScores{Final: scoreTable(tableFor("Final scores"))},
	}
	if cc := fjdiv.first(byClass("category_comments")); cc != nil {
		fj.CategoryComments = nullify(cc.rtext())
	}
	if c := capture(fjTextRE, html); c != nil {
		fj.Text = ptr(ctext(c["t"]))
	}
	if m := fjRCellRE.FindStringSubmatch(html); m != nil {
		fjr := m[1]
		if c := capture(correctRespRE, fjr); c != nil {
			fj.CorrectResponse = ptr(ctext(c["r"]))
		}
		host := trim(unescape(stripTags(strings.SplitN(fjr, "<table", 2)[0])))
		if c := capture(parenRE, host); c != nil {
			fj.HostComment = ptr(squeeze(c["said"]))
		}
		for _, c := range captures(fjResponseRE, fjr) {
			fj.Responses = append(fj.Responses, FinalResponse{
				Contestant:    unescape(c["name"]),
				Result:        result(c["cls"]),
				GivenResponse: ctext(c["gr"]),
				Wager:         money(c["w"]),
			})
		}
	}
	g.Rounds.FinalJeopardy = fj

	g.CoryatScores = []CoryatScore{}
	ct := tableFor("Coryat scores")
	remarks := ct.all(byClass("score_remarks"))
	for i, s := range scoreTable(ct) {
		rem := ""
		if i < len(remarks) {
			rem = remarks[i].rtext()
		}
		g.CoryatScores = append(g.CoryatScores, CoryatScore{
			Contestant:            s.Contestant,
			Score:                 s.Score,
			Correct:               capInt(coryatCorrectRE, rem),
			Incorrect:             capInt(coryatWrongRE, rem),
			DailyDoublesCorrect:   capInt(coryatDDRightRE, rem),
			DailyDoublesIncorrect: capInt(coryatDDWrongRE, rem),
		})
	}
	if c := capture(combinedCoryatRE, html); c != nil {
		g.CombinedCoryat = intp(money(c["n"]))
	}
	return g, nil
}
