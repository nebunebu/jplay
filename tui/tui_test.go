package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"jplay/config"
	"jplay/db"
	"jplay/game"
)

const fixture = "../jarch/testdata/7154.json"

func press(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	mm, _ := m.Update(msg)
	return mm.(Model)
}

func key(k tea.KeyType) tea.KeyMsg { return tea.KeyMsg{Type: k} }
func rune1(r string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(r)}
}

func testModel(t *testing.T) Model {
	t.Helper()
	if _, err := os.Stat(fixture); err != nil {
		t.Skip("jarch fixture not available")
	}
	dir := t.TempDir()
	cfg := config.Default()
	cfg.DBPath = filepath.Join(dir, "history.db")
	cfg.JarchCacheDir = dir

	m := New(cfg, Options{Mode: "play", GamePath: fixture})
	if m.db == nil {
		t.Fatalf("db not opened: %v", m.dbErr)
	}
	m = press(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m = press(t, m, m.Init()()) // gameLoadedMsg
	if m.screen != ScreenBoard {
		t.Fatalf("screen = %v after load", m.screen)
	}
	return m
}

// TestFullPlaythrough drives the entire game via Update messages: every
// board clue (answering correctly, passing, or missing), the Daily Doubles,
// Final Jeopardy, the end screen, and the SQLite record.
func TestFullPlaythrough(t *testing.T) {
	m := testModel(t)
	p := m.play

	i := 0
	for m.screen == ScreenBoard {
		if i++; i > 10000 {
			t.Fatal("play loop did not terminate")
		}
		switch p.phase {
		case phaseNone:
			m = press(t, m, key(tea.KeyEnter))
			if p.phase == phaseNone {
				t.Fatal("enter on board did not open a clue")
			}
		case phaseWager:
			p.wagerInput.SetValue("5")
			m = press(t, m, key(tea.KeyEnter))
			if p.phase != phaseAnswer {
				t.Fatalf("wager not accepted: %q", p.wagerErr)
			}
		case phaseAnswer:
			switch i % 3 {
			case 0: // exact correct
				p.input.SetValue(p.active.CorrectResponse)
				m = press(t, m, key(tea.KeyEnter))
			case 1: // pass
				m = press(t, m, key(tea.KeyEscape))
			default: // hopelessly wrong
				p.input.SetValue("zzz qqq xxx")
				m = press(t, m, key(tea.KeyEnter))
			}
		case phaseFuzzy:
			m = press(t, m, rune1("y"))
		case phaseReveal:
			if strings.Contains(m.View(), "panic") {
				t.Fatal("bad reveal view")
			}
			m = press(t, m, key(tea.KeySpace))
		default:
			t.Fatalf("unexpected phase %v", p.phase)
		}
		_ = m.View() // every state must render at every step
	}

	if m.screen != ScreenFinal {
		t.Fatalf("expected Final Jeopardy, got screen %v", m.screen)
	}
	total := p.counts[game.StateCorrect] + p.counts[game.StateIncorrect] + p.counts[game.StatePassed]
	if total != 60 {
		t.Errorf("board clues played = %d, want 60", total)
	}

	// Final Jeopardy: wager 0 (score may be negative), answer correctly.
	p.wagerInput.SetValue("0")
	m = press(t, m, key(tea.KeyEnter))
	if p.fj != fjAnswer {
		t.Fatalf("FJ wager rejected: %q", p.wagerErr)
	}
	p.input.SetValue(p.g.Final.CorrectResponse)
	m = press(t, m, key(tea.KeyEnter))
	if p.fj != fjReveal {
		t.Fatalf("FJ answer did not reveal, fj=%v", p.fj)
	}
	_ = m.View()
	m = press(t, m, key(tea.KeySpace))
	if m.screen != ScreenEnd {
		t.Fatalf("expected end screen, got %v", m.screen)
	}
	view := m.View()
	if !strings.Contains(view, "Final Score") || !strings.Contains(view, "MATT") {
		t.Errorf("end screen missing content:\n%s", view)
	}
	if !strings.Contains(view, "R/W") {
		t.Errorf("end screen missing contestant R/W counts:\n%s", view)
	}

	// Persisted exactly once, with 61 clue rows.
	recent, err := m.db.RecentGames(10)
	if err != nil || len(recent) != 1 {
		t.Fatalf("recent = %v, %v", recent, err)
	}
	if recent[0].GameID != "J#8478" {
		t.Errorf("saved game id = %q", recent[0].GameID)
	}
	if got := recent[0].Correct + recent[0].Incorrect + recent[0].Passed; got != 61 {
		t.Errorf("saved clue rows = %d, want 61", got)
	}
	if recent[0].Score != p.score {
		t.Errorf("saved score %d != play score %d", recent[0].Score, p.score)
	}

	// d → per-game drill-down from the end screen ('d' issues a batch of
	// reload + loadDetail commands; the test delivers both messages by hand).
	if p.savedRowID == 0 {
		t.Fatal("savedRowID not set after finishGame")
	}
	m = press(t, m, rune1("d"))
	if m.screen != ScreenStats {
		t.Fatalf("d did not open stats")
	}
	m = press(t, m, m.stats.reload(m.db)())
	m = press(t, m, m.stats.loadDetail(m.db, m.stats.recent[0])())
	if m.stats.detailGame == nil || len(m.stats.detailRows) != 61 {
		t.Fatalf("detail rows = %d, want 61", len(m.stats.detailRows))
	}
	dview := m.View()
	if !strings.Contains(dview, "J#8478") || !strings.Contains(dview, "Correct response") {
		t.Errorf("detail view missing content:\n%s", dview)
	}
	m = press(t, m, key(tea.KeyEscape)) // back to the tab view
	if m.stats.detailGame != nil {
		t.Fatal("esc did not close detail")
	}
	if !strings.Contains(m.View(), "J#8478") {
		t.Errorf("stats recent tab missing game")
	}
	for i := 0; i < 5; i++ {
		m = press(t, m, key(tea.KeyTab))
		_ = m.View()
	}
}

// TestResizeRendering renders board + modal at many sizes; layout must be
// computed from current size without panics.
func TestResizeRendering(t *testing.T) {
	m := testModel(t)
	m = press(t, m, key(tea.KeyEnter)) // open a clue modal

	for _, sz := range [][2]int{{120, 40}, {80, 24}, {60, 18}, {38, 12}, {24, 8}} {
		m = press(t, m, tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})
		out := m.View()
		if out == "" {
			t.Errorf("empty view at %dx%d", sz[0], sz[1])
		}
		for i, line := range strings.Split(out, "\n") {
			if w := visibleWidth(line); w > sz[0] {
				t.Errorf("%dx%d: line %d overflows (%d cells): %q", sz[0], sz[1], i, w, line)
			}
		}
	}
}

