package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"social-network/pkg/ws"
)

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

// FRONTEND_ORIGIN may list several origins: each listed one may open /ws,
// anything else is refused.
func TestWebSocketOriginList(t *testing.T) {
	app := newTestApp(t)
	app.Hub = ws.NewHub(ws.ParseOrigins("http://localhost:3000, http://127.0.0.1:3000/")...)
	srv := httptest.NewServer(app.Routes())
	defer srv.Close()
	alice, _ := register(t, srv.URL, "alice@example.com", "Alice")

	for origin, want := range map[string]int{
		"http://localhost:3000":  http.StatusSwitchingProtocols,
		"http://127.0.0.1:3000":  http.StatusSwitchingProtocols,
		"http://evil.example":    http.StatusForbidden,
		"http://localhost:3001":  http.StatusForbidden,
		"http://localhost:3000/": http.StatusForbidden, // exact match only
	} {
		conn, res, err := dialWS(t, srv, alice, origin)
		if conn != nil {
			conn.Close()
		}
		if res == nil {
			t.Errorf("%s: no response (%v)", origin, err)
			continue
		}
		if res.StatusCode != want {
			t.Errorf("%s: status %d, want %d", origin, res.StatusCode, want)
		}
	}
}

// Logout closes every socket of that user: an event sent afterwards (a
// follow request) never reaches the old tab, which sees a close instead.
func TestLogoutClosesTheUsersSockets(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.Routes())
	defer srv.Close()
	alice, aliceID := register(t, srv.URL, "alice@example.com", "Alice")
	bob, _ := register(t, srv.URL, "bob@example.com", "Bob")
	if status, _ := alice.json("PATCH", "/api/me/privacy", `{"is_public": false}`); status != http.StatusOK {
		t.Fatalf("alice goes private = %d", status)
	}

	tab, _, err := dialWS(t, srv, alice, "http://localhost:3000")
	if err != nil {
		t.Fatal(err)
	}
	defer tab.Close()
	waitFor(t, func() bool { return app.Hub.Connections(aliceID) == 1 })

	if status, _ := alice.do("POST", "/api/logout", "", nil); status != http.StatusNoContent {
		t.Fatalf("logout = %d, want 204", status)
	}
	waitFor(t, func() bool { return app.Hub.Connections(aliceID) == 0 })
	if status, _ := bob.do("POST", fmt.Sprintf("/api/users/%d/follow", aliceID), "", nil); status != http.StatusOK {
		t.Fatalf("bob follows alice = %d", status)
	}

	tab.SetReadDeadline(time.Now().Add(2 * time.Second))
	msgType, data, err := tab.ReadMessage()
	if err == nil {
		t.Fatalf("socket still open after logout: got frame type %d %s", msgType, data)
	}
	if !websocket.IsCloseError(err, websocket.CloseNoStatusReceived, websocket.CloseNormalClosure) {
		t.Errorf("want the server to close the socket, got %v", err)
	}
}
