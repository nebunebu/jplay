package tui

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"jplay/db"
)

var statsTabs = []string{"Recent", "By Category", "By Value", "Missed", "Passed"}

type statsModel struct {
	tab    int
	scroll map[int]int

	recent []db.RecentGame
	cats   []db.AccuracyRow
	vals   []db.AccuracyRow
	missed []db.MissRow
	passed []db.MissRow

	// Per-game drill-down; non-nil detailGame replaces the tab view.
	detailGame   *db.RecentGame
	detailRows   []db.GameClueRow
	detailScroll int

	loaded bool
	err    string
}

type statsDataMsg struct {
	recent []db.RecentGame
	cats   []db.AccuracyRow
	vals   []db.AccuracyRow
	missed []db.MissRow
	passed []db.MissRow
	err    error
}

type statsDetailMsg struct {
	game db.RecentGame
	rows []db.GameClueRow
	err  error
}

// loadDetail fetches one saved game's clue list for the drill-down view.
func (s statsModel) loadDetail(d *db.DB, g db.RecentGame) tea.Cmd {
	return func() tea.Msg {
		if d == nil {
			return statsDetailMsg{err: fmt.Errorf("history database unavailable")}
		}
		rows, err := d.GameClues(g.RowID)
		return statsDetailMsg{game: g, rows: rows, err: err}
	}
}

func newStatsModel(m *Model) statsModel {
	return statsModel{scroll: map[int]int{}}
}

// reload queries all tabs' data in one command.
func (s statsModel) reload(d *db.DB) tea.Cmd {
	return func() tea.Msg {
		if d == nil {
			return statsDataMsg{err: fmt.Errorf("history database unavailable")}
		}
		var msg statsDataMsg
		var err error
		if msg.recent, err = d.RecentGames(200); err != nil {
			return statsDataMsg{err: err}
		}
		if msg.cats, err = d.ByCategory(); err != nil {
			return statsDataMsg{err: err}
		}
		if msg.vals, err = d.ByValue(); err != nil {
			return statsDataMsg{err: err}
		}
		if msg.missed, err = d.CluesByState("incorrect"); err != nil {
			return statsDataMsg{err: err}
		}
		if msg.passed, err = d.CluesByState("passed"); err != nil {
			return statsDataMsg{err: err}
		}
		return msg
	}
}

