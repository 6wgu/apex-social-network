package handlers

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// sendTypingFrame sends a typing frame with the given payload.
func sendTypingFrame(t *testing.T, conn *websocket.Conn, requestID string, payload map[string]any) {
	t.Helper()
	if err := conn.WriteJSON(map[string]any{"type": "typing", "request_id": requestID, "payload": payload}); err != nil {
		t.Fatal(err)
	}
}

// typingReply returns the answer to one request id (typing or error).
func typingReply(conn *websocket.Conn, requestID string) (wsEvent, bool) {
	for {
		ev, ok := next(conn, 2*time.Second, "typing", "error")
		if !ok || ev.RequestID == requestID {
			return ev, ok
		}
	}
}

func typingUserID(ev wsEvent) any {
	u, _ := ev.Payload["user"].(map[string]any)
	return u["id"]
}

// Private chat: typing reaches the recipient (with who is typing), not the
// sender's other tabs, and never someone the sender may not message.
func TestTypingPrivate(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.Routes())
	defer srv.Close()
	ann, annID := register(t, srv.URL, "ann@example.com", "Ann")
	ben, benID := register(t, srv.URL, "ben@example.com", "Ben")
	carl, carlID := register(t, srv.URL, "carl@example.com", "Carl")
	ann.do("POST", fmt.Sprintf("/api/users/%d/follow", benID), "", nil) // Ben is public: instant

	annTab1 := connect(t, app, srv, ann, annID)
	annTab2 := connect(t, app, srv, ann, annID)
	benTab := connect(t, app, srv, ben, benID)
	carlTab := connect(t, app, srv, carl, carlID)

	sendTypingFrame(t, annTab1, "t1", map[string]any{"recipient_id": benID})
	if ev, ok := typingReply(annTab1, "t1"); !ok || ev.Type != "typing" {
		t.Fatalf("ann's answer: %+v (ok %v)", ev, ok)
	}
	ev, ok := next(benTab, 2*time.Second, "typing")
	if !ok || typingUserID(ev) != float64(annID) || ev.Payload["recipient_id"] != float64(benID) || ev.RequestID != "" || ev.Payload["group_id"] != nil {
		t.Fatalf("ben got %+v (ok %v), want ann typing to him", ev, ok)
	}
	if u, _ := ev.Payload["user"].(map[string]any); u["first_name"] != "Ann" {
		t.Errorf("typing user = %v, want Ann's summary", u)
	}

	// Ann and Carl do not follow each other: refused, and Carl hears nothing.
	sendTypingFrame(t, annTab1, "t2", map[string]any{"recipient_id": carlID})
	if ev, ok := typingReply(annTab1, "t2"); !ok || ev.Type != "error" || ev.Payload["message"] != errNotAllowedToMessage || ev.Payload["for"] != "typing" {
		t.Errorf("typing to a stranger: %+v (ok %v), want the follow-rule error", ev, ok)
	}
	sendTypingFrame(t, carlTab, "t3", map[string]any{"recipient_id": benID})
	if ev, ok := typingReply(carlTab, "t3"); !ok || ev.Type != "error" || ev.Payload["message"] != errNotAllowedToMessage {
		t.Errorf("carl typing to ben: %+v (ok %v), want the follow-rule error", ev, ok)
	}
	if ev, ok := next(carlTab, 300*time.Millisecond, "typing"); ok {
		t.Errorf("a stranger received %+v", ev)
	}
	if ev, ok := next(annTab2, 300*time.Millisecond, "typing"); ok {
		t.Errorf("the sender's other tab received %+v", ev)
	}
	if ev, ok := next(benTab, 300*time.Millisecond, "typing"); ok {
		t.Errorf("ben received a refused typing event %+v", ev)
	}
	if n := messageCount(t, app); n != 0 {
		t.Errorf("typing stored %d messages", n)
	}
}

