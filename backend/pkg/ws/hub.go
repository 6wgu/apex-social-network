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
	writeWait        = 10 * time.Second
	pongWait         = 60 * time.Second
	pingPeriod       = 30 * time.Second
	MaxFrameSize     = 16 << 10
	DefaultRateLimit = 10
	DefaultRateBurst = 10
	sendBuffer       = 32
)

type Event struct {
	Type      string `json:"type"`
	RequestID string `json:"request_id,omitempty"`
	Payload   any    `json:"payload"`
}

type Incoming struct {
	Type      string          `json:"type"`
	RequestID string          `json:"request_id"`
	Payload   json.RawMessage `json:"payload"`
}

type Handler func(c *Client, in Incoming)

const EventError = "error"

type ErrorPayload struct {
	Message string `json:"message"`
	For     string `json:"for,omitempty"`
}

type Client struct {
	hub    *Hub
	userID int64
	conn   *websocket.Conn
	send   chan []byte

	tokens   float64
	lastSeen time.Time
}

func (c *Client) allow(now time.Time) bool {
	rate, burst := c.hub.rateLimit, c.hub.rateBurst
	if c.lastSeen.IsZero() {
		c.tokens = burst
	} else {
		c.tokens += now.Sub(c.lastSeen).Seconds() * rate
		if c.tokens > burst {
			c.tokens = burst
		}
	}
	c.lastSeen = now
	if c.tokens < 1 {
		return false
	}
	c.tokens--
	return true
}

type Hub struct {
	upgrader  websocket.Upgrader
	handler   Handler
	rateLimit float64
	rateBurst float64

	mu      sync.Mutex
	clients map[int64]map[*Client]bool
}

func ParseOrigins(list string) []string {
	var origins []string
	for _, o := range strings.Split(list, ",") {
		if o = strings.TrimRight(strings.TrimSpace(o), "/"); o != "" {
			origins = append(origins, o)
		}
	}
	return origins
}

func NewHub(allowedOrigins ...string) *Hub {
	allowed := map[string]bool{}
	for _, o := range allowedOrigins {
		allowed[o] = true
	}
	h := &Hub{clients: map[int64]map[*Client]bool{}, rateLimit: DefaultRateLimit, rateBurst: DefaultRateBurst}
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

func (h *Hub) Serve(w http.ResponseWriter, r *http.Request, userID int64) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade for user %d: %v", userID, err)
		return
	}
	c := &Client{hub: h, userID: userID, conn: conn, send: make(chan []byte, sendBuffer)}
	h.register(c)
	go c.writeLoop()
	c.readLoop()
}

func (h *Hub) SetRateLimit(perSecond, burst float64) {
	h.rateLimit, h.rateBurst = perSecond, burst
}

func (h *Hub) SetHandler(fn Handler) {
	h.handler = fn
}

func (c *Client) UserID() int64 {
	return c.userID
}

func (h *Hub) SendToUser(userID int64, eventType string, payload any) {
	h.SendToUserExcept(userID, nil, eventType, payload)
}

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

func (h *Hub) Reply(c *Client, eventType, requestID string, payload any) {
	data, ok := marshal(Event{Type: eventType, RequestID: requestID, Payload: payload})
	if !ok {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[c.userID][c] {
		h.enqueueLocked(c, data)
	}
}

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

func (h *Hub) enqueueLocked(c *Client, data []byte) {
	select {
	case c.send <- data:
	default:
		h.removeLocked(c)
	}
}

func (h *Hub) CloseUser(userID int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients[userID] {
		h.removeLocked(c)
	}
}

func (h *Hub) OnlineUsers() []int64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	ids := make([]int64, 0, len(h.clients))
	for id := range h.clients {
		ids = append(ids, id)
	}
	return ids
}

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

func (c *Client) readLoop() {
	defer func() {
		c.hub.unregister(c)
		c.conn.Close()
	}()
	c.conn.SetReadLimit(MaxFrameSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		now := time.Now()
		c.conn.SetReadDeadline(now.Add(pongWait))

		var in Incoming
		parseErr := json.Unmarshal(data, &in)
		if !c.allow(now) {
			c.hub.ReplyError(c, in, "too many messages: slow down")
			continue
		}
		if parseErr != nil || in.Type == "" {
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
