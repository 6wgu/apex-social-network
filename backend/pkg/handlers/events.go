package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"time"

	"social-network/pkg/models"
)

// Group events: a title, a description and a date/time, and each member's
// answer, going or not going, which they can change at any time. Events are
// members-only, like everything inside a group.

const (
	maxEventTitleLen       = 100
	maxEventDescriptionLen = 2000

	responseGoing    = "going"
	responseNotGoing = "not_going"

	// sqliteTime is the format SQLite uses for DATETIME columns; event_time
	// is stored in it, in UTC.
	sqliteTime = "2006-01-02 15:04:05"
)

// eventTimeLayouts are the event_time formats POST accepts. Browsers send
// RFC 3339 with a zone (toISOString()); the forms without a zone, like the
// value of an <input type="datetime-local">, are taken as UTC.
var eventTimeLayouts = []string{time.RFC3339, "2006-01-02T15:04", "2006-01-02T15:04:05", sqliteTime}

// event is the JSON shape of an event.
type event struct {
	ID            int64              `json:"id"`
	GroupID       int64              `json:"group_id"`
	Title         string             `json:"title"`
	Description   string             `json:"description"`
	EventTime     string             `json:"event_time"`
	Creator       models.UserSummary `json:"creator"`
	GoingCount    int                `json:"going_count"`
	NotGoingCount int                `json:"not_going_count"`
	MyResponse    *string            `json:"my_response"` // null until I answer
	CreatedAt     string             `json:"created_at"`
}

// eventColumns selects an event (alias e) with its creator (alias u), as
// seen by the viewer bound as ?1.
// SQL constant: never built from user input; every value is a bound parameter.
const eventColumns = `e.id, e.group_id, e.title, e.description,
	strftime('%Y-%m-%d %H:%M:%S', e.event_time), strftime('%Y-%m-%d %H:%M:%S', e.created_at),
	u.id, u.first_name, u.last_name, u.nickname, u.avatar_path,
	(SELECT COUNT(*) FROM event_responses r WHERE r.event_id = e.id AND r.response = 'going'),
	(SELECT COUNT(*) FROM event_responses r WHERE r.event_id = e.id AND r.response = 'not_going'),
	(SELECT r.response FROM event_responses r WHERE r.event_id = e.id AND r.user_id = ?1)`

// SQL constant: never built from user input; every value is a bound parameter.
const eventFrom = `FROM events e JOIN users u ON u.id = e.creator_id`

func scanEvent(row interface{ Scan(...any) error }) (event, error) {
	var e event
	var mine sql.NullString
	c := &e.Creator
	err := row.Scan(&e.ID, &e.GroupID, &e.Title, &e.Description, &e.EventTime, &e.CreatedAt,
		&c.ID, &c.FirstName, &c.LastName, &c.Nickname, &c.AvatarPath,
		&e.GoingCount, &e.NotGoingCount, &mine)
	if mine.Valid {
		e.MyResponse = &mine.String
	}
	return e, err
}

func (a *App) eventByID(viewerID, eventID int64) (event, error) {
	return scanEvent(a.DB.QueryRow(`SELECT `+eventColumns+` `+eventFrom+` WHERE e.id = ?2`, viewerID, eventID))
}

