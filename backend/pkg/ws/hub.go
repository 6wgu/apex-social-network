// Package ws keeps the live WebSocket connections of logged-in users and lets
// the rest of the backend push JSON events to them (notifications now, chat
// messages in Phase 4).
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

// Event is the envelope of every frame: {"type": "...", "payload": {...}}.
type Event struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

// client is one open connection (one browser tab) of one user.
type client struct {
	hub    *Hub
	userID int64
	conn   *websocket.Conn
	send   chan []byte
}

// Hub tracks every open connection, grouped by user, so one user with
// several tabs gets each event in every tab.
type Hub struct {
	upgrader websocket.Upgrader

	mu      sync.Mutex
	clients map[int64]map[*client]bool
}

// NewHub returns a hub that accepts upgrades from browsers on allowedOrigin
// (the frontend, e.g. http://localhost:3000). The Next.js proxy forwards the
// browser's Origin header unchanged, so this is the origin to allow. Requests
// without an Origin header are not from a browser page (curl, tests) and are
// allowed: the session cookie is still required.
func NewHub(allowedOrigin string) *Hub {
	h := &Hub{clients: map[int64]map[*client]bool{}}
	h.upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			return origin == "" || origin == allowedOrigin
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
	c := &client{hub: h, userID: userID, conn: conn, send: make(chan []byte, sendBuffer)}
	h.register(c)
	go c.writeLoop()
	c.readLoop() // returns when the connection is gone
}

// SendToUser delivers an event to every open connection of userID. It never
// blocks: a connection whose buffer is full is too slow and gets closed, and
// a user with no connection simply misses the push (the data is in the
// database, so the page reloads it).
func (h *Hub) SendToUser(userID int64, eventType string, payload any) {
	data, err := json.Marshal(Event{Type: eventType, Payload: payload})
	if err != nil {
		log.Printf("ws marshal %q: %v", eventType, err)
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients[userID] {
		select {
		case c.send <- data:
		default:
			h.removeLocked(c)
		}
	}
}

// Connections returns how many connections userID has open (used by tests).
func (h *Hub) Connections(userID int64) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients[userID])
}

func (h *Hub) register(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[c.userID] == nil {
		h.clients[c.userID] = map[*client]bool{}
	}
	h.clients[c.userID][c] = true
}

func (h *Hub) unregister(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.removeLocked(c)
}

// removeLocked drops c from the hub and closes its send channel, which tells
// writeLoop to close the connection. h.mu must be held. It is safe to call
// twice for the same client.
func (h *Hub) removeLocked(c *client) {
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

// readLoop reads until the browser disconnects or stops answering pings.
// No incoming message types exist yet (chat comes in Phase 4), so frames are
// read and ignored; reading is still needed to process pongs and closes.
func (c *client) readLoop() {
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
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
	}
}

// writeLoop sends queued events and a ping every pingPeriod. It closes the
// connection when the hub closes the send channel or a write fails.
func (c *client) writeLoop() {
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
