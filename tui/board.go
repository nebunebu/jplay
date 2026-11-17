package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"jplay/db"
	"jplay/game"
)

const scorePanelMinWidth = 18
const minColWidth = 8

// updateBoard handles the board screen including its modal phases.
func (m Model) updateBoard(msg tea.Msg) (tea.Model, tea.Cmd) {
	p := m.play
	key, isKey := msg.(tea.KeyMsg)

	switch p.phase {
	case phaseNone:
		if !isKey {
			return m, nil
		}
		switch {
		case keyUp(key):
			p.move(-1, 0)
		case keyDown(key):
			p.move(1, 0)
		case keyLeft(key):
			p.move(0, -1)
		case keyRight(key):
			p.move(0, 1)
		case key.String() == "enter":
			p.openClue()
		case key.String() == "q":
			p.phase = phaseQuitConfirm
		}
		return m, nil

	case phaseQuitConfirm:
		if !isKey {
			return m, nil
		}
		switch key.String() {
		case "y":
			// Abandoned mid-game: nothing is written to the DB.
			m.play = nil
			m.screen = ScreenLanding
			m.landing = newLandingModel(&m)
		case "n", "q", "esc":
			p.phase = phaseNone
		}
		return m, nil

	case phaseWager:
		if isKey && key.String() == "enter" {
			p.confirmWager()
			return m, nil
		}
		var cmd tea.Cmd
		p.wagerInput, cmd = p.wagerInput.Update(msg)
		return m, cmd

	case phaseAnswer:
		if isKey {
			switch key.String() {
			case "enter":
				if strings.TrimSpace(p.input.Value()) == "" {
					p.phase = phasePassConfirm // empty submit: confirm pass
					return m, nil
				}
				p.submitAnswer()
				return m, nil
			case "esc":
				p.resolve(game.StatePassed)
				return m, nil
			case " ":
				// Space in an empty input means pass, not a leading space.
				if p.input.Value() == "" {
					p.phase = phasePassConfirm
					return m, nil
				}
			}
		}
		var cmd tea.Cmd
		p.input, cmd = p.input.Update(msg)
		return m, cmd

	case phasePassConfirm:
		if !isKey {
			return m, nil
		}
		switch key.String() {
		case "y", "enter":
			p.resolve(game.StatePassed)
		case "n", "esc":
			p.phase = phaseAnswer
		}
		return m, nil

	case phaseFuzzy:
		if !isKey {
			return m, nil
		}
		switch key.String() {
		case "y":
			p.resolve(game.StateCorrect)
		case "n":
			p.input.SetValue("")
			p.phase = phaseAnswer
		}
		return m, nil

	case phaseReveal:
		if !isKey {
			return m, nil
		}
		switch key.String() {
		case " ", "enter", "esc":
			if p.dismissReveal() {
				return m.leaveBoard()
			}
		case "o":
			p.overrideCorrect()
		}
		return m, nil
	}
	return m, nil
}

// leaveBoard advances past the last board clue: Final Jeopardy if the game
// has one, otherwise straight to the end screen.
func (m Model) leaveBoard() (tea.Model, tea.Cmd) {
	if m.play.g.Final != nil {
		m.play.startFinal()
		m.screen = ScreenFinal
		return m, nil
	}
	return m.finishGame()
}

// finishGame shows the end screen and persists the play-through.
func (m Model) finishGame() (tea.Model, tea.Cmd) {
	m.screen = ScreenEnd
	p := m.play
	if p.saved || m.db == nil {
		return m, nil
	}
	p.saved = true
	rec := db.GameRecord{
		GameID:      p.g.Label(),
		ArchiveID:   p.g.GameID,
		PlayedAt:    time.Now(),
		PlayerScore: p.score,
		Clues:       p.results,
	}
	finals := p.finalScores()
	for _, c := range p.g.Contestants {
		rec.Contestants = append(rec.Contestants, db.ContestantScore{Name: c.Name, Score: finals[c.Short]})
	}
	rowID, err := m.db.SaveGame(rec)
	if err != nil {
		m.flash = fmt.Sprintf("save failed: %v", err)
	} else {
		p.savedRowID = rowID
	}
	return m, nil
}

// --- rendering ------------------------------------------------------------

