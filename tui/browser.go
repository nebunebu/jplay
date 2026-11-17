package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"jplay/db"
	"jplay/game"
	"jplay/jarch"
)

// seasonEntry is one row of the seasons list.
type seasonEntry = jarch.Season

// episodeEntry is one row of a season's episode list.
type episodeEntry = jarch.GameRow

type browserModel struct {
	seasons   []seasonEntry
	seasonIdx int
	episodes  map[string][]episodeEntry // season id -> episodes
	epIdx     int
	focus     int    // 0 = seasons pane, 1 = episodes pane
	loading   string // human-readable in-flight operation, "" when idle
	err       string

	// Games already played to completion (from the history DB): archive
	// ids, plus display labels for rows saved before ids were recorded.
	played       map[int]bool
	playedLabels map[string]bool
}

func newBrowserModel(d *db.DB) browserModel {
	b := browserModel{episodes: map[string][]episodeEntry{}, loading: "loading seasons…"}
	if d != nil {
		b.played, b.playedLabels, _ = d.Played() // best-effort: badges only
	}
	return b
}

func (b browserModel) isPlayed(ep episodeEntry, label string) bool {
	return b.played[ep.GameID] || b.playedLabels[label]
}

// --- archive fetch commands ------------------------------------------------

type seasonsMsg struct {
	seasons []seasonEntry
	err     error
}

type episodesMsg struct {
	seasonID string
	episodes []episodeEntry
	err      error
}

type fetchedMsg struct {
	gameID int
	err    error
}

// jarchClient returns a fetcher using the configured cache directory.
// Parser warnings are discarded: stderr would corrupt the alt screen.
func (m Model) jarchClient() *jarch.Client {
	return jarch.New(m.cfg.ResolvedCacheDir())
}

func loadSeasonsCmd(c *jarch.Client) tea.Cmd {
	return func() tea.Msg {
		seasons, err := c.Seasons()
		return seasonsMsg{seasons: seasons, err: err}
	}
}

func loadEpisodesCmd(c *jarch.Client, seasonID string) tea.Cmd {
	return func() tea.Msg {
		eps, err := c.Season(seasonID)
		return episodesMsg{seasonID: seasonID, episodes: eps, err: err}
	}
}

// loadGameCmd fetches (cache-first) and parses a game.
func loadGameCmd(c *jarch.Client, gameID int) tea.Cmd {
	return func() tea.Msg {
		jg, err := c.Game(gameID)
		if err != nil {
			return gameLoadedMsg{err: err}
		}
		data, err := jarch.Marshal(jg)
		if err != nil {
			return gameLoadedMsg{err: err}
		}
		g, err := game.Load(data)
		return gameLoadedMsg{game: g, err: err}
	}
}

// fetchOnlyCmd caches a game without playing it (the `f` binding).
func fetchOnlyCmd(c *jarch.Client, gameID int) tea.Cmd {
	return func() tea.Msg {
		_, err := c.Game(gameID)
		return fetchedMsg{gameID: gameID, err: err}
	}
}

// --- update ---------------------------------------------------------------

func (m Model) updateBrowser(msg tea.Msg) (tea.Model, tea.Cmd) {
	b := &m.browser

	switch msg := msg.(type) {
	case seasonsMsg:
		b.loading = ""
		if msg.err != nil {
			b.err = msg.err.Error()
			return m, nil
		}
		b.seasons = msg.seasons
		if len(b.seasons) > 0 {
			return m, m.selectSeason(0)
		}
		return m, nil

	case episodesMsg:
		b.loading = ""
		if msg.err != nil {
			b.err = msg.err.Error()
			return m, nil
		}
		b.episodes[msg.seasonID] = msg.episodes
		return m, nil

	case fetchedMsg:
		b.loading = ""
		if msg.err != nil {
			b.err = msg.err.Error()
		} else {
			m.flash = fmt.Sprintf("fetched game %d", msg.gameID)
		}
		return m, nil

	case tea.KeyMsg:
		if b.loading != "" {
			// Only allow backing out while a fetch is in flight.
			if msg.String() == "q" {
				m.screen = ScreenLanding
				m.landing = newLandingModel(&m)
			}
			return m, nil
		}
		b.err = ""
		switch {
		case msg.String() == "tab", msg.String() == "shift+tab":
			b.focus = 1 - b.focus
		case keyLeft(msg):
			b.focus = 0 // seasons pane
		case keyRight(msg):
			b.focus = 1 // episodes pane
		case keyDown(msg):
			if b.focus == 0 {
				if b.seasonIdx < len(b.seasons)-1 {
					return m, m.selectSeason(b.seasonIdx + 1)
				}
			} else if b.epIdx < len(m.currentEpisodes())-1 {
				b.epIdx++
			}
		case keyUp(msg):
			if b.focus == 0 {
				if b.seasonIdx > 0 {
					return m, m.selectSeason(b.seasonIdx - 1)
				}
			} else if b.epIdx > 0 {
				b.epIdx--
			}
		case msg.String() == "enter":
			if b.focus == 0 {
				b.focus = 1 // into the episode list
				return m, nil
			}
			if ep, ok := m.currentEpisode(); ok {
				b.loading = fmt.Sprintf("loading game %d…", ep.GameID)
				return m, loadGameCmd(m.jarchClient(), ep.GameID)
			}
		case msg.String() == "f":
			if ep, ok := m.currentEpisode(); ok {
				b.loading = fmt.Sprintf("fetching game %d…", ep.GameID)
				return m, fetchOnlyCmd(m.jarchClient(), ep.GameID)
			}
		case msg.String() == "q":
			m.screen = ScreenLanding
			m.landing = newLandingModel(&m)
		}
	}
	return m, nil
}

