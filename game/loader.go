package game

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// jarch JSON schema (as actually emitted by jarch, which differs from the
// early sketch in SPEC.md: rounds is an object keyed by round name, clues are
// a flat list with board positions, and unrevealed clues are simply absent).

type jarchGame struct {
	GameID      int               `json:"game_id"`
	ShowNumber  int               `json:"show_number"`
	AirDate     string            `json:"air_date"`
	Contestants []jarchContestant `json:"contestants"`
	Rounds      jarchRounds       `json:"rounds"`
}

type jarchContestant struct {
	Name        string `json:"name"`
	Nickname    string `json:"nickname"` // absent in JSON from older jarch versions
	Description string `json:"description"`
}

type jarchRounds struct {
	Jeopardy       *jarchRound `json:"jeopardy"`
	DoubleJeopardy *jarchRound `json:"double_jeopardy"`
	FinalJeopardy  *jarchFinal `json:"final_jeopardy"`
}

type jarchRound struct {
	Categories []jarchCategory `json:"categories"`
	Clues      []jarchClue     `json:"clues"`
}

type jarchCategory struct {
	Column int    `json:"column"`
	Name   string `json:"name"`
}

type jarchClue struct {
	ClueID        int `json:"clue_id"`
	BoardPosition struct {
		Column int `json:"column"`
		Row    int `json:"row"`
	} `json:"board_position"`
	Value           int             `json:"value"`
	IsDailyDouble   bool            `json:"is_daily_double"`
	Wager           *int            `json:"wager"`
	Text            string          `json:"text"`
	CorrectResponse string          `json:"correct_response"`
	TripleStumper   bool            `json:"triple_stumper"`
	Responses       []jarchResponse `json:"responses"`
}

type jarchResponse struct {
	Contestant    string `json:"contestant"`
	Result        string `json:"result"`
	GivenResponse string `json:"given_response"`
	Wager         *int   `json:"wager"`
}

type jarchFinal struct {
	Category        string          `json:"category"`
	Text            string          `json:"text"`
	CorrectResponse string          `json:"correct_response"`
	Responses       []jarchResponse `json:"responses"`
	Scores          struct {
		Final []struct {
			Contestant string `json:"contestant"`
			Score      int    `json:"score"`
		} `json:"final"`
	} `json:"scores"`
}

// LoadFile parses a jarch game JSON file into a Game.
func LoadFile(path string) (*Game, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Load(data)
}

// Load parses jarch game JSON bytes into a Game.
func Load(data []byte) (*Game, error) {
	var jg jarchGame
	if err := json.Unmarshal(data, &jg); err != nil {
		return nil, fmt.Errorf("parsing jarch JSON: %w", err)
	}
	if jg.Rounds.Jeopardy == nil && jg.Rounds.DoubleJeopardy == nil {
		return nil, fmt.Errorf("jarch JSON has no board rounds (not a game file?)")
	}

	g := &Game{
		GameID:     jg.GameID,
		ShowNumber: jg.ShowNumber,
		AirDate:    jg.AirDate,
	}
	for _, c := range jg.Contestants {
		// The nickname keys clue responses and score tables. Older jarch
		// output lacks it; fall back to the first name.
		short := c.Nickname
		if short == "" {
			short = c.Name
			if i := strings.IndexByte(c.Name, ' '); i > 0 {
				short = c.Name[:i]
			}
		}
		g.Contestants = append(g.Contestants, Contestant{
			Name:        c.Name,
			Short:       short,
			Description: c.Description,
		})
	}

	if jg.Rounds.Jeopardy != nil {
		g.Rounds = append(g.Rounds, buildRound("Jeopardy!", jg.Rounds.Jeopardy))
	}
	if jg.Rounds.DoubleJeopardy != nil {
		g.Rounds = append(g.Rounds, buildRound("Double Jeopardy!", jg.Rounds.DoubleJeopardy))
	}
	if fj := jg.Rounds.FinalJeopardy; fj != nil && fj.Text != "" {
		final := &FinalJeopardy{
			Category:        fj.Category,
			Text:            fj.Text,
			CorrectResponse: fj.CorrectResponse,
			FinalScores:     map[string]int{},
		}
		for _, r := range fj.Responses {
			final.Responses = append(final.Responses, toResponse(r))
		}
		for _, s := range fj.Scores.Final {
			final.FinalScores[s.Contestant] = s.Score
		}
		g.Final = final
	}
	return g, nil
}

func toResponse(r jarchResponse) Response {
	resp := Response{
		Contestant:    r.Contestant,
		Result:        r.Result,
		GivenResponse: r.GivenResponse,
	}
	if r.Wager != nil {
		resp.Wager = *r.Wager
	}
	return resp
}

func buildRound(name string, jr *jarchRound) Round {
	round := Round{Name: name}

	// Categories, ordered by column. jarch columns are 1-based.
	cats := append([]jarchCategory(nil), jr.Categories...)
	sort.Slice(cats, func(i, j int) bool { return cats[i].Column < cats[j].Column })
	byCol := map[int]int{} // jarch column -> index in round.Categories
	for _, c := range cats {
		byCol[c.Column] = len(round.Categories)
		round.Categories = append(round.Categories, Category{Name: c.Name})
	}

	for i := range jr.Clues {
		jc := jr.Clues[i]
		col, ok := byCol[jc.BoardPosition.Column]
		row := jc.BoardPosition.Row - 1
		if !ok || row < 0 || row > 4 {
			continue // malformed position; skip rather than fail the load
		}
		clue := &Clue{
			ID:              fmt.Sprintf("%d", jc.ClueID),
			Value:           jc.Value,
			Text:            jc.Text,
			CorrectResponse: jc.CorrectResponse,
			DailyDouble:     jc.IsDailyDouble,
			TripleStumper:   jc.TripleStumper,
			Category:        round.Categories[col].Name,
			Row:             row,
			Col:             col,
		}
		if jc.Wager != nil {
			clue.DDWager = *jc.Wager
		}
		for _, r := range jc.Responses {
			clue.Responses = append(clue.Responses, toResponse(r))
		}
		round.Categories[col].Clues[row] = clue
	}
	return round
}