func visibleWidth(line string) int {
	return len([]rune(stripANSI(line))) // upper bound; fine for these glyphs
}

// TestDDWagerValidation exercises inline wager errors.
func TestDDWagerValidation(t *testing.T) {
	m := testModel(t)
	p := m.play
	// Navigate to the round-1 DD at row 2, col 1 (0-based).
	p.curRow, p.curCol = 2, 1
	if !p.selectable(2, 1) {
		t.Fatal("DD cell not selectable")
	}
	m = press(t, m, key(tea.KeyEnter))
	if p.phase != phaseWager {
		t.Fatalf("DD did not prompt for wager, phase=%v", p.phase)
	}
	for _, bad := range []string{"abc", "4", "999999"} {
		p.wagerInput.SetValue(bad)
		m = press(t, m, key(tea.KeyEnter))
		if p.phase != phaseWager || p.wagerErr == "" {
			t.Errorf("wager %q should be rejected inline", bad)
		}
	}
	p.wagerInput.SetValue("200")
	m = press(t, m, key(tea.KeyEnter))
	if p.phase != phaseAnswer || p.active.PlayerWager != 200 {
		t.Errorf("valid wager rejected: %q", p.wagerErr)
	}
	_ = m.View()
}

// TestRevealShowsClueTextAndStumper: the reveal modal repeats the clue text
// and flags triple stumpers.
func TestRevealShowsClueTextAndStumper(t *testing.T) {
	m := testModel(t)
	p := m.play

	// Find a triple stumper in round 1 (the fixture has several).
	found := false
	for c := 0; c < len(p.round().Categories) && !found; c++ {
		for r := 0; r < 5 && !found; r++ {
			if cl := p.clueAt(r, c); cl != nil && cl.TripleStumper && !cl.DailyDouble {
				p.curRow, p.curCol = r, c
				found = true
			}
		}
	}
	if !found {
		t.Skip("fixture has no round-1 triple stumper")
	}
	m = press(t, m, key(tea.KeyEnter))
	p.input.SetValue("zzz qqq")
	m = press(t, m, key(tea.KeyEnter))
	view := stripANSI(m.View())
	if !strings.Contains(view, "TRIPLE STUMPER") {
		t.Errorf("reveal missing triple stumper badge:\n%s", view)
	}
	if word := strings.Fields(p.active.Text)[0]; !strings.Contains(view, word) {
		t.Errorf("reveal missing clue text (looked for %q):\n%s", word, view)
	}
}

