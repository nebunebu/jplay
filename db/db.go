// Package db persists play history to SQLite.
package db

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS games (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  game_id      TEXT    NOT NULL,   -- display label, e.g. "J#1834"
  archive_id   INTEGER,            -- J! Archive game_id; NULL for rows saved before v0.2
  played_at    TEXT    NOT NULL,   -- ISO 8601 UTC
  player_score INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS contestants (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  game_row_id INTEGER NOT NULL REFERENCES games(id),
  name        TEXT    NOT NULL,
  final_score INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS clue_results (
  id               INTEGER PRIMARY KEY AUTOINCREMENT,
  game_row_id      INTEGER NOT NULL REFERENCES games(id),
  clue_id          TEXT    NOT NULL,
  category         TEXT    NOT NULL,
  value            INTEGER NOT NULL,
  clue_text        TEXT    NOT NULL,
  correct_response TEXT    NOT NULL,
  state            TEXT    NOT NULL CHECK(state IN ('correct','incorrect','passed')),
  player_answer    TEXT,
  player_wager     INTEGER,
  is_daily_double  INTEGER NOT NULL DEFAULT 0,
  is_final         INTEGER NOT NULL DEFAULT 0
);
`

// DB wraps the history database.
type DB struct {
	sql *sql.DB
}

// Open opens (creating directories and schema as needed) the history DB.
func Open(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := conn.Exec(schema); err != nil {
		conn.Close()
		return nil, err
	}
	if err := migrate(conn); err != nil {
		conn.Close()
		return nil, err
	}
	return &DB{sql: conn}, nil
}

// Close closes the database.
func (d *DB) Close() error { return d.sql.Close() }

// migrate upgrades databases created by older versions in place.
func migrate(conn *sql.DB) error {
	// v0.2: games.archive_id (J! Archive game_id) for exact "played" lookup.
	var n int
	if err := conn.QueryRow(
		`SELECT COUNT(*) FROM pragma_table_info('games') WHERE name = 'archive_id'`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		if _, err := conn.Exec(`ALTER TABLE games ADD COLUMN archive_id INTEGER`); err != nil {
			return err
		}
	}
	return nil
}
