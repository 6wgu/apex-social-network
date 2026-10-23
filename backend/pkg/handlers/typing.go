package handlers

import (
	"social-network/pkg/models"
	"social-network/pkg/ws"
)

// Typing indicator. A tab sends "typing" while its user writes in a chat
// (the page throttles it to about one frame every 2 s); the server relays it
// to the other side and stores nothing. It is allowed exactly when sending a
// message would be: the follow rule for a private chat, membership for a
// group chat. So a typing event never reaches someone the sender may not
// message, and a non-member never learns anything about a group's chat.

// eventTyping is the WebSocket type for typing, both ways.
const eventTyping = "typing"

// typingEvent is the server -> client payload: who is typing, and where.
// Exactly one of RecipientID (private chat: me) and GroupID is set.
type typingEvent struct {
	User        models.UserSummary `json:"user"`
	RecipientID int64              `json:"recipient_id,omitempty"`
	GroupID     int64              `json:"group_id,omitempty"`
}

const errTypingPayload = `payload must be {"recipient_id": <user id>} or {"group_id": <group id>}`

// sendTyping handles a typing frame:
// {"type":"typing","request_id":"r1","payload":{"recipient_id":7}} or
// {"type":"typing","request_id":"r1","payload":{"group_id":3}}.
// The other party's tabs (private) or every other member's tabs (group) get
// a typing event; the sender's own tabs get nothing, except the sending tab
// which gets the same payload back as the answer when it sent a request_id.
func (a *App) sendTyping(c *ws.Client, in ws.Incoming) {
	var p struct {
		RecipientID int64 `json:"recipient_id"`
		GroupID     int64 `json:"group_id"`
	}
	if !decodePayload(in.Payload, &p) || p.RecipientID < 0 || p.GroupID < 0 || (p.RecipientID > 0) == (p.GroupID > 0) {
		a.Hub.ReplyError(c, in, errTypingPayload)
		return
	}
	me := c.UserID()

	var recipients []int64
	if p.RecipientID > 0 {
		if p.RecipientID == me {
			a.Hub.ReplyError(c, in, "you can't message yourself")
			return
		}
		var exists, allowed bool
		err := a.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM users WHERE id = ?2), `+canMessageSQL, me, p.RecipientID).Scan(&exists, &allowed)
		if err != nil {
			a.socketServerError(c, in, err)
			return
		}
		if !exists {
			a.Hub.ReplyError(c, in, "user not found")
			return
		}
		if !allowed {
			a.Hub.ReplyError(c, in, errNotAllowedToMessage)
			return
		}
		recipients = []int64{p.RecipientID}
	} else {
		var exists, member bool
		err := a.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM groups WHERE id = ?1),
			EXISTS(SELECT 1 FROM group_members WHERE group_id = ?1 AND user_id = ?2)`, p.GroupID, me).Scan(&exists, &member)
		if err != nil {
			a.socketServerError(c, in, err)
			return
		}
		if !exists {
			a.Hub.ReplyError(c, in, "group not found")
			return
		}
		if !member {
			a.Hub.ReplyError(c, in, "only members can send messages to this group")
			return
		}
		if recipients, err = groupMemberIDs(a.DB, p.GroupID); err != nil {
			a.socketServerError(c, in, err)
			return
		}
	}

	ev := typingEvent{RecipientID: p.RecipientID, GroupID: p.GroupID}
	u := &ev.User
	if err := a.DB.QueryRow(`SELECT id, first_name, last_name, nickname, avatar_path FROM users WHERE id = ?`, me).
		Scan(&u.ID, &u.FirstName, &u.LastName, &u.Nickname, &u.AvatarPath); err != nil {
		a.socketServerError(c, in, err)
		return
	}
	for _, id := range recipients {
		if id != me {
			a.Hub.SendToUser(id, eventTyping, ev)
		}
	}
	if in.RequestID != "" {
		a.Hub.Reply(c, eventTyping, in.RequestID, ev)
	}
}
