// Package ws keeps the live WebSocket connections of logged-in users, lets
// the rest of the backend push JSON events to them, and hands the frames
// browsers send (chat messages) to a handler set with SetHandler.
//
// Every connection gets two goroutines: readLoop owns reading (and notices
// when the browser goes away) and writeLoop owns writing, because a
// gorilla/websocket connection supports one reader and one writer at a time.
// Other code never touches a connection directly; it puts bytes on the
// client's send channel through the Hub.
package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// writeWait is how long one write may take before the connection is dropped.
	writeWait = 10 * time.Second
	// pongWait is how long we wait for any frame (a pong at least) from the browser.
	pongWait = 60 * time.Second
	// pingPeriod must be shorter than pongWait so a healthy browser always answers in time.
	pingPeriod = 30 * time.Second
	// maxMessageSize caps a frame sent by the browser.
	maxMessageSize = 64 << 10
	// sendBuffer is how many events may queue up for one slow connection.
	sendBuffer = 32
)

// Event is the envelope of every frame the server sends:
// {"type": "...", "payload": {...}}, plus "request_id" when it answers a
// frame the browser sent with one (so the page knows which send it was).
type Event struct {
	Type      string `json:"type"`
	RequestID string `json:"request_id,omitempty"`
	Payload   any    `json:"payload"`
}

// Incoming is a frame a browser sent: {"type", "request_id", "payload"}.
// request_id is optional and chosen by the browser; it is echoed back in the
// answer (the stored message, or an error).
type Incoming struct {
	Type      string          `json:"type"`
	RequestID string          `json:"request_id"`
	Payload   json.RawMessage `json:"payload"`
}

// Handler processes one incoming frame from c. It runs on c's reader
// goroutine, so frames from one tab are handled in order, one at a time.
type Handler func(c *Client, in Incoming)

// EventError is the type of the event sent back when a frame is rejected;
// its payload is ErrorPayload.
const EventError = "error"

// ErrorPayload says why a frame was rejected. For is the type of the
// rejected frame, when it could be read.
type ErrorPayload struct {
	Message string `json:"message"`
	For     string `json:"for,omitempty"`
}

// Client is one open connection (one browser tab) of one user. Handlers get
// it with each incoming frame, to answer that tab (Hub.Reply) or skip it
// (Hub.SendToUserExcept).
type Client struct {
	hub    *Hub
	userID int64
	conn   *websocket.Conn
	send   chan []byte
}

// Hub tracks every open connection, grouped by user, so one user with
// several tabs gets each event in every tab.
type Hub struct {
	upgrader websocket.Upgrader
	handler  Handler

	mu      sync.Mutex
	clients map[int64]map[*Client]bool
}

// ParseOrigins splits a comma-separated FRONTEND_ORIGIN value such as
// "http://localhost:3000, http://127.0.0.1:3000" into its origins, dropping
// blanks and any trailing slash.
func ParseOrigins(list string) []string {
	var origins []string
	for _, o := range strings.Split(list, ",") {
		if o = strings.TrimRight(strings.TrimSpace(o), "/"); o != "" {
			origins = append(origins, o)
		}
	}
	return origins
}

// NewHub returns a hub that accepts upgrades from browsers on one of
// allowedOrigins (the frontend, e.g. http://localhost:3000). Only exact
// matches pass; there is no wildcard. The Next.js proxy forwards the
// browser's Origin header unchanged, so these are the origins to allow.
// Requests without an Origin header are not from a browser page (curl,
// tests) and are allowed: the session cookie is still required.
func NewHub(allowedOrigins ...string) *Hub {
	allowed := map[string]bool{}
	for _, o := range allowedOrigins {
		allowed[o] = true
	}
	h := &Hub{clients: map[int64]map[*Client]bool{}}
	h.upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			return origin == "" || allowed[origin]
		},
	}
	return h
}

// Serve upgrades the request to a WebSocket for userID and runs the
// connection until it closes. The caller must already have checked the
// session (GET /ws is behind RequireAuth).
func (h *Hub) Serve(w http.ResponseWriter, r *http.Request, userID int64) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		// Upgrade has already written the error response (e.g. 403 for a bad origin).
		log.Printf("ws upgrade for user %d: %v", userID, err)
		return
	}
	c := &Client{hub: h, userID: userID, conn: conn, send: make(chan []byte, sendBuffer)}
	h.register(c)
	go c.writeLoop()
	c.readLoop() // returns when the connection is gone
}

