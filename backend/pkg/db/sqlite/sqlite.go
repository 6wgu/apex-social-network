// Package sqlite opens the SQLite database and applies the migrations.
package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	migratesqlite "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/mattn/go-sqlite3"
)


func Open(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}
	dsn := fmt.Sprintf("file:%s?_foreign_keys=on&_journal_mode=WAL&_busy_timeout=5000", path)
	db, err := sql.Open("sqlite3", dsn)
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
