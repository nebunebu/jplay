// Package game defines jplay's internal game model and the loader that
// builds it from jarch JSON output.
package game

import "fmt"

// ClueState is the player's outcome on a clue.
type ClueState string

const (
	StateCorrect   ClueState = "correct"
	StateIncorrect ClueState = "incorrect"
	StatePassed    ClueState = "passed"
)

// Response is one contestant's recorded response to a clue.
type Response struct {
	Contestant    string // first-name key as used by jarch
	Result        string // "correct" or "incorrect"
	GivenResponse string // verbatim response when jarch captured one
	Wager         int    // Final Jeopardy wager (0 elsewhere)
}

// Clue is a single board clue.
type Clue struct {
	ID              string
	Value           int
	Text            string
	CorrectResponse string
	DailyDouble     bool
	DDWager         int // contestant's actual DD wager (0 if none)
	Responses       []Response
	TripleStumper   bool

	Category string
	Row, Col int // 0-based board coordinates

	// Play state (owned by the TUI, persisted at game end).
	Played       bool
	PlayerState  ClueState
	PlayerAnswer string
	PlayerWager  int // player's DD wager
}

// Category is one board column.
type Category struct {
	Name  string
	Clues [5]*Clue // nil = unrevealed / never aired
}

// Round is one of the two board rounds.
type Round struct {
	Name       string // "Jeopardy!" or "Double Jeopardy!"
	Categories []Category
}

// RemainingClues counts playable, not-yet-played clues.
func (r *Round) RemainingClues() int {
	n := 0
	for _, cat := range r.Categories {
		for _, c := range cat.Clues {
			if c != nil && !c.Played {
				n++
			}
		}
	}
	return n
}

// FinalJeopardy is the final clue with contestant wagers and final scores.
type FinalJeopardy struct {
	Category        string
	Text            string
	CorrectResponse string
	Responses       []Response
	FinalScores     map[string]int // contestant short name -> final score
}

// Contestant is a historical player of the game.
type Contestant struct {
	Name        string // full name
	Short       string // first-name key used in clue responses
	Description string
}

// Game is a fully loaded jarch game.
type Game struct {
	GameID      int // jarch/J!Archive numeric game id
	ShowNumber  int // aired episode number ("J#8478")
	AirDate     string
	Contestants []Contestant
	Rounds      []Round
	Final       *FinalJeopardy
}

// Label is the display id, preferring the aired show number.
func (g *Game) Label() string {
	if g.ShowNumber > 0 {
		return fmt.Sprintf("J#%d", g.ShowNumber)
	}
	return fmt.Sprintf("game %d", g.GameID)
}

// ContestantDelta returns the score change a historical contestant earned on
// a clue (DD wagers included; Final Jeopardy handled separately).
func ContestantDelta(c *Clue, short string) int {
	for _, r := range c.Responses {
		if r.Contestant != short {
			continue
		}
		amount := c.Value
		if c.DailyDouble && c.DDWager > 0 {
			amount = c.DDWager
		}
		switch r.Result {
		case "correct":
			return amount
		case "incorrect":
			return -amount
		}
	}
	return 0
}
