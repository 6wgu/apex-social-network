package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

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

// Unknown emails go through bcrypt too, so they take about as long as a
// wrong password (a plain lookup miss would answer in well under 1 ms).
func TestLoginUnknownEmailRunsBcrypt(t *testing.T) {
	srv := httptest.NewServer(newTestApp(t).Routes())
	defer srv.Close()
	c, _ := register(t, srv.URL, "ada@example.com", "Ada")

	timeLogin := func(email string) time.Duration {
		start := time.Now()
		status, body := c.json("POST", "/api/login", `{"email":"`+email+`","password":"wrong password"}`)
		if status != http.StatusUnauthorized || body["error"] != "wrong email or password" {
			t.Fatalf("login %s = %d %v", email, status, body)
		}
		return time.Since(start)
	}
	known, unknown := timeLogin("ada@example.com"), timeLogin("nobody@example.com")
	if unknown < known/3 {
		t.Errorf("unknown email took %v, wrong password %v: unknown emails skip bcrypt", unknown, known)
	}
}

func dialWS(t *testing.T, srv *httptest.Server, c *testClient, origin string) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	header := http.Header{}
	for _, ck := range c.http.Jar.Cookies(mustURL(t, srv.URL)) {
		header.Add("Cookie", ck.Name+"="+ck.Value)
	}
	if origin != "" {
		header.Set("Origin", origin)
	}
	return websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/ws", header)
}

func TestWebSocketConnectsAndReceivesEvents(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.Routes())
	defer srv.Close()
	alice, aliceID := register(t, srv.URL, "alice@example.com", "Alice")
	_, bobID := register(t, srv.URL, "bob@example.com", "Bob")

	// Without a session the upgrade is refused with 401.
	if _, res, err := dialWS(t, srv, newClient(t, srv.URL), ""); err == nil || res == nil || res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("dial without a session: err %v, response %v; want 401", err, res)
	}
	// A page on another origin is refused with 403.
	if _, res, err := dialWS(t, srv, alice, "http://evil.example"); err == nil || res == nil || res.StatusCode != http.StatusForbidden {
		t.Fatalf("dial from a foreign origin: err %v, response %v; want 403", err, res)
	}

	// Two tabs for Alice, from the frontend origin.
	tab1, _, err := dialWS(t, srv, alice, "http://localhost:3000")
	if err != nil {
		t.Fatal(err)
	}
	defer tab1.Close()
	tab2, _, err := dialWS(t, srv, alice, "http://localhost:3000")
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return app.Hub.Connections(aliceID) == 2 })

	app.Hub.SendToUser(bobID, "notification", map[string]any{"for": "bob"}) // Bob is offline: no effect
	app.Hub.SendToUser(aliceID, "notification", map[string]any{"text": "hello 👋"})
	for i, tab := range []*websocket.Conn{tab1, tab2} {
		tab.SetReadDeadline(time.Now().Add(2 * time.Second))
		var ev struct {
			Type    string         `json:"type"`
			Payload map[string]any `json:"payload"`
		}
		if err := tab.ReadJSON(&ev); err != nil {
			t.Fatalf("tab %d: %v", i+1, err)
		}
		if ev.Type != "notification" || ev.Payload["text"] != "hello 👋" {
			t.Errorf("tab %d got %+v", i+1, ev)
		}
	}

	// Closing a tab unregisters it; the hub keeps working.
	tab2.Close()
	waitFor(t, func() bool { return app.Hub.Connections(aliceID) == 1 })
}

// waitFor polls cond for up to 2 seconds.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if cond() {
			return
		}
	}
	t.Fatal("condition not met within 2s")
}
