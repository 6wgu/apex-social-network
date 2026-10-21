package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"unicode/utf8"

	"social-network/pkg/models"
	"social-network/pkg/ws"
)

// Private messages. Two users may write to each other while at least one of
// them follows the other with an accepted follow. The rule is checked inside
// the INSERT of every message, so it holds at the moment of storing: an
// unfollow blocks the very next message. Messages already exchanged stay
// readable by both.

const (
	maxMessageLen   = 2000 // characters (runes), not bytes, so emojis count as one or a few
	messagesPerPage = 50

	// WebSocket event types for private messages.
	eventPrivateMessage = "private_message" // both ways; server -> client payload: message
	eventMessagesRead   = "messages_read"   // server -> client: {"user_id": 7}, I read that conversation in another tab

	errNotAllowedToMessage = "you can only message people you follow or who follow you"
)

// canMessageSQL is the follow rule as a SQL condition: users ?1 and ?2 may
// exchange messages when either follows the other with an accepted follow.
// SQL constant: never built from user input; every value is a bound parameter.
const canMessageSQL = `EXISTS (SELECT 1 FROM follows f WHERE f.status = 'accepted'
	AND ((f.follower_id = ?1 AND f.following_id = ?2) OR (f.follower_id = ?2 AND f.following_id = ?1)))`

// message is the JSON shape of a private message.
type message struct {
	ID          int64              `json:"id"`
	Sender      models.UserSummary `json:"sender"`
	RecipientID int64              `json:"recipient_id"`
	Content     string             `json:"content"`
	CreatedAt   string             `json:"created_at"`
	ReadAt      *string            `json:"read_at"` // null until the recipient opens the conversation
}

// messageColumns selects a message (alias m) with its sender (alias s).
// SQL constant: never built from user input; every value is a bound parameter.
const messageColumns = `m.id, m.recipient_id, m.content, strftime('%Y-%m-%d %H:%M:%S', m.created_at),
	strftime('%Y-%m-%d %H:%M:%S', m.read_at), s.id, s.first_name, s.last_name, s.nickname, s.avatar_path`

// SQL constant: never built from user input; every value is a bound parameter.
const messageFrom = `FROM messages m JOIN users s ON s.id = m.sender_id`

func scanMessage(row interface{ Scan(...any) error }, extra ...any) (message, error) {
	var m message
	var readAt sql.NullString
	s := &m.Sender
	dest := append([]any{&m.ID, &m.RecipientID, &m.Content, &m.CreatedAt, &readAt,
		&s.ID, &s.FirstName, &s.LastName, &s.Nickname, &s.AvatarPath}, extra...)
	err := row.Scan(dest...)
	if readAt.Valid {
		m.ReadAt = &readAt.String
	}
	return m, err
}

// canMessage reports whether a and b may exchange messages right now.
func (a *App) canMessage(userA, userB int64) (bool, error) {
	var ok bool
	err := a.DB.QueryRow(`SELECT `+canMessageSQL, userA, userB).Scan(&ok)
	return ok, err
}

// checkMessageText trims s and checks it is not empty and at most
// maxMessageLen characters. It returns the trimmed text and a problem ("" when fine).
func checkMessageText(s string) (string, string) {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return "", "a message can't be empty"
	case !utf8.ValidString(s):
		return "", "a message must be valid UTF-8 text"
	case utf8.RuneCountInString(s) > maxMessageLen:
		return "", fmt.Sprintf("a message must be %d characters or fewer", maxMessageLen)
	}
	return s, ""
}

// decodePayload reads a frame's payload into dst, rejecting unknown fields.
func decodePayload(raw json.RawMessage, dst any) bool {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	return len(raw) > 0 && dec.Decode(dst) == nil
}

// handleSocketEvent dispatches every frame a browser sends over /ws.
func (a *App) handleSocketEvent(c *ws.Client, in ws.Incoming) {
	switch in.Type {
	case eventPrivateMessage:
		a.sendPrivateMessage(c, in)
	case eventGroupMessage:
		a.sendGroupMessage(c, in)
	case eventTyping:
		a.sendTyping(c, in)
	default:
		a.Hub.ReplyError(c, in, fmt.Sprintf("unknown event type %q", in.Type))
	}
}

