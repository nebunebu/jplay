package game

import (
	"encoding/json"
	"os"
	"testing"
)

const fixture = "../jarch/testdata/7154.json"

func loadFixture(t *testing.T) *Game {
	t.Helper()
	if _, err := os.Stat(fixture); err != nil {
		t.Skip("jarch fixture not available")
	}
	g, err := LoadFile(fixture)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	return g
}

func TestLoadFixture(t *testing.T) {
	g := loadFixture(t)

	if g.GameID != 7154 || g.ShowNumber != 8478 || g.AirDate != "2021-09-29" {
		t.Errorf("game identity = %d/%d/%s", g.GameID, g.ShowNumber, g.AirDate)
	}
	if g.Label() != "J#8478" {
		t.Errorf("Label() = %q", g.Label())
	}
	if len(g.Contestants) != 3 {
		t.Fatalf("contestants = %d", len(g.Contestants))
	}
	if g.Contestants[2].Short != "Matt" {
		t.Errorf("short name = %q", g.Contestants[2].Short)
	}
	if len(g.Rounds) != 2 {
		t.Fatalf("rounds = %d", len(g.Rounds))
	}
	r1 := g.Rounds[0]
	if len(r1.Categories) != 6 {
		t.Fatalf("round 1 categories = %d", len(r1.Categories))
	}
	if r1.RemainingClues() != 30 {
		t.Errorf("round 1 clues = %d", r1.RemainingClues())
	}
	// DD in round 1 at column 2, row 3 (1-based) = [1] cat, [2] row.
	dd := r1.Categories[1].Clues[2]
	if dd == nil || !dd.DailyDouble || dd.DDWager != 3800 {
		t.Errorf("DD clue = %+v", dd)
	}
	if g.Final == nil || g.Final.Category != "MYTHOLOGY" {
		t.Fatalf("final = %+v", g.Final)
	}
	if g.Final.FinalScores["Matt"] != 50600 {
		t.Errorf("final scores = %v", g.Final.FinalScores)
	}
	if got := ContestantDelta(dd, "Matt"); got != 3800 {
		t.Errorf("ContestantDelta(DD) = %d", got)
	}
}

// TestLoadNickname: jarch's explicit nickname wins over the first-name
// heuristic (which 7154.json, predating the field, exercises above).
func TestLoadNickname(t *testing.T) {
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Skip("fixture not available")
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	for _, c := range doc["contestants"].([]any) {
		c := c.(map[string]any)
		if c["name"] == "Matt Amodio" {
			c["name"] = "Matthew Amodio"
			c["nickname"] = "Matt"
		}
	}
	data, _ = json.Marshal(doc)
	g, err := Load(data)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range g.Contestants {
		if c.Name == "Matthew Amodio" {
			found = true
			if c.Short != "Matt" {
				t.Errorf("Short = %q, want Matt (from nickname)", c.Short)
			}
		}
	}
	if !found {
		t.Fatal("contestant not found")
	}
}
