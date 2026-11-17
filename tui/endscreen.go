package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"jplay/db"
	"jplay/game"
)

// updateEnd handles the end screen keys: s stats, p play again, q quit.
func (m Model) updateEnd(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, isKey := msg.(tea.KeyMsg)
	if !isKey {
		return m, nil
	}
	switch key.String() {
	case "s":
		m.screen = ScreenStats
		m.stats = newStatsModel(&m)
		return m, m.stats.reload(m.db)
	case "d":
		if m.play.savedRowID == 0 || m.db == nil {
			return m, nil
		}
		p := m.play
		m.screen = ScreenStats
		m.stats = newStatsModel(&m)
		return m, tea.Batch(m.stats.reload(m.db),
			m.stats.loadDetail(m.db, db.RecentGame{
				RowID:    p.savedRowID,
				GameID:   p.g.Label(),
				PlayedAt: time.Now().Format("2006-01-02"),
				Score:    p.score,
			}))
	case "p":
		m.play = nil
		m.screen = ScreenLanding
		m.landing = newLandingModel(&m)
		return m, nil
	case "q", "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) viewEnd() string {
	p := m.play
	w := m.modalWidth()

	var b strings.Builder
	b.WriteString(m.styles.Label.Render("Final Score") + "\n")
	b.WriteString(m.styles.Gold.Render(money(p.score)) + "\n")
	b.WriteString(m.styles.Hint.Render(fmt.Sprintf("%s · played %s",
		p.g.Label(), time.Now().Format("2006-01-02"))) + "\n\n")

	// Rank table: contestants (recorded finals) + YOU, sorted by score.
	type row struct {
		name  string
		score int
		rw    string // correct/incorrect counts
		you   bool
	}
	rows := []row{{"YOU", p.score,
		fmt.Sprintf("%d/%d", p.counts[game.StateCorrect], p.counts[game.StateIncorrect]), true}}
	finals := p.finalScores()
	counts := p.contestantCounts()
	for _, c := range p.g.Contestants {
		n := counts[c.Short]
		rows = append(rows, row{strings.ToUpper(c.Short), finals[c.Short],
			fmt.Sprintf("%d/%d", n[0], n[1]), false})
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].score > rows[j].score })

	ranks := []string{"1st", "2nd", "3rd", "4th", "5th", "6th"}
	b.WriteString(m.styles.Label.Render(pad("Rank", 6)+pad("Player", 14)+pad("Final", 10)+pad("R/W", 7)+"vs You") + "\n")
	for i, r := range rows {
		nameSt := m.styles.Label
		if r.you {
			nameSt = m.styles.You
		}
		diff := "—"
		if !r.you {
			diff = plusMoney(r.score - p.score) // positive: you lead them
		}
		rank := "?"
		if i < len(ranks) {
			rank = ranks[i]
		}
		b.WriteString(m.styles.Text.Render(pad(rank, 6)) +
			nameSt.Render(pad(truncate(r.name, 13), 14)) +
			m.styles.Gold.Render(pad(money(r.score), 10)) +
			m.styles.Text.Render(pad(r.rw, 7)) +
			m.styles.Hint.Render(diff) + "\n")
	}

	// Summary counts (3-cell grid).
	cell := func(n int, label string, st lipgloss.Style) string {
		return st.Width(12).Align(lipgloss.Center).
			Render(fmt.Sprintf("%d", n)) + "\n" +
			m.styles.Hint.Width(12).Align(lipgloss.Center).Render(label)
	}
	summary := lipgloss.JoinHorizontal(lipgloss.Top,
		m.styles.Panel.Render(cell(p.counts[game.StateCorrect], "Correct", m.styles.Correct)),
		m.styles.Panel.Render(cell(p.counts[game.StateIncorrect], "Incorrect", m.styles.Wrong)),
		m.styles.Panel.Render(cell(p.counts[game.StatePassed], "Passed", m.styles.Hint)))
	b.WriteString("\n" + summary)

	panel := m.styles.Modal.Width(w).Render(b.String())
	body := lipgloss.Place(m.width, m.height-1, lipgloss.Center, lipgloss.Center, panel)
	return body + "\n" + m.statusBar("d", "game detail", "s", "stats", "p", "play again", "q", "quit")
}

func pad(s string, w int) string {
	if lipgloss.Width(s) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-lipgloss.Width(s))
}
