package tui

import (
	"github.com/charmbracelet/lipgloss"

	"jplay/config"
)

// Theme is the single source of color truth; every style in the TUI is
// derived from these 13 roles. Fields are TerminalColor so `term` can use
// NoColor (terminal defaults) while hex themes use Color.
type Theme struct {
	Base, Surface, Overlay lipgloss.TerminalColor
	Muted, Subtle, Text    lipgloss.TerminalColor
	Love, Gold, Pine       lipgloss.TerminalColor
	Foam, Iris             lipgloss.TerminalColor
	HlMed, HlLow           lipgloss.TerminalColor
}

func hexTheme(c config.Colors) Theme {
	return Theme{
		Base:    lipgloss.Color(c.Base),
		Surface: lipgloss.Color(c.Surface),
		Overlay: lipgloss.Color(c.Overlay),
		Muted:   lipgloss.Color(c.Muted),
		Subtle:  lipgloss.Color(c.Subtle),
		Text:    lipgloss.Color(c.Text),
		Love:    lipgloss.Color(c.Love),
		Gold:    lipgloss.Color(c.Gold),
		Pine:    lipgloss.Color(c.Pine),
		Foam:    lipgloss.Color(c.Foam),
		Iris:    lipgloss.Color(c.Iris),
		HlMed:   lipgloss.Color(c.HlMed),
		HlLow:   lipgloss.Color(c.HlLow),
	}
}

// TermTheme maps roles onto terminal ANSI colors, keeping backgrounds
// transparent so the user's terminal palette (Stylix etc.) shows through.
func TermTheme() Theme {
	return Theme{
		Base:    lipgloss.NoColor{},
		Surface: lipgloss.NoColor{},
		Overlay: lipgloss.Color("8"),
		Muted:   lipgloss.Color("8"),
		Subtle:  lipgloss.Color("7"),
		Text:    lipgloss.NoColor{},
		Love:    lipgloss.Color("1"),
		Gold:    lipgloss.Color("3"),
		Pine:    lipgloss.Color("6"),
		Foam:    lipgloss.Color("14"),
		Iris:    lipgloss.Color("13"),
		HlMed:   lipgloss.Color("8"),
		HlLow:   lipgloss.Color("0"),
	}
}

var rosePine = config.Colors{
	Base: "#191724", Surface: "#1f1d2e", Overlay: "#26233a",
	Muted: "#6e6a86", Subtle: "#908caa", Text: "#e0def4",
	Love: "#eb6f92", Gold: "#f6c177", Pine: "#31748f",
	Foam: "#9ccfd8", Iris: "#c4a7e7",
	HlMed: "#403d52", HlLow: "#21202e",
}

var rosePineDawn = config.Colors{
	Base: "#faf4ed", Surface: "#fffaf3", Overlay: "#f2e9e1",
	Muted: "#9893a5", Subtle: "#797593", Text: "#575279",
	Love: "#b4637a", Gold: "#ea9d34", Pine: "#286983",
	Foam: "#56949f", Iris: "#907aa9",
	HlMed: "#dfdad9", HlLow: "#f4ede8",
}

var gruvbox = config.Colors{
	Base: "#282828", Surface: "#3c3836", Overlay: "#504945",
	Muted: "#928374", Subtle: "#bdae93", Text: "#ebdbb2",
	Love: "#fb4934", Gold: "#fabd2f", Pine: "#b8bb26",
	Foam: "#8ec07c", Iris: "#d3869b",
	HlMed: "#504945", HlLow: "#32302f",
}

var catppuccin = config.Colors{ // Mocha
	Base: "#1e1e2e", Surface: "#313244", Overlay: "#45475a",
	Muted: "#6c7086", Subtle: "#a6adc8", Text: "#cdd6f4",
	Love: "#f38ba8", Gold: "#f9e2af", Pine: "#a6e3a1",
	Foam: "#94e2d5", Iris: "#cba6f7",
	HlMed: "#45475a", HlLow: "#292c3c",
}

// LoadTheme resolves the active theme: an explicit colors {} block wins,
// then named presets, then terminal defaults.
func LoadTheme(cfg config.Config) Theme {
	if cfg.Colors != nil && cfg.Colors.Text != "" {
		return hexTheme(*cfg.Colors)
	}
	switch cfg.Theme {
	case "rosepine":
		return hexTheme(rosePine)
	case "rosepine-dawn":
		return hexTheme(rosePineDawn)
	case "gruvbox":
		return hexTheme(gruvbox)
	case "catppuccin":
		return hexTheme(catppuccin)
	default: // "term"
		return TermTheme()
	}
}

