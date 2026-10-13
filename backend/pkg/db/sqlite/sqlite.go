// Package sqlite opens the SQLite database and applies the migrations.
package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	migratesqlite "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/mattn/go-sqlite3"
)

// uriEscaper escapes the characters SQLite treats specially in a file: URI,
// so a path containing them still opens the file it names.
var uriEscaper = strings.NewReplacer("%", "%25", "?", "%3f", "#", "%23")

// dsn builds the connection string. The pragmas are part of it so they apply
// to every pooled connection. _txlock=immediate makes transactions take the
// write lock up front: a deferred transaction that reads and then writes
// fails with "database is locked" when another writer got in between, and the
// busy timeout does not cover that case.
func dsn(path string) string {
	return "file:" + uriEscaper.Replace(path) +
		"?_foreign_keys=on&_journal_mode=WAL&_busy_timeout=5000&_txlock=immediate"
}

// Open opens (creating it if needed) the SQLite database at path.
func Open(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}
	db, err := sql.Open("sqlite3", dsn(path))
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return db, nil
}

// newMigrator builds a golang-migrate instance reading files from sourceURL,
// e.g. "file://pkg/db/migrations/sqlite".
func newMigrator(db *sql.DB, sourceURL string) (*migrate.Migrate, error) {
	driver, err := migratesqlite.WithInstance(db, &migratesqlite.Config{})
	if err != nil {
		return nil, err
	}
	return migrate.NewWithDatabaseInstance(sourceURL, "sqlite3", driver)
}

// MigrateUp applies every pending up migration. Running it on an
// up-to-date database is a no-op.
func MigrateUp(db *sql.DB, sourceURL string) error {
	m, err := newMigrator(db, sourceURL)
	if err != nil {
		return fmt.Errorf("init migrations: %w", err)
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

// MigrateDown rolls back every applied migration.
func MigrateDown(db *sql.DB, sourceURL string) error {
	m, err := newMigrator(db, sourceURL)
	if err != nil {
		return fmt.Errorf("init migrations: %w", err)
	}
	if err := m.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate down: %w", err)
	}
	return nil
}
