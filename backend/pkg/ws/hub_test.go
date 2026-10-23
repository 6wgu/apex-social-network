package ws

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

const testOrigin = "http://localhost:3000"

func newTestServer(t *testing.T, h *Hub) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.ParseInt(r.URL.Query().Get("user"), 10, 64)
		h.Serve(w, r, id)
	}))
	t.Cleanup(srv.Close)
	return "ws" + strings.TrimPrefix(srv.URL, "http")
}

func dial(t *testing.T, h *Hub, url string, userID int64) *websocket.Conn {
	t.Helper()
	before := h.Connections(userID)
	conn, _, err := websocket.DefaultDialer.Dial(url+"?user="+strconv.FormatInt(userID, 10),
		http.Header{"Origin": {testOrigin}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	waitFor(t, func() bool { return h.Connections(userID) == before+1 })
	return conn
}

func waitFor(t *testing.T, ok func() bool) {
	t.Helper()
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		if ok() {
			return
		}
	}
	t.Fatal("timed out waiting")
}

func read(t *testing.T, conn *websocket.Conn) Event {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var ev Event
	if err := conn.ReadJSON(&ev); err != nil {
		t.Fatal(err)
	}
	return ev
}

func nothingFor(t *testing.T, conn *websocket.Conn) {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	if _, data, err := conn.ReadMessage(); err == nil {
		t.Fatalf("unexpected event %s", data)
	}
}

func TestParseOrigins(t *testing.T) {
	got := ParseOrigins(" http://localhost:3000/ , ,http://127.0.0.1:3000")
	if strings.Join(got, " ") != "http://localhost:3000 http://127.0.0.1:3000" {
		t.Fatalf("ParseOrigins = %q", got)
	}
}

func TestAllowIsATokenBucket(t *testing.T) {
	h := NewHub()
	h.SetRateLimit(2, 3)
	c := &Client{hub: h}
	now := time.Now()
	for i := range 3 {
		if !c.allow(now) {
			t.Fatalf("frame %d of the burst was refused", i+1)
		}
	}
	if c.allow(now) {
		t.Fatal("a frame over the burst was allowed")
	}
	if !c.allow(now.Add(500 * time.Millisecond)) {
		t.Fatal("no token came back after half a second")
	}
	if c.allow(now.Add(500 * time.Millisecond)) {
		t.Fatal("two frames allowed on one refilled token")
	}
}

func TestOtherOriginsAreRefused(t *testing.T) {
	url := newTestServer(t, NewHub(testOrigin))
	_, res, err := websocket.DefaultDialer.Dial(url+"?user=1", http.Header{"Origin": {"http://evil.example"}})
	if err == nil {
		t.Fatal("connected from another origin")
	}
	if res == nil || res.StatusCode != http.StatusForbidden {
		t.Fatalf("response = %v, want 403", res)
	}
}

func TestSendToUserReachesEveryTabOfThatUserOnly(t *testing.T) {
	h := NewHub(testOrigin)
	url := newTestServer(t, h)
	tab1, tab2 := dial(t, h, url, 1), dial(t, h, url, 1)
	other := dial(t, h, url, 2)

	h.SendToUser(1, "hello", map[string]int{"n": 1})
	for _, tab := range []*websocket.Conn{tab1, tab2} {
		if ev := read(t, tab); ev.Type != "hello" {
			t.Fatalf("event type = %q, want hello", ev.Type)
		}
	}
	nothingFor(t, other)
}

func TestFramesReachTheHandlerAndRepliesEchoTheRequestID(t *testing.T) {
	h := NewHub(testOrigin)
	h.SetHandler(func(c *Client, in Incoming) {
		h.Reply(c, "echo", in.RequestID, map[string]any{"user": c.UserID(), "payload": in.Payload})
	})
	url := newTestServer(t, h)
	conn := dial(t, h, url, 7)

	conn.WriteJSON(map[string]any{"type": "ping", "request_id": "r1", "payload": map[string]string{"x": "y"}})
	ev := read(t, conn)
	if ev.Type != "echo" || ev.RequestID != "r1" {
		t.Fatalf("got %+v, want an echo for r1", ev)
	}
	got, _ := json.Marshal(ev.Payload)
	if string(got) != `{"payload":{"x":"y"},"user":7}` {
		t.Fatalf("payload = %s", got)
	}
}

func TestBadFramesGetAnErrorAndTheConnectionStays(t *testing.T) {
	h := NewHub(testOrigin)
	h.SetHandler(func(c *Client, in Incoming) { h.Reply(c, "ok", in.RequestID, nil) })
	url := newTestServer(t, h)
	conn := dial(t, h, url, 1)

	for _, frame := range []string{"not json", `{"payload":{}}`} {
		conn.WriteMessage(websocket.TextMessage, []byte(frame))
		if ev := read(t, conn); ev.Type != EventError {
			t.Fatalf("frame %q: got %q, want an error event", frame, ev.Type)
		}
	}
	conn.WriteJSON(map[string]string{"type": "x", "request_id": "r2"})
	if ev := read(t, conn); ev.Type != "ok" || ev.RequestID != "r2" {
		t.Fatalf("after bad frames: got %+v, want ok for r2", ev)
	}
}

func TestFramesOverTheRateLimitAreDropped(t *testing.T) {
	h := NewHub(testOrigin)
	h.SetRateLimit(0.001, 2)
	h.SetHandler(func(c *Client, in Incoming) { h.Reply(c, "ok", in.RequestID, nil) })
	url := newTestServer(t, h)
	conn := dial(t, h, url, 1)

	for _, id := range []string{"a", "b", "c"} {
		conn.WriteJSON(map[string]string{"type": "x", "request_id": id})
	}
	read(t, conn)
	read(t, conn)
	ev := read(t, conn)
	if ev.Type != EventError || ev.RequestID != "c" {
		t.Fatalf("third frame: got %+v, want an error for c", ev)
	}
}

func TestCloseUserAndDisconnects(t *testing.T) {
	h := NewHub(testOrigin)
	url := newTestServer(t, h)
	conn := dial(t, h, url, 1)
	stay := dial(t, h, url, 2)

	h.CloseUser(1)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, _, err := conn.ReadMessage(); !websocket.IsCloseError(err, websocket.CloseNoStatusReceived, websocket.CloseNormalClosure) {
		t.Fatalf("after CloseUser read err = %v, want a close", err)
	}
	if h.Connections(1) != 0 {
		t.Fatal("CloseUser left a connection in the hub")
	}

	stay.Close()
	waitFor(t, func() bool { return len(h.OnlineUsers()) == 0 })
}
