// Package store opens and migrates the SQLite database and stores resolved targets and their
// aliases.
package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// TimeLayout is the fixed-width UTC layout of every stored timestamp. Fixed width makes text
// comparison in SQL chronological, which the expiry queries rely on.
const TimeLayout = "2006-01-02T15:04:05.000000000Z"

// FormatTime renders t in UTC with TimeLayout.
func FormatTime(t time.Time) string { return t.UTC().Format(TimeLayout) }

// ParseTime parses a timestamp written by FormatTime.
func ParseTime(s string) (time.Time, error) { return time.Parse(TimeLayout, s) }

// NullTime returns FormatTime(t) as a valid sql.NullString.
func NullTime(t time.Time) sql.NullString { return sql.NullString{String: FormatTime(t), Valid: true} }

// NullString maps the empty string to NULL.
func NullString(s string) sql.NullString { return sql.NullString{String: s, Valid: s != ""} }

// Open opens (creating if needed) the SQLite database at path with WAL, a 5 s busy timeout and
// foreign keys enabled, and checks that it is reachable. The caller must Close it.
//
// _txlock=immediate makes every transaction take the write lock up front, so a
// read-then-write transaction cannot fail with SQLITE_BUSY_SNAPSHOT under WAL.
func Open(path string) (*sql.DB, error) {
	dsn := "file:" + path +
		"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_txlock=immediate"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	return db, nil
}

// Migrate applies, in file-name order, each embedded migration not yet recorded in
// schema_migrations, one transaction per file. It is safe to run on every start.
func Migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx,
		`CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at TEXT NOT NULL)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		var applied int
		if err := db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, entry.Name()).Scan(&applied); err != nil {
			return err
		}
		if applied > 0 {
			continue
		}
		body, err := migrationFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return err
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, string(body)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply %s: %w", entry.Name(), err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
			entry.Name(), FormatTime(time.Now())); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