// parseEventTime reads an event time in one of eventTimeLayouts and returns
// it in UTC.
func parseEventTime(s string) (time.Time, bool) {
	for _, layout := range eventTimeLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

// createEvent handles POST /api/groups/{id}/events
// {"title","description","event_time"}. Members only. The time must be in
// the future. Every other member gets a group_event notification.
func (a *App) createEvent(w http.ResponseWriter, r *http.Request) {
	groupID, ok := a.memberGroupID(w, r, "only members can create events in this group")
	if !ok {
		return
	}
	var body struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		EventTime   string `json:"event_time"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	title, problem := checkText(body.Title, "title", maxEventTitleLen)
	if problem != "" {
		writeError(w, http.StatusBadRequest, problem)
		return
	}
	description, problem := checkText(body.Description, "description", maxEventDescriptionLen)
	if problem != "" {
		writeError(w, http.StatusBadRequest, problem)
		return
	}
	when, ok := parseEventTime(body.EventTime)
	switch {
	case !ok:
		writeError(w, http.StatusBadRequest, "event_time must be a date and time, e.g. 2026-11-01T18:00:00Z")
		return
	case !when.After(time.Now()):
		writeError(w, http.StatusBadRequest, "event_time must be in the future")
		return
	}

	me := currentUserID(r)
	tx, err := a.DB.Begin()
	if err != nil {
		serverError(w, err)
		return
	}
	defer tx.Rollback()
	res, err := tx.Exec(`INSERT INTO events (group_id, creator_id, title, description, event_time) VALUES (?, ?, ?, ?, ?)`,
		groupID, me, title, description, when.Format(sqliteTime))
	if err != nil {
		serverError(w, err)
		return
	}
	eventID, err := res.LastInsertId()
	if err != nil {
		serverError(w, err)
		return
	}
	members, err := memberIDs(tx, groupID)
	if err != nil {
		serverError(w, err)
		return
	}
	var n notifier
	for _, id := range members {
		if err := n.add(tx, id, me, notifGroupEvent, &eventID); err != nil { // add skips me
			serverError(w, err)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		serverError(w, err)
		return
	}
	a.push(&n)
	e, err := a.eventByID(me, eventID)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, e)
}

// memberIDs lists the ids of groupID's members inside tx.
func memberIDs(tx *sql.Tx, groupID int64) ([]int64, error) {
	rows, err := tx.Query(`SELECT user_id FROM group_members WHERE group_id = ?`, groupID)
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

// listEvents handles GET /api/groups/{id}/events: the group's events in
// date order, soonest first. Members only (403 otherwise).
func (a *App) listEvents(w http.ResponseWriter, r *http.Request) {
	groupID, ok := a.memberGroupID(w, r, "only members can see this group's events")
	if !ok {
		return
	}
	rows, err := a.DB.Query(`SELECT `+eventColumns+` `+eventFrom+`
		WHERE e.group_id = ?2 ORDER BY e.event_time, e.id`, currentUserID(r), groupID)
	if err != nil {
		serverError(w, err)
		return
	}
	defer rows.Close()
	events := []event{}
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			serverError(w, err)
			return
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, events)
}

// respondToEvent handles POST /api/events/{id}/respond {"response":"going"}
// or "not_going". Answering again replaces the earlier answer. An event of
// a group I am not in is a 404, the same as one that does not exist.
func (a *App) respondToEvent(w http.ResponseWriter, r *http.Request) {
	eventID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || eventID <= 0 {
		writeError(w, http.StatusNotFound, "event not found")
		return
	}
	var body struct {
		Response string `json:"response"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if body.Response != responseGoing && body.Response != responseNotGoing {
		writeError(w, http.StatusBadRequest, "response must be going or not_going")
		return
	}
	// The membership check is part of the INSERT, so the answer is only
	// stored for a member of the event's group.
	me := currentUserID(r)
	res, err := a.DB.Exec(`INSERT INTO event_responses (event_id, user_id, response)
		SELECT e.id, ?2, ?3 FROM events e
		WHERE e.id = ?1 AND EXISTS (SELECT 1 FROM group_members m WHERE m.group_id = e.group_id AND m.user_id = ?2)
		ON CONFLICT (event_id, user_id) DO UPDATE SET response = excluded.response, responded_at = CURRENT_TIMESTAMP`,
		eventID, me, body.Response)
	if err != nil {
		serverError(w, err)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeError(w, http.StatusNotFound, "event not found")
		return
	}
	e, err := a.eventByID(me, eventID)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "event not found")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, e)
}
