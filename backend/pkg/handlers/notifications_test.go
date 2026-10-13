package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// types returns the "type" of each notification in a list, in order.
func types(items []map[string]any) []string {
	out := []string{}
	for _, it := range items {
		out = append(out, it["type"].(string))
	}
	return out
}

func unread(t *testing.T, c *testClient) float64 {
	t.Helper()
	status, body := c.get("/api/notifications/unread-count")
	if status != http.StatusOK {
		t.Fatalf("unread-count = %d %v", status, body)
	}
	return body["unread"].(float64)
}

// readEvent reads one WebSocket event, failing after 2 seconds.
func readEvent(t *testing.T, conn *websocket.Conn) (string, map[string]any) {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var ev struct {
		Type    string         `json:"type"`
		Payload map[string]any `json:"payload"`
	}
	if err := conn.ReadJSON(&ev); err != nil {
		t.Fatalf("waiting for a WebSocket event: %v", err)
	}
	return ev.Type, ev.Payload
}

func TestFollowRequestNotificationLifecycle(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.Routes())
	defer srv.Close()
	alice, aliceID := register(t, srv.URL, "alice@example.com", "Alice")
	bob, bobID := register(t, srv.URL, "bob@example.com", "Bob")
	carol, _ := register(t, srv.URL, "carol@example.com", "Carol")
	alice.json("PATCH", "/api/me/privacy", `{"is_public": false}`)
	follow := fmt.Sprintf("/api/users/%d/follow", aliceID)

	// Alice is online: the request arrives live.
	conn, _, err := dialWS(t, srv, alice, "http://localhost:3000")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	waitFor(t, func() bool { return app.Hub.Connections(aliceID) == 1 })

	bob.do("POST", follow, "", nil)
	typ, payload := readEvent(t, conn)
	actor, _ := payload["actor"].(map[string]any)
	if typ != "notification" || payload["type"] != "follow_request" || actor["first_name"] != "Bob" || payload["is_read"] != false {
		t.Fatalf("live event = %s %v", typ, payload)
	}
	if _, leaked := actor["email"]; leaked {
		t.Errorf("notification actor leaks email: %v", actor)
	}
	noteID := payload["id"].(float64)

	_, list := alice.list("/api/notifications")
	if len(list) != 1 || list[0]["id"] != noteID || list[0]["type"] != "follow_request" {
		t.Fatalf("alice's notifications = %v", list)
	}
	if n := unread(t, alice); n != 1 {
		t.Errorf("unread = %v, want 1", n)
	}
	// Nobody else sees or touches Alice's notification.
	if _, list := bob.list("/api/notifications"); len(list) != 0 {
		t.Errorf("bob sees notifications: %v", list)
	}
	if status, _ := carol.do("POST", fmt.Sprintf("/api/notifications/%d/read", int64(noteID)), "", nil); status != http.StatusNotFound {
		t.Errorf("carol marks alice's notification read = %d, want 404", status)
	}
	carol.do("POST", "/api/notifications/read-all", "", nil)
	if n := unread(t, alice); n != 1 {
		t.Errorf("unread after carol's read-all = %v, want 1", n)
	}

	// Mark read.
	if status, _ := alice.do("POST", fmt.Sprintf("/api/notifications/%d/read", int64(noteID)), "", nil); status != http.StatusNoContent {
		t.Errorf("mark read = %d, want 204", status)
	}
	if n := unread(t, alice); n != 0 {
		t.Errorf("unread after read = %v, want 0", n)
	}

	// Declining resolves it (removed, live) and gives Bob nothing.
	alice.do("POST", fmt.Sprintf("/api/follow-requests/%d/decline", bobID), "", nil)
	if typ, payload := readEvent(t, conn); typ != "notification_removed" || payload["id"] != noteID {
		t.Errorf("after decline got %s %v, want notification_removed %v", typ, payload, noteID)
	}
	if _, list := alice.list("/api/notifications"); len(list) != 0 {
		t.Errorf("after decline alice has %v", list)
	}
	if _, list := bob.list("/api/notifications"); len(list) != 0 {
		t.Errorf("declined bob got %v", list)
	}

	// Cancelling resolves it too.
	bob.do("POST", follow, "", nil)
	readEvent(t, conn) // the new request
	bob.do("DELETE", follow, "", nil)
	if typ, _ := readEvent(t, conn); typ != "notification_removed" {
		t.Errorf("after cancel got %s, want notification_removed", typ)
	}
	if _, list := alice.list("/api/notifications"); len(list) != 0 {
		t.Errorf("after cancel alice has %v", list)
	}

	// Accepting resolves it and tells Bob.
	bob.do("POST", follow, "", nil)
	readEvent(t, conn)
	alice.do("POST", fmt.Sprintf("/api/follow-requests/%d/accept", bobID), "", nil)
	if typ, _ := readEvent(t, conn); typ != "notification_removed" {
		t.Errorf("after accept got %s, want notification_removed", typ)
	}
	if _, list := alice.list("/api/notifications"); len(list) != 0 {
		t.Errorf("after accept alice has %v", list)
	}
	_, list = bob.list("/api/notifications")
	if fmt.Sprint(types(list)) != "[follow_accepted]" {
		t.Errorf("bob after accept = %v, want [follow_accepted]", list)
	}
	if actor, _ := list[0]["actor"].(map[string]any); actor["first_name"] != "Alice" {
		t.Errorf("follow_accepted actor = %v", list[0]["actor"])
	}
}

