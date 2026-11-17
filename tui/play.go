package tui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"

	"jplay/config"
	"jplay/db"
	"jplay/game"
)

// cluePhase tracks the modal sub-state while a clue (or quit prompt) is open.
type cluePhase int

const (
	phaseNone        cluePhase = iota
	phaseWager                 // Daily Double wager entry
	phaseAnswer                // typing an answer
	phaseFuzzy                 // "close match detected" confirm
	phaseReveal                // correct response + contestant results
	phaseQuitConfirm           // q pressed on the board
	phasePassConfirm           // empty submit / Space: confirm intent to pass
)

// fjPhase tracks Final Jeopardy sub-state.
type fjPhase int

const (
	fjWager fjPhase = iota
	fjAnswer
	fjFuzzy
	fjReveal
)

// playModel holds all state for one play-through of a game.
type playModel struct {
	g   *game.Game
	cfg config.Config

	roundIdx       int
	curRow, curCol int
	score          int
	oppScores      map[string]int // contestant short name -> simulated score
	results        []db.ClueResult
	counts         map[game.ClueState]int

	phase       cluePhase
	active      *game.Clue
	input       textinput.Model
	wagerInput  textinput.Model
	wagerErr    string
	revealState game.ClueState
	revealDelta int
	revealNote  string // extra reveal banner ("You ran the category!")

	fj         fjPhase
	fjWagerN   int
	saved      bool  // game written to DB (end screen shown)
	savedRowID int64 // games row id from SaveGame, for the stats drill-down
}

func newPlayModel(g *game.Game, cfg config.Config) *playModel {
	in := textinput.New()
	in.Prompt = "> "
	in.CharLimit = 200
	wi := textinput.New()
	wi.Prompt = "Wager $ "
	wi.CharLimit = 7

	p := &playModel{
		g:          g,
		cfg:        cfg,
		oppScores:  map[string]int{},
		counts:     map[game.ClueState]int{},
		input:      in,
		wagerInput: wi,
	}
	for _, c := range g.Contestants {
		p.oppScores[c.Short] = 0
	}
	p.ensureCursorSelectable()
	return p
}

func (p *playModel) round() *game.Round { return &p.g.Rounds[p.roundIdx] }

func (p *playModel) judgeOpts() game.JudgeOptions {
	return game.JudgeOptions{
		Threshold:            p.cfg.FuzzyThreshold,
		StripArticles:        p.cfg.StripArticles,
		StripQuestionFraming: p.cfg.StripQuestionFraming,
	}
}

func (p *playModel) clueAt(row, col int) *game.Clue {
	cats := p.round().Categories
	if col < 0 || col >= len(cats) || row < 0 || row >= 5 {
		return nil
	}
	return cats[col].Clues[row]
}

func (p *playModel) selectable(row, col int) bool {
	c := p.clueAt(row, col)
	return c != nil && !c.Played
}

// move steps the cursor in (dr,dc), wrapping at edges and skipping used and
// unrevealed cells. If the scanned line has no selectable cell, stay put.
func (p *playModel) move(dr, dc int) {
	rows, cols := 5, len(p.round().Categories)
	if cols == 0 {
		return
	}
	r, c := p.curRow, p.curCol
	for i := 0; i < rows*cols; i++ {
		r = (r + dr + rows) % rows
		c = (c + dc + cols) % cols
		if p.selectable(r, c) {
			p.curRow, p.curCol = r, c
			return
		}
	}
}

// ensureCursorSelectable relocates the cursor to some selectable cell after
// a clue is consumed or a round starts. Preference order: next tile down in
// the same category, then upward in the same category, then anywhere.
func (p *playModel) ensureCursorSelectable() {
	if p.selectable(p.curRow, p.curCol) {
		return
	}
	for r := p.curRow + 1; r < 5; r++ {
		if p.selectable(r, p.curCol) {
			p.curRow = r
			return
		}
	}
	for r := p.curRow - 1; r >= 0; r-- {
		if p.selectable(r, p.curCol) {
			p.curRow = r
			return
		}
	}
	cols := len(p.round().Categories)
	for r := 0; r < 5; r++ {
		for c := 0; c < cols; c++ {
			if p.selectable(r, c) {
				p.curRow, p.curCol = r, c
				return
			}
		}
	}
}