func (m Model) viewBoard() string {
	p := m.play
	base := m.renderBoardBase()

	var modal string
	switch p.phase {
	case phaseWager:
		modal = m.renderDDModal()
	case phaseAnswer, phaseFuzzy, phasePassConfirm:
		modal = m.renderClueModal()
	case phaseReveal:
		modal = m.renderRevealModal()
	case phaseQuitConfirm:
		modal = m.styles.Modal.Render(
			m.styles.Text.Render("Quit this game? Progress will not be saved.") + "\n\n" +
				m.statusBar("y", "quit", "n", "keep playing"))
	default:
		return base
	}
	return m.overlayCenter(base, modal)
}

// renderBoardBase draws the full board screen (title, grid, scores, status).
func (m Model) renderBoardBase() string {
	p := m.play
	round := p.round()

	showPanel := m.cfg.ShowContestantScores &&
		m.width-scorePanelMinWidth-2 >= minColWidth*len(round.Categories)
	panelW := 0
	if showPanel {
		panelW = scorePanelMinWidth + 2
	}
	cols := len(round.Categories)
	colW := (m.width - panelW - 1) / cols
	if colW < minColWidth {
		colW = minColWidth
	}
	gridW := colW * cols

	title := m.styles.Title.Render("JEOPARDY!") +
		m.styles.Hint.Render(fmt.Sprintf("  · %s · Round %d of %d · %d clues remaining",
			p.g.Label(), p.roundIdx+1, len(p.g.Rounds), round.RemainingClues()))
	title = truncate(title, m.width)

	// Vertical budget: title(1) + gap(1) + header + 5 rows + status(1).
	headH := 2
	cellH := (m.height - 4 - headH) / 5
	if cellH < 1 {
		cellH = 1
	}
	if cellH > 3 {
		cellH = 3
	}

	// Category header row.
	var heads []string
	for _, cat := range round.Categories {
		st := m.styles.CatCell.Width(colW - 1).Height(headH).AlignVertical(lipgloss.Center)
		heads = append(heads, st.Render(wrapOrTruncate(cat.Name, colW-1, headH)))
	}
	header := lipgloss.JoinHorizontal(lipgloss.Top, interleaveGap(heads)...)

	// Clue rows.
	var rows []string
	for r := 0; r < 5; r++ {
		var cells []string
		for c := 0; c < cols; c++ {
			clue := round.Categories[c].Clues[r]
			var st lipgloss.Style
			var label string
			switch {
			case clue == nil:
				st, label = m.styles.CellNA, "N/A"
			case clue.Played:
				st, label = m.styles.CellUsed, "·"
			case r == p.curRow && c == p.curCol:
				st, label = m.styles.CellCursor, money(clue.Value)
			default:
				st, label = m.styles.CellAvail, money(clue.Value)
			}
			cells = append(cells, st.Width(colW-1).Height(cellH).AlignVertical(lipgloss.Center).Render(label))
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, interleaveGap(cells)...))
	}
	grid := lipgloss.JoinVertical(lipgloss.Left, append([]string{header}, rows...)...)
	grid = lipgloss.NewStyle().Width(gridW).Render(grid)

	main := grid
	if showPanel {
		main = lipgloss.JoinHorizontal(lipgloss.Top, grid, " ", m.renderScorePanel())
	}

	status := m.statusBar("↑↓←→/hjkl", "move", "Enter", "select", "q", "quit")
	if !showPanel {
		status = m.statusBar("↑↓←→", "move", "Enter", "select", "q", "quit",
			"YOU", money(m.play.score))
	}
	return m.page(title+"\n\n"+main, status)
}

// renderScorePanel shows YOU + contestants, leader marked ①.
func (m Model) renderScorePanel() string {
	p := m.play
	type row struct {
		name  string
		score int
		you   bool
	}
	rows := []row{{"YOU", p.score, true}}
	for _, c := range p.g.Contestants {
		rows = append(rows, row{strings.ToUpper(c.Short), p.oppScores[c.Short], false})
	}
	leader := 0
	for i, r := range rows {
		if r.score > rows[leader].score {
			leader = i
		}
	}
	var b strings.Builder
	b.WriteString(m.styles.Label.Render("Scores") + "\n")
	for i, r := range rows {
		mark := "  "
		if i == leader {
			mark = "① "
		}
		name := truncate(r.name, scorePanelMinWidth-10)
		nameSt := m.styles.Label
		if r.you {
			nameSt = m.styles.You
		}
		pad := scorePanelMinWidth - 2 - lipgloss.Width(name) - lipgloss.Width(money(r.score))
		if pad < 1 {
			pad = 1
		}
		b.WriteString(mark + nameSt.Render(name) + strings.Repeat(" ", pad) +
			m.styles.Gold.Render(money(r.score)) + "\n")
	}
	return m.styles.Panel.Render(strings.TrimRight(b.String(), "\n"))
}