// TestCategoryRunAnnounced: answering all five clues of a column correctly
// shows the run banner on the final reveal.
func TestCategoryRunAnnounced(t *testing.T) {
	m := testModel(t)
	p := m.play
	for r := 0; r < 5; r++ {
		p.curRow, p.curCol = r, 0 // fixture round-1 col 0 has no DD
		m = press(t, m, key(tea.KeyEnter))
		if p.phase != phaseAnswer {
			t.Fatalf("clue (%d,0) phase = %v", r, p.phase)
		}
		p.input.SetValue(p.active.CorrectResponse)
		m = press(t, m, key(tea.KeyEnter))
		if p.phase == phaseFuzzy {
			m = press(t, m, rune1("y"))
		}
		if r < 4 {
			if p.revealNote != "" {
				t.Errorf("run announced early at row %d", r)
			}
			m = press(t, m, key(tea.KeySpace))
		}
	}
	if p.revealNote == "" {
		t.Fatal("category run not announced")
	}
	if !strings.Contains(stripANSI(m.View()), "ran the category") {
		t.Errorf("reveal missing run banner:\n%s", m.View())
	}
}

// TestDDPassForfeitsWager: a committed Daily Double wager is lost on a pass
// even with pass_penalty off.
func TestDDPassForfeitsWager(t *testing.T) {
	m := testModel(t)
	p := m.play
	if p.cfg.PassPenalty {
		t.Fatal("default config should have pass_penalty off")
	}
	p.curRow, p.curCol = 2, 1 // round-1 DD
	m = press(t, m, key(tea.KeyEnter))
	if p.phase != phaseWager {
		t.Fatalf("DD did not prompt for wager, phase=%v", p.phase)
	}
	p.wagerInput.SetValue("200")
	m = press(t, m, key(tea.KeyEnter))
	m = press(t, m, key(tea.KeyEscape)) // pass the DD
	if p.revealState != game.StatePassed {
		t.Fatalf("state = %v, want passed", p.revealState)
	}
	if p.score != -200 || p.revealDelta != -200 {
		t.Errorf("score=%d delta=%d, want -200/-200 (wager forfeited)", p.score, p.revealDelta)
	}
}

// TestQuitConfirmNoWrite: abandoning a game must not write to the DB.
func TestQuitConfirmNoWrite(t *testing.T) {
	m := testModel(t)
	m = press(t, m, rune1("q"))
	if m.play.phase != phaseQuitConfirm {
		t.Fatal("q did not prompt")
	}
	_ = m.View()
	m = press(t, m, rune1("y"))
	if m.screen != ScreenLanding {
		t.Fatal("quit did not return to landing")
	}
	recent, _ := m.db.RecentGames(10)
	if len(recent) != 0 {
		t.Errorf("abandoned game was written: %v", recent)
	}
}

// TestCursorSkipsUsedCells: movement lands only on selectable cells.
func TestCursorSkipsUsedCells(t *testing.T) {
	m := testModel(t)
	p := m.play
	// Play the clue under the cursor, then move right: cursor must skip it.
	startR, startC := p.curRow, p.curCol
	m = press(t, m, key(tea.KeyEnter))
	m = press(t, m, key(tea.KeyEscape)) // pass
	m = press(t, m, key(tea.KeySpace))  // dismiss reveal
	if p.selectable(startR, startC) {
		t.Fatal("played clue still selectable")
	}
	if p.curRow == startR && p.curCol == startC {
		t.Error("cursor left on used cell")
	}
	// Walk left across the whole row; must never land on the used cell.
	for i := 0; i < 12; i++ {
		m = press(t, m, rune1("h"))
		if p.curRow == startR && p.curCol == startC {
			t.Fatal("cursor landed on used cell")
		}
		if !p.selectable(p.curRow, p.curCol) {
			t.Fatal("cursor on unselectable cell")
		}
	}
}

// TestCursorMovesDownInCategory: after a clue, cursor defaults to the next
// tile down in the same category.
func TestCursorMovesDownInCategory(t *testing.T) {
	m := testModel(t)
	p := m.play
	startC := p.curCol
	m = press(t, m, key(tea.KeyEnter))
	m = press(t, m, key(tea.KeyEscape)) // pass
	m = press(t, m, key(tea.KeySpace))  // dismiss reveal
	if p.curCol != startC || p.curRow != 1 {
		t.Errorf("cursor = (%d,%d), want next down in same category (1,%d)", p.curRow, p.curCol, startC)
	}
}

