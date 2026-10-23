package handlers

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strconv"

	"social-network/pkg/models"
)

const (
	notifFollowRequest        = "follow_request"
	notifNewFollower          = "new_follower"
	notifFollowAccepted       = "follow_accepted"
	notifGroupInvite          = "group_invite"
	notifGroupJoinRequest     = "group_join_request"
	notifGroupRequestAccepted = "group_request_accepted"
	notifGroupEvent           = "group_event"
)

const (
	eventNotification        = "notification"
	eventNotificationRemoved = "notification_removed"
	eventGroupChanged        = "group_changed"
)

const maxNotificationsPage = 50

type notification struct {
	ID        int64               `json:"id"`
	Type      string              `json:"type"`
	Actor     *models.UserSummary `json:"actor"`
	RefID     *int64              `json:"ref_id"`
	Group     *groupRef           `json:"group"`
	Event     *eventRef           `json:"event"`
	IsRead    bool                `json:"is_read"`
	CreatedAt string              `json:"created_at"`
}

type groupRef struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
}

type eventRef struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	EventTime string `json:"event_time"`
}

const notificationColumns = `n.id, n.type, n.ref_id, n.is_read, strftime('%Y-%m-%d %H:%M:%S', n.created_at),
	u.id, u.first_name, u.last_name, u.nickname, u.avatar_path,
	g.id, g.title, e.id, e.title, strftime('%Y-%m-%d %H:%M:%S', e.event_time)`

const notificationFrom = `FROM notifications n
	LEFT JOIN users u ON u.id = n.actor_id
	LEFT JOIN events e ON n.type = 'group_event' AND e.id = n.ref_id
	LEFT JOIN groups g ON g.id = CASE
		WHEN n.type = 'group_event' THEN e.group_id
		WHEN n.type IN ('group_invite', 'group_join_request', 'group_request_accepted') THEN n.ref_id
	END`

func scanNotification(row interface{ Scan(...any) error }, extra ...any) (notification, error) {
	var n notification
	var refID, actorID sql.NullInt64
	var first, last, nick, avatar sql.NullString
	var groupID, eventID sql.NullInt64
	var groupTitle, eventTitle, eventTime sql.NullString
	dest := append([]any{&n.ID, &n.Type, &refID, &n.IsRead, &n.CreatedAt, &actorID, &first, &last, &nick, &avatar,
		&groupID, &groupTitle, &eventID, &eventTitle, &eventTime}, extra...)
	err := row.Scan(dest...)
	if groupID.Valid {
		n.Group = &groupRef{ID: groupID.Int64, Title: groupTitle.String}
	}
	if eventID.Valid {
		n.Event = &eventRef{ID: eventID.Int64, Title: eventTitle.String, EventTime: eventTime.String}
	}
	if refID.Valid {
		n.RefID = &refID.Int64
	}
	if actorID.Valid {
		n.Actor = &models.UserSummary{ID: actorID.Int64, FirstName: first.String, LastName: last.String,
			Nickname: nick.String, AvatarPath: avatar.String}
	}
	return n, err
}

type notifier struct {
	created       []int64
	removed       []removedNotification
	changedGroups []int64
}

func (n *notifier) groupChanged(groupID int64) {
	n.changedGroups = append(n.changedGroups, groupID)
}

type removedNotification struct {
	userID, id int64
}

func (n *notifier) add(tx *sql.Tx, userID, actorID int64, typ string, refID *int64) error {
	if userID == actorID {
		return nil
	}
	res, err := tx.Exec(`INSERT INTO notifications (user_id, type, actor_id, ref_id) VALUES (?, ?, ?, ?)`,
		userID, typ, actorID, refID)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	n.created = append(n.created, id)
	return nil
}

func (n *notifier) remove(tx *sql.Tx, userID, actorID int64, typ string) error {
	return n.removeRows(tx, userID, `DELETE FROM notifications WHERE user_id = ? AND actor_id = ? AND type = ? RETURNING id`,
		userID, actorID, typ)
}

func (n *notifier) removeRef(tx *sql.Tx, userID, actorID int64, typ string, refID int64) error {
	return n.removeRows(tx, userID, `DELETE FROM notifications WHERE user_id = ? AND actor_id = ? AND type = ? AND ref_id = ? RETURNING id`,
		userID, actorID, typ, refID)
}

func (n *notifier) removeRows(tx *sql.Tx, userID int64, query string, args ...any) error {
	rows, err := tx.Query(query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return err
		}
		n.removed = append(n.removed, removedNotification{userID: userID, id: id})
	}
	return rows.Err()
}

func (a *App) push(n *notifier) {
	for _, id := range n.created {
		var userID int64
		row := a.DB.QueryRow(`SELECT `+notificationColumns+`, n.user_id `+notificationFrom+` WHERE n.id = ?`, id)
		note, err := scanNotification(row, &userID)
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				log.Printf("push notification %d: %v", id, err)
			}
			continue
		}
		a.Hub.SendToUser(userID, eventNotification, note)
	}
	for _, r := range n.removed {
		a.Hub.SendToUser(r.userID, eventNotificationRemoved, map[string]int64{"id": r.id})
	}
	for _, groupID := range n.changedGroups {
		members, err := groupMemberIDs(a.DB, groupID)
		if err != nil {
			log.Printf("push group_changed %d: %v", groupID, err)
			continue
		}
		for _, id := range members {
			a.Hub.SendToUser(id, eventGroupChanged, map[string]int64{"group_id": groupID})
		}
	}
}

func (a *App) listNotifications(w http.ResponseWriter, r *http.Request) {
	before, ok := beforeParam(w, r)
	if !ok {
		return
	}
	rows, err := a.DB.Query(`SELECT `+notificationColumns+` `+notificationFrom+`
		WHERE n.user_id = ? AND n.id < ?
		ORDER BY n.id DESC LIMIT ?`, currentUserID(r), before, maxNotificationsPage)
	if err != nil {
		serverError(w, err)
		return
	}
	defer rows.Close()
	list := []notification{}
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			serverError(w, err)
			return
		}
		list = append(list, n)
	}
	if err := rows.Err(); err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (a *App) unreadNotifications(w http.ResponseWriter, r *http.Request) {
	var n int
	if err := a.DB.QueryRow(`SELECT COUNT(*) FROM notifications WHERE user_id = ? AND is_read = 0`, currentUserID(r)).Scan(&n); err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"unread": n})
}

func (a *App) readNotification(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusNotFound, "notification not found")
		return
	}
	res, err := a.DB.Exec(`UPDATE notifications SET is_read = 1 WHERE id = ? AND user_id = ?`, id, currentUserID(r))
	if err != nil {
		serverError(w, err)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeError(w, http.StatusNotFound, "notification not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) readAllNotifications(w http.ResponseWriter, r *http.Request) {
	if _, err := a.DB.Exec(`UPDATE notifications SET is_read = 1 WHERE user_id = ? AND is_read = 0`, currentUserID(r)); err != nil {
		serverError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