// openClue enters the modal flow for the cell under the cursor.
func (p *playModel) openClue() {
	c := p.clueAt(p.curRow, p.curCol)
	if c == nil || c.Played {
		return
	}
	p.active = c
	p.input.SetValue("")
	p.input.Focus()
	if c.DailyDouble {
		p.wagerInput.SetValue("")
		p.wagerErr = ""
		p.wagerInput.Focus()
		p.phase = phaseWager
	} else {
		p.phase = phaseAnswer
	}
}

// maxWager is the Daily Double ceiling: max(current score, clue value).
func (p *playModel) maxWager() int {
	if p.score > p.active.Value {
		return p.score
	}
	return p.active.Value
}

// confirmWager validates the DD wager input; on success moves to answering.
func (p *playModel) confirmWager() {
	n, err := strconv.Atoi(strings.TrimSpace(p.wagerInput.Value()))
	if err != nil {
		p.wagerErr = "enter a number"
		return
	}
	if n < 5 || n > p.maxWager() {
		p.wagerErr = "wager between $5 and " + money(p.maxWager())
		return
	}
	p.active.PlayerWager = n
	p.wagerErr = ""
	p.phase = phaseAnswer
}

// submitAnswer judges the typed answer.
func (p *playModel) submitAnswer() {
	switch game.Judge(p.input.Value(), p.active.CorrectResponse, p.judgeOpts()) {
	case game.VerdictCorrect:
		p.resolve(game.StateCorrect)
	case game.VerdictFuzzy:
		p.phase = phaseFuzzy
	default:
		p.resolve(game.StateIncorrect)
	}
}

// resolve settles the active clue: scores the player, replays the historical
// contestants, records the result, and enters the reveal state.
func (p *playModel) resolve(state game.ClueState) {
	c := p.active
	amount := c.Value
	if c.DailyDouble && c.PlayerWager > 0 {
		amount = c.PlayerWager
	}
	delta := 0
	switch state {
	case game.StateCorrect:
		delta = amount
	case game.StateIncorrect:
		delta = -amount
	case game.StatePassed:
		// A Daily Double wager is committed once entered: passing after
		// seeing the clue forfeits it, regardless of the pass_penalty knob.
		if p.cfg.PassPenalty || (c.DailyDouble && c.PlayerWager > 0) {
			delta = -amount
		}
	}
	p.score += delta
	p.revealDelta = delta
	p.revealState = state
	for short := range p.oppScores {
		p.oppScores[short] += game.ContestantDelta(c, short)
	}

	c.Played = true
	c.PlayerState = state
	c.PlayerAnswer = p.input.Value()
	p.counts[state]++
	p.revealNote = ""
	if p.categoryRun(c.Col) {
		p.revealNote = "You ran the category!"
	}

	res := db.ClueResult{
		ClueID:          c.ID,
		Category:        c.Category,
		Value:           c.Value,
		ClueText:        c.Text,
		CorrectResponse: c.CorrectResponse,
		State:           string(state),
		PlayerAnswer:    c.PlayerAnswer,
		IsDailyDouble:   c.DailyDouble,
	}
	if c.DailyDouble && c.PlayerWager > 0 {
		w := c.PlayerWager
		res.PlayerWager = &w
	}
	p.results = append(p.results, res)
	p.phase = phaseReveal
}

// overrideCorrect flips a just-judged incorrect answer to correct (manual
// override at the reveal screen for false negatives from the judge).
func (p *playModel) overrideCorrect() {
	if p.revealState != game.StateIncorrect || len(p.results) == 0 {
		return
	}
	amount := -p.revealDelta // what was deducted
	p.score += 2 * amount
	p.revealDelta = amount
	p.revealState = game.StateCorrect
	p.counts[game.StateIncorrect]--
	p.counts[game.StateCorrect]++
	p.results[len(p.results)-1].State = string(game.StateCorrect)
	if p.active != nil {
		p.active.PlayerState = game.StateCorrect
		if p.categoryRun(p.active.Col) {
			p.revealNote = "You ran the category!"
		}
	}
}

