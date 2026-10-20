package sqlite

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
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
	for _, want := range []string{"users", "sessions", "follows", "notifications", "posts", "post_viewers", "comments",
		"groups", "group_members", "group_invites", "group_requests", "events", "event_responses", "messages", "group_messages"} {
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

// Transactions that read and then write must not fail with "database is
// locked" when several run at once (needs _txlock=immediate).
func TestConcurrentReadThenWrite(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "conc.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := MigrateUp(db, testMigrations); err != nil {
		t.Fatal(err)
	}

	const workers, perWorker = 8, 40
	errs := make(chan error, workers*perWorker)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				email := fmt.Sprintf("u%d_%d@example.com", w, i)
				tx, err := db.Begin()
				if err != nil {
					errs <- err
					continue
				}
				var n int
				if err := tx.QueryRow(`SELECT COUNT(*) FROM users WHERE email = ?`, email).Scan(&n); err != nil {
					tx.Rollback()
					errs <- err
					continue
				}
				if _, err := tx.Exec(`INSERT INTO users (email, password_hash, first_name, last_name, date_of_birth) VALUES (?, 'h', 'A', 'B', '1990-01-01')`, email); err != nil {
					tx.Rollback()
					errs <- err
					continue
				}
				if err := tx.Commit(); err != nil {
					errs <- err
				}
			}
		}(w)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("transaction failed: %v", err)
	}
}

// A path with URI special characters must open the file it names.
func TestOpenEscapesPath(t *testing.T) {
	for _, name := range []string{"plain.db", "with space.db", "hash#tag.db", "pct%41.db"} {
		path := filepath.Join(t.TempDir(), name)
		db, err := Open(path)
		if err != nil {
			t.Errorf("%q: open: %v", name, err)
			continue
		}
		_, err = db.Exec(`CREATE TABLE x (a)`)
		db.Close()
		if err != nil {
			t.Errorf("%q: exec: %v", name, err)
		}
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%q: file was not created at the requested path: %v", name, err)
		}
	}
}

// Every migration on its own: roll it back, apply it, roll it back again.
func TestEachMigrationDownThenUp(t *testing.T) {
	files, err := filepath.Glob("../migrations/sqlite/*.up.sql")
	if err != nil || len(files) == 0 {
		t.Fatalf("no migrations found: %v", err)
	}
	db, err := Open(filepath.Join(t.TempDir(), "step.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := MigrateUp(db, testMigrations); err != nil {
		t.Fatal(err)
	}
	step := func(n int, what string) {
		t.Helper()
		m, err := newMigrator(db, testMigrations)
		if err != nil {
			t.Fatal(err)
		}
		if err := m.Steps(n); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
	}
	for i := len(files) - 1; i >= 0; i-- {
		name := filepath.Base(files[i])
		step(-1, name+" down")
		step(+1, name+" up again")
		step(-1, name+" down again")
	}
	if left := tables(t, db); len(left) != 0 {
		t.Errorf("tables left after stepping everything down: %v", left)
	}
	if err := MigrateUp(db, testMigrations); err != nil {
		t.Fatalf("up after stepping down: %v", err)
	}
}

// Migration 000019: nicknames become unique without regard to case, any
// number of users can have none, an existing duplicate is cleared on all but
// the oldest account, and the migration can be rolled back and applied again.
func TestUniqueNicknameMigration(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "nick.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := MigrateUp(db, testMigrations); err != nil {
		t.Fatal(err)
	}
	step := func(n int) {
		t.Helper()
		m, err := newMigrator(db, testMigrations)
		if err != nil {
			t.Fatal(err)
		}
		if err := m.Steps(n); err != nil {
			t.Fatalf("steps(%d): %v", n, err)
		}
	}
	addUser := func(email, nickname string) error {
		_, err := db.Exec(`INSERT INTO users (email, password_hash, first_name, last_name, date_of_birth, nickname)
			VALUES (?, 'h', 'A', 'B', '1990-01-01', ?)`, email, nickname)
		return err
	}
	hasIndex := func() bool {
		var n int
		db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = 'idx_users_nickname'`).Scan(&n)
		return n == 1
	}

	// Down: the index is gone, so duplicates (as old databases have) go in.
	step(-1)
	if hasIndex() {
		t.Fatal("idx_users_nickname still there after down")
	}
	for _, u := range [][2]string{{"a@x.com", "ali"}, {"b@x.com", "ALI"}, {"c@x.com", "Ali"}, {"d@x.com", ""}, {"e@x.com", ""}, {"f@x.com", "bob"}} {
		if err := addUser(u[0], u[1]); err != nil {
			t.Fatalf("insert %v before the index: %v", u, err)
		}
	}

	// Up again: the oldest "ali" keeps it, the later two lose it, the rest is untouched.
	step(+1)
	if !hasIndex() {
		t.Fatal("idx_users_nickname missing after up")
	}
	rows, err := db.Query(`SELECT email, nickname FROM users ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for rows.Next() {
		var email, nickname string
		rows.Scan(&email, &nickname)
		got[email] = nickname
	}
	rows.Close()
	want := map[string]string{"a@x.com": "ali", "b@x.com": "", "c@x.com": "", "d@x.com": "", "e@x.com": "", "f@x.com": "bob"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("nicknames after up = %v, want %v", got, want)
	}

	if err := addUser("g@x.com", "BOB"); err == nil {
		t.Error("a second BOB went in; the index should refuse it")
	}
	if err := addUser("h@x.com", ""); err != nil {
		t.Errorf("another user without a nickname: %v", err)
	}
	if err := addUser("i@x.com", "carol"); err != nil {
		t.Errorf("a new nickname: %v", err)
	}
}
