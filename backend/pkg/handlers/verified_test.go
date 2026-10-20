package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUnverifiedUsersAreLockedOut(t *testing.T) {
	app := newTestApp(t)
	app.RequireVerified = true
	srv := httptest.NewServer(app.Routes())
	defer srv.Close()
	c, id := register(t, srv.URL, "new@example.com", "New")

	// Allowed before verifying: who am I, and the email code.
	if got := c.status("/api/me"); got != http.StatusOK {
		t.Errorf("GET /api/me unverified = %d, want 200", got)
	}
	if status, body := c.do("POST", "/api/otp/send", "application/json", nil); status != http.StatusOK {
		t.Errorf("POST /api/otp/send unverified = %d %v, want 200", status, body)
	}

	// Everything else is 403 until the email is verified, API and WebSocket alike.
	locked := []struct{ method, path string }{
		{"GET", "/api/feed"},
		{"GET", "/api/users"},
		{"GET", fmt.Sprintf("/api/users/%d", id)},
		{"GET", "/api/groups"},
		{"GET", "/api/conversations"},
		{"GET", "/api/notifications"},
		{"POST", "/api/groups"},
		{"PATCH", "/api/me/privacy"},
		{"GET", "/ws"},
		{"GET", "/uploads/whatever.png"},
	}
	for _, l := range locked {
		if status, body := c.do(l.method, l.path, "application/json", nil); status != http.StatusForbidden || (l.path != "/ws" && body["error"] != "verify your email first") {
			t.Errorf("%s %s unverified = %d %v, want 403", l.method, l.path, status, body)
		}
	}

	// Once verified, the same session works.
	if _, err := app.DB.Exec(`UPDATE users SET is_verified = 1 WHERE id = ?`, id); err != nil {
		t.Fatal(err)
	}
	if got := c.status("/api/feed"); got != http.StatusOK {
		t.Errorf("GET /api/feed verified = %d, want 200", got)
	}

	// Logging in still works for an unverified account (to reach the code page).
	other, _ := register(t, srv.URL, "other@example.com", "Other")
	other.do("POST", "/api/logout", "", nil)
	if status, body := other.json("POST", "/api/login", `{"email":"other@example.com","password":"`+testPassword+`"}`); status != http.StatusOK {
		t.Errorf("unverified login = %d %v, want 200", status, body)
	}
}
