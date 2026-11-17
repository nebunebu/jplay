package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSpecExample(t *testing.T) {
	src := `
gameplay {
    fuzzy_threshold 85
    strip_articles true
    strip_question_framing true
    pass_penalty false
}

display {
    theme "term"
    show_contestant_scores true
}

data {
    db_path "~/.local/share/jplay/history.db"
    jarch_cache_dir "~/.cache/jarch/"
    export_format "csv"
}

colors {
    // Omit this section entirely to use terminal default colors.
    base    "#191724"
    surface "#1f1d2e"
    overlay "#26233a"
    muted   "#6e6a86"
    subtle  "#908caa"
    text    "#e0def4"
    love    "#eb6f92"
    gold    "#f6c177"
    pine    "#31748f"
    foam    "#9ccfd8"
    iris    "#c4a7e7"
    hlmed   "#403d52"
    hllow   "#21202e"
}
`
	path := filepath.Join(t.TempDir(), "config.kdl")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if cfg.FuzzyThreshold != 85 || !cfg.StripArticles || cfg.PassPenalty {
		t.Errorf("gameplay = %+v", cfg)
	}
	if cfg.Theme != "term" || !cfg.ShowContestantScores {
		t.Errorf("display = %+v", cfg)
	}
	if cfg.ExportFormat != "csv" {
		t.Errorf("export_format = %q", cfg.ExportFormat)
	}
	if cfg.Colors == nil || cfg.Colors.Gold != "#f6c177" || cfg.Colors.HlLow != "#21202e" {
		t.Errorf("colors = %+v", cfg.Colors)
	}
}

func TestMissingFileIsDefaults(t *testing.T) {
	cfg, err := LoadFrom(filepath.Join(t.TempDir(), "nope.kdl"))
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if cfg != Default() {
		t.Errorf("cfg = %+v", cfg)
	}
	if cfg.Theme != "term" {
		t.Errorf("default theme = %q", cfg.Theme)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	cfg := Default()
	cfg.FuzzyThreshold = 70
	cfg.Theme = "rosepine"
	cfg.PassPenalty = true
	cfg.Colors = &Colors{Base: "#000000", Text: "#ffffff"}

	path := filepath.Join(t.TempDir(), "sub", "config.kdl")
	if err := SaveTo(cfg, path); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}
	got, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if got.FuzzyThreshold != 70 || got.Theme != "rosepine" || !got.PassPenalty {
		t.Errorf("round trip = %+v", got)
	}
	if got.Colors == nil || got.Colors.Base != "#000000" {
		t.Errorf("colors round trip = %+v", got.Colors)
	}
}

func TestParseErrors(t *testing.T) {
	for _, src := range []string{"gameplay {", "}", `display { theme "unterminated }`} {
		if _, err := parseKDL(src); err == nil {
			t.Errorf("parseKDL(%q) should fail", src)
		}
	}
}