// Styles bundles the lipgloss styles derived from a Theme.
type Styles struct {
	Theme Theme

	Title      lipgloss.Style // app title / headers
	Label      lipgloss.Style // subtle labels
	Hint       lipgloss.Style // muted hint text
	Text       lipgloss.Style
	Gold       lipgloss.Style // dollar values, scores
	Correct    lipgloss.Style // pine
	Wrong      lipgloss.Style // love
	You        lipgloss.Style // foam
	CategoryHd lipgloss.Style // iris category names

	CellAvail   lipgloss.Style
	CellCursor  lipgloss.Style
	CellUsed    lipgloss.Style
	CellNA      lipgloss.Style
	CatCell     lipgloss.Style
	Panel       lipgloss.Style // bordered panel (score panel etc.)
	Modal       lipgloss.Style // clue modal frame
	ClueText    lipgloss.Style // iris left border block
	StatusKey   lipgloss.Style
	StatusBar   lipgloss.Style
	TabActive   lipgloss.Style
	TabInactive lipgloss.Style
	SelectedRow lipgloss.Style
	DimBoard    lipgloss.Style // dimmed board behind modal
	InputPrompt lipgloss.Style
	BadgeCached lipgloss.Style
	BadgeRemote lipgloss.Style
}

// NewStyles builds all styles from a theme; nothing else in the package may
// hardcode a color.
func NewStyles(t Theme) Styles {
	s := Styles{Theme: t}
	s.Title = lipgloss.NewStyle().Foreground(t.Iris).Bold(true)
	s.Label = lipgloss.NewStyle().Foreground(t.Subtle)
	s.Hint = lipgloss.NewStyle().Foreground(t.Muted)
	s.Text = lipgloss.NewStyle().Foreground(t.Text)
	s.Gold = lipgloss.NewStyle().Foreground(t.Gold).Bold(true)
	s.Correct = lipgloss.NewStyle().Foreground(t.Pine).Bold(true)
	s.Wrong = lipgloss.NewStyle().Foreground(t.Love).Bold(true)
	s.You = lipgloss.NewStyle().Foreground(t.Foam).Bold(true)
	s.CategoryHd = lipgloss.NewStyle().Foreground(t.Iris).Bold(true)

	s.CellAvail = lipgloss.NewStyle().Foreground(t.Gold).Bold(true).Align(lipgloss.Center)
	s.CellCursor = lipgloss.NewStyle().Foreground(t.Foam).Background(t.HlMed).Bold(true).Align(lipgloss.Center)
	s.CellUsed = lipgloss.NewStyle().Foreground(t.Muted).Align(lipgloss.Center)
	s.CellNA = lipgloss.NewStyle().Foreground(t.Muted).Faint(true).Align(lipgloss.Center)
	s.CatCell = lipgloss.NewStyle().Foreground(t.Iris).Bold(true).Align(lipgloss.Center)
	s.Panel = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(t.Overlay).Padding(0, 1)
	s.Modal = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(t.Iris).Padding(1, 2)
	s.ClueText = lipgloss.NewStyle().Foreground(t.Text).
		Border(lipgloss.ThickBorder(), false, false, false, true).
		BorderForeground(t.Iris).PaddingLeft(1)
	s.StatusKey = lipgloss.NewStyle().Foreground(t.Gold)
	s.StatusBar = lipgloss.NewStyle().Foreground(t.Muted)
	s.TabActive = lipgloss.NewStyle().Foreground(t.Iris).Bold(true).Underline(true)
	s.TabInactive = lipgloss.NewStyle().Foreground(t.Muted)
	s.SelectedRow = lipgloss.NewStyle().Foreground(t.Foam).Background(t.HlMed).Bold(true)
	s.DimBoard = lipgloss.NewStyle().Foreground(t.Muted).Faint(true)
	s.InputPrompt = lipgloss.NewStyle().Foreground(t.Foam).Bold(true)
	s.BadgeCached = lipgloss.NewStyle().Foreground(t.Pine)
	s.BadgeRemote = lipgloss.NewStyle().Foreground(t.Muted)
	return s
}
