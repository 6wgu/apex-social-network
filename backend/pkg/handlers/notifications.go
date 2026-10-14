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
// 000004); this list is the source of truth. Phase 3 adds group_invite,
// group_join_request and group_event, with ref_id pointing at the group or
// event.
const (
	// notifFollowRequest: actor asked to follow user (a private profile).
	// Removed again when the request is accepted, declined or cancelled.
	notifFollowRequest = "follow_request"
	// notifNewFollower: actor started following user (a public profile).
	notifNewFollower = "new_follower"
	// notifFollowAccepted: user's request to follow actor was accepted.
	notifFollowAccepted = "follow_accepted"
)

// WebSocket event types for the bell.
const (
	eventNotification        = "notification"         // payload: Notification
	eventNotificationRemoved = "notification_removed" // payload: {"id": 12}
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
	IsRead    bool                `json:"is_read"`
	CreatedAt string              `json:"created_at"`
}

// notificationColumns and scanNotification read one notification with its
// actor. The actor is a LEFT JOIN as a safeguard; with ON DELETE CASCADE on
// actor_id every listed notification has one.
const notificationColumns = `n.id, n.type, n.ref_id, n.is_read, strftime('%Y-%m-%d %H:%M:%S', n.created_at),
	u.id, u.first_name, u.last_name, u.nickname, u.avatar_path`

const notificationFrom = `FROM notifications n LEFT JOIN users u ON u.id = n.actor_id`

// extra receives any columns selected after notificationColumns.
func scanNotification(row interface{ Scan(...any) error }, extra ...any) (notification, error) {
	var n notification
	var refID, actorID sql.NullInt64
	var first, last, nick, avatar sql.NullString
	dest := append([]any{&n.ID, &n.Type, &refID, &n.IsRead, &n.CreatedAt, &actorID, &first, &last, &nick, &avatar}, extra...)
	err := row.Scan(dest...)
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
	created []int64
	removed []removedNotification
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
	rows, err := tx.Query(`DELETE FROM notifications WHERE user_id = ? AND actor_id = ? AND type = ? RETURNING id`,
		userID, actorID, typ)
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
