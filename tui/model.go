// Package tui implements jplay's bubbletea interface.
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"jplay/config"
	"jplay/db"
	"jplay/game"
)

// Screen identifies the active top-level screen.
type Screen int

const (
	ScreenLanding Screen = iota
	ScreenBrowser
	ScreenBoard // includes clue/DD overlays
	ScreenFinal // Final Jeopardy (full screen)
	ScreenEnd
	ScreenStats
	ScreenConfig
)

// Model is the root bubbletea model. All sub-screens render from
// width/height on every View call; nothing caches layout.
type Model struct {
	width, height int

	cfg    config.Config
	styles Styles
	db     *db.DB // nil when the history DB could not be opened
	dbErr  error

	screen          Screen
	statsStandalone bool // launched as `jplay --stats`: q quits instead of going back
	landing         landingModel
	browser browserModel
	play    *playModel
	stats   statsModel
	confSc  configModel

	flash string // transient status-line message
}

// Options selects the initial screen.
type Options struct {
	Mode     string // "play", "stats", "config"
	GamePath string // non-empty: load this file immediately
}

// New builds the root model. The DB is opened here; failure degrades
// gracefully (history features disabled) rather than aborting.
func New(cfg config.Config, opts Options) Model {
	m := Model{
		cfg:    cfg,
		styles: NewStyles(LoadTheme(cfg)),
	}
	d, err := db.Open(cfg.ResolvedDBPath())
	if err != nil {
		m.dbErr = err
	} else {
		m.db = d
	}

	switch opts.Mode {
	case "stats":
		m.screen = ScreenStats
		m.statsStandalone = true
		m.stats = newStatsModel(&m)
	case "config":
		m.screen = ScreenConfig
		m.confSc = newConfigModel(cfg)
	default:
		m.screen = ScreenLanding
		m.landing = newLandingModel(&m)
		if opts.GamePath != "" {
			m.landing.pendingPath = opts.GamePath
		}
	}
	return m
}

// Init starts a direct-file load when one was given on the command line.
func (m Model) Init() tea.Cmd {
	if m.screen == ScreenLanding && m.landing.pendingPath != "" {
		return loadGameFileCmd(m.landing.pendingPath)
	}
	if m.screen == ScreenStats {
		return m.stats.reload(m.db)
	}
	return nil
}

// Update routes messages to the active screen.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case gameLoadedMsg:
		if msg.err != nil {
			m.flash = fmt.Sprintf("load failed: %v", msg.err)
			return m, nil
		}
		m.startGame(msg.game)
		return m, nil
	}

	m.flash = ""
	switch m.screen {
	case ScreenLanding:
		return m.updateLanding(msg)
	case ScreenBrowser:
		return m.updateBrowser(msg)
	case ScreenBoard:
		return m.updateBoard(msg)
	case ScreenFinal:
		return m.updateFinal(msg)
	case ScreenEnd:
		return m.updateEnd(msg)
	case ScreenStats:
		return m.updateStats(msg)
	case ScreenConfig:
		return m.updateConfig(msg)
	}
	return m, nil
}

// View renders the active screen at the current size.
func (m Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return "loading…"
	}
	switch m.screen {
	case ScreenLanding:
		return m.viewLanding()
	case ScreenBrowser:
		return m.viewBrowser()
	case ScreenBoard:
		return m.viewBoard()
	case ScreenFinal:
		return m.viewFinal()
	case ScreenEnd:
		return m.viewEnd()
	case ScreenStats:
		return m.viewStats()
	case ScreenConfig:
		return m.viewConfig()
	}
	return ""
}

// startGame switches into play mode with a freshly loaded game.
func (m *Model) startGame(g *game.Game) {
	m.play = newPlayModel(g, m.cfg)
	m.screen = ScreenBoard
}

// gameLoadedMsg carries the result of an async game load.
type gameLoadedMsg struct {
	game *game.Game
	err  error
}

func loadGameFileCmd(path string) tea.Cmd {
	return func() tea.Msg {
		g, err := game.LoadFile(config.ExpandPath(path))
		return gameLoadedMsg{g, err}
	}
}

// --- shared key helpers -------------------------------------------------

func keyUp(k tea.KeyMsg) bool    { s := k.String(); return s == "k" || s == "up" }
func keyDown(k tea.KeyMsg) bool  { s := k.String(); return s == "j" || s == "down" }
func keyLeft(k tea.KeyMsg) bool  { s := k.String(); return s == "h" || s == "left" }
func keyRight(k tea.KeyMsg) bool { s := k.String(); return s == "l" || s == "right" }

// --- shared render helpers ----------------------------------------------

// statusBar renders the single bottom row from key/desc pairs, dropping
// hints from the right when the terminal is too narrow. Never wraps.
func (m Model) statusBar(pairs ...string) string {
	var parts []string
	for i := 0; i+1 < len(pairs); i += 2 {
		parts = append(parts,
			m.styles.StatusKey.Render(pairs[i])+" "+m.styles.StatusBar.Render(pairs[i+1]))
	}
	if m.flash != "" {
		parts = append([]string{m.styles.Wrong.Render(m.flash)}, parts...)
	}
	sep := m.styles.StatusBar.Render("  ·  ")
	line := ""
	for i, p := range parts {
		candidate := line
		if i > 0 {
			candidate += sep
		}
		candidate += p
		if lipgloss.Width(candidate) > m.width {
			break
		}
		line = candidate
	}
	return line
}

// page stacks content above a pinned status bar, padding to full height.
func (m Model) page(content, status string) string {
	content = lipgloss.NewStyle().MaxHeight(m.height - 1).MaxWidth(m.width).Render(content)
	gap := m.height - 1 - lipgloss.Height(content)
	if gap < 0 {
		gap = 0
	}
	return content + strings.Repeat("\n", gap+1) + status
}

// overlayCenter draws a modal centered over a dimmed backdrop.
func (m Model) overlayCenter(backdrop, modal string) string {
	return placeOverlayDim(stripANSI(backdrop), modal, m.width, m.height, m.styles.DimBoard)
}

// truncate shortens s to width cells with a … marker.
func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && lipgloss.Width(string(r)) > width-1 {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}

func money(n int) string {
	neg := n < 0
	if neg {
		n = -n
	}
	s := fmt.Sprintf("%d", n)
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	if neg {
		return "−$" + string(out)
	}
	return "$" + string(out)
}