// selectSeason moves the season cursor and lazily loads its episode list.
func (m *Model) selectSeason(idx int) tea.Cmd {
	b := &m.browser
	b.seasonIdx = idx
	b.epIdx = 0
	id := b.seasons[idx].ID()
	if _, ok := b.episodes[id]; ok || id == "" {
		return nil
	}
	b.loading = "loading " + b.seasons[idx].Title + "…"
	return loadEpisodesCmd(m.jarchClient(), id)
}

func (m Model) currentEpisodes() []episodeEntry {
	b := m.browser
	if b.seasonIdx >= len(b.seasons) {
		return nil
	}
	return b.episodes[b.seasons[b.seasonIdx].ID()]
}

func (m Model) currentEpisode() (episodeEntry, bool) {
	eps := m.currentEpisodes()
	if m.browser.epIdx < len(eps) {
		return eps[m.browser.epIdx], true
	}
	return episodeEntry{}, false
}

// --- view -------------------------------------------------------------------

func (m Model) viewBrowser() string {
	b := m.browser

	leftW := 22
	if m.width < 60 {
		leftW = 16
	}
	rightW := m.width - leftW - 3

	title := m.styles.Title.Render("jplay — browse archive")
	if b.loading != "" {
		title += "  " + m.styles.Hint.Render(b.loading)
	}
	if b.err != "" {
		title += "  " + m.styles.Wrong.Render(truncate(b.err, m.width-30))
	}

	listH := m.height - 4
	if listH < 3 {
		listH = 3
	}

	// Left pane: seasons. The focused pane gets a highlighted header and the
	// bright cursor row; the unfocused pane's cursor is dimmed.
	cursorStyle := func(focused bool) lipgloss.Style {
		if focused {
			return m.styles.SelectedRow
		}
		return m.styles.You
	}
	paneHead := func(label string, focused bool) string {
		if focused {
			return m.styles.TabActive.Render("▸ " + label)
		}
		return m.styles.Label.Render("  " + label)
	}
	var left strings.Builder
	left.WriteString(paneHead("SEASONS", b.focus == 0) + "\n")
	start := windowStart(b.seasonIdx, len(b.seasons), listH-1)
	for i := start; i < len(b.seasons) && i-start < listH-1; i++ {
		s := b.seasons[i]
		line := truncate(s.Title, leftW-6) + " " + fmt.Sprint(s.GamesArchived)
		if i == b.seasonIdx {
			left.WriteString(cursorStyle(b.focus == 0).Width(leftW).Render(truncate(line, leftW)) + "\n")
		} else {
			left.WriteString(m.styles.Text.Render(truncate(line, leftW)) + "\n")
		}
	}

	// Right pane: episodes of the selected season.
	var right strings.Builder
	if len(b.seasons) > 0 {
		s := b.seasons[b.seasonIdx]
		right.WriteString(paneHead("EPISODES", b.focus == 1) + " " +
			m.styles.CategoryHd.Render(truncate(s.Title, rightW-32)) + " " +
			m.styles.Hint.Render(fmt.Sprintf("%s → %s · %d episodes", s.StartDate, deref(s.EndDate), s.GamesArchived)) + "\n")
	}
	eps := m.currentEpisodes()
	jc := m.jarchClient()
	epStart := windowStart(b.epIdx, len(eps), listH-1)
	for i := epStart; i < len(eps) && i-epStart < listH-1; i++ {
		ep := eps[i]
		id := fmt.Sprintf("J#%d", ep.EpisodeNumber)
		if ep.EpisodeNumber == 0 {
			id = fmt.Sprintf("game %d", ep.GameID)
		}
		badge := m.styles.BadgeRemote.Render("remote")
		switch {
		case b.isPlayed(ep, id):
			badge = m.styles.Correct.Render("played")
		case jc.IsCached(jarch.GamePage(ep.GameID)):
			badge = m.styles.BadgeCached.Render("cached")
		}
		names := strings.Join(ep.Contestants, ", ")
		line := fmt.Sprintf("%s  %s  %s", pad(id, 8), ep.AirDate, names)
		line = pad(truncate(line, rightW-8), rightW-8) + " " + badge
		if i == b.epIdx {
			right.WriteString(cursorStyle(b.focus == 1).Render(line) + "\n")
		} else {
			right.WriteString(m.styles.Text.Render(line) + "\n")
		}
	}
	if len(eps) == 0 && b.loading == "" {
		right.WriteString(m.styles.Hint.Render("no episodes") + "\n")
	}

	panes := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(leftW).Render(left.String()),
		" │ ",
		lipgloss.NewStyle().Width(rightW).Render(right.String()))

	status := m.statusBar("j/k", "navigate", "h/l/Tab", "switch pane",
		"Enter", "play / fetch+play", "f", "fetch only", "q", "back")
	return m.page(title+"\n\n"+panes, status)
}

// windowStart scrolls a list window to keep the cursor visible.
func windowStart(cursor, total, visible int) int {
	if total <= visible || visible <= 0 {
		return 0
	}
	start := cursor - visible/2
	if start < 0 {
		start = 0
	}
	if start > total-visible {
		start = total - visible
	}
	return start
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
