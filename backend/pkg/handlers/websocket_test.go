package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
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
