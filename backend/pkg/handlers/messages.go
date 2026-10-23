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

const (
	maxMessageLen   = 2000
	messagesPerPage = 50

	eventPrivateMessage = "private_message"
	eventMessagesRead   = "messages_read"

	errNotAllowedToMessage = "you can only message people you follow or who follow you"
)

const canMessageSQL = `EXISTS (SELECT 1 FROM follows f WHERE f.status = 'accepted'
	AND ((f.follower_id = ?1 AND f.following_id = ?2) OR (f.follower_id = ?2 AND f.following_id = ?1)))`

type message struct {
	ID          int64              `json:"id"`
	Sender      models.UserSummary `json:"sender"`
	RecipientID int64              `json:"recipient_id"`
	Content     string             `json:"content"`
	CreatedAt   string             `json:"created_at"`
	ReadAt      *string            `json:"read_at"`
}

const messageColumns = `m.id, m.recipient_id, m.content, strftime('%Y-%m-%d %H:%M:%S', m.created_at),
	strftime('%Y-%m-%d %H:%M:%S', m.read_at), s.id, s.first_name, s.last_name, s.nickname, s.avatar_path`

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

func decodePayload(raw json.RawMessage, dst any) bool {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	return len(raw) > 0 && dec.Decode(dst) == nil
}

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

func (a *App) socketServerError(c *ws.Client, in ws.Incoming, err error) {
	log.Printf("internal error (%s from user %d): %v", in.Type, c.UserID(), err)
	a.Hub.ReplyError(c, in, "internal server error")
}

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

func (a *App) unreadMessages(w http.ResponseWriter, r *http.Request) {
	var n int
	if err := a.DB.QueryRow(`SELECT COUNT(*) FROM messages WHERE recipient_id = ? AND read_at IS NULL`, currentUserID(r)).Scan(&n); err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"unread": n})
}

type conversation struct {
	User        models.UserSummary `json:"user"`
	LastMessage message            `json:"last_message"`
	UnreadCount int                `json:"unread_count"`
	CanMessage  bool               `json:"can_message"`
}

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
