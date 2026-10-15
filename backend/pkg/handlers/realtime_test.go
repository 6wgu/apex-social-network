package handlers

import (
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"social-network/pkg/ws"
)

// Frames that are not a valid envelope (text, binary, JSON that is not an
// object, invalid UTF-8) are answered with an error event and the
// connection keeps working.
func TestGarbageFramesKeepTheConnection(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.Routes())
	defer srv.Close()
	ann, annID := register(t, srv.URL, "ann@example.com", "Ann")
	_, benID := register(t, srv.URL, "ben@example.com", "Ben")
	ann.do("POST", fmt.Sprintf("/api/users/%d/follow", benID), "", nil)
	conn := connect(t, app, srv, ann, annID)

	garbage := []struct {
		kind int
		data []byte
	}{
		{websocket.TextMessage, []byte("not json at all")},
		{websocket.TextMessage, []byte(`[1, 2, 3]`)},
		{websocket.TextMessage, []byte(`"just a string"`)},
		{websocket.TextMessage, []byte(`null`)},
		{websocket.TextMessage, []byte(`{"type": 42}`)},
		{websocket.TextMessage, []byte(`{"type": "private_message", "payload": "not an object"}`)},
		{websocket.BinaryMessage, []byte{0xff, 0x00, 0xfe, 0x01}},
	}
	for i, g := range garbage {
		if err := conn.WriteMessage(g.kind, g.data); err != nil {
			t.Fatal(err)
		}
		if ev, ok := next(conn, 2*time.Second, "error"); !ok {
			t.Fatalf("garbage %d (%q): no error event (ok %v, %+v)", i, g.data, ok, ev)
		}
	}
	// Invalid UTF-8 inside an otherwise valid frame is not valid JSON: the
	// frame is rejected before its request id can be read, and not stored.
	bad := fmt.Sprintf(`{"type":"private_message","request_id":"u","payload":{"recipient_id":%d,"content":"bad \xff\xfe bytes"}}`, benID)
	conn.WriteMessage(websocket.TextMessage, []byte(bad))
	if ev, ok := next(conn, 2*time.Second, "error"); !ok || !strings.Contains(fmt.Sprint(ev.Payload["message"]), "must be JSON") {
		t.Errorf("invalid UTF-8: got %+v (ok %v), want the JSON error", ev, ok)
	}
	// Still connected and working.
	sendPM(t, conn, "after", benID, "still here")
	if ev, ok := reply(conn, "after"); !ok || ev.Type != "private_message" {
		t.Fatalf("after garbage: %+v (ok %v)", ev, ok)
	}
	if n := messageCount(t, app); n != 1 {
		t.Errorf("%d messages stored, want 1", n)
	}
}

// A frame bigger than MaxFrameSize closes that connection with code 1009;
// the hub forgets it and the user can connect again.
func TestOversizedFrameClosesTheConnection(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.Routes())
	defer srv.Close()
	ann, annID := register(t, srv.URL, "ann@example.com", "Ann")
	conn := connect(t, app, srv, ann, annID)

	big := `{"type":"private_message","payload":{"recipient_id":2,"content":"` + strings.Repeat("x", ws.MaxFrameSize) + `"}}`
	if err := conn.WriteMessage(websocket.TextMessage, []byte(big)); err != nil {
		t.Fatal(err)
	}
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var err error
	for err == nil {
		_, _, err = conn.ReadMessage()
	}
	var closeErr *websocket.CloseError
	if !errors.As(err, &closeErr) || closeErr.Code != websocket.CloseMessageTooBig {
		t.Errorf("after an oversized frame: %v, want close 1009 (message too big)", err)
	}
	waitFor(t, func() bool { return app.Hub.Connections(annID) == 0 })
	if n := messageCount(t, app); n != 0 {
		t.Errorf("%d messages stored", n)
	}
	connect(t, app, srv, ann, annID) // reconnecting works
}

