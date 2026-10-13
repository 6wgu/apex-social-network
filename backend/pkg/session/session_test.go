package session

import (
	"database/sql"
	"errors"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"social-network/pkg/db/sqlite"
)

// Run from backend/: go test ./pkg/session
const testMigrations = "file://../db/migrations/sqlite"

func newDB(t *testing.T) (*sql.DB, int64) {
	t.Helper()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := sqlite.MigrateUp(db, testMigrations); err != nil {
		t.Fatal(err)
	}
	res, err := db.Exec(`INSERT INTO users (email, password_hash, first_name, last_name, date_of_birth) VALUES ('a@b.co', 'h', 'A', 'B', '1990-01-01')`)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	return db, id
}

func TestCreateGetDelete(t *testing.T) {
	db, userID := newDB(t)

	id, _, err := Create(db, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(id) != 36 {
		t.Errorf("session id %q is not a UUID", id)
	}
	if got, err := Get(db, id); err != nil || got != userID {
		t.Errorf("Get = %d, %v; want %d, nil", got, err, userID)
	}
	if err := Delete(db, id); err != nil {
		t.Fatal(err)
	}
	if _, err := Get(db, id); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get after Delete = %v, want ErrNotFound", err)
	}
	if _, err := Get(db, "not-a-session"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get unknown = %v, want ErrNotFound", err)
	}
}

func TestExpiredSessionIsRejected(t *testing.T) {
	db, userID := newDB(t)
	if _, err := db.Exec(`INSERT INTO sessions (id, user_id, expires_at) VALUES ('old', ?, datetime('now', '-1 minute'))`, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := Get(db, "old"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get expired = %v, want ErrNotFound", err)
	}
	if err := DeleteExpired(db); err != nil {
		t.Fatal(err)
	}
	var n int
	db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&n)
	if n != 0 {
		t.Errorf("%d sessions left after DeleteExpired, want 0", n)
	}
}

func TestCookieFlags(t *testing.T) {
	db, userID := newDB(t)
	id, expires, err := Create(db, userID)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	SetCookie(rec, id, expires)
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("got %d cookies, want 1", len(cookies))
	}
	c := cookies[0]
	if c.Name != CookieName || c.Value != id || !c.HttpOnly || c.Path != "/" || c.MaxAge < 6*24*3600 {
		t.Errorf("unexpected cookie: %+v", c)
	}
	if got := rec.Header().Get("Set-Cookie"); !strings.Contains(got, "SameSite=Lax") {
		t.Errorf("Set-Cookie %q has no SameSite=Lax", got)
	}
}

func TestSecureFlagFollowsSetting(t *testing.T) {
	defer func() { SecureCookies = false }()
	for _, secure := range []bool{false, true} {
		SecureCookies = secure
		rec := httptest.NewRecorder()
		SetCookie(rec, "id", time.Now().Add(time.Hour))
		ClearCookie(rec)
		for _, c := range rec.Result().Cookies() {
			if c.Secure != secure {
				t.Errorf("SecureCookies=%v: cookie %q (MaxAge %d) has Secure=%v", secure, c.Name, c.MaxAge, c.Secure)
			}
		}
	}
}