// Group chat: typing reaches the other members only; a non-member can
// neither send it nor receive it.
func TestTypingGroup(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.Routes())
	defer srv.Close()
	creator, creatorID := register(t, srv.URL, "creator@example.com", "Creator")
	ann, annID := register(t, srv.URL, "ann@example.com", "Ann")
	out, outID := register(t, srv.URL, "out@example.com", "Out")
	gid := createGroup(t, creator, "Hikers")
	joinByInvite(t, creator, ann, gid, annID)

	creatorTab := connect(t, app, srv, creator, creatorID)
	annTab := connect(t, app, srv, ann, annID)
	outTab := connect(t, app, srv, out, outID)

	sendTypingFrame(t, annTab, "g1", map[string]any{"group_id": gid})
	if ev, ok := typingReply(annTab, "g1"); !ok || ev.Type != "typing" {
		t.Fatalf("ann's answer: %+v (ok %v)", ev, ok)
	}
	ev, ok := next(creatorTab, 2*time.Second, "typing")
	if !ok || typingUserID(ev) != float64(annID) || ev.Payload["group_id"] != float64(gid) || ev.Payload["recipient_id"] != nil {
		t.Fatalf("creator got %+v (ok %v), want ann typing in the group", ev, ok)
	}

	sendTypingFrame(t, outTab, "g2", map[string]any{"group_id": gid})
	if ev, ok := typingReply(outTab, "g2"); !ok || ev.Type != "error" || ev.Payload["message"] != "only members can send messages to this group" {
		t.Errorf("non-member typing: %+v (ok %v), want the members-only error", ev, ok)
	}
	sendTypingFrame(t, outTab, "g3", map[string]any{"group_id": 99999})
	if ev, ok := typingReply(outTab, "g3"); !ok || ev.Type != "error" || ev.Payload["message"] != "group not found" {
		t.Errorf("unknown group: %+v (ok %v)", ev, ok)
	}

	sendTypingFrame(t, creatorTab, "g4", map[string]any{"group_id": gid})
	if ev, ok := next(annTab, 2*time.Second, "typing"); !ok || typingUserID(ev) != float64(creatorID) {
		t.Errorf("ann got %+v (ok %v), want the creator typing", ev, ok)
	}
	if ev, ok := next(outTab, 300*time.Millisecond, "typing"); ok {
		t.Errorf("non-member received %+v", ev)
	}
	if ev, ok := next(creatorTab, 2*time.Second, "typing"); !ok || ev.RequestID != "g4" {
		t.Errorf("creator's answer: %+v (ok %v)", ev, ok)
	}
	if n := groupMessageCount(t, app); n != 0 {
		t.Errorf("typing stored %d group messages", n)
	}
}

// Wrongly shaped typing frames are rejected with an error.
func TestTypingValidation(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.Routes())
	defer srv.Close()
	ann, annID := register(t, srv.URL, "ann@example.com", "Ann")
	conn := connect(t, app, srv, ann, annID)

	cases := []struct {
		name, frame, want string
	}{
		{"no payload", `{"type":"typing","request_id":"x"}`, "payload must be"},
		{"empty payload", `{"type":"typing","request_id":"x","payload":{}}`, "payload must be"},
		{"both targets", `{"type":"typing","request_id":"x","payload":{"recipient_id":2,"group_id":3}}`, "payload must be"},
		{"negative id", `{"type":"typing","request_id":"x","payload":{"recipient_id":-2}}`, "payload must be"},
		{"wrong type", `{"type":"typing","request_id":"x","payload":{"recipient_id":"2"}}`, "payload must be"},
		{"extra field", `{"type":"typing","request_id":"x","payload":{"recipient_id":2,"content":"hi"}}`, "payload must be"},
		{"myself", fmt.Sprintf(`{"type":"typing","request_id":"x","payload":{"recipient_id":%d}}`, annID), "yourself"},
		{"unknown user", `{"type":"typing","request_id":"x","payload":{"recipient_id":99999}}`, "user not found"},
	}
	for _, tc := range cases {
		if err := conn.WriteMessage(websocket.TextMessage, []byte(tc.frame)); err != nil {
			t.Fatal(err)
		}
		ev, ok := next(conn, 2*time.Second, "error", "typing")
		if !ok || ev.Type != "error" || ev.Payload["for"] != "typing" || !strings.Contains(fmt.Sprint(ev.Payload["message"]), tc.want) {
			t.Errorf("%s: got %+v (ok %v), want error containing %q", tc.name, ev, ok, tc.want)
		}
	}
}
