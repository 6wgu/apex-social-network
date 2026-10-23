package handlers

import (
	"database/sql"
	"log"
	"net/http"

	"social-network/pkg/models"
	"social-network/pkg/ws"
)

const eventGroupMessage = "group_message"

type groupMessage struct {
	ID        int64              `json:"id"`
	GroupID   int64              `json:"group_id"`
	Sender    models.UserSummary `json:"sender"`
	Content   string             `json:"content"`
	CreatedAt string             `json:"created_at"`
}

const groupMessageColumns = `m.id, m.group_id, m.content, strftime('%Y-%m-%d %H:%M:%S', m.created_at),
	s.id, s.first_name, s.last_name, s.nickname, s.avatar_path`

const groupMessageFrom = `FROM group_messages m JOIN users s ON s.id = m.sender_id`

func scanGroupMessage(row interface{ Scan(...any) error }) (groupMessage, error) {
	var m groupMessage
	s := &m.Sender
	err := row.Scan(&m.ID, &m.GroupID, &m.Content, &m.CreatedAt, &s.ID, &s.FirstName, &s.LastName, &s.Nickname, &s.AvatarPath)
	return m, err
}

func (a *App) sendGroupMessage(c *ws.Client, in ws.Incoming) {
	var p struct {
		GroupID int64  `json:"group_id"`
		Content string `json:"content"`
	}
	if !decodePayload(in.Payload, &p) || p.GroupID <= 0 {
		a.Hub.ReplyError(c, in, `payload must be {"group_id": <group id>, "content": "<text>"}`)
		return
	}
	content, problem := checkMessageText(p.Content)
	if problem != "" {
		a.Hub.ReplyError(c, in, problem)
		return
	}
	me := c.UserID()
	res, err := a.DB.Exec(`INSERT INTO group_messages (group_id, sender_id, content)
		SELECT ?1, ?2, ?3 WHERE EXISTS (SELECT 1 FROM group_members WHERE group_id = ?1 AND user_id = ?2)`,
		p.GroupID, me, content)
	if err != nil {
		a.socketServerError(c, in, err)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		var exists bool
		if err := a.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM groups WHERE id = ?)`, p.GroupID).Scan(&exists); err != nil {
			a.socketServerError(c, in, err)
			return
		}
		if exists {
			a.Hub.ReplyError(c, in, "only members can send messages to this group")
		} else {
			a.Hub.ReplyError(c, in, "group not found")
		}
		return
	}
	id, err := res.LastInsertId()
	if err != nil {
		a.socketServerError(c, in, err)
		return
	}
	m, err := scanGroupMessage(a.DB.QueryRow(`SELECT `+groupMessageColumns+` `+groupMessageFrom+` WHERE m.id = ?`, id))
	if err != nil {
		a.socketServerError(c, in, err)
		return
	}
	members, err := groupMemberIDs(a.DB, p.GroupID)
	if err != nil {
		log.Printf("group_message %d fan-out: %v", m.ID, err)
	}
	for _, id := range members {
		if id != me {
			a.Hub.SendToUser(id, eventGroupMessage, m)
		}
	}
	a.Hub.SendToUserExcept(me, c, eventGroupMessage, m)
	a.Hub.Reply(c, eventGroupMessage, in.RequestID, m)
}

func groupMemberIDs(q interface {
	Query(string, ...any) (*sql.Rows, error)
}, groupID int64) ([]int64, error) {
	rows, err := q.Query(`SELECT user_id FROM group_members WHERE group_id = ?`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (a *App) groupMessageHistory(w http.ResponseWriter, r *http.Request) {
	groupID, ok := a.memberGroupID(w, r, "only members can read this group's chat")
	if !ok {
		return
	}
	before, ok := beforeParam(w, r)
	if !ok {
		return
	}
	rows, err := a.DB.Query(`SELECT `+groupMessageColumns+` `+groupMessageFrom+`
		WHERE m.group_id = ? AND m.id < ?
		ORDER BY m.id DESC LIMIT ?`, groupID, before, messagesPerPage)
	if err != nil {
		serverError(w, err)
		return
	}
	defer rows.Close()
	list := []groupMessage{}
	for rows.Next() {
		m, err := scanGroupMessage(rows)
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
