package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"jplay/db"
)

// landingModel: Browse Archive / Open File choice + recent games.
type landingModel struct {
	choice      int // 0 = Browse Archive, 1 = Open File
	entering    bool
	pathInput   textinput.Model
	recent      []db.RecentGame
	pendingPath string // set when a file was passed on the command line
}

func newLandingModel(m *Model) landingModel {
	in := textinput.New()
	in.Prompt = "path> "
	in.CharLimit = 400
	l := landingModel{pathInput: in}
	if m.db != nil {
		l.recent, _ = m.db.RecentGames(5)
	}
	return l
}

func (m Model) updateLanding(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, isKey := msg.(tea.KeyMsg)
	l := &m.landing

	if l.entering {
		if isKey {
			switch key.String() {
			case "enter":
				path := strings.TrimSpace(l.pathInput.Value())
				if path != "" {
					return m, loadGameFileCmd(path)
				}
				return m, nil
			case "esc":
				l.entering = false
				return m, nil
			}
		}
		var cmd tea.Cmd
		l.pathInput, cmd = l.pathInput.Update(msg)
		return m, cmd
	}

	if !isKey {
		return m, nil
	}
	switch {
	case keyLeft(key), keyRight(key), key.String() == "tab", key.String() == "shift+tab":
		l.choice = 1 - l.choice
	case key.String() == "s":
		m.screen = ScreenStats
		m.stats = newStatsModel(&m)
		return m, m.stats.reload(m.db)
	case key.String() == "enter":
		if l.choice == 0 {
			m.screen = ScreenBrowser
			m.browser = newBrowserModel()
			return m, loadSeasonsCmd(m.jarchClient())
		}
		l.entering = true
		l.pathInput.SetValue("")
		l.pathInput.Focus()
	case key.String() == "q", key.String() == "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) viewLanding() string {
	l := m.landing
	cardW := (m.width - 8) / 2
	if cardW > 40 {
		cardW = 40
	}
	if cardW < 16 {
		cardW = 16
	}

	card := func(title, desc string, selected bool) string {
		st := m.styles.Panel
		titleSt := m.styles.Text.Bold(true)
		if selected {
			st = st.BorderForeground(m.styles.Theme.Foam)
			titleSt = m.styles.You
		}
		return st.Width(cardW).Render(
			titleSt.Render(title) + "\n" +
				m.styles.Hint.Width(cardW-2).Render(desc))
	}
	cards := lipgloss.JoinHorizontal(lipgloss.Top,
		card("Browse Archive", "Navigate seasons and episodes, fetch via jarch", l.choice == 0),
		"  ",
		card("Open File", "Play a local game.json produced by jarch", l.choice == 1))

	var b strings.Builder
	b.WriteString(m.styles.Title.Render("jplay") + "  " +
		m.styles.Hint.Render("Jeopardy archive player · v0.1.0") + "\n\n")
	b.WriteString(cards + "\n\n")

	if l.entering {
		l.pathInput.Width = m.width - 12
		b.WriteString(m.styles.Label.Render("Path to game JSON:") + "\n" +
			l.pathInput.View() + "\n\n")
	}

	if len(l.recent) > 0 {
		b.WriteString(m.styles.Label.Render("Recent") + "\n")
		for _, r := range l.recent {
			line := fmt.Sprintf("%s  %s  %s  %d correct",
				pad(r.GameID, 9), r.PlayedAt, pad(money(r.Score), 9), r.Correct)
			if f := r.Finish(); f != "" {
				line += "  " + pad(f, 9)
			}
			b.WriteString(m.styles.Text.Render(truncate(line, m.width-2)) + "\n")
		}
	} else if m.dbErr != nil {
		b.WriteString(m.styles.Hint.Render("history unavailable: "+m.dbErr.Error()) + "\n")
	}

	var status string
	if l.entering {
		status = m.statusBar("Enter", "load file", "Esc", "cancel")
	} else {
		status = m.statusBar("←→/Tab", "choose", "Enter", "confirm", "s", "stats", "q", "quit")
	}
	content := lipgloss.NewStyle().PaddingTop(1).PaddingLeft(2).Render(b.String())
	return m.page(content, status)
}