// modalWidth per spec: min(60, width-4).
func (m Model) modalWidth() int {
	w := 60
	if m.width-4 < w {
		w = m.width - 4
	}
	if w < 20 {
		w = 20
	}
	return w
}

// renderClueModal shows the clue with the answer input (4a) or the fuzzy
// confirm panel (4b).
func (m Model) renderClueModal() string {
	p := m.play
	w := m.modalWidth()
	inner := w - 6

	head := m.styles.CategoryHd.Render(p.active.Category) + "  " +
		m.styles.Gold.Render(money(p.active.Value))
	if p.active.DailyDouble {
		head += m.styles.Wrong.Render("  DAILY DOUBLE") +
			m.styles.Hint.Render("  wager "+money(p.active.PlayerWager))
	}
	clue := m.styles.ClueText.Width(inner).Render(p.active.Text)

	var tail string
	switch p.phase {
	case phaseFuzzy:
		tail = m.styles.Gold.Render("Close match detected") + "\n" +
			m.styles.Text.Width(inner).Render(fmt.Sprintf("%q", canonicalQuestion(p.active.CorrectResponse))) + "\n\n" +
			m.statusBar("y", "accept", "n", "reject & re-enter")
	case phasePassConfirm:
		prompt := "Pass this clue?"
		if p.active.DailyDouble {
			prompt = "Pass this clue? Your wager is forfeited."
		}
		tail = m.styles.Gold.Render(prompt) + "\n\n" +
			m.statusBar("y", "pass", "n", "keep answering")
	default:
		passHint := "pass"
		if p.active.DailyDouble {
			passHint = "pass (forfeit wager)"
		}
		p.input.Width = inner - 3
		tail = p.input.View() + "\n\n" + m.statusBar("Enter", "lock in", "Esc", passHint)
	}
	return m.styles.Modal.Width(w).Render(head + "\n\n" + clue + "\n\n" + tail)
}

// canonicalQuestion frames a raw response as a Jeopardy question.
func canonicalQuestion(response string) string {
	low := strings.ToLower(response)
	for _, p := range []string{"what", "who", "where"} {
		if strings.HasPrefix(low, p+" ") {
			return response
		}
	}
	return "What is " + response + "?"
}

// renderRevealModal is state 4c: correct response, everyone's outcomes.
func (m Model) renderRevealModal() string {
	p := m.play
	w := m.modalWidth()
	inner := w - 6

	var b strings.Builder
	b.WriteString(m.styles.CategoryHd.Render(p.active.Category) + "  " +
		m.styles.Gold.Render(money(p.active.Value)) + "\n\n")
	b.WriteString(m.styles.Hint.Width(inner).Render(p.active.Text) + "\n\n")

	switch p.revealState {
	case game.StateCorrect:
		b.WriteString(m.styles.Correct.Render("✓ correct "+plusMoney(p.revealDelta)) + "\n")
	case game.StateIncorrect:
		b.WriteString(m.styles.Wrong.Render("✗ incorrect "+plusMoney(p.revealDelta)) + "\n")
	default:
		if p.revealDelta != 0 {
			b.WriteString(m.styles.Hint.Render("— passed "+plusMoney(p.revealDelta)) + "\n")
		} else {
			b.WriteString(m.styles.Hint.Render("— passed") + "\n")
		}
	}
	if p.active.TripleStumper {
		badge := m.styles.Wrong.Render("TRIPLE STUMPER")
		if p.revealState == game.StateCorrect {
			badge += m.styles.Gold.Render("  — you got what no contestant did!")
		} else {
			badge += m.styles.Hint.Render("  — no contestant got this one either")
		}
		b.WriteString(badge + "\n")
	}
	if p.revealNote != "" {
		b.WriteString(m.styles.Gold.Render("★ "+p.revealNote) + "\n")
	}
	b.WriteString("\n" + m.styles.Label.Render("Correct response") + "\n" +
		m.styles.Text.Width(inner).Bold(true).Render(canonicalQuestion(p.active.CorrectResponse)) + "\n\n")

	// Player line, then each contestant's historical outcome.
	b.WriteString(m.renderResponseLine("YOU", p.input.Value(), string(p.revealState), p.revealDelta, inner, true))
	for _, c := range p.g.Contestants {
		resp, delta := contestantOutcome(p.active, c.Short)
		given, result := "", "passed"
		if resp != nil {
			given, result = resp.GivenResponse, resp.Result
		}
		b.WriteString(m.renderResponseLine(strings.ToUpper(c.Short), given, result, delta, inner, false))
	}
	b.WriteString("\n" + m.styles.Gold.Render(plusMoney(p.revealDelta)+" → "+money(p.score)) + "\n\n")
	if p.revealState == game.StateIncorrect {
		b.WriteString(m.statusBar("Space", "back to board", "o", "mark correct"))
	} else {
		b.WriteString(m.statusBar("Space", "back to board"))
	}
	return m.styles.Modal.Width(w).Render(strings.TrimRight(b.String(), "\n"))
}

