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

var uriEscaper = strings.NewReplacer("%", "%25", "?", "%3f", "#", "%23")

func dsn(path string) string {
	return "file:" + uriEscaper.Replace(path) +
		"?_foreign_keys=on&_journal_mode=WAL&_busy_timeout=5000&_txlock=immediate"
}

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

func newMigrator(db *sql.DB, sourceURL string) (*migrate.Migrate, error) {
	driver, err := migratesqlite.WithInstance(db, &migratesqlite.Config{})
	if err != nil {
		return nil, err
	}
	return migrate.NewWithDatabaseInstance(sourceURL, "sqlite3", driver)
}

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