func (m Model) updateStats(msg tea.Msg) (tea.Model, tea.Cmd) {
	s := &m.stats
	switch msg := msg.(type) {
	case statsDataMsg:
		s.loaded = true
		if msg.err != nil {
			s.err = msg.err.Error()
			return m, nil
		}
		s.recent, s.cats, s.vals = msg.recent, msg.cats, msg.vals
		s.missed, s.passed = msg.missed, msg.passed
		return m, nil

	case statsDetailMsg:
		if msg.err != nil {
			m.flash = "game detail: " + msg.err.Error()
			return m, nil
		}
		g := msg.game
		s.detailGame, s.detailRows, s.detailScroll = &g, msg.rows, 0
		return m, nil

	case tea.KeyMsg:
		if s.detailGame != nil {
			switch {
			case keyDown(msg):
				if s.detailScroll < len(s.detailRows)-1 {
					s.detailScroll++
				}
			case keyUp(msg):
				if s.detailScroll > 0 {
					s.detailScroll--
				}
			case msg.String() == "q", msg.String() == "esc":
				s.detailGame, s.detailRows = nil, nil
			case msg.String() == "ctrl+c":
				return m, tea.Quit
			}
			return m, nil
		}
		switch {
		case msg.String() == "tab":
			s.tab = (s.tab + 1) % len(statsTabs)
		case msg.String() == "shift+tab":
			s.tab = (s.tab + len(statsTabs) - 1) % len(statsTabs)
		case keyDown(msg):
			if s.scroll[s.tab] < s.rowCount()-1 {
				s.scroll[s.tab]++
			}
		case keyUp(msg):
			if s.scroll[s.tab] > 0 {
				s.scroll[s.tab]--
			}
		case msg.String() == "enter":
			if s.tab == 0 && s.scroll[0] < len(s.recent) {
				return m, s.loadDetail(m.db, s.recent[s.scroll[0]])
			}
		case msg.String() == "e":
			path, err := s.export(m.cfg.ExportFormat)
			if err != nil {
				m.flash = "export failed: " + err.Error()
			} else {
				m.flash = "exported " + path
			}
		case msg.String() == "q", msg.String() == "esc":
			if m.statsStandalone {
				return m, tea.Quit
			}
			m.screen = ScreenLanding
			m.landing = newLandingModel(&m)
		case msg.String() == "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (s *statsModel) rowCount() int {
	switch s.tab {
	case 0:
		return len(s.recent)
	case 1:
		return len(s.cats)
	case 2:
		return len(s.vals)
	case 3:
		return len(s.missed)
	default:
		return len(s.passed)
	}
}

// tabData renders the current tab as header + rows of plain cells; the view
// and the exporter share this representation.
func (s *statsModel) tabData() (header []string, rows [][]string) {
	switch s.tab {
	case 0:
		header = []string{"Game", "Date", "Score", "Finish", "Correct", "Wrong", "Passed"}
		for _, r := range s.recent {
			rows = append(rows, []string{r.GameID, r.PlayedAt, money(r.Score), r.Finish(),
				fmt.Sprint(r.Correct), fmt.Sprint(r.Incorrect), fmt.Sprint(r.Passed)})
		}
	case 1, 2:
		data := s.cats
		key := "Category"
		if s.tab == 2 {
			data, key = s.vals, "Value"
		}
		header = []string{key, "Seen", "Correct", "Accuracy"}
		for _, r := range data {
			rows = append(rows, []string{r.Key, fmt.Sprint(r.Seen),
				fmt.Sprint(r.Correct), fmt.Sprintf("%d%%", r.Accuracy())})
		}
	default:
		data := s.missed
		if s.tab == 4 {
			data = s.passed
		}
		header = []string{"Date", "Category", "Value", "Clue", "Correct response", "Your answer"}
		for _, r := range data {
			rows = append(rows, []string{r.PlayedAt, r.Category, money(r.Value),
				r.ClueText, r.CorrectResponse, r.PlayerAnswer})
		}
	}
	return header, rows
}

// export writes the current tab in the configured format to the CWD.
func (s *statsModel) export(format string) (string, error) {
	header, rows := s.tabData()
	name := strings.ToLower(strings.ReplaceAll(statsTabs[s.tab], " ", "-"))
	path := fmt.Sprintf("jplay-%s-%s.%s", name, time.Now().Format("2006-01-02"), format)

	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	switch format {
	case "json":
		var objs []map[string]string
		for _, row := range rows {
			obj := map[string]string{}
			for i, h := range header {
				obj[h] = row[i]
			}
			objs = append(objs, obj)
		}
		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")
		return path, enc.Encode(objs)
	case "tsv":
		for _, row := range append([][]string{header}, rows...) {
			if _, err := fmt.Fprintln(f, strings.Join(row, "\t")); err != nil {
				return "", err
			}
		}
		return path, nil
	default: // csv
		w := csv.NewWriter(f)
		if err := w.Write(header); err != nil {
			return "", err
		}
		if err := w.WriteAll(rows); err != nil {
			return "", err
		}
		w.Flush()
		return path, w.Error()
	}
}

func (m Model) viewStats() string {
	s := m.stats
	if s.detailGame != nil {
		return m.viewGameDetail()
	}

	var tabs []string
	for i, t := range statsTabs {
		if i == s.tab {
			tabs = append(tabs, m.styles.TabActive.Render(t))
		} else {
			tabs = append(tabs, m.styles.TabInactive.Render(t))
		}
	}
	head := m.styles.Title.Render("jplay --stats") + "\n" +
		strings.Join(tabs, m.styles.Hint.Render("  ·  "))

	var body string
	switch {
	case s.err != "":
		body = m.styles.Wrong.Render(s.err)
	case !s.loaded:
		body = m.styles.Hint.Render("loading…")
	default:
		body = m.renderStatsTable()
	}

	qHint := "quit"
	if !m.statsStandalone {
		qHint = "back"
	}
	status := m.statusBar("Tab", "switch tab", "j/k", "scroll", "Enter", "game detail",
		"e", "export "+strings.ToUpper(m.cfg.ExportFormat), "q", qHint)
	return m.page(head+"\n\n"+body, status)
}

// viewGameDetail renders the per-game drill-down: every clue of one saved
// play-through, in play order.
func (m Model) viewGameDetail() string {
	s := m.stats
	g := s.detailGame
	head := m.styles.Title.Render(g.GameID) + "  " +
		m.styles.Hint.Render(g.PlayedAt) + "  " +
		m.styles.Gold.Render(money(g.Score))
	if f := g.Finish(); f != "" {
		head += m.styles.Hint.Render("  · finished " + f)
	}

	header := []string{"Value", "Category", "Result", "Clue", "Correct response", "Your answer"}
	var rows [][]string
	for _, r := range s.detailRows {
		val := money(r.Value)
		switch {
		case r.IsFinal:
			val = "FJ"
		case r.IsDailyDouble:
			val = "DD"
		}
		if r.PlayerWager != nil {
			val += " " + money(*r.PlayerWager)
		}
		rows = append(rows, []string{val, r.Category, r.State,
			r.ClueText, r.CorrectResponse, r.PlayerAnswer})
	}
	body := m.renderTable(header, rows, 3, s.detailScroll, nil)
	if len(rows) == 0 {
		body = m.styles.Hint.Render("no clue records for this game")
	}
	status := m.statusBar("j/k", "scroll", "q/Esc", "back")
	return m.page(head+"\n\n"+body, status)
}

// renderStatsTable lays the active tab out to the full terminal width; the
// widest column (clue text / bar) flexes to fill remaining space.
func (m Model) renderStatsTable() string {
	s := m.stats
	header, rows := s.tabData()
	if len(rows) == 0 {
		return m.styles.Hint.Render("no data yet — play some games")
	}

	// Fixed widths for all but the flexible column.
	flexCol := 0 // Recent has no flex; category/value tabs flex a bar column
	switch s.tab {
	case 1, 2:
		flexCol = -1 // accuracy bar appended separately
	case 3, 4:
		flexCol = 3 // clue text
	}
	var bar func(i int) int
	if s.tab == 1 || s.tab == 2 {
		bar = func(i int) int {
			if s.tab == 1 {
				return s.cats[i].Accuracy()
			}
			return s.vals[i].Accuracy()
		}
	}
	return m.renderTable(header, rows, flexCol, s.scroll[s.tab], bar)
}

// renderTable renders header + rows to the full terminal width with the
// selected row highlighted; flexCol (if >= 0) absorbs the remaining width,
// and a non-nil bar draws a percentage bar in that leftover space instead.
func (m Model) renderTable(header []string, rows [][]string, flexCol, sel int, bar func(i int) int) string {
	visible := m.height - 6
	if visible < 1 {
		visible = 1
	}
	start := windowStart(sel, len(rows), visible)

	widths := make([]int, len(header))
	for i, h := range header {
		widths[i] = lipgloss.Width(h)
		for _, row := range rows {
			if i != flexCol && lipgloss.Width(row[i]) > widths[i] {
				widths[i] = lipgloss.Width(row[i])
			}
		}
		if widths[i] > 28 {
			widths[i] = 28
		}
	}
	fixed := 0
	for i, w := range widths {
		if i != flexCol {
			fixed += w + 2
		}
	}
	flexW := m.width - fixed - 2
	if flexW < 10 {
		flexW = 10
	}
	if flexCol >= 0 {
		widths[flexCol] = flexW
	}

	line := func(cells []string, st lipgloss.Style, pct int) string {
		var parts []string
		for i, c := range cells {
			parts = append(parts, pad(truncate(c, widths[i]), widths[i]+2))
		}
		out := st.Render(strings.TrimRight(strings.Join(parts, ""), " "))
		if pct >= 0 {
			w := flexW * pct / 100
			out += "  " + m.styles.Correct.Render(strings.Repeat("█", w)) +
				m.styles.Hint.Render(strings.Repeat("░", flexW-w))
		}
		return out
	}

	var b strings.Builder
	b.WriteString(line(header, m.styles.Label, -1) + "\n")
	for i := start; i < len(rows) && i-start < visible; i++ {
		pct := -1
		if bar != nil {
			pct = bar(i)
		}
		st := m.styles.Text
		if i == sel {
			st = m.styles.SelectedRow
		}
		b.WriteString(line(rows[i], st, pct) + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}
