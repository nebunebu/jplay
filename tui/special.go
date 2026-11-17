package tui

import (
	"fmt"
	"sort"
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
		if m.cfg.ShowContestantScores && len(p.g.Contestants) > 0 {
			b.WriteString(m.renderFJStandings() + "\n")
		}
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

// renderFJStandings lists the scores going into Final Jeopardy, highest
// first, with a one-line wagering hint.
func (m Model) renderFJStandings() string {
	p := m.play
	var b strings.Builder
	b.WriteString(m.styles.Label.Render("GOING INTO FINAL") + "\n")
	var opp []int
	for _, c := range p.g.Contestants {
		opp = append(opp, p.oppScores[c.Short])
	}
	type row struct {
		name  string
		score int
	}
	rows := []row{{"YOU", p.score}}
	for i, c := range p.g.Contestants {
		rows = append(rows, row{strings.ToUpper(c.Short), opp[i]})
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].score > rows[j].score })
	for _, r := range rows {
		st := m.styles.Text
		if r.name == "YOU" {
			st = m.styles.You
		}
		b.WriteString(st.Render(fmt.Sprintf("  %-12s %s", truncate(r.name, 12), money(r.score))) + "\n")
	}
	if h := fjWagerHint(p.score, opp); h != "" {
		b.WriteString(m.styles.Hint.Render(h) + "\n")
	}
	return b.String()
}

// fjWagerHint gives classic Final Jeopardy wagering guidance relative to the
// strongest opponent (assuming opponents may double their score).
func fjWagerHint(you int, opponents []int) string {
	if you <= 0 || len(opponents) == 0 {
		return ""
	}
	best := opponents[0]
	for _, o := range opponents[1:] {
		best = max(best, o)
	}
	switch {
	case you > 2*best:
		return fmt.Sprintf("Runaway: wager at most %s and you win even if you miss.", money(you-2*best-1))
	case you > best:
		need := 2*best - you + 1
		return fmt.Sprintf("Leading: wager at least %s to stay ahead if 2nd place doubles up.", money(need))
	case you == best:
		return "Tied for the lead: betting everything guarantees at least a tie if you're right."
	default:
		return fmt.Sprintf("Trailing the leader by %s.", money(best-you))
	}
}