// sendPrivateMessage handles a private_message frame:
// {"type":"private_message","request_id":"r1","payload":{"recipient_id":7,"content":"hi"}}.
// The message is stored only if the follow rule allows it at that moment;
// then it is pushed to the recipient's tabs and to all of the sender's tabs
// (the sending tab gets it with its request_id, as the confirmation).
func (a *App) sendPrivateMessage(c *ws.Client, in ws.Incoming) {
	var p struct {
		RecipientID int64  `json:"recipient_id"`
		Content     string `json:"content"`
	}
	if !decodePayload(in.Payload, &p) || p.RecipientID <= 0 {
		a.Hub.ReplyError(c, in, `payload must be {"recipient_id": <user id>, "content": "<text>"}`)
		return
	}
	me := c.UserID()
	if p.RecipientID == me {
		a.Hub.ReplyError(c, in, "you can't message yourself")
		return
	}
	content, problem := checkMessageText(p.Content)
	if problem != "" {
		a.Hub.ReplyError(c, in, problem)
		return
	}

	// The follow rule is part of the INSERT: nothing is stored unless it
	// holds when the row is written.
	res, err := a.DB.Exec(`INSERT INTO messages (sender_id, recipient_id, content)
		SELECT ?1, ?2, ?3 WHERE `+canMessageSQL, me, p.RecipientID, content)
	if err != nil {
		a.socketServerError(c, in, err)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		var exists bool
		if err := a.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM users WHERE id = ?)`, p.RecipientID).Scan(&exists); err != nil {
			a.socketServerError(c, in, err)
			return
		}
		if !exists {
			a.Hub.ReplyError(c, in, "user not found")
		} else {
			a.Hub.ReplyError(c, in, errNotAllowedToMessage)
		}
		return
	}
	id, err := res.LastInsertId()
	if err != nil {
		a.socketServerError(c, in, err)
		return
	}
	m, err := scanMessage(a.DB.QueryRow(`SELECT `+messageColumns+` `+messageFrom+` WHERE m.id = ?`, id))
	if err != nil {
		a.socketServerError(c, in, err)
		return
	}
	a.Hub.SendToUser(p.RecipientID, eventPrivateMessage, m)
	a.Hub.SendToUserExcept(me, c, eventPrivateMessage, m)
	a.Hub.Reply(c, eventPrivateMessage, in.RequestID, m)
}

// socketServerError logs an unexpected failure and tells the tab, without details.
func (a *App) socketServerError(c *ws.Client, in ws.Incoming, err error) {
	log.Printf("internal error (%s from user %d): %v", in.Type, c.UserID(), err)
	a.Hub.ReplyError(c, in, "internal server error")
}

// chatPartnerFromPath reads {userId}: 404 for an unknown user, 400 for yourself.
func (a *App) chatPartnerFromPath(w http.ResponseWriter, r *http.Request) (int64, bool) {
	other, ok := a.userIDFromPath(w, r, "userId")
	if !ok {
		return 0, false
	}
	if other == currentUserID(r) {
		writeError(w, http.StatusBadRequest, "you can't chat with yourself")
		return 0, false
	}
	return other, true
}

// messageHistory handles GET /api/messages/{userId}?before={messageId}: my
// conversation with that user, newest first, 50 per page. It is readable
// while the follow rule allows chatting, and afterwards if messages were
// already exchanged (they are the reader's own conversation). Anyone else
// gets 403: a third user never sees a conversation they are not part of,
// because the query only ever selects messages between me and {userId}.
func (a *App) messageHistory(w http.ResponseWriter, r *http.Request) {
	other, ok := a.chatPartnerFromPath(w, r)
	if !ok {
		return
	}
	before, ok := beforeParam(w, r)
	if !ok {
		return
	}
	me := currentUserID(r)
	var allowed bool
	err := a.DB.QueryRow(`SELECT `+canMessageSQL+` OR EXISTS (SELECT 1 FROM messages
		WHERE (sender_id = ?1 AND recipient_id = ?2) OR (sender_id = ?2 AND recipient_id = ?1))`, me, other).Scan(&allowed)
	if err != nil {
		serverError(w, err)
		return
	}
	if !allowed {
		writeError(w, http.StatusForbidden, errNotAllowedToMessage)
		return
	}
	rows, err := a.DB.Query(`SELECT `+messageColumns+` `+messageFrom+`
		WHERE ((m.sender_id = ?1 AND m.recipient_id = ?2) OR (m.sender_id = ?2 AND m.recipient_id = ?1)) AND m.id < ?3
		ORDER BY m.id DESC LIMIT ?4`, me, other, before, messagesPerPage)
	if err != nil {
		serverError(w, err)
		return
	}
	defer rows.Close()
	list := []message{}
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			serverError(w, err)
			return
		}
		list = append(list, m)
	}
	if err := rows.Err(); err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// markConversationRead handles POST /api/messages/{userId}/read: every
// message {userId} sent me is marked read. My other tabs are told
// (messages_read), so their unread badges drop too.
func (a *App) markConversationRead(w http.ResponseWriter, r *http.Request) {
	other, ok := a.chatPartnerFromPath(w, r)
	if !ok {
		return
	}
	me := currentUserID(r)
	if _, err := a.DB.Exec(`UPDATE messages SET read_at = CURRENT_TIMESTAMP
		WHERE sender_id = ? AND recipient_id = ? AND read_at IS NULL`, other, me); err != nil {
		serverError(w, err)
		return
	}
	a.Hub.SendToUser(me, eventMessagesRead, map[string]int64{"user_id": other})
	w.WriteHeader(http.StatusNoContent)
}

// unreadMessages handles GET /api/messages/unread-count: my unread private
// messages in total. It is separate from the notification count on purpose.
func (a *App) unreadMessages(w http.ResponseWriter, r *http.Request) {
	var n int
	if err := a.DB.QueryRow(`SELECT COUNT(*) FROM messages WHERE recipient_id = ? AND read_at IS NULL`, currentUserID(r)).Scan(&n); err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"unread": n})
}

// conversation is one entry of GET /api/conversations.
type conversation struct {
	User        models.UserSummary `json:"user"`
	LastMessage message            `json:"last_message"`
	UnreadCount int                `json:"unread_count"`
	CanMessage  bool               `json:"can_message"` // the follow rule, right now
}

// listConversations handles GET /api/conversations: everyone I have
// exchanged messages with, the last message and my unread count, most
// recent conversation first.
func (a *App) listConversations(w http.ResponseWriter, r *http.Request) {
	me := currentUserID(r)
	rows, err := a.DB.Query(`
		WITH mine AS (
			SELECT id, CASE WHEN sender_id = ?1 THEN recipient_id ELSE sender_id END AS other_id
			FROM messages WHERE sender_id = ?1 OR recipient_id = ?1
		), latest AS (
			SELECT other_id, MAX(id) AS last_id FROM mine GROUP BY other_id
		)
		SELECT `+messageColumns+`,
			o.id, o.first_name, o.last_name, o.nickname, o.avatar_path,
			(SELECT COUNT(*) FROM messages x WHERE x.sender_id = l.other_id AND x.recipient_id = ?1 AND x.read_at IS NULL),
			EXISTS (SELECT 1 FROM follows f WHERE f.status = 'accepted'
				AND ((f.follower_id = ?1 AND f.following_id = l.other_id) OR (f.follower_id = l.other_id AND f.following_id = ?1)))
		FROM latest l
		JOIN messages m ON m.id = l.last_id
		JOIN users s ON s.id = m.sender_id
		JOIN users o ON o.id = l.other_id
		ORDER BY m.id DESC`, me)
	if err != nil {
		serverError(w, err)
		return
	}
	defer rows.Close()
	list := []conversation{}
	for rows.Next() {
		var cv conversation
		u := &cv.User
		cv.LastMessage, err = scanMessage(rows, &u.ID, &u.FirstName, &u.LastName, &u.Nickname, &u.AvatarPath, &cv.UnreadCount, &cv.CanMessage)
		if err != nil {
			serverError(w, err)
			return
		}
		list = append(list, cv)
	}
	if err := rows.Err(); err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// chatContacts handles GET /api/chat-contacts: everyone I may message right
// now (I follow them or they follow me, accepted), sorted by name.
func (a *App) chatContacts(w http.ResponseWriter, r *http.Request) {
	users, err := a.userSummaries(`
		SELECT u.id, u.first_name, u.last_name, u.nickname, u.avatar_path
		FROM users u
		WHERE u.id <> ?1 AND EXISTS (SELECT 1 FROM follows f WHERE f.status = 'accepted'
			AND ((f.follower_id = ?1 AND f.following_id = u.id) OR (f.follower_id = u.id AND f.following_id = ?1)))
		ORDER BY u.first_name, u.last_name, u.id`, currentUserID(r))
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, users)
}
