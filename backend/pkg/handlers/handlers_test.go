package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"social-network/pkg/db/sqlite"
)

// Run from backend/: go test ./pkg/handlers
const testMigrations = "file://../db/migrations/sqlite"

// newTestApp returns an App backed by a fresh migrated database and an empty
// upload directory, both inside the test's temp dir.
func newTestApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	db, err := sqlite.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := sqlite.MigrateUp(db, testMigrations); err != nil {
		t.Fatal(err)
	}
	uploads := filepath.Join(dir, "uploads")
	if err := os.MkdirAll(uploads, 0o755); err != nil {
		t.Fatal(err)
	}
	return &App{DB: db, UploadDir: uploads}
}

func TestWrongMethodIs405(t *testing.T) {
	srv := httptest.NewServer(newTestApp(t).Routes())
	defer srv.Close()

	res, err := http.Post(srv.URL+"/api/health", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("POST /api/health = %d, want 405", res.StatusCode)
	}
	if got := res.Header.Get("Allow"); got != "GET" {
		t.Errorf("Allow = %q, want GET", got)
	}
	if got := res.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}

	res2, err := http.Get(srv.URL + "/api/nope")
	if err != nil {
		t.Fatal(err)
	}
	defer res2.Body.Close()
	if res2.StatusCode != http.StatusNotFound {
		t.Errorf("GET /api/nope = %d, want 404", res2.StatusCode)
	}
}
