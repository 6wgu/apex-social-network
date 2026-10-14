package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"social-network/pkg/session"
)

// Logout is public: with no cookie, an unknown cookie or an expired one it
// still answers 204 and tells the browser to drop the cookie.
func TestLogoutAlwaysClearsTheCookie(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.Routes())
	defer srv.Close()
	_, userID := register(t, srv.URL, "ada@example.com", "Ada")

	if _, err := app.DB.Exec(`INSERT INTO sessions (id, user_id, expires_at) VALUES ('expired', ?, datetime('now', '-1 minute'))`, userID); err != nil {
		t.Fatal(err)
	}
	for name, cookie := range map[string]string{"no cookie": "", "unknown cookie": "nope", "expired cookie": "expired"} {
		req, _ := http.NewRequest("POST", srv.URL+"/api/logout", nil)
		if cookie != "" {
			req.AddCookie(&http.Cookie{Name: session.CookieName, Value: cookie})
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		cleared := false
		for _, c := range res.Cookies() {
			cleared = cleared || (c.Name == session.CookieName && c.MaxAge < 0)
		}
		if res.StatusCode != http.StatusNoContent || !cleared {
			t.Errorf("%s: logout = %d, cookie cleared %v; want 204 and cleared", name, res.StatusCode, cleared)
		}
	}
	var n int
	app.DB.QueryRow(`SELECT COUNT(*) FROM sessions WHERE id = 'expired'`).Scan(&n)
	if n != 0 {
		t.Error("logout with an expired cookie left its session row")
	}
}
