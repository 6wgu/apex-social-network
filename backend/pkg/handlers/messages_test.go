package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// wsEvent is one frame from the server, as tests read it.
type wsEvent struct {
	Type      string         `json:"type"`
	RequestID string         `json:"request_id"`
	Payload   map[string]any `json:"payload"`
}

// connect opens a socket for c from the frontend origin and waits until the
// hub has registered it.
func connect(t *testing.T, app *App, srv *httptest.Server, c *testClient, userID int64) *websocket.Conn {
	t.Helper()
	before := app.Hub.Connections(userID)
	conn, _, err := dialWS(t, srv, c, "http://localhost:3000")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	waitFor(t, func() bool { return app.Hub.Connections(userID) == before+1 })
	return conn
}

// sendPM sends a private_message frame.
func sendPM(t *testing.T, conn *websocket.Conn, requestID string, recipientID int64, content string) {
	t.Helper()
	frame := map[string]any{"type": "private_message", "request_id": requestID,
		"payload": map[string]any{"recipient_id": recipientID, "content": content}}
	if err := conn.WriteJSON(frame); err != nil {
		t.Fatal(err)
	}
}

// next returns the next frame of one of the given types, skipping others,
// or ok=false when none arrives within wait. A read that times out leaves a
// gorilla/websocket connection unusable, so only let it time out as the
// last read on that connection.
func next(conn *websocket.Conn, wait time.Duration, types ...string) (wsEvent, bool) {
	deadline := time.Now().Add(wait)
	for {
		conn.SetReadDeadline(deadline)
		var ev wsEvent
		if err := conn.ReadJSON(&ev); err != nil {
			return wsEvent{}, false
		}
		for _, t := range types {
			if ev.Type == t {
				return ev, true
			}
		}
	}
}

// reply returns the answer to one request id (the stored message or an
// error), skipping every other frame.
func reply(conn *websocket.Conn, requestID string) (wsEvent, bool) {
	for {
		ev, ok := next(conn, 2*time.Second, "private_message", "group_message", "error")
		if !ok || ev.RequestID == requestID {
			return ev, ok
		}
	}
}

