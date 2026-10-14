package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemoveAvatar(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.Routes())
	defer srv.Close()
	alice, _ := register(t, srv.URL, "alice@example.com", "Alice")

	status, user := alice.form("PATCH", "/api/me", map[string]string{}, tinyPNG(t))
	path, _ := user["avatar_path"].(string)
	if status != http.StatusOK || path == "" {
		t.Fatalf("upload avatar = %d %v", status, user)
	}
	file := filepath.Join(app.UploadDir, strings.TrimPrefix(path, "/uploads/"))
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("avatar file missing: %v", err)
	}

	if status, body := alice.form("PATCH", "/api/me", map[string]string{"remove_avatar": "1"}, tinyPNG(t)); status != http.StatusBadRequest {
		t.Errorf("avatar + remove_avatar = %d %v, want 400", status, body)
	}
	if status, body := alice.form("PATCH", "/api/me", map[string]string{"remove_avatar": "yes"}, nil); status != http.StatusBadRequest {
		t.Errorf("remove_avatar=yes = %d %v, want 400", status, body)
	}

	status, user = alice.form("PATCH", "/api/me", map[string]string{"remove_avatar": "1"}, nil)
	if status != http.StatusOK || user["avatar_path"] != "" {
		t.Fatalf("remove_avatar=1 = %d %v, want 200 with empty avatar_path", status, user)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Errorf("old avatar file still on disk (err %v)", err)
	}
	if _, me := alice.get("/api/me"); me["avatar_path"] != "" {
		t.Errorf("GET /api/me after removal = %v", me)
	}
	// Removing when there is no avatar is harmless.
	if status, _ := alice.form("PATCH", "/api/me", map[string]string{"remove_avatar": "1"}, nil); status != http.StatusOK {
		t.Errorf("second remove_avatar = %d, want 200", status)
	}
}