// renderResponseLine formats one row of the reveal table.
func (m Model) renderResponseLine(name, given, result string, delta, width int, you bool) string {
	nameSt := m.styles.Label
	if you {
		nameSt = m.styles.You
	}
	var badge string
	switch result {
	case "correct":
		badge = m.styles.Correct.Render("✓ " + plusMoney(delta))
	case "incorrect":
		badge = m.styles.Wrong.Render("✗ " + plusMoney(delta))
	default:
		badge = m.styles.Hint.Render("— passed")
	}
	quoted := ""
	if given != "" {
		quoted = truncate(fmt.Sprintf("%q", given), width-lipgloss.Width(name)-14)
	}
	return nameSt.Render(name) + " " + m.styles.Text.Render(quoted) + " " + badge + "\n"
}

// contestantOutcome finds a contestant's response and score delta on a clue.
func contestantOutcome(c *game.Clue, short string) (*game.Response, int) {
	for i := range c.Responses {
		if c.Responses[i].Contestant == short {
			return &c.Responses[i], game.ContestantDelta(c, short)
		}
	}
	return nil, 0
}

// renderDDModal is the Daily Double splash (5a).
func (m Model) renderDDModal() string {
	p := m.play
	w := m.modalWidth()
	p.wagerInput.Width = 12

	var b strings.Builder
	b.WriteString(m.styles.Hint.Render("You found a") + "\n" +
		m.styles.Gold.Render("DAILY DOUBLE!") + "\n\n" +
		m.styles.CategoryHd.Render(p.active.Category) + "  " +
		m.styles.Gold.Render(money(p.active.Value)) + "\n\n" +
		m.styles.Label.Render("Current score  ") + m.styles.Gold.Render(money(p.score)) + "\n" +
		m.styles.Label.Render("Max wager      ") + m.styles.Gold.Render(money(p.maxWager())) + "\n\n" +
		p.wagerInput.View() + "\n")
	if p.wagerErr != "" {
		b.WriteString(m.styles.Wrong.Render(p.wagerErr) + "\n")
	}
	b.WriteString("\n" + m.statusBar("Enter", "confirm & see clue") + "\n" +
		m.styles.Hint.Render(fmt.Sprintf("Wager between $5 and %s", money(p.maxWager()))))
	return m.styles.Modal.Width(w).Render(b.String())
}

// --- small helpers --------------------------------------------------------

func plusMoney(n int) string {
	if n >= 0 {
		return "+" + money(n)
	}
	return money(n)
}

// interleaveGap inserts a 1-char gap between cells.
func interleaveGap(cells []string) []string {
	var out []string
	for i, c := range cells {
		if i > 0 {
			out = append(out, " ")
		}
		out = append(out, c)
	}
	return out
}

// wrapOrTruncate fits a category name into width×lines.
func wrapOrTruncate(s string, width, lines int) string {
	wrapped := lipgloss.NewStyle().Width(width).Render(s)
	parts := strings.Split(wrapped, "\n")
	if len(parts) > lines {
		parts = parts[:lines]
		parts[lines-1] = truncate(parts[lines-1], width)
	}
	return strings.Join(parts, "\n")
}
