package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"time"

	"social-network/pkg/models"
)

const (
	maxEventTitleLen       = 100
	maxEventDescriptionLen = 2000

	responseGoing    = "going"
	responseNotGoing = "not_going"

	sqliteTime = "2006-01-02 15:04:05"
)

var eventTimeLayouts = []string{time.RFC3339, "2006-01-02T15:04", "2006-01-02T15:04:05", sqliteTime}

type event struct {
	ID            int64              `json:"id"`
	GroupID       int64              `json:"group_id"`
	Title         string             `json:"title"`
	Description   string             `json:"description"`
	EventTime     string             `json:"event_time"`
	Creator       models.UserSummary `json:"creator"`
	GoingCount    int                `json:"going_count"`
	NotGoingCount int                `json:"not_going_count"`
	MyResponse    *string            `json:"my_response"`
	CreatedAt     string             `json:"created_at"`
}

const eventColumns = `e.id, e.group_id, e.title, e.description,
	strftime('%Y-%m-%d %H:%M:%S', e.event_time), strftime('%Y-%m-%d %H:%M:%S', e.created_at),
	u.id, u.first_name, u.last_name, u.nickname, u.avatar_path,
	(SELECT COUNT(*) FROM event_responses r WHERE r.event_id = e.id AND r.response = 'going'),
	(SELECT COUNT(*) FROM event_responses r WHERE r.event_id = e.id AND r.response = 'not_going'),
	(SELECT r.response FROM event_responses r WHERE r.event_id = e.id AND r.user_id = ?1)`

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

func parseEventTime(s string) (time.Time, bool) {
	for _, layout := range eventTimeLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

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
	members, err := groupMemberIDs(tx, groupID)
	if err != nil {
		serverError(w, err)
		return
	}
	var n notifier
	for _, id := range members {
		if err := n.add(tx, id, me, notifGroupEvent, &eventID); err != nil {
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
	me := currentUserID(r)
	res, err := a.DB.Exec(`INSERT INTO event_responses (event_id, user_id, response)
		SELECT e.id, ?2, ?3 FROM events e
		WHERE e.id = ?1 AND e.event_time > CURRENT_TIMESTAMP
		  AND EXISTS (SELECT 1 FROM group_members m WHERE m.group_id = e.group_id AND m.user_id = ?2)
		ON CONFLICT (event_id, user_id) DO UPDATE SET response = excluded.response, responded_at = CURRENT_TIMESTAMP`,
		eventID, me, body.Response)
	if err != nil {
		serverError(w, err)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		var past bool
		err := a.DB.QueryRow(`SELECT e.event_time <= CURRENT_TIMESTAMP FROM events e
			WHERE e.id = ?1 AND EXISTS (SELECT 1 FROM group_members m WHERE m.group_id = e.group_id AND m.user_id = ?2)`,
			eventID, me).Scan(&past)
		switch {
		case err == nil && past:
			writeError(w, http.StatusBadRequest, "this event has already happened")
		case err == nil || errors.Is(err, sql.ErrNoRows):
			writeError(w, http.StatusNotFound, "event not found")
		default:
			serverError(w, err)
		}
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
