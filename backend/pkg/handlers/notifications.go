package handlers

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strconv"

	"social-network/pkg/models"
)

// Notification types. The database does not restrict type (see migration
// 000004); this list is the source of truth. The follow types have no
// ref_id; the group types point ref_id at a group or an event, and the
// API adds that group / event to the notification.
const (
	// notifFollowRequest: actor asked to follow user (a private profile).
	// Removed again when the request is accepted, declined or cancelled.
	notifFollowRequest = "follow_request"
	// notifNewFollower: actor started following user (a public profile).
	notifNewFollower = "new_follower"
	// notifFollowAccepted: user's request to follow actor was accepted.
	notifFollowAccepted = "follow_accepted"
	// notifGroupInvite: actor invited user to a group (ref_id: group).
	// Removed when the invitation is accepted or declined.
	notifGroupInvite = "group_invite"
	// notifGroupJoinRequest: actor asked to join a group user created
	// (ref_id: group). Removed when the creator answers it, the requester
	// cancels it, or the requester joins through an invitation instead.
	notifGroupJoinRequest = "group_join_request"
	// notifGroupRequestAccepted: the creator (actor) accepted user's request
	// to join a group (ref_id: group).
	notifGroupRequestAccepted = "group_request_accepted"
	// notifGroupEvent: actor created an event in a group user is a member
	// of (ref_id: event).
	notifGroupEvent = "group_event"
)

// WebSocket event types for the bell.
const (
	eventNotification        = "notification"         // payload: Notification
	eventNotificationRemoved = "notification_removed" // payload: {"id": 12}
	// eventGroupChanged tells a group's members that its member list changed
	// (someone joined), so an open group page reloads. Nothing is stored for
	// it: it is a live hint, not a notification.
	eventGroupChanged = "group_changed" // payload: {"group_id": 3}
)

// maxNotificationsPage caps one page of GET /api/notifications.
const maxNotificationsPage = 50

// notification is the JSON shape of one notification.
type notification struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
	// Actor is a pointer because the query uses a LEFT JOIN, but in practice
	// it is never null: deleting a user deletes their notifications too
	// (actor_id is ON DELETE CASCADE, migration 000004).
	Actor     *models.UserSummary `json:"actor"`
	RefID     *int64              `json:"ref_id"`
	Group     *groupRef           `json:"group"` // set for the group types
	Event     *eventRef           `json:"event"` // set for group_event
	IsRead    bool                `json:"is_read"`
	CreatedAt string              `json:"created_at"`
}

// groupRef and eventRef name what a group notification is about, so the
// bell can say "invited you to Hiking club" without another request.
type groupRef struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
}

type eventRef struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	EventTime string `json:"event_time"`
}

// notificationColumns and scanNotification read one notification with its
// actor, and for the group types the group (and event) it is about. The
// actor is a LEFT JOIN as a safeguard; with ON DELETE CASCADE on actor_id
// every listed notification has one.
// SQL constant: never built from user input; every value is a bound parameter.
const notificationColumns = `n.id, n.type, n.ref_id, n.is_read, strftime('%Y-%m-%d %H:%M:%S', n.created_at),
	u.id, u.first_name, u.last_name, u.nickname, u.avatar_path,
	g.id, g.title, e.id, e.title, strftime('%Y-%m-%d %H:%M:%S', e.event_time)`

// What ref_id points at depends on the type: an event for group_event (its
// group comes through the event), a group for the other group types.
// SQL constant: never built from user input; every value is a bound parameter.
const notificationFrom = `FROM notifications n
	LEFT JOIN users u ON u.id = n.actor_id
	LEFT JOIN events e ON n.type = 'group_event' AND e.id = n.ref_id
	LEFT JOIN groups g ON g.id = CASE
		WHEN n.type = 'group_event' THEN e.group_id
		WHEN n.type IN ('group_invite', 'group_join_request', 'group_request_accepted') THEN n.ref_id
	END`

// extra receives any columns selected after notificationColumns.
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

