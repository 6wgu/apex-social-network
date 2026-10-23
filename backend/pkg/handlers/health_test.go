package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"social-network/pkg/db/sqlite"
)

func checkHealth(t *testing.T, a *App) (int, map[string]string) {
	t.Helper()
	rec := httptest.NewRecorder()
	a.health(rec, httptest.NewRequest("GET", "/api/health", nil))
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	return rec.Code, body
}

func TestHealth(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	a := &App{DB: db}

	if code, body := checkHealth(t, a); code != http.StatusOK || body["database"] != "ok" {
		t.Fatalf("open database: %d %v, want 200 and database ok", code, body)
	}
	db.Close()
	if code, body := checkHealth(t, a); code != http.StatusServiceUnavailable || body["database"] != "unreachable" {
		t.Fatalf("closed database: %d %v, want 503 and database unreachable", code, body)
	}
}
