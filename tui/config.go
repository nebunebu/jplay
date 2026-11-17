package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"jplay/config"
)

var themes = []string{"rosepine", "rosepine-dawn", "gruvbox", "catppuccin", "term"}
var exportFormats = []string{"csv", "json", "tsv"}

// rowKind describes how a settings row is edited.
type rowKind int

const (
	rowSection rowKind = iota
	rowInt             // ←/→ adjust
	rowBool            // Space toggle
	rowEnum            // ←/→ cycle
	rowPath            // Enter edit
)

type confRow struct {
	kind rowKind
	name string
	hint string
	get  func(*config.Config) string
	adj  func(*config.Config, int) // rowInt/rowEnum: delta -1/+1
	tog  func(*config.Config)      // rowBool
	set  func(*config.Config, string)
}

type configModel struct {
	cfg     config.Config // working copy; saved only on w
	rows    []confRow
	cursor  int
	editing bool
	input   textinput.Model
}

func cycle(list []string, cur string, d int) string {
	for i, v := range list {
		if v == cur {
			return list[(i+d+len(list))%len(list)]
		}
	}
	return list[0]
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

func newConfigModel(cfg config.Config) configModel {
	in := textinput.New()
	in.Prompt = "> "
	in.CharLimit = 400

	rows := []confRow{
		{kind: rowSection, name: "Gameplay"},
		{kind: rowInt, name: "fuzzy_threshold", hint: "←→ adjust · 0–100",
			get: func(c *config.Config) string { return fmt.Sprint(c.FuzzyThreshold) },
			adj: func(c *config.Config, d int) {
				c.FuzzyThreshold += d * 5
				if c.FuzzyThreshold < 0 {
					c.FuzzyThreshold = 0
				}
				if c.FuzzyThreshold > 100 {
					c.FuzzyThreshold = 100
				}
			}},
		{kind: rowBool, name: "strip_articles", hint: "Space toggle",
			get: func(c *config.Config) string { return onOff(c.StripArticles) },
			tog: func(c *config.Config) { c.StripArticles = !c.StripArticles }},
		{kind: rowBool, name: "strip_question_framing", hint: "Space toggle",
			get: func(c *config.Config) string { return onOff(c.StripQuestionFraming) },
			tog: func(c *config.Config) { c.StripQuestionFraming = !c.StripQuestionFraming }},
		{kind: rowBool, name: "pass_penalty", hint: "Space toggle",
			get: func(c *config.Config) string { return onOff(c.PassPenalty) },
			tog: func(c *config.Config) { c.PassPenalty = !c.PassPenalty }},

		{kind: rowSection, name: "Display"},
		{kind: rowEnum, name: "theme", hint: "←→ cycle: " + strings.Join(themes, " · "),
			get: func(c *config.Config) string { return c.Theme },
			adj: func(c *config.Config, d int) { c.Theme = cycle(themes, c.Theme, d) }},
		{kind: rowBool, name: "show_contestant_scores", hint: "Space toggle",
			get: func(c *config.Config) string { return onOff(c.ShowContestantScores) },
			tog: func(c *config.Config) { c.ShowContestantScores = !c.ShowContestantScores }},

		{kind: rowSection, name: "Data"},
		{kind: rowPath, name: "db_path", hint: "Enter edit",
			get: func(c *config.Config) string { return c.DBPath },
			set: func(c *config.Config, v string) { c.DBPath = v }},
		{kind: rowPath, name: "jarch_cache_dir", hint: "Enter edit",
			get: func(c *config.Config) string { return c.JarchCacheDir },
			set: func(c *config.Config, v string) { c.JarchCacheDir = v }},
		{kind: rowEnum, name: "export_format", hint: "←→ cycle: " + strings.Join(exportFormats, " · "),
			get: func(c *config.Config) string { return c.ExportFormat },
			adj: func(c *config.Config, d int) { c.ExportFormat = cycle(exportFormats, c.ExportFormat, d) }},
	}
	m := configModel{cfg: cfg, rows: rows, input: in}
	m.cursor = 1 // first editable row
	return m
}

func (c *configModel) moveCursor(d int) {
	i := c.cursor
	for {
		i += d
		if i < 0 || i >= len(c.rows) {
			return
		}
		if c.rows[i].kind != rowSection {
			c.cursor = i
			return
		}
	}
}

func (m Model) updateConfig(msg tea.Msg) (tea.Model, tea.Cmd) {
	c := &m.confSc
	key, isKey := msg.(tea.KeyMsg)
	if !isKey {
		return m, nil
	}
	row := &c.rows[c.cursor]

	if c.editing {
		switch key.String() {
		case "enter":
			row.set(&c.cfg, strings.TrimSpace(c.input.Value()))
			c.editing = false
		case "esc":
			c.editing = false
		default:
			var cmd tea.Cmd
			c.input, cmd = c.input.Update(msg)
			return m, cmd
		}
		return m, nil
	}

	switch {
	case keyUp(key):
		c.moveCursor(-1)
	case keyDown(key):
		c.moveCursor(1)
	case keyLeft(key):
		if row.adj != nil {
			row.adj(&c.cfg, -1)
		}
	case keyRight(key):
		if row.adj != nil {
			row.adj(&c.cfg, 1)
		}
	case key.String() == " ":
		if row.tog != nil {
			row.tog(&c.cfg)
		}
	case key.String() == "enter":
		if row.kind == rowPath {
			c.editing = true
			c.input.SetValue(row.get(&c.cfg))
			c.input.Focus()
		}
	case key.String() == "w":
		if err := config.Save(c.cfg); err != nil {
			m.flash = "save failed: " + err.Error()
			return m, nil
		}
		return m, tea.Quit
	case key.String() == "q", key.String() == "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) viewConfig() string {
	c := m.confSc
	var b strings.Builder
	b.WriteString(m.styles.Title.Render("jplay --config") + "\n")

	nameW := 24
	valW := m.width / 3
	if valW > 36 {
		valW = 36
	}
	for i, row := range c.rows {
		if row.kind == rowSection {
			b.WriteString("\n" + m.styles.CategoryHd.Render(row.name) + "\n")
			continue
		}
		val := row.get(&c.cfg)
		line := pad(row.name, nameW) + pad(truncate(val, valW), valW+2)
		if i == c.cursor {
			if c.editing {
				c.input.Width = m.width - nameW - 8
				b.WriteString(m.styles.SelectedRow.Render(pad(row.name, nameW)) + " " + c.input.View() + "\n")
			} else {
				b.WriteString(m.styles.SelectedRow.Render(line) + m.styles.Hint.Render("  "+row.hint) + "\n")
			}
		} else {
			b.WriteString(m.styles.Text.Render(pad(row.name, nameW)) + m.styles.Gold.Render(pad(truncate(val, valW), valW+2)) + "\n")
		}
	}

	var status string
	if c.editing {
		status = m.statusBar("Enter", "apply", "Esc", "cancel")
	} else {
		status = m.statusBar("↑↓", "navigate", "←→", "adjust", "Space", "toggle",
			"Enter", "edit", "w", "save", "q", "discard")
	}
	return m.page(b.String(), status)
}