// notifier collects the notifications a handler created or removed inside
// its transaction, and pushes them over the WebSocket once the transaction
// has committed (never before: a rolled-back notification must not show up).
type notifier struct {
	created       []int64
	removed       []removedNotification
	changedGroups []int64 // groups whose members get a group_changed event
}

// groupChanged asks push to send group_changed to groupID's members.
func (n *notifier) groupChanged(groupID int64) {
	n.changedGroups = append(n.changedGroups, groupID)
}

type removedNotification struct {
	userID, id int64
}

// add inserts a notification for userID inside tx. refID may be nil.
func (n *notifier) add(tx *sql.Tx, userID, actorID int64, typ string, refID *int64) error {
	if userID == actorID {
		return nil // nobody is notified about their own actions
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

// remove deletes userID's notifications of type typ caused by actorID
// inside tx, e.g. the follow_request from a requester once it is answered.
func (n *notifier) remove(tx *sql.Tx, userID, actorID int64, typ string) error {
	return n.removeRows(tx, userID, `DELETE FROM notifications WHERE user_id = ? AND actor_id = ? AND type = ? RETURNING id`,
		userID, actorID, typ)
}

// removeRef is remove for the group types, which also match on ref_id: the
// same inviter can invite the same user to several groups.
func (n *notifier) removeRef(tx *sql.Tx, userID, actorID int64, typ string, refID int64) error {
	return n.removeRows(tx, userID, `DELETE FROM notifications WHERE user_id = ? AND actor_id = ? AND type = ? AND ref_id = ? RETURNING id`,
		userID, actorID, typ, refID)
}

// removeRows runs a DELETE … RETURNING id on userID's notifications and
// remembers the ids, so push can tell userID's open tabs.
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

// push sends what was collected to the recipients' open tabs. Call it after
// a successful commit. Failures are only logged: the notifications are in
// the database either way, and the bell reloads them.
func (a *App) push(n *notifier) {
	for _, id := range n.created {
		var userID int64
		row := a.DB.QueryRow(`SELECT `+notificationColumns+`, n.user_id `+notificationFrom+` WHERE n.id = ?`, id)
		note, err := scanNotification(row, &userID)
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) { // already removed again: nothing to push
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
		rows, err := a.DB.Query(`SELECT user_id FROM group_members WHERE group_id = ?`, groupID)
		if err != nil {
			log.Printf("push group_changed %d: %v", groupID, err)
			continue
		}
		var members []int64
		for rows.Next() {
			var id int64
			if rows.Scan(&id) == nil {
				members = append(members, id)
			}
		}
		// A read that stops early is logged; the members read so far still get the event.
		if err := rows.Err(); err != nil {
			log.Printf("push group_changed %d: %v", groupID, err)
		}
		rows.Close()
		for _, id := range members {
			a.Hub.SendToUser(id, eventGroupChanged, map[string]int64{"group_id": groupID})
		}
	}
}

// listNotifications handles GET /api/notifications?before={id}: my
// notifications, newest first, 50 per page. Pass the last id of a page as
// before to get the next one.
func (a *App) listNotifications(w http.ResponseWriter, r *http.Request) {
	before := int64(1<<63 - 1)
	if v := r.URL.Query().Get("before"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			writeError(w, http.StatusBadRequest, "before must be a notification id")
			return
		}
		before = id
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

// unreadNotifications handles GET /api/notifications/unread-count.
func (a *App) unreadNotifications(w http.ResponseWriter, r *http.Request) {
	var n int
	if err := a.DB.QueryRow(`SELECT COUNT(*) FROM notifications WHERE user_id = ? AND is_read = 0`, currentUserID(r)).Scan(&n); err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"unread": n})
}

// readNotification handles POST /api/notifications/{id}/read. The row must
// belong to the logged-in user; someone else's id is a 404, like an unknown one.
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

// readAllNotifications handles POST /api/notifications/read-all.
func (a *App) readAllNotifications(w http.ResponseWriter, r *http.Request) {
	if _, err := a.DB.Exec(`UPDATE notifications SET is_read = 1 WHERE user_id = ? AND is_read = 0`, currentUserID(r)); err != nil {
		serverError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