// TestEmptySubmitAndSpacePass: empty Enter or Space prompts to pass.
func TestEmptySubmitAndSpacePass(t *testing.T) {
	m := testModel(t)
	p := m.play
	m = press(t, m, key(tea.KeyEnter)) // open clue
	m = press(t, m, key(tea.KeyEnter)) // empty submit
	if p.phase != phasePassConfirm {
		t.Fatalf("empty submit: phase = %v, want pass confirm", p.phase)
	}
	m = press(t, m, rune1("n"))
	if p.phase != phaseAnswer {
		t.Fatalf("n did not return to answering")
	}
	m = press(t, m, key(tea.KeySpace)) // space in empty input
	if p.phase != phasePassConfirm {
		t.Fatalf("space: phase = %v, want pass confirm", p.phase)
	}
	m = press(t, m, rune1("y"))
	if p.phase != phaseReveal || p.revealState != game.StatePassed {
		t.Fatalf("y did not pass: phase=%v state=%v", p.phase, p.revealState)
	}
	// Esc dismisses the reveal too.
	m = press(t, m, key(tea.KeyEscape))
	if p.phase != phaseNone {
		t.Fatalf("esc did not dismiss reveal")
	}
}

// TestManualOverride: `o` at the reveal flips a false negative to correct.
func TestManualOverride(t *testing.T) {
	m := testModel(t)
	p := m.play
	m = press(t, m, key(tea.KeyEnter))
	p.input.SetValue("totally wrong answer")
	m = press(t, m, key(tea.KeyEnter))
	if p.revealState != game.StateIncorrect {
		t.Fatalf("expected incorrect, got %v", p.revealState)
	}
	before := p.score
	value := p.active.Value
	m = press(t, m, rune1("o"))
	if p.revealState != game.StateCorrect || p.score != before+2*value {
		t.Errorf("override: state=%v score=%d (was %d)", p.revealState, p.score, before)
	}
	if p.results[len(p.results)-1].State != "correct" {
		t.Errorf("recorded state not updated")
	}
	m = press(t, m, rune1("o")) // idempotent: only incorrect can flip
	if p.score != before+2*value {
		t.Errorf("second o changed score again")
	}
}

func TestFJWagerHint(t *testing.T) {
	cases := []struct {
		you  int
		opp  []int
		want string
	}{
		{10000, []int{4000, 3000}, "Runaway: wager at most $1,999"},
		{10000, []int{6000}, "Leading: wager at least $2,001"},
		{5000, []int{5000}, "Tied for the lead"},
		{3000, []int{5000, 1000}, "Trailing the leader by $2,000"},
		{0, []int{5000}, ""},
	}
	for _, c := range cases {
		got := fjWagerHint(c.you, c.opp)
		if !strings.HasPrefix(got, c.want) || (c.want == "" && got != "") {
			t.Errorf("fjWagerHint(%d, %v) = %q, want prefix %q", c.you, c.opp, got, c.want)
		}
	}
}

func TestFJWagerShowsStandings(t *testing.T) {
	m := testModel(t)
	m.play.score = 5000
	for _, c := range m.play.g.Contestants {
		m.play.oppScores[c.Short] = 1000
	}
	m.play.startFinal()
	m.screen = ScreenFinal
	view := m.View()
	if !strings.Contains(view, "GOING INTO FINAL") || !strings.Contains(view, "Runaway") {
		t.Errorf("FJ wager screen missing standings/hint:\n%s", view)
	}
	for _, c := range m.play.g.Contestants {
		if !strings.Contains(view, strings.ToUpper(c.Short)) {
			t.Errorf("FJ wager screen missing contestant %s", c.Short)
		}
	}
}

func TestBrowserPlayedBadge(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.DBPath = filepath.Join(dir, "history.db")
	cfg.JarchCacheDir = dir
	m := New(cfg, Options{Mode: "play"})
	if m.db == nil {
		t.Fatal(m.dbErr)
	}
	if _, err := m.db.SaveGame(db.GameRecord{GameID: "J#1", ArchiveID: 111, PlayedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	m = press(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})
	m.screen = ScreenBrowser
	m.browser = newBrowserModel(m.db)
	m.browser.loading = ""
	m.browser.seasons = []seasonEntry{{Title: "Season 1", Href: "showseason.php?season=1"}}
	m.browser.episodes["1"] = []episodeEntry{
		{EpisodeNumber: 1, GameID: 111, Contestants: []string{"Ann"}},
		{EpisodeNumber: 2, GameID: 222, Contestants: []string{"Bob"}},
	}
	view := m.View()
	if !strings.Contains(view, "played") || !strings.Contains(view, "remote") {
		t.Errorf("expected one played and one remote badge:\n%s", view)
	}
}
