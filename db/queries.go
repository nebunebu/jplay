package db

import (
	"database/sql"
	"strconv"
	"time"
)

// ClueResult is one row destined for clue_results.
type ClueResult struct {
	ClueID          string
	Category        string
	Value           int
	ClueText        string
	CorrectResponse string
	State           string // correct | incorrect | passed
	PlayerAnswer    string
	PlayerWager     *int
	IsDailyDouble   bool
	IsFinal         bool
}

// ContestantScore is a historical contestant's final score.
type ContestantScore struct {
	Name  string
	Score int
}

// GameRecord is a completed play-through, written in one transaction.
type GameRecord struct {
	GameID      string
	PlayedAt    time.Time
	PlayerScore int
	Contestants []ContestantScore
	Clues       []ClueResult
}

// SaveGame writes a finished game atomically and returns its games row id.
// Nothing is written for games abandoned before the end screen (the caller
// simply never calls this).
func (d *DB) SaveGame(rec GameRecord) (int64, error) {
	tx, err := d.sql.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	res, err := tx.Exec(
		`INSERT INTO games (game_id, played_at, player_score) VALUES (?, ?, ?)`,
		rec.GameID, rec.PlayedAt.UTC().Format(time.RFC3339), rec.PlayerScore)
	if err != nil {
		return 0, err
	}
	gameRowID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	for _, c := range rec.Contestants {
		if _, err := tx.Exec(
			`INSERT INTO contestants (game_row_id, name, final_score) VALUES (?, ?, ?)`,
			gameRowID, c.Name, c.Score); err != nil {
			return 0, err
		}
	}
	for _, cl := range rec.Clues {
		var wager any
		if cl.PlayerWager != nil {
			wager = *cl.PlayerWager
		}
		var answer any
		if cl.PlayerAnswer != "" {
			answer = cl.PlayerAnswer
		}
		if _, err := tx.Exec(
			`INSERT INTO clue_results
			   (game_row_id, clue_id, category, value, clue_text, correct_response,
			    state, player_answer, player_wager, is_daily_double, is_final)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			gameRowID, cl.ClueID, cl.Category, cl.Value, cl.ClueText,
			cl.CorrectResponse, cl.State, answer, wager,
			boolInt(cl.IsDailyDouble), boolInt(cl.IsFinal)); err != nil {
			return 0, err
		}
	}
	return gameRowID, tx.Commit()
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// RecentGame is one row of the Recent tab / landing list.
type RecentGame struct {
	RowID     int64 // games.id, for drill-down queries
	GameID    string
	PlayedAt  string // date portion
	Score     int
	Correct   int
	Incorrect int
	Passed    int
	Position  int // 1-based finish among player + contestants (ties share a rank)
	Players   int // field size including the player
}

// Finish renders the finish position, e.g. "2nd of 4"; empty when the game
// has no recorded contestants to rank against.
func (r RecentGame) Finish() string {
	if r.Players < 2 {
		return ""
	}
	return ordinal(r.Position) + " of " + strconv.Itoa(r.Players)
}

// RecentGames returns the most recent plays, newest first.
func (d *DB) RecentGames(limit int) ([]RecentGame, error) {
	rows, err := d.sql.Query(`
		SELECT g.id, g.game_id, substr(g.played_at, 1, 10), g.player_score,
		       COALESCE(SUM(c.state = 'correct'), 0),
		       COALESCE(SUM(c.state = 'incorrect'), 0),
		       COALESCE(SUM(c.state = 'passed'), 0),
		       (SELECT COUNT(*) + 1 FROM contestants k
		         WHERE k.game_row_id = g.id AND k.final_score > g.player_score),
		       (SELECT COUNT(*) + 1 FROM contestants k WHERE k.game_row_id = g.id)
		  FROM games g
		  LEFT JOIN clue_results c ON c.game_row_id = g.id
		 GROUP BY g.id
		 ORDER BY g.played_at DESC, g.id DESC
		 LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RecentGame
	for rows.Next() {
		var r RecentGame
		if err := rows.Scan(&r.RowID, &r.GameID, &r.PlayedAt, &r.Score,
			&r.Correct, &r.Incorrect, &r.Passed, &r.Position, &r.Players); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GameClueRow is one clue of a saved game, in play order.
type GameClueRow struct {
	Category        string
	Value           int
	ClueText        string
	CorrectResponse string
	State           string
	PlayerAnswer    string
	PlayerWager     *int
	IsDailyDouble   bool
	IsFinal         bool
}

// GameClues returns every clue result of one saved game, in play order.
func (d *DB) GameClues(gameRowID int64) ([]GameClueRow, error) {
	rows, err := d.sql.Query(`
		SELECT category, value, clue_text, correct_response, state,
		       COALESCE(player_answer, ''), player_wager, is_daily_double, is_final
		  FROM clue_results
		 WHERE game_row_id = ?
		 ORDER BY id`, gameRowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GameClueRow
	for rows.Next() {
		var r GameClueRow
		var wager sql.NullInt64
		var dd, fin int
		if err := rows.Scan(&r.Category, &r.Value, &r.ClueText, &r.CorrectResponse,
			&r.State, &r.PlayerAnswer, &wager, &dd, &fin); err != nil {
			return nil, err
		}
		if wager.Valid {
			w := int(wager.Int64)
			r.PlayerWager = &w
		}
		r.IsDailyDouble, r.IsFinal = dd != 0, fin != 0
		out = append(out, r)
	}
	return out, rows.Err()
}

// ordinal renders 1 → "1st", 2 → "2nd", 11 → "11th", …
func ordinal(n int) string {
	suffix := "th"
	if n%100 < 11 || n%100 > 13 {
		switch n % 10 {
		case 1:
			suffix = "st"
		case 2:
			suffix = "nd"
		case 3:
			suffix = "rd"
		}
	}
	return strconv.Itoa(n) + suffix
}

// AccuracyRow is a Seen/Correct aggregate for a category or value bucket.
type AccuracyRow struct {
	Key     string
	Seen    int
	Correct int
}

// Accuracy returns Correct/Seen as a percentage (0 when unseen).
func (a AccuracyRow) Accuracy() int {
	if a.Seen == 0 {
		return 0
	}
	return int(float64(a.Correct)/float64(a.Seen)*100 + 0.5)
}

// ByCategory aggregates accuracy per category, most-seen first.
func (d *DB) ByCategory() ([]AccuracyRow, error) {
	return d.accuracyQuery(`
		SELECT category, COUNT(*), COALESCE(SUM(state = 'correct'), 0)
		  FROM clue_results
		 GROUP BY category
		 ORDER BY COUNT(*) DESC, category`)
}

// ByValue aggregates accuracy per dollar tier, ascending (Final last).
func (d *DB) ByValue() ([]AccuracyRow, error) {
	return d.accuracyQuery(`
		SELECT CASE WHEN is_final THEN 'Final'
		            ELSE '$' || CAST(value AS TEXT) END AS bucket,
		       COUNT(*), COALESCE(SUM(state = 'correct'), 0)
		  FROM clue_results
		 GROUP BY bucket
		 ORDER BY MAX(is_final), MIN(value)`)
}

func (d *DB) accuracyQuery(query string) ([]AccuracyRow, error) {
	rows, err := d.sql.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AccuracyRow
	for rows.Next() {
		var r AccuracyRow
		if err := rows.Scan(&r.Key, &r.Seen, &r.Correct); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// MissRow is one clue the player got wrong or passed.
type MissRow struct {
	ClueText        string
	CorrectResponse string
	Category        string
	Value           int
	PlayerAnswer    string
	PlayedAt        string
}

// CluesByState lists clue records with the given state, newest first.
func (d *DB) CluesByState(state string) ([]MissRow, error) {
	rows, err := d.sql.Query(`
		SELECT c.clue_text, c.correct_response, c.category, c.value,
		       COALESCE(c.player_answer, ''), substr(g.played_at, 1, 10)
		  FROM clue_results c
		  JOIN games g ON g.id = c.game_row_id
		 WHERE c.state = ?
		 ORDER BY g.played_at DESC, c.id DESC`, state)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MissRow
	for rows.Next() {
		var r MissRow
		if err := rows.Scan(&r.ClueText, &r.CorrectResponse, &r.Category, &r.Value, &r.PlayerAnswer, &r.PlayedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
