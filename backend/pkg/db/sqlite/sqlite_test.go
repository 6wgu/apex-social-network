package sqlite

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// Run from backend/: go test ./pkg/db/sqlite
const testMigrations = "file://../migrations/sqlite"

func tables(t *testing.T, db *sql.DB) map[string]bool {
	t.Helper()
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name <> 'schema_migrations'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		got[name] = true
	}
	return got
}

// Gate 0: a fresh DB gets every table, every down works, and up runs again cleanly.
func TestMigrationsUpDownUp(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := MigrateUp(db, testMigrations); err != nil {
		t.Fatalf("first up: %v", err)
	}
	for _, want := range []string{"users", "sessions"} {
		if !tables(t, db)[want] {
			t.Errorf("table %q missing after up", want)
		}
	}
	if err := MigrateUp(db, testMigrations); err != nil {
		t.Fatalf("second up should be a no-op: %v", err)
	}

	if err := MigrateDown(db, testMigrations); err != nil {
		t.Fatalf("down: %v", err)
	}
	if left := tables(t, db); len(left) != 0 {
		t.Errorf("tables left after down: %v", left)
	}

	if err := MigrateUp(db, testMigrations); err != nil {
		t.Fatalf("up after down: %v", err)
	}
}

func TestForeignKeysOn(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "fk.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var on int
	if err := db.QueryRow(`PRAGMA foreign_keys`).Scan(&on); err != nil || on != 1 {
		t.Fatalf("foreign_keys = %d, err %v; want 1", on, err)
	}
}
