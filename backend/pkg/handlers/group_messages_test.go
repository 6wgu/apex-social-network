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

// sendGM sends a group_message frame.
func sendGM(t *testing.T, conn *websocket.Conn, requestID string, groupID int64, content string) {
	t.Helper()
	frame := map[string]any{"type": "group_message", "request_id": requestID,
		"payload": map[string]any{"group_id": groupID, "content": content}}
	if err := conn.WriteJSON(frame); err != nil {
		t.Fatal(err)
	}
}

func groupMessageCount(t *testing.T, app *App) int {
	t.Helper()
	var n int
	if err := app.DB.QueryRow(`SELECT COUNT(*) FROM group_messages`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// Only members can send: a stranger, an invited user who has not accepted
// and a user whose join request is pending all get an error and nothing is
// stored. Membership is checked on every message.
func TestGroupMessageMembershipRule(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.Routes())
	defer srv.Close()
	creator, creatorID := register(t, srv.URL, "creator@example.com", "Creator")
	ann, annID := register(t, srv.URL, "ann@example.com", "Ann")
	ben, benID := register(t, srv.URL, "ben@example.com", "Ben")
	gid := createGroup(t, creator, "Hikers")
	invite(t, creator, gid, annID)                                      // Ann invited, not accepted yet
	ben.do("POST", fmt.Sprintf("/api/groups/%d/request", gid), "", nil) // Ben asked, not accepted yet

	creatorConn := connect(t, app, srv, creator, creatorID)
	annConn := connect(t, app, srv, ann, annID)
	benConn := connect(t, app, srv, ben, benID)

	for name, conn := range map[string]*websocket.Conn{"invited ann": annConn, "requesting ben": benConn} {
		sendGM(t, conn, "x", gid, "let me in")
		if ev, ok := reply(conn, "x"); !ok || ev.Type != "error" || ev.Payload["message"] != "only members can send messages to this group" || ev.Payload["for"] != "group_message" {
			t.Errorf("%s: got %+v (ok %v), want the members-only error", name, ev, ok)
		}
	}
	sendGM(t, annConn, "y", 99999, "hello")
	if ev, ok := reply(annConn, "y"); !ok || ev.Type != "error" || ev.Payload["message"] != "group not found" {
		t.Errorf("unknown group: got %+v", ev)
	}
	if n := groupMessageCount(t, app); n != 0 {
		t.Fatalf("%d group messages stored from non-members", n)
	}

	// Validation is the same as for private messages.
	for _, tc := range []struct{ content, want string }{
		{"   ", "can't be empty"}, {strings.Repeat("x", 2001), "2000 characters or fewer"},
	} {
		sendGM(t, creatorConn, "v", gid, tc.content)
		if ev, ok := reply(creatorConn, "v"); !ok || ev.Type != "error" || !strings.Contains(fmt.Sprint(ev.Payload["message"]), tc.want) {
			t.Errorf("content %.10q: got %+v", tc.content, ev)
		}
	}

	// Ann accepts the invitation: from now on she can send.
	_, g := ann.get(fmt.Sprintf("/api/groups/%d", gid))
	ann.do("POST", fmt.Sprintf("/api/group-invites/%d/accept", int64(g["invite_id"].(float64))), "", nil)
	sendGM(t, annConn, "z", gid, "hi everyone")
	if ev, ok := reply(annConn, "z"); !ok || ev.Type != "group_message" || ev.Payload["content"] != "hi everyone" {
		t.Errorf("member after joining: got %+v (ok %v)", ev, ok)
	}
	if n := groupMessageCount(t, app); n != 1 {
		t.Errorf("%d group messages stored, want 1", n)
	}
}

// A stored message reaches every tab of every member (the sending tab with
// its request id) and nobody else.
func TestGroupMessageDelivery(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.Routes())
	defer srv.Close()
	creator, creatorID := register(t, srv.URL, "creator@example.com", "Creator")
	ann, annID := register(t, srv.URL, "ann@example.com", "Ann")
	ben, benID := register(t, srv.URL, "ben@example.com", "Ben")
	out, outID := register(t, srv.URL, "out@example.com", "Out")
	gid := createGroup(t, creator, "Readers")
	joinByInvite(t, creator, ann, gid, annID)
	joinByInvite(t, creator, ben, gid, benID)
	other := createGroup(t, out, "Other group") // Out is a member elsewhere only

	creatorTab := connect(t, app, srv, creator, creatorID)
	annTab1 := connect(t, app, srv, ann, annID)
	annTab2 := connect(t, app, srv, ann, annID)
	benTab := connect(t, app, srv, ben, benID)
	outTab := connect(t, app, srv, out, outID)

	text := "meeting at 6 \U0001F555 \U0001F468‍\U0001F469‍\U0001F467 \U0001F1EF\U0001F1F5"
	sendGM(t, annTab1, "g1", gid, text)
	for name, tc := range map[string]struct {
		conn  *websocket.Conn
		reqID string
	}{"ann sending tab": {annTab1, "g1"}, "ann other tab": {annTab2, ""}, "creator": {creatorTab, ""}, "ben": {benTab, ""}} {
		ev, ok := next(tc.conn, 2*time.Second, "group_message")
		sender, _ := ev.Payload["sender"].(map[string]any)
		if !ok || ev.Payload["content"] != text || ev.Payload["group_id"] != float64(gid) || sender["id"] != float64(annID) || ev.RequestID != tc.reqID || ev.Payload["id"] == nil {
			t.Errorf("%s got %+v (ok %v)", name, ev, ok)
		}
	}
	// Out's own group still works, and Out never sees the Readers chat.
	sendGM(t, outTab, "o1", other, "only me here")
	if ev, ok := reply(outTab, "o1"); !ok || ev.Type != "group_message" || ev.Payload["group_id"] != float64(other) {
		t.Fatalf("out in own group: %+v", ev)
	}
	if ev, ok := next(outTab, 300*time.Millisecond, "group_message"); ok {
		t.Errorf("non-member received %+v", ev)
	}
}