func messageCount(t *testing.T, app *App) int {
	t.Helper()
	var n int
	if err := app.DB.QueryRow(`SELECT COUNT(*) FROM messages`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// The follow rule is checked on every message: no follow, no message; a
// pending request is not enough; an accepted follow either way is; and an
// unfollow blocks the very next message.
func TestPrivateMessageFollowRule(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.Routes())
	defer srv.Close()
	ann, annID := register(t, srv.URL, "ann@example.com", "Ann")
	ben, benID := register(t, srv.URL, "ben@example.com", "Ben")
	annConn := connect(t, app, srv, ann, annID)
	benConn := connect(t, app, srv, ben, benID)

	expectError := func(conn *websocket.Conn, reqID, want string) {
		t.Helper()
		ev, ok := next(conn, 2*time.Second, "error")
		if !ok || ev.RequestID != reqID || !strings.Contains(fmt.Sprint(ev.Payload["message"]), want) || ev.Payload["for"] != "private_message" {
			t.Fatalf("request %s: got %+v (ok %v), want error containing %q", reqID, ev, ok, want)
		}
	}

	// Strangers: rejected, nothing stored.
	sendPM(t, annConn, "r1", benID, "hello")
	expectError(annConn, "r1", "only message people you follow")

	// Ben asks to follow Ann, who is private: pending is not enough.
	ann.json("PATCH", "/api/me/privacy", `{"is_public": false}`)
	ben.do("POST", fmt.Sprintf("/api/users/%d/follow", annID), "", nil)
	sendPM(t, benConn, "r2", annID, "hi")
	expectError(benConn, "r2", "only message people you follow")
	if n := messageCount(t, app); n != 0 {
		t.Fatalf("%d messages stored before any accepted follow", n)
	}

	// Ann accepts: now both directions work (Ben follows Ann).
	ann.do("POST", fmt.Sprintf("/api/follow-requests/%d/accept", benID), "", nil)
	sendPM(t, annConn, "r3", benID, "welcome")
	if ev, ok := reply(annConn, "r3"); !ok || ev.Type != "private_message" {
		t.Fatalf("ann -> ben after accept: %+v", ev)
	}
	sendPM(t, benConn, "r4", annID, "thanks")
	if ev, ok := reply(benConn, "r4"); !ok || ev.Type != "private_message" {
		t.Fatalf("ben -> ann after accept: %+v", ev)
	}

	// Ben unfollows: the next message is refused, in both directions.
	ben.do("DELETE", fmt.Sprintf("/api/users/%d/follow", annID), "", nil)
	sendPM(t, benConn, "r5", annID, "still there?")
	expectError(benConn, "r5", "only message people you follow")
	sendPM(t, annConn, "r6", benID, "bye")
	expectError(annConn, "r6", "only message people you follow")
	if n := messageCount(t, app); n != 2 {
		t.Errorf("%d messages stored, want 2 (only while the follow existed)", n)
	}
}

// Invalid frames are rejected with an error event that echoes the request
// id, and nothing is stored. The connection stays usable.
func TestPrivateMessageValidation(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.Routes())
	defer srv.Close()
	ann, annID := register(t, srv.URL, "ann@example.com", "Ann")
	_, benID := register(t, srv.URL, "ben@example.com", "Ben")
	ann.do("POST", fmt.Sprintf("/api/users/%d/follow", benID), "", nil) // Ben is public: instant
	conn := connect(t, app, srv, ann, annID)

	cases := []struct {
		name  string
		frame string
		want  string
	}{
		{"not JSON", `hello`, "must be JSON"},
		{"no type", `{"payload":{}}`, "must be JSON"},
		{"unknown type", `{"type":"shout","request_id":"x"}`, "unknown event type"},
		{"no payload", `{"type":"private_message","request_id":"x"}`, "payload must be"},
		{"extra field", fmt.Sprintf(`{"type":"private_message","request_id":"x","payload":{"recipient_id":%d,"content":"hi","admin":true}}`, benID), "payload must be"},
		{"empty", fmt.Sprintf(`{"type":"private_message","request_id":"x","payload":{"recipient_id":%d,"content":"   "}}`, benID), "can't be empty"},
		{"too long", fmt.Sprintf(`{"type":"private_message","request_id":"x","payload":{"recipient_id":%d,"content":%q}}`, benID, strings.Repeat("a", 2001)), "2000 characters or fewer"},
		{"to myself", fmt.Sprintf(`{"type":"private_message","request_id":"x","payload":{"recipient_id":%d,"content":"me"}}`, annID), "yourself"},
		{"unknown user", `{"type":"private_message","request_id":"x","payload":{"recipient_id":99999,"content":"hi"}}`, "user not found"},
	}
	for _, tc := range cases {
		if err := conn.WriteMessage(websocket.TextMessage, []byte(tc.frame)); err != nil {
			t.Fatal(err)
		}
		ev, ok := next(conn, 2*time.Second, "error", "private_message")
		if !ok || ev.Type != "error" || !strings.Contains(fmt.Sprint(ev.Payload["message"]), tc.want) {
			t.Errorf("%s: got %+v (ok %v), want error containing %q", tc.name, ev, ok, tc.want)
		}
	}
	if n := messageCount(t, app); n != 0 {
		t.Errorf("%d messages stored from invalid frames", n)
	}

	// Length is counted in characters: 2000 emojis (8000 bytes) are fine.
	sendPM(t, conn, "ok", benID, strings.Repeat("😀", 2000))
	if ev, ok := next(conn, 2*time.Second, "private_message", "error"); !ok || ev.Type != "private_message" || ev.RequestID != "ok" {
		t.Errorf("2000 emojis: got %+v (ok %v), want the stored message", ev, ok)
	}
}