func TestPublicFollowAndGoingPublicNotify(t *testing.T) {
	srv := httptest.NewServer(newTestApp(t).Routes())
	defer srv.Close()
	alice, aliceID := register(t, srv.URL, "alice@example.com", "Alice")
	bob, _ := register(t, srv.URL, "bob@example.com", "Bob")
	carol, carolID := register(t, srv.URL, "carol@example.com", "Carol")

	// Following a public profile: new_follower, nothing to answer.
	bob.do("POST", fmt.Sprintf("/api/users/%d/follow", aliceID), "", nil)
	if _, list := alice.list("/api/notifications"); fmt.Sprint(types(list)) != "[new_follower]" {
		t.Errorf("alice = %v, want [new_follower]", list)
	}

	// Carol is private; Alice and Bob ask. Going public resolves both requests.
	carol.json("PATCH", "/api/me/privacy", `{"is_public": false}`)
	alice.do("POST", fmt.Sprintf("/api/users/%d/follow", carolID), "", nil)
	bob.do("POST", fmt.Sprintf("/api/users/%d/follow", carolID), "", nil)
	if _, list := carol.list("/api/notifications"); fmt.Sprint(types(list)) != "[follow_request follow_request]" {
		t.Fatalf("carol = %v", types(list))
	}
	carol.json("PATCH", "/api/me/privacy", `{"is_public": true}`)
	if _, list := carol.list("/api/notifications"); len(list) != 0 {
		t.Errorf("carol after going public = %v, want none", types(list))
	}
	for name, c := range map[string]*testClient{"alice": alice, "bob": bob} {
		_, list := c.list("/api/notifications")
		if len(list) == 0 || list[0]["type"] != "follow_accepted" {
			t.Errorf("%s after carol went public = %v, want follow_accepted first", name, types(list))
		}
	}
}

func TestNotificationListPagingAndReadAll(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.Routes())
	defer srv.Close()
	alice, aliceID := register(t, srv.URL, "alice@example.com", "Alice")
	_, bobID := register(t, srv.URL, "bob@example.com", "Bob")

	for i := 0; i < 55; i++ {
		if _, err := app.DB.Exec(`INSERT INTO notifications (user_id, type, actor_id) VALUES (?, 'new_follower', ?)`, aliceID, bobID); err != nil {
			t.Fatal(err)
		}
	}
	_, page1 := alice.list("/api/notifications")
	if len(page1) != 50 {
		t.Fatalf("first page = %d items, want 50", len(page1))
	}
	if page1[0]["id"].(float64) < page1[49]["id"].(float64) {
		t.Error("first page is not newest first")
	}
	last := int64(page1[49]["id"].(float64))
	_, page2 := alice.list(fmt.Sprintf("/api/notifications?before=%d", last))
	if len(page2) != 5 || page2[0]["id"].(float64) >= float64(last) {
		t.Errorf("second page = %d items starting at %v", len(page2), page2[0]["id"])
	}
	if status, _ := alice.get("/api/notifications?before=abc"); status != http.StatusBadRequest {
		t.Errorf("before=abc = %d, want 400", status)
	}

	if n := unread(t, alice); n != 55 {
		t.Errorf("unread = %v, want 55", n)
	}
	if status, _ := alice.do("POST", "/api/notifications/read-all", "", nil); status != http.StatusNoContent {
		t.Errorf("read-all = %d", status)
	}
	if n := unread(t, alice); n != 0 {
		t.Errorf("unread after read-all = %v", n)
	}
	for _, path := range []string{"/api/notifications/999999/read", "/api/notifications/abc/read"} {
		if status, _ := alice.do("POST", path, "", nil); status != http.StatusNotFound {
			t.Errorf("POST %s = %d, want 404", path, status)
		}
	}

	anon := newClient(t, srv.URL)
	for _, route := range [][2]string{
		{"GET", "/api/notifications"}, {"GET", "/api/notifications/unread-count"},
		{"POST", "/api/notifications/1/read"}, {"POST", "/api/notifications/read-all"},
	} {
		if status, _ := anon.do(route[0], route[1], "", nil); status != http.StatusUnauthorized {
			t.Errorf("%s %s without a session = %d, want 401", route[0], route[1], status)
		}
	}
}