// SetHandler sets the function that processes frames sent by browsers.
// Call it once, before connections are served.
func (h *Hub) SetHandler(fn Handler) {
	h.handler = fn
}

// UserID is the user this connection belongs to (from the session that
// opened it).
func (c *Client) UserID() int64 {
	return c.userID
}

// SendToUser delivers an event to every open connection of userID. It never
// blocks: a connection whose buffer is full is too slow and gets closed, and
// a user with no connection simply misses the push (the data is in the
// database, so the page reloads it).
func (h *Hub) SendToUser(userID int64, eventType string, payload any) {
	h.SendToUserExcept(userID, nil, eventType, payload)
}

// SendToUserExcept is SendToUser without the connection except (the tab
// that sent the frame being answered, which gets Reply instead).
func (h *Hub) SendToUserExcept(userID int64, except *Client, eventType string, payload any) {
	data, ok := marshal(Event{Type: eventType, Payload: payload})
	if !ok {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients[userID] {
		if c != except {
			h.enqueueLocked(c, data)
		}
	}
}

// Reply sends an event to the single connection c, echoing requestID.
func (h *Hub) Reply(c *Client, eventType, requestID string, payload any) {
	data, ok := marshal(Event{Type: eventType, RequestID: requestID, Payload: payload})
	if !ok {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[c.userID][c] { // a connection already gone gets nothing
		h.enqueueLocked(c, data)
	}
}

// ReplyError rejects a frame from c with an error event.
func (h *Hub) ReplyError(c *Client, in Incoming, message string) {
	h.Reply(c, EventError, in.RequestID, ErrorPayload{Message: message, For: in.Type})
}

func marshal(ev Event) ([]byte, bool) {
	data, err := json.Marshal(ev)
	if err != nil {
		log.Printf("ws marshal %q: %v", ev.Type, err)
		return nil, false
	}
	return data, true
}

// enqueueLocked queues data for c without blocking; a connection whose
// buffer is full is too slow and gets closed. h.mu must be held, which
// guarantees c.send is still open.
func (h *Hub) enqueueLocked(c *Client, data []byte) {
	select {
	case c.send <- data:
	default:
		h.removeLocked(c)
	}
}

// CloseUser closes every open connection of userID, e.g. on logout, so a tab
// that is still open stops receiving that user's events. Each writeLoop sends
// a close frame and closes its socket. A tab whose session is still valid
// (another browser) simply reconnects.
func (h *Hub) CloseUser(userID int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients[userID] {
		h.removeLocked(c)
	}
}

// Connections returns how many connections userID has open (used by tests).
func (h *Hub) Connections(userID int64) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients[userID])
}

func (h *Hub) register(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[c.userID] == nil {
		h.clients[c.userID] = map[*Client]bool{}
	}
	h.clients[c.userID][c] = true
}

func (h *Hub) unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.removeLocked(c)
}

// removeLocked drops c from the hub and closes its send channel, which tells
// writeLoop to close the connection. h.mu must be held. It is safe to call
// twice for the same client.
func (h *Hub) removeLocked(c *Client) {
	conns, ok := h.clients[c.userID]
	if !ok || !conns[c] {
		return
	}
	delete(conns, c)
	if len(conns) == 0 {
		delete(h.clients, c.userID)
	}
	close(c.send)
}

// readLoop reads until the browser disconnects or stops answering pings,
// and hands every frame to the hub's handler. A frame that is not a JSON
// envelope is answered with an error event; the connection stays open.
func (c *Client) readLoop() {
	defer func() {
		c.hub.unregister(c)
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		c.conn.SetReadDeadline(time.Now().Add(pongWait))

		var in Incoming
		if err := json.Unmarshal(data, &in); err != nil || in.Type == "" {
			c.hub.ReplyError(c, in, `a frame must be JSON like {"type": "...", "payload": {...}}`)
			continue
		}
		if c.hub.handler == nil {
			c.hub.ReplyError(c, in, "unknown event type")
			continue
		}
		c.hub.handler(c, in)
	}
}

// writeLoop sends queued events and a ping every pingPeriod. It closes the
// connection when the hub closes the send channel or a write fails.
func (c *Client) writeLoop() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case data, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
