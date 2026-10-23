package session

import (
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"social-network/pkg/db/sqlite"
)

func newTestDB(t *testing.T) (*sql.DB, int64) {
	t.Helper()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := sqlite.MigrateUp(db, "file://../db/migrations/sqlite"); err != nil {
		t.Fatal(err)
	}
	res, err := db.Exec(`INSERT INTO users (email, password_hash, first_name, last_name, date_of_birth)
		VALUES ('a@example.com', 'x', 'Ada', 'Lovelace', '1990-01-01')`)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	return db, id
}

func TestCreateGetDelete(t *testing.T) {
	db, userID := newTestDB(t)
	id, expires, err := Create(db, userID)
	if err != nil {
		t.Fatal(err)
	}
	if d := time.Until(expires); d < Lifetime-time.Minute || d > Lifetime {
		t.Fatalf("expires in %v, want about %v", d, Lifetime)
	}
	if got, err := Get(db, id); err != nil || got != userID {
		t.Fatalf("Get = %d, %v; want %d", got, err, userID)
	}
	if err := Delete(db, id); err != nil {
		t.Fatal(err)
	}
	if _, err := Get(db, id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get after Delete err = %v, want ErrNotFound", err)
	}
	if err := Delete(db, id); err != nil {
		t.Fatalf("deleting a deleted session: %v", err)
	}
}

func TestUnknownAndEmptyIDs(t *testing.T) {
	db, _ := newTestDB(t)
	for _, id := range []string{"", "not-a-session"} {
		if _, err := Get(db, id); !errors.Is(err, ErrNotFound) {
			t.Errorf("Get(%q) err = %v, want ErrNotFound", id, err)
		}
	}
}

func TestExpiredSessions(t *testing.T) {
	db, userID := newTestDB(t)
	past := time.Now().UTC().Add(-time.Hour).Format(timeLayout)
	if _, err := db.Exec(`INSERT INTO sessions (id, user_id, expires_at) VALUES ('old', ?, ?)`, userID, past); err != nil {
		t.Fatal(err)
	}
	live, _, err := Create(db, userID)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := Get(db, "old"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get(expired) err = %v, want ErrNotFound", err)
	}
	if err := DeleteExpired(db); err != nil {
		t.Fatal(err)
	}
	var n int
	db.QueryRow(`SELECT COUNT(*) FROM sessions WHERE id = 'old'`).Scan(&n)
	if n != 0 {
		t.Fatal("DeleteExpired kept the expired session")
	}
	if _, err := Get(db, live); err != nil {
		t.Fatalf("DeleteExpired removed a live session: %v", err)
	}
}

func TestFromRequest(t *testing.T) {
	db, userID := newTestDB(t)
	id, _, _ := Create(db, userID)

	r := httptest.NewRequest("GET", "/", nil)
	if _, err := FromRequest(db, r); !errors.Is(err, ErrNotFound) {
		t.Fatalf("no cookie: err = %v, want ErrNotFound", err)
	}
	r.AddCookie(&http.Cookie{Name: CookieName, Value: id})
	if got, err := FromRequest(db, r); err != nil || got != userID {
		t.Fatalf("FromRequest = %d, %v; want %d", got, err, userID)
	}
}

func TestCookies(t *testing.T) {
	rec := httptest.NewRecorder()
	SetCookie(rec, "abc", time.Now().Add(time.Hour))
	c := rec.Result().Cookies()[0]
	if c.Name != CookieName || c.Value != "abc" || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Path != "/" {
		t.Fatalf("SetCookie sent %+v", c)
	}

	rec = httptest.NewRecorder()
	ClearCookie(rec)
	if c := rec.Result().Cookies()[0]; c.Value != "" || c.MaxAge >= 0 {
		t.Fatalf("ClearCookie sent %+v", c)
	}
}
