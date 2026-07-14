// Package eventlog is a SQLite-backed event log shared across FlowCast's
// Go pipeline and Python narrator, used to prove that both write into the
// same ordered log and can be replayed together (v2 scope amendment,
// CLAUDE.md). Uses modernc.org/sqlite (pure Go, no CGO) so builds and CI
// stay simple across platforms.
package eventlog

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Event is one row of the shared events table.
type Event struct {
	ID         int64
	Ts         string // RFC3339Nano
	SourceLang string // "go" or "python"
	Component  string
	EventType  string
	Sample     string // may be empty
	Payload    string // JSON text
}

const schema = `
CREATE TABLE IF NOT EXISTS events (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	ts          TEXT NOT NULL,
	source_lang TEXT NOT NULL,
	component   TEXT NOT NULL,
	event_type  TEXT NOT NULL,
	sample      TEXT,
	payload     TEXT NOT NULL
);
`

// DB wraps a shared event log SQLite file. Both the Go pipeline and the
// Python narrator open the same file with these same pragmas — WAL mode so
// a process opening the file after another has written to it doesn't hit a
// locked database, and a busy_timeout so a process that finds the file
// briefly locked waits instead of failing immediately. This has only been
// exercised with one writer at a time, sequentially (Go run, then Python
// run) — not simultaneous concurrent writers from both languages at once.
type DB struct {
	sql *sql.DB
}

// Open creates (if needed) and opens the shared event log at path.
func Open(path string) (*DB, error) {
	sqlDB, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, fmt.Errorf("eventlog: open %s: %w", path, err)
	}
	if _, err := sqlDB.Exec(schema); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("eventlog: create schema in %s: %w", path, err)
	}
	return &DB{sql: sqlDB}, nil
}

// Close closes the underlying SQLite connection.
func (d *DB) Close() error {
	return d.sql.Close()
}

// Emit records one event, source_lang "go", with the current time and
// payload marshaled to JSON.
func (d *DB) Emit(component, eventType, sample string, payload any) error {
	buf, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("eventlog: marshal payload for %s/%s: %w", component, eventType, err)
	}
	_, err = d.sql.Exec(
		`INSERT INTO events (ts, source_lang, component, event_type, sample, payload) VALUES (?, ?, ?, ?, ?, ?)`,
		time.Now().UTC().Format(time.RFC3339Nano), "go", component, eventType, sample, string(buf),
	)
	if err != nil {
		return fmt.Errorf("eventlog: insert %s/%s: %w", component, eventType, err)
	}
	return nil
}

// List returns all events in the log at path, ordered by timestamp, for
// replay. It does not require the caller to keep the DB open beforehand —
// List opens, reads, and closes the file itself.
func List(path string) ([]Event, error) {
	sqlDB, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, fmt.Errorf("eventlog: open %s: %w", path, err)
	}
	defer sqlDB.Close()
	if _, err := sqlDB.Exec(schema); err != nil {
		return nil, fmt.Errorf("eventlog: create schema in %s: %w", path, err)
	}

	rows, err := sqlDB.Query(`SELECT id, ts, source_lang, component, event_type, COALESCE(sample, ''), payload FROM events ORDER BY ts ASC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("eventlog: query %s: %w", path, err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.Ts, &e.SourceLang, &e.Component, &e.EventType, &e.Sample, &e.Payload); err != nil {
			return nil, fmt.Errorf("eventlog: scan row in %s: %w", path, err)
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("eventlog: read rows in %s: %w", path, err)
	}
	return events, nil
}
