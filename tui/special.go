package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"jplay/game"
)

// updateFinal drives Final Jeopardy: wager → answer → (fuzzy) → reveal.
func (m Model) updateFinal(msg tea.Msg) (tea.Model, tea.Cmd) {
	p := m.play
	key, isKey := msg.(tea.KeyMsg)

	switch p.fj {
	case fjWager:
		if isKey && key.String() == "enter" {
			p.confirmFJWager()
			return m, nil
		}
		var cmd tea.Cmd
		p.wagerInput, cmd = p.wagerInput.Update(msg)
		return m, cmd

	case fjAnswer:
		// No pass option in Final Jeopardy: Esc is ignored.
		if isKey && key.String() == "enter" {
			switch game.Judge(p.input.Value(), p.g.Final.CorrectResponse, p.judgeOpts()) {
			case game.VerdictCorrect:
				p.resolveFinal(game.StateCorrect)
			case game.VerdictFuzzy:
				p.fj = fjFuzzy
			default:
				p.resolveFinal(game.StateIncorrect)
			}
			return m, nil
		}
		var cmd tea.Cmd
		p.input, cmd = p.input.Update(msg)
		return m, cmd

	case fjFuzzy:
		if !isKey {
			return m, nil
		}
		switch key.String() {
		case "y":
			p.resolveFinal(game.StateCorrect)
		case "n":
			p.input.SetValue("")
			p.fj = fjAnswer
		}
		return m, nil

	case fjReveal:
		if !isKey {
			return m, nil
		}
		switch key.String() {
		case " ", "enter", "esc":
			return m.finishGame()
		case "o":
			p.overrideCorrect()
		}
	}
	return m, nil
}

// viewFinal is the full-screen Final Jeopardy layout (5b), re-centered on
// every render from current width/height.
func (m Model) viewFinal() string {
	p := m.play
	fj := p.g.Final
	w := m.modalWidth()
	inner := w - 6

	var b strings.Builder
	b.WriteString(m.styles.Title.Render("Final Jeopardy!") + "\n\n")
	b.WriteString(m.styles.CategoryHd.Render(fj.Category) + "\n\n")
	b.WriteString(m.styles.Label.Render("YOUR SCORE  ") + m.styles.Gold.Render(money(p.score)) + "\n\n")

	switch p.fj {
	case fjWager:
		p.wagerInput.Width = 12
		b.WriteString(m.styles.Label.Render("Your wager") + "\n" + p.wagerInput.View() + "\n")
		if p.wagerErr != "" {
			b.WriteString(m.styles.Wrong.Render(p.wagerErr) + "\n")
		}
		b.WriteString("\n" + m.statusBar("Enter", "confirm wager & see clue") + "\n" +
			m.styles.Hint.Render(fmt.Sprintf("Wager between $0 and %s", money(p.fjMaxWager()))))

	case fjAnswer, fjFuzzy:
		b.WriteString(m.styles.ClueText.Width(inner).Render(fj.Text) + "\n\n")
		b.WriteString(m.styles.Hint.Render("Your wager: "+money(p.fjWagerN)) + "\n\n")
		if p.fj == fjFuzzy {
			b.WriteString(m.styles.Gold.Render("Close match detected") + "\n" +
				m.styles.Text.Width(inner).Render(fmt.Sprintf("%q", canonicalQuestion(fj.CorrectResponse))) + "\n\n" +
				m.statusBar("y", "accept", "n", "reject & re-enter"))
		} else {
			p.input.Width = inner - 3
			b.WriteString(m.styles.Label.Render("Your response") + "\n" + p.input.View() + "\n\n" +
				m.statusBar("Enter", "lock in — no pass option in Final Jeopardy"))
		}

	case fjReveal:
		b.WriteString(m.styles.ClueText.Width(inner).Render(fj.Text) + "\n\n")
		b.WriteString(m.styles.Label.Render("Correct response") + "\n" +
			m.styles.Text.Bold(true).Width(inner).Render(canonicalQuestion(fj.CorrectResponse)) + "\n\n")

		st := m.styles.Correct
		badge := "✓"
		if p.revealState != game.StateCorrect {
			st, badge = m.styles.Wrong, "✗"
		}
		b.WriteString(m.styles.You.Render("YOU") + " " +
			m.styles.Text.Render(fmt.Sprintf("%q", p.input.Value())) + " " +
			st.Render(fmt.Sprintf("%s %s (wager %s)", badge, plusMoney(p.revealDelta), money(p.fjWagerN))) + "\n")
		for _, c := range p.g.Contestants {
			line := m.styles.Label.Render(strings.ToUpper(c.Short)) + " "
			found := false
			for _, r := range fj.Responses {
				if r.Contestant != c.Short {
					continue
				}
				found = true
				rst, rb, delta := m.styles.Correct, "✓", r.Wager
				if r.Result != "correct" {
					rst, rb, delta = m.styles.Wrong, "✗", -r.Wager
				}
				line += m.styles.Text.Render(truncate(fmt.Sprintf("%q", r.GivenResponse), inner-24)) + " " +
					rst.Render(fmt.Sprintf("%s %s (wager %s)", rb, plusMoney(delta), money(r.Wager)))
			}
			if !found {
				line += m.styles.Hint.Render("— no response recorded")
			}
			b.WriteString(line + "\n")
		}
		b.WriteString("\n" + m.styles.Gold.Render("Final: "+money(p.score)) + "\n\n")
		if p.revealState == game.StateIncorrect {
			b.WriteString(m.statusBar("Space", "final results", "o", "mark correct"))
		} else {
			b.WriteString(m.statusBar("Space", "final results"))
		}
	}

	panel := m.styles.Modal.Width(w).Render(strings.TrimRight(b.String(), "\n"))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, panel)
}
