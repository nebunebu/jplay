// Package config loads and saves jplay's KDL configuration.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/adrg/xdg"
)

// Colors is an optional fully-specified 13-role palette from the colors {}
// block. Nil ColorsPresent means terminal-default colors.
type Colors struct {
	Base, Surface, Overlay string
	Muted, Subtle, Text    string
	Love, Gold, Pine       string
	Foam, Iris             string
	HlMed, HlLow           string
}

// Config holds every setting from SPEC.md's config table.
type Config struct {
	// gameplay
	FuzzyThreshold       int
	StripArticles        bool
	StripQuestionFraming bool
	PassPenalty          bool
	// display
	Theme                string
	ShowContestantScores bool
	// data
	DBPath        string
	JarchCacheDir string
	ExportFormat  string
	// colors (optional block)
	Colors *Colors
}

// Default returns the spec defaults. Theme default is "term" per the visual
// design section (the config-table "rosepine" row is overridden there).
func Default() Config {
	return Config{
		FuzzyThreshold:       85,
		StripArticles:        true,
		StripQuestionFraming: true,
		PassPenalty:          false,
		Theme:                "term",
		ShowContestantScores: true,
		DBPath:               "~/.local/share/jplay/history.db",
		JarchCacheDir:        "~/.cache/jarch/",
		ExportFormat:         "csv",
	}
}

// Path returns the config file location, honoring $XDG_CONFIG_HOME.
func Path() string {
	return filepath.Join(xdg.ConfigHome, "jplay", "config.kdl")
}

// ExpandPath resolves a leading ~ and environment variables.
func ExpandPath(p string) string {
	p = os.ExpandEnv(p)
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			p = filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}
	return p
}

// ResolvedDBPath is DBPath with XDG defaults applied and ~ expanded.
func (c Config) ResolvedDBPath() string {
	if c.DBPath == Default().DBPath {
		return filepath.Join(xdg.DataHome, "jplay", "history.db")
	}
	return ExpandPath(c.DBPath)
}

// ResolvedCacheDir is JarchCacheDir with ~ expanded.
func (c Config) ResolvedCacheDir() string {
	if c.JarchCacheDir == Default().JarchCacheDir {
		return filepath.Join(xdg.CacheHome, "jarch")
	}
	return ExpandPath(c.JarchCacheDir)
}

// Load reads Path(); a missing file yields pure defaults (and is not created).
func Load() (Config, error) {
	return LoadFrom(Path())
}

// LoadFrom reads a config file; missing file returns defaults without error.
func LoadFrom(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	nodes, err := parseKDL(string(data))
	if err != nil {
		return cfg, fmt.Errorf("%s: %w", path, err)
	}
	doc := kdlNode{Children: nodes}

	if g := doc.findChild("gameplay"); g != nil {
		cfg.FuzzyThreshold = g.integer("fuzzy_threshold", cfg.FuzzyThreshold)
		cfg.StripArticles = g.boolean("strip_articles", cfg.StripArticles)
		cfg.StripQuestionFraming = g.boolean("strip_question_framing", cfg.StripQuestionFraming)
		cfg.PassPenalty = g.boolean("pass_penalty", cfg.PassPenalty)
	}
	if d := doc.findChild("display"); d != nil {
		cfg.Theme = d.str("theme", cfg.Theme)
		cfg.ShowContestantScores = d.boolean("show_contestant_scores", cfg.ShowContestantScores)
	}
	if d := doc.findChild("data"); d != nil {
		cfg.DBPath = d.str("db_path", cfg.DBPath)
		cfg.JarchCacheDir = d.str("jarch_cache_dir", cfg.JarchCacheDir)
		cfg.ExportFormat = d.str("export_format", cfg.ExportFormat)
	}
	if c := doc.findChild("colors"); c != nil {
		cfg.Colors = &Colors{
			Base:    c.str("base", ""),
			Surface: c.str("surface", ""),
			Overlay: c.str("overlay", ""),
			Muted:   c.str("muted", ""),
			Subtle:  c.str("subtle", ""),
			Text:    c.str("text", ""),
			Love:    c.str("love", ""),
			Gold:    c.str("gold", ""),
			Pine:    c.str("pine", ""),
			Foam:    c.str("foam", ""),
			Iris:    c.str("iris", ""),
			HlMed:   c.str("hlmed", ""),
			HlLow:   c.str("hllow", ""),
		}
	}
	if cfg.FuzzyThreshold < 0 {
		cfg.FuzzyThreshold = 0
	}
	if cfg.FuzzyThreshold > 100 {
		cfg.FuzzyThreshold = 100
	}
	return cfg, nil
}

// Save writes the config to Path(), creating parent directories.
func Save(cfg Config) error {
	return SaveTo(cfg, Path())
}

// SaveTo serializes the config as KDL to the given path.
func SaveTo(cfg Config, path string) error {
	var b strings.Builder
	line := func(indent, name string, v any) {
		fmt.Fprintf(&b, "%s%s %s\n", indent, name, kdlValue(v))
	}
	b.WriteString("gameplay {\n")
	line("    ", "fuzzy_threshold", cfg.FuzzyThreshold)
	line("    ", "strip_articles", cfg.StripArticles)
	line("    ", "strip_question_framing", cfg.StripQuestionFraming)
	line("    ", "pass_penalty", cfg.PassPenalty)
	b.WriteString("}\n\ndisplay {\n")
	line("    ", "theme", cfg.Theme)
	line("    ", "show_contestant_scores", cfg.ShowContestantScores)
	b.WriteString("}\n\ndata {\n")
	line("    ", "db_path", cfg.DBPath)
	line("    ", "jarch_cache_dir", cfg.JarchCacheDir)
	line("    ", "export_format", cfg.ExportFormat)
	b.WriteString("}\n")
	if c := cfg.Colors; c != nil {
		b.WriteString("\ncolors {\n")
		for _, kv := range [][2]string{
			{"base", c.Base}, {"surface", c.Surface}, {"overlay", c.Overlay},
			{"muted", c.Muted}, {"subtle", c.Subtle}, {"text", c.Text},
			{"love", c.Love}, {"gold", c.Gold}, {"pine", c.Pine},
			{"foam", c.Foam}, {"iris", c.Iris},
			{"hlmed", c.HlMed}, {"hllow", c.HlLow},
		} {
			line("    ", kv[0], kv[1])
		}
		b.WriteString("}\n")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}