// More than the allowed frames per second get "too many messages" errors
// and are not processed; the connection stays open and works again once
// the rate drops.
func TestRateLimit(t *testing.T) {
	app := newTestApp(t)
	app.Hub.SetRateLimit(ws.DefaultRateLimit, ws.DefaultRateBurst) // the production values
	srv := httptest.NewServer(app.Routes())
	defer srv.Close()
	ann, annID := register(t, srv.URL, "ann@example.com", "Ann")
	_, benID := register(t, srv.URL, "ben@example.com", "Ben")
	ann.do("POST", fmt.Sprintf("/api/users/%d/follow", benID), "", nil)
	conn := connect(t, app, srv, ann, annID)

	const burst = 30
	start := time.Now()
	for i := 0; i < burst; i++ {
		sendPM(t, conn, fmt.Sprint(i), benID, fmt.Sprintf("flood %d", i))
	}
	stored, limited := 0, 0
	for i := 0; i < burst; i++ {
		ev, ok := reply(conn, fmt.Sprint(i))
		if !ok {
			t.Fatalf("no answer to frame %d", i)
		}
		switch {
		case ev.Type == "private_message":
			stored++
		case ev.Type == "error" && ev.Payload["message"] == "too many messages: slow down":
			limited++
		default:
			t.Fatalf("frame %d: unexpected %+v", i, ev)
		}
	}
	allowed := ws.DefaultRateBurst + int(time.Since(start).Seconds()*ws.DefaultRateLimit) + 1
	if stored > allowed || limited == 0 || stored+limited != burst {
		t.Errorf("stored %d, limited %d of %d (at most %d may pass)", stored, limited, burst, allowed)
	}
	if n := messageCount(t, app); n != stored {
		t.Errorf("%d rows for %d confirmed messages", n, stored)
	}

	time.Sleep(1100 * time.Millisecond) // tokens refill
	sendPM(t, conn, "later", benID, "calm again")
	if ev, ok := reply(conn, "later"); !ok || ev.Type != "private_message" {
		t.Errorf("after waiting: %+v (ok %v)", ev, ok)
	}
}

// A client that disappears while events are being pushed to it (its TCP
// connection is cut, no close frame) never crashes or blocks the hub:
// other tabs and users keep receiving, and the dead tab is dropped. Run
// with -race.
func TestHubSurvivesAClientVanishingMidSend(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.Routes())
	defer srv.Close()
	ann, annID := register(t, srv.URL, "ann@example.com", "Ann")
	ben, benID := register(t, srv.URL, "ben@example.com", "Ben")
	ann.do("POST", fmt.Sprintf("/api/users/%d/follow", benID), "", nil)

	doomed := connect(t, app, srv, ann, annID)
	survivor := connect(t, app, srv, ann, annID)
	benConn := connect(t, app, srv, ben, benID)

	// Flood Ann from several goroutines while her first tab is cut off.
	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				app.Hub.SendToUser(annID, "notification", map[string]any{"n": i, "g": g})
			}
		}(g)
	}
	time.Sleep(5 * time.Millisecond)
	doomed.NetConn().Close() // gone, mid-stream, without a close frame
	wg.Wait()
	waitFor(t, func() bool { return app.Hub.Connections(annID) <= 1 })

	// Ben's real message still reaches Ann's surviving tab... unless that
	// tab was dropped as too slow during the flood, in which case Ann
	// simply reconnects, like the page does.
	if app.Hub.Connections(annID) == 0 {
		survivor = connect(t, app, srv, ann, annID)
	}
	sendPM(t, benConn, "b1", annID, "you there?")
	if ev, ok := reply(benConn, "b1"); !ok || ev.Type != "private_message" {
		t.Fatalf("ben's send after the flood: %+v (ok %v)", ev, ok)
	}
	got := false
	for !got {
		ev, ok := next(survivor, 3*time.Second, "private_message")
		if !ok {
			break
		}
		got = ev.Payload["content"] == "you there?"
	}
	if !got {
		t.Error("ann's surviving tab did not get ben's message")
	}
	// And a brand new connection works.
	connect(t, app, srv, ann, annID)
}