// History: members only, newest first, paged; emojis unchanged; a new
// member sees what was said before they joined.
func TestGroupMessageHistory(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.Routes())
	defer srv.Close()
	creator, creatorID := register(t, srv.URL, "creator@example.com", "Creator")
	late, lateID := register(t, srv.URL, "late@example.com", "Late")
	out, _ := register(t, srv.URL, "out@example.com", "Out")
	gid := createGroup(t, creator, "Chess")
	conn := connect(t, app, srv, creator, creatorID)
	for i := 0; i < 55; i++ {
		sendGM(t, conn, fmt.Sprint(i), gid, fmt.Sprintf("move %d ♟️", i))
		if _, ok := reply(conn, fmt.Sprint(i)); !ok {
			t.Fatalf("message %d not confirmed", i)
		}
	}
	path := fmt.Sprintf("/api/groups/%d/messages", gid)

	if status, body := out.do("GET", path, "", nil); status != http.StatusForbidden || body["error"] == nil {
		t.Errorf("non-member history = %d %v, want 403", status, body)
	}
	if status, _ := out.list("/api/groups/99999/messages"); status != http.StatusNotFound {
		t.Errorf("unknown group history = %d, want 404", status)
	}

	joinByInvite(t, creator, late, gid, lateID)
	_, page1 := late.list(path)
	if len(page1) != 50 || page1[0]["content"] != "move 54 ♟️" || page1[49]["content"] != "move 5 ♟️" {
		t.Fatalf("page 1: %d, first %v", len(page1), page1[0]["content"])
	}
	_, page2 := late.list(fmt.Sprintf("%s?before=%d", path, int64(page1[49]["id"].(float64))))
	if len(page2) != 5 || page2[4]["content"] != "move 0 ♟️" {
		t.Errorf("page 2: %d", len(page2))
	}
	if sender, _ := page1[0]["sender"].(map[string]any); sender["first_name"] != "Creator" || sender["email"] != nil {
		t.Errorf("sender = %v", page1[0]["sender"])
	}
	if status, _ := late.list(path + "?before=x"); status != http.StatusBadRequest {
		t.Errorf("before=x = %d, want 400", status)
	}
	if status, _ := newClient(t, srv.URL).list(path); status != http.StatusUnauthorized {
		t.Errorf("anonymous history = %d, want 401", status)
	}
}