// categoryRun reports whether the player has answered every clue of a full
// board column correctly — a "run" of the category.
func (p *playModel) categoryRun(col int) bool {
	for _, c := range p.round().Categories[col].Clues {
		if c == nil || !c.Played || c.PlayerState != game.StateCorrect {
			return false
		}
	}
	return true
}

// contestantCounts tallies each historical contestant's correct/incorrect
// responses across the whole game, Final Jeopardy included.
func (p *playModel) contestantCounts() map[string][2]int {
	counts := map[string][2]int{}
	tally := func(rs []game.Response) {
		for _, r := range rs {
			n := counts[r.Contestant]
			switch r.Result {
			case "correct":
				n[0]++
			case "incorrect":
				n[1]++
			}
			counts[r.Contestant] = n
		}
	}
	for _, rd := range p.g.Rounds {
		for _, cat := range rd.Categories {
			for _, c := range cat.Clues {
				if c != nil {
					tally(c.Responses)
				}
			}
		}
	}
	if p.g.Final != nil {
		tally(p.g.Final.Responses)
	}
	return counts
}

// dismissReveal returns to the board (or advances the round / final).
// Reports whether the board round is exhausted.
func (p *playModel) dismissReveal() (roundDone bool) {
	p.phase = phaseNone
	p.active = nil
	if p.round().RemainingClues() > 0 {
		p.ensureCursorSelectable()
		return false
	}
	if p.roundIdx+1 < len(p.g.Rounds) {
		p.roundIdx++
		p.curRow, p.curCol = 0, 0
		p.ensureCursorSelectable()
		return false
	}
	return true
}

// --- Final Jeopardy -----------------------------------------------------

// fjMaxWager: everything you have (never negative).
func (p *playModel) fjMaxWager() int {
	if p.score > 0 {
		return p.score
	}
	return 0
}

func (p *playModel) startFinal() {
	p.fj = fjWager
	p.wagerInput.SetValue("")
	p.wagerErr = ""
	p.wagerInput.Focus()
	p.input.SetValue("")
}

func (p *playModel) confirmFJWager() bool {
	n, err := strconv.Atoi(strings.TrimSpace(p.wagerInput.Value()))
	if err != nil {
		p.wagerErr = "enter a number"
		return false
	}
	if n < 0 || n > p.fjMaxWager() {
		p.wagerErr = "wager between $0 and " + money(p.fjMaxWager())
		return false
	}
	p.fjWagerN = n
	p.wagerErr = ""
	p.fj = fjAnswer
	p.input.Focus()
	return true
}

// resolveFinal settles Final Jeopardy and appends its record.
func (p *playModel) resolveFinal(state game.ClueState) {
	fj := p.g.Final
	delta := p.fjWagerN
	if state != game.StateCorrect {
		delta = -delta
	}
	p.score += delta
	p.revealDelta = delta
	p.revealState = state
	p.counts[state]++

	// Contestants: apply their historical FJ outcomes to the simulation.
	for _, r := range fj.Responses {
		if _, ok := p.oppScores[r.Contestant]; ok {
			if r.Result == "correct" {
				p.oppScores[r.Contestant] += r.Wager
			} else {
				p.oppScores[r.Contestant] -= r.Wager
			}
		}
	}

	w := p.fjWagerN
	p.results = append(p.results, db.ClueResult{
		ClueID:          "final",
		Category:        fj.Category,
		Value:           0,
		ClueText:        fj.Text,
		CorrectResponse: fj.CorrectResponse,
		State:           string(state),
		PlayerAnswer:    p.input.Value(),
		PlayerWager:     &w,
		IsFinal:         true,
	})
	p.fj = fjReveal
}

// finalScores returns contestant end-of-game scores for the rank table,
// preferring the archive's recorded final scores over the simulation.
func (p *playModel) finalScores() map[string]int {
	if p.g.Final != nil && len(p.g.Final.FinalScores) > 0 {
		return p.g.Final.FinalScores
	}
	return p.oppScores
}
