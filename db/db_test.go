package db

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

func openTest(t *testing.T) *DB {
	t.Helper()
	d, err := Open(filepath.Join(t.TempDir(), "sub", "history.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestSaveAndQuery(t *testing.T) {
	d := openTest(t)
	wager := 1500
	rec := GameRecord{
		GameID:      "J#8478",
		PlayedAt:    time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC),
		PlayerScore: 8200,
		Contestants: []ContestantScore{{"Matt Amodio", 50600}, {"Daniel Pecoraro", 5378}},
		Clues: []ClueResult{
			{ClueID: "1", Category: "CLASSIC LIT", Value: 600, ClueText: "novella...",
				CorrectResponse: "The Old Man and the Sea", State: "correct",
				PlayerAnswer: "the old man and the sea"},
			{ClueID: "2", Category: "CLASSIC LIT", Value: 800, ClueText: "x",
				CorrectResponse: "y", State: "incorrect", PlayerAnswer: "z"},
			{ClueID: "3", Category: "THE 90s", Value: 200, ClueText: "x",
				CorrectResponse: "y", State: "passed"},
			{ClueID: "4", Category: "STELLAR SCIENCE", Value: 800, ClueText: "dd",
				CorrectResponse: "y", State: "correct", PlayerWager: &wager, IsDailyDouble: true},
			{ClueID: "5", Category: "MYTHOLOGY", Value: 0, ClueText: "fj",
				CorrectResponse: "Pegasus", State: "incorrect", PlayerAnswer: "centaur",
				PlayerWager: &wager, IsFinal: true},
		},
	}
	rowID, err := d.SaveGame(rec)
	if err != nil {
		t.Fatalf("SaveGame: %v", err)
	}
	if rowID == 0 {
		t.Fatal("SaveGame returned zero row id")
	}

	recent, err := d.RecentGames(10)
	if err != nil {
		t.Fatalf("RecentGames: %v", err)
	}
	if len(recent) != 1 {
		t.Fatalf("recent = %d rows", len(recent))
	}
	r := recent[0]
	if r.GameID != "J#8478" || r.PlayedAt != "2026-07-03" || r.Score != 8200 ||
		r.Correct != 2 || r.Incorrect != 2 || r.Passed != 1 {
		t.Errorf("recent row = %+v", r)
	}
	// 8200 trails Matt (50600) but beats Daniel (5378): 2nd of 3.
	if r.RowID != rowID || r.Position != 2 || r.Players != 3 || r.Finish() != "2nd of 3" {
		t.Errorf("finish = %q (row %+v)", r.Finish(), r)
	}

	clues, err := d.GameClues(rowID)
	if err != nil {
		t.Fatalf("GameClues: %v", err)
	}
	if len(clues) != 5 {
		t.Fatalf("game clues = %d rows", len(clues))
	}
	if clues[0].Category != "CLASSIC LIT" || clues[0].State != "correct" || clues[0].PlayerWager != nil {
		t.Errorf("clue[0] = %+v", clues[0])
	}
	if !clues[3].IsDailyDouble || clues[3].PlayerWager == nil || *clues[3].PlayerWager != 1500 {
		t.Errorf("clue[3] = %+v", clues[3])
	}
	if !clues[4].IsFinal || clues[4].PlayerAnswer != "centaur" {
		t.Errorf("clue[4] = %+v", clues[4])
	}

	cats, err := d.ByCategory()
	if err != nil {
		t.Fatalf("ByCategory: %v", err)
	}
	if len(cats) != 4 || cats[0].Key != "CLASSIC LIT" || cats[0].Seen != 2 || cats[0].Correct != 1 {
		t.Errorf("by category = %+v", cats)
	}
	if cats[0].Accuracy() != 50 {
		t.Errorf("accuracy = %d", cats[0].Accuracy())
	}

	vals, err := d.ByValue()
	if err != nil {
		t.Fatalf("ByValue: %v", err)
	}
	if len(vals) != 4 || vals[0].Key != "$200" || vals[len(vals)-1].Key != "Final" {
		t.Errorf("by value = %+v", vals)
	}

	missed, err := d.CluesByState("incorrect")
	if err != nil {
		t.Fatalf("CluesByState: %v", err)
	}
	if len(missed) != 2 {
		t.Errorf("missed = %+v", missed)
	}
	passed, _ := d.CluesByState("passed")
	if len(passed) != 1 || passed[0].PlayerAnswer != "" {
		t.Errorf("passed = %+v", passed)
	}
}

func TestBadStateRejected(t *testing.T) {
	d := openTest(t)
	_, err := d.SaveGame(GameRecord{
		GameID: "x", PlayedAt: time.Now(),
		Clues: []ClueResult{{ClueID: "1", State: "bogus"}},
	})
	if err == nil {
		t.Error("CHECK constraint should reject bogus state")
	}
	// The transaction must have rolled back entirely.
	recent, _ := d.RecentGames(10)
	if len(recent) != 0 {
		t.Errorf("partial write leaked: %+v", recent)
	}
}

func TestPlayedAndMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")

	// A pre-v0.2 database: games table without archive_id, one legacy row.
	old, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := old.Exec(`CREATE TABLE games (id INTEGER PRIMARY KEY AUTOINCREMENT,
		game_id TEXT NOT NULL, played_at TEXT NOT NULL, player_score INTEGER NOT NULL);
		INSERT INTO games (game_id, played_at, player_score) VALUES ('J#1834', '2026-07-01T00:00:00Z', 100);`); err != nil {
		t.Fatal(err)
	}
	old.Close()

	d, err := Open(path) // migrates
	if err != nil {
		t.Fatalf("Open (migrate): %v", err)
	}
	defer d.Close()
	if _, err := d.SaveGame(GameRecord{GameID: "J#9000", ArchiveID: 7154, PlayedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	ids, labels, err := d.Played()
	if err != nil {
		t.Fatal(err)
	}
	if !ids[7154] || len(ids) != 1 {
		t.Errorf("ids = %v, want {7154}", ids)
	}
	if !labels["J#1834"] || labels["J#9000"] {
		t.Errorf("labels = %v, want only legacy J#1834", labels)
	}

	// Re-opening an already-migrated DB is a no-op.
	d2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	d2.Close()
}