// A stored message goes to every tab of the recipient and of the sender
// (the sending tab with its request id), and to nobody else. Emojis,
// including multi-code-point ones, come back unchanged from the database.
func TestPrivateMessageDelivery(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.Routes())
	defer srv.Close()
	ann, annID := register(t, srv.URL, "ann@example.com", "Ann")
	ben, benID := register(t, srv.URL, "ben@example.com", "Ben")
	cal, calID := register(t, srv.URL, "cal@example.com", "Cal")
	ann.do("POST", fmt.Sprintf("/api/users/%d/follow", benID), "", nil)
	cal.do("POST", fmt.Sprintf("/api/users/%d/follow", annID), "", nil) // Cal may chat with Ann, not with Ben

	annTab1 := connect(t, app, srv, ann, annID)
	annTab2 := connect(t, app, srv, ann, annID)
	benTab1 := connect(t, app, srv, ben, benID)
	benTab2 := connect(t, app, srv, ben, benID)
	calTab := connect(t, app, srv, cal, calID)

	text := "family 👨‍👩‍👧 flag 🇸🇦 and é"
	sendPM(t, annTab1, "abc", benID, "  "+text+"  ")

	check := func(name string, conn *websocket.Conn, wantReqID string) {
		t.Helper()
		ev, ok := next(conn, 2*time.Second, "private_message")
		if !ok {
			t.Fatalf("%s got no private_message", name)
		}
		sender, _ := ev.Payload["sender"].(map[string]any)
		if ev.Payload["content"] != text || ev.Payload["recipient_id"] != float64(benID) || sender["id"] != float64(annID) ||
			ev.Payload["id"] == nil || ev.Payload["created_at"] == nil || ev.Payload["read_at"] != nil || ev.RequestID != wantReqID {
			t.Errorf("%s got %+v", name, ev)
		}
	}
	check("ann's sending tab", annTab1, "abc")
	check("ann's other tab", annTab2, "")
	check("ben tab 1", benTab1, "")
	check("ben tab 2", benTab2, "")
	if ev, ok := next(calTab, 300*time.Millisecond, "private_message"); ok {
		t.Errorf("third user received %+v", ev)
	}

	var stored string
	app.DB.QueryRow(`SELECT content FROM messages`).Scan(&stored)
	if stored != text {
		t.Errorf("stored %q, want %q (trimmed, emojis intact)", stored, text)
	}

	// The same text over REST, after "reload".
	_, history := ben.list(fmt.Sprintf("/api/messages/%d", annID))
	if len(history) != 1 || history[0]["content"] != text {
		t.Errorf("history = %v", history)
	}
}

// REST: conversations with unread counts, paged history (newest first),
// marking read, chat contacts, and no access to other people's conversations.
func TestPrivateMessageREST(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.Routes())
	defer srv.Close()
	ann, annID := register(t, srv.URL, "ann@example.com", "Ann")
	ben, benID := register(t, srv.URL, "ben@example.com", "Ben")
	cal, calID := register(t, srv.URL, "cal@example.com", "Cal")
	dee, _ := register(t, srv.URL, "dee@example.com", "Dee")
	ann.do("POST", fmt.Sprintf("/api/users/%d/follow", benID), "", nil)
	cal.do("POST", fmt.Sprintf("/api/users/%d/follow", annID), "", nil)

	// Ben is offline the whole time: Ann's 55 messages are stored for him.
	annConn := connect(t, app, srv, ann, annID)
	for i := 0; i < 55; i++ {
		sendPM(t, annConn, fmt.Sprint(i), benID, fmt.Sprintf("m%d", i))
		if _, ok := next(annConn, 2*time.Second, "private_message"); !ok {
			t.Fatalf("message %d not confirmed", i)
		}
	}
	calConn := connect(t, app, srv, cal, calID)
	sendPM(t, calConn, "c", annID, "hello ann")
	next(calConn, 2*time.Second, "private_message")

	// Ben logs in later: unread count and the conversation are waiting.
	if _, body := ben.get("/api/messages/unread-count"); body["unread"] != float64(55) {
		t.Errorf("ben unread = %v, want 55", body["unread"])
	}
	_, convs := ben.list("/api/conversations")
	if len(convs) != 1 {
		t.Fatalf("ben conversations = %v", convs)
	}
	if u := convs[0]["user"].(map[string]any); u["id"] != float64(annID) || convs[0]["unread_count"] != float64(55) || convs[0]["can_message"] != true {
		t.Errorf("ben's conversation = %v", convs[0])
	}
	if last := convs[0]["last_message"].(map[string]any); last["content"] != "m54" {
		t.Errorf("last message = %v", last)
	}

	// History: newest first, 50 per page, then the rest with ?before=.
	_, page1 := ben.list(fmt.Sprintf("/api/messages/%d", annID))
	if len(page1) != 50 || page1[0]["content"] != "m54" || page1[49]["content"] != "m5" {
		t.Fatalf("page 1: %d messages, first %v", len(page1), page1[0]["content"])
	}
	last := int64(page1[49]["id"].(float64))
	_, page2 := ben.list(fmt.Sprintf("/api/messages/%d?before=%d", annID, last))
	if len(page2) != 5 || page2[4]["content"] != "m0" {
		t.Errorf("page 2: %d messages", len(page2))
	}

	// Ann's conversations: Cal's is newest; Ann has 1 unread (from Cal).
	_, convs = ann.list("/api/conversations")
	if len(convs) != 2 || convs[0]["user"].(map[string]any)["id"] != float64(calID) || convs[0]["unread_count"] != float64(1) || convs[1]["unread_count"] != float64(0) {
		t.Errorf("ann conversations = %v", convs)
	}

	// Marking read: Ben's other tab is told, the count drops to 0, read_at is set.
	benConn := connect(t, app, srv, ben, benID)
	if status, _ := ben.do("POST", fmt.Sprintf("/api/messages/%d/read", annID), "", nil); status != http.StatusNoContent {
		t.Fatalf("mark read = %d", status)
	}
	if ev, ok := next(benConn, 2*time.Second, "messages_read"); !ok || ev.Payload["user_id"] != float64(annID) {
		t.Errorf("messages_read event = %+v (ok %v)", ev, ok)
	}
	if _, body := ben.get("/api/messages/unread-count"); body["unread"] != float64(0) {
		t.Errorf("unread after read = %v", body["unread"])
	}
	if _, h := ann.list(fmt.Sprintf("/api/messages/%d", benID)); h[0]["read_at"] == nil {
		t.Errorf("ann does not see read_at on her message: %v", h[0])
	}

	// Chat contacts: whoever I follow or who follows me (accepted).
	_, contacts := ann.list("/api/chat-contacts")
	if fmt.Sprint(ids(contacts)) != fmt.Sprint([]int64{benID, calID}) {
		t.Errorf("ann contacts = %v", ids(contacts))
	}
	if _, contacts := dee.list("/api/chat-contacts"); len(contacts) != 0 {
		t.Errorf("dee contacts = %v", contacts)
	}

	// No leaks: Dee (no follow, no messages) can't open Ann's or Ben's
	// history, and Cal asking for Ben gets only Cal-Ben (nothing, forbidden).
	for name, path := range map[string]string{
		"dee -> ann": fmt.Sprintf("/api/messages/%d", annID),
		"dee -> ben": fmt.Sprintf("/api/messages/%d", benID),
	} {
		if status, _ := dee.list(path); status != http.StatusForbidden {
			t.Errorf("%s history = %d, want 403", name, status)
		}
	}
	if status, _ := cal.list(fmt.Sprintf("/api/messages/%d", benID)); status != http.StatusForbidden {
		t.Errorf("cal reading ben's history = %d, want 403", status)
	}
	if _, convs := dee.list("/api/conversations"); len(convs) != 0 {
		t.Errorf("dee conversations = %v", convs)
	}
	for _, path := range []string{"/api/messages/99999", "/api/messages/abc"} {
		if status, _ := ann.list(path); status != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", path, status)
		}
	}
	if status, _ := ann.list(fmt.Sprintf("/api/messages/%d", annID)); status != http.StatusBadRequest {
		t.Errorf("own history = %d, want 400", status)
	}

	// After an unfollow the old conversation is still readable by both.
	ann.do("DELETE", fmt.Sprintf("/api/users/%d/follow", benID), "", nil)
	if status, h := ben.list(fmt.Sprintf("/api/messages/%d", annID)); status != http.StatusOK || len(h) != 50 {
		t.Errorf("history after unfollow = %d, %d messages", status, len(h))
	}
	if _, convs := ben.list("/api/conversations"); convs[0]["can_message"] != false {
		t.Errorf("can_message after unfollow = %v", convs[0]["can_message"])
	}

	anon := newClient(t, srv.URL)
	for _, route := range [][2]string{
		{"GET", "/api/conversations"}, {"GET", "/api/chat-contacts"}, {"GET", "/api/messages/unread-count"},
		{"GET", fmt.Sprintf("/api/messages/%d", annID)}, {"POST", fmt.Sprintf("/api/messages/%d/read", annID)},
	} {
		if status, _ := anon.do(route[0], route[1], "", nil); status != http.StatusUnauthorized {
			t.Errorf("%s %s without a session = %d, want 401", route[0], route[1], status)
		}
	}
}
