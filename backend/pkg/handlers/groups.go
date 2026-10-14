package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/mattn/go-sqlite3"

	"social-network/pkg/models"
)

// Groups. Every logged-in user can browse them (title, description,
// creator, member count) and ask to join; everything inside a group (its
// members, posts with their comments and images, events) is for members
// only, and every endpoint checks that here, in the backend. There are two
// ways in:
//   - an invitation from any member, which only the invitee can accept or
//     decline
//   - a join request, which only the group's creator can accept or decline
//
// Joining either way also clears the other one, so nobody keeps a stale
// invitation or request for a group they are already in. Members are never
// removed (there is no leave or kick), so a membership check cannot go
// stale between a check and the write that follows it.

const (
	maxGroupTitleLen       = 100
	maxGroupDescriptionLen = 2000
)

// Membership states as the API shows them (viewer -> group).
const (
	groupNone      = "none"
	groupMember    = "member"
	groupInvited   = "invited"
	groupRequested = "requested"
)

// group is the JSON shape of a group. Members and InvitedUsers are only
// filled in for members (GET /api/groups/{id}); they are left out otherwise.
type group struct {
	ID           int64                 `json:"id"`
	Title        string                `json:"title"`
	Description  string                `json:"description"`
	Creator      models.UserSummary    `json:"creator"`
	MemberCount  int                   `json:"member_count"`
	MyStatus     string                `json:"my_status"`
	InviteID     *int64                `json:"invite_id"` // my pending invitation, when my_status is "invited"
	CreatedAt    string                `json:"created_at"`
	Members      *[]models.UserSummary `json:"members,omitempty"`
	InvitedUsers *[]models.UserSummary `json:"invited_users,omitempty"`
}

// groupColumns selects a group (alias g) with its creator (alias u), seen
// by the viewer bound as ?1. A member never has an invitation or a request
// (joining deletes them), so the CASE order only matters for invited and
// requested: an invitation wins, because accepting it is all it takes.
// SQL constant: never built from user input; every value is a bound parameter.
const groupColumns = `g.id, g.title, g.description, strftime('%Y-%m-%d %H:%M:%S', g.created_at),
	u.id, u.first_name, u.last_name, u.nickname, u.avatar_path,
	(SELECT COUNT(*) FROM group_members m WHERE m.group_id = g.id),
	CASE
		WHEN EXISTS (SELECT 1 FROM group_members m WHERE m.group_id = g.id AND m.user_id = ?1) THEN 'member'
		WHEN EXISTS (SELECT 1 FROM group_invites i WHERE i.group_id = g.id AND i.invitee_id = ?1) THEN 'invited'
		WHEN EXISTS (SELECT 1 FROM group_requests r WHERE r.group_id = g.id AND r.user_id = ?1) THEN 'requested'
		ELSE 'none'
	END,
	(SELECT i.id FROM group_invites i WHERE i.group_id = g.id AND i.invitee_id = ?1)`

// SQL constant: never built from user input; every value is a bound parameter.
const groupFrom = `FROM groups g JOIN users u ON u.id = g.creator_id`

func scanGroup(row interface{ Scan(...any) error }) (group, error) {
	var g group
	var inviteID sql.NullInt64
	c := &g.Creator
	err := row.Scan(&g.ID, &g.Title, &g.Description, &g.CreatedAt,
		&c.ID, &c.FirstName, &c.LastName, &c.Nickname, &c.AvatarPath,
		&g.MemberCount, &g.MyStatus, &inviteID)
	if inviteID.Valid {
		g.InviteID = &inviteID.Int64
	}
	return g, err
}

// groupByID loads a group as viewerID sees it, or sql.ErrNoRows.
func (a *App) groupByID(viewerID, groupID int64) (group, error) {
	return scanGroup(a.DB.QueryRow(`SELECT `+groupColumns+` `+groupFrom+` WHERE g.id = ?2`, viewerID, groupID))
}

// IsGroupMember reports whether userID is a member of groupID. Every
// members-only endpoint uses it, and so does group chat (Phase 4).
func (a *App) IsGroupMember(userID, groupID int64) (bool, error) {
	var member bool
	err := a.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM group_members WHERE group_id = ? AND user_id = ?)`,
		groupID, userID).Scan(&member)
	return member, err
}

// groupIDFromPath reads {id} from the path. It writes a 404 and returns
// false when it is not a positive integer or no such group exists.
func (a *App) groupIDFromPath(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusNotFound, "group not found")
		return 0, false
	}
	var exists bool
	if err := a.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM groups WHERE id = ?)`, id).Scan(&exists); err != nil {
		serverError(w, err)
		return 0, false
	}
	if !exists {
		writeError(w, http.StatusNotFound, "group not found")
		return 0, false
	}
	return id, true
}

// memberGroupID is groupIDFromPath for members-only endpoints: a group that
// exists but that the logged-in user is not a member of is a 403 with the
// given message. Groups are public to browse, so their existence is no secret.
func (a *App) memberGroupID(w http.ResponseWriter, r *http.Request, forbidden string) (int64, bool) {
	groupID, ok := a.groupIDFromPath(w, r)
	if !ok {
		return 0, false
	}
	member, err := a.IsGroupMember(currentUserID(r), groupID)
	if err != nil {
		serverError(w, err)
		return 0, false
	}
	if !member {
		writeError(w, http.StatusForbidden, forbidden)
		return 0, false
	}
	return groupID, true
}

// checkText trims s and checks it is present and at most max characters.
// It returns the trimmed text and a 400 message ("" when fine).
func checkText(s, field string, max int) (string, string) {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return "", field + " is required"
	case utf8.RuneCountInString(s) > max:
		return "", fmt.Sprintf("%s must be %d characters or fewer", field, max)
	}
	return s, ""
}

// createGroup handles POST /api/groups {"title","description"}. The
// creator becomes the first member.
func (a *App) createGroup(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	title, problem := checkText(body.Title, "title", maxGroupTitleLen)
	if problem != "" {
		writeError(w, http.StatusBadRequest, problem)
		return
	}
	description, problem := checkText(body.Description, "description", maxGroupDescriptionLen)
	if problem != "" {
		writeError(w, http.StatusBadRequest, problem)
		return
	}

	me := currentUserID(r)
	tx, err := a.DB.Begin()
	if err != nil {
		serverError(w, err)
		return
	}
	defer tx.Rollback()
	res, err := tx.Exec(`INSERT INTO groups (creator_id, title, description) VALUES (?, ?, ?)`, me, title, description)
	if err != nil {
		serverError(w, err)
		return
	}
	groupID, err := res.LastInsertId()
	if err != nil {
		serverError(w, err)
		return
	}
	if _, err := tx.Exec(`INSERT INTO group_members (group_id, user_id) VALUES (?, ?)`, groupID, me); err != nil {
		serverError(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		serverError(w, err)
		return
	}
	g, err := a.groupByID(me, groupID)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, g)
}

// listGroups handles GET /api/groups: every group, newest first, each with
// my_status so the page can show Join / Invited / Requested / Open.
func (a *App) listGroups(w http.ResponseWriter, r *http.Request) {
	rows, err := a.DB.Query(`SELECT `+groupColumns+` `+groupFrom+` ORDER BY g.id DESC`, currentUserID(r))
	if err != nil {
		serverError(w, err)
		return
	}
	defer rows.Close()
	groups := []group{}
	for rows.Next() {
		g, err := scanGroup(rows)
		if err != nil {
			serverError(w, err)
			return
		}
		groups = append(groups, g)
	}
	if err := rows.Err(); err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, groups)
}

// getGroup handles GET /api/groups/{id}. Everyone gets the public part;
// members also get the member list and who has a pending invitation.
func (a *App) getGroup(w http.ResponseWriter, r *http.Request) {
	groupID, ok := a.groupIDFromPath(w, r)
	if !ok {
		return
	}
	g, err := a.groupByID(currentUserID(r), groupID)
	if err != nil {
		serverError(w, err)
		return
	}
	if g.MyStatus == groupMember {
		members, err := a.userSummaries(`
			SELECT u.id, u.first_name, u.last_name, u.nickname, u.avatar_path
			FROM group_members m JOIN users u ON u.id = m.user_id
			WHERE m.group_id = ?
			ORDER BY m.joined_at, u.id`, groupID)
		if err != nil {
			serverError(w, err)
			return
		}
		invited, err := a.userSummaries(`
			SELECT u.id, u.first_name, u.last_name, u.nickname, u.avatar_path
			FROM group_invites i JOIN users u ON u.id = i.invitee_id
			WHERE i.group_id = ?
			ORDER BY i.id`, groupID)
		if err != nil {
			serverError(w, err)
			return
		}
		g.Members, g.InvitedUsers = &members, &invited
	}
	writeJSON(w, http.StatusOK, g)
}

// inviteToGroup handles POST /api/groups/{id}/invite {"user_id": 7}. Any
// member can invite anyone who is not a member and not already invited.
// The invitee gets a group_invite notification.
func (a *App) inviteToGroup(w http.ResponseWriter, r *http.Request) {
	groupID, ok := a.memberGroupID(w, r, "only members can invite people to this group")
	if !ok {
		return
	}
	var body struct {
		UserID int64 `json:"user_id"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if body.UserID <= 0 {
		writeError(w, http.StatusBadRequest, "user_id must be a user id")
		return
	}
	me := currentUserID(r)
	tx, err := a.DB.Begin()
	if err != nil {
		serverError(w, err)
		return
	}
	defer tx.Rollback()

	var exists, member bool
	err = tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM users WHERE id = ?1),
		EXISTS(SELECT 1 FROM group_members WHERE group_id = ?2 AND user_id = ?1)`, body.UserID, groupID).Scan(&exists, &member)
	if err != nil {
		serverError(w, err)
		return
	}
	switch {
	case !exists:
		writeError(w, http.StatusNotFound, "user not found")
		return
	case member:
		writeError(w, http.StatusConflict, "this user is already a member of the group")
		return
	}
	res, err := tx.Exec(`INSERT INTO group_invites (group_id, inviter_id, invitee_id) VALUES (?, ?, ?)`, groupID, me, body.UserID)
	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) && sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique {
		writeError(w, http.StatusConflict, "this user is already invited to the group")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	inviteID, err := res.LastInsertId()
	if err != nil {
		serverError(w, err)
		return
	}
	var n notifier
	if err := n.add(tx, body.UserID, me, notifGroupInvite, &groupID); err != nil {
		serverError(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		serverError(w, err)
		return
	}
	a.push(&n)
	writeJSON(w, http.StatusCreated, map[string]int64{"id": inviteID, "group_id": groupID, "user_id": body.UserID})
}

// answerGroupInvite handles POST /api/group-invites/{inviteId}/accept and
// /decline. The invitation is looked up together with the logged-in user
// as invitee, so nobody can answer someone else's: that is a 404, like an
// unknown id. Either answer deletes the invitation and its notification;
// accepting also clears my own join request to the same group.
func (a *App) answerGroupInvite(accept bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		inviteID, err := strconv.ParseInt(r.PathValue("inviteId"), 10, 64)
		if err != nil || inviteID <= 0 {
			writeError(w, http.StatusNotFound, "invitation not found")
			return
		}
		me := currentUserID(r)
		tx, err := a.DB.Begin()
		if err != nil {
			serverError(w, err)
			return
		}
		defer tx.Rollback()

		var groupID, inviterID int64
		err = tx.QueryRow(`DELETE FROM group_invites WHERE id = ? AND invitee_id = ? RETURNING group_id, inviter_id`,
			inviteID, me).Scan(&groupID, &inviterID)
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "invitation not found")
			return
		}
		if err != nil {
			serverError(w, err)
			return
		}
		var n notifier
		if err := n.removeRef(tx, me, inviterID, notifGroupInvite, groupID); err != nil {
			serverError(w, err)
			return
		}
		if accept {
			if err := join(tx, &n, groupID, me); err != nil {
				serverError(w, err)
				return
			}
		}
		if err := tx.Commit(); err != nil {
			serverError(w, err)
			return
		}
		a.push(&n)
		w.WriteHeader(http.StatusNoContent)
	}
}

// join makes userID a member of groupID inside tx and clears whatever else
// was pending between them: a join request (and the creator's notification
// about it) and an invitation (and userID's notification about it).
func join(tx *sql.Tx, n *notifier, groupID, userID int64) error {
	if _, err := tx.Exec(`INSERT OR IGNORE INTO group_members (group_id, user_id) VALUES (?, ?)`, groupID, userID); err != nil {
		return err
	}
	var creatorID int64
	err := tx.QueryRow(`SELECT g.creator_id FROM group_requests r JOIN groups g ON g.id = r.group_id
		WHERE r.group_id = ? AND r.user_id = ?`, groupID, userID).Scan(&creatorID)
	switch {
	case err == nil:
		if _, err := tx.Exec(`DELETE FROM group_requests WHERE group_id = ? AND user_id = ?`, groupID, userID); err != nil {
			return err
		}
		if err := n.removeRef(tx, creatorID, userID, notifGroupJoinRequest, groupID); err != nil {
			return err
		}
	case !errors.Is(err, sql.ErrNoRows):
		return err
	}
	var inviterID int64
	err = tx.QueryRow(`DELETE FROM group_invites WHERE group_id = ? AND invitee_id = ? RETURNING inviter_id`,
		groupID, userID).Scan(&inviterID)
	switch {
	case err == nil:
		return n.removeRef(tx, userID, inviterID, notifGroupInvite, groupID)
	case errors.Is(err, sql.ErrNoRows):
		return nil
	default:
		return err
	}
}

// requestToJoin handles POST /api/groups/{id}/request: a non-member asks to
// join. The group's creator gets a group_join_request notification.
func (a *App) requestToJoin(w http.ResponseWriter, r *http.Request) {
	groupID, ok := a.groupIDFromPath(w, r)
	if !ok {
		return
	}
	me := currentUserID(r)
	tx, err := a.DB.Begin()
	if err != nil {
		serverError(w, err)
		return
	}
	defer tx.Rollback()

	var creatorID int64
	var member, invited bool
	err = tx.QueryRow(`SELECT g.creator_id,
		EXISTS(SELECT 1 FROM group_members WHERE group_id = g.id AND user_id = ?2),
		EXISTS(SELECT 1 FROM group_invites WHERE group_id = g.id AND invitee_id = ?2)
		FROM groups g WHERE g.id = ?1`, groupID, me).Scan(&creatorID, &member, &invited)
	if err != nil {
		serverError(w, err)
		return
	}
	switch {
	case member:
		writeError(w, http.StatusConflict, "you are already a member of this group")
		return
	case invited:
		writeError(w, http.StatusConflict, "you are already invited to this group: accept the invitation instead")
		return
	}
	res, err := tx.Exec(`INSERT INTO group_requests (group_id, user_id) VALUES (?, ?)`, groupID, me)
	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) && sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique {
		writeError(w, http.StatusConflict, "you already asked to join this group")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	requestID, err := res.LastInsertId()
	if err != nil {
		serverError(w, err)
		return
	}
	var n notifier
	if err := n.add(tx, creatorID, me, notifGroupJoinRequest, &groupID); err != nil {
		serverError(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		serverError(w, err)
		return
	}
	a.push(&n)
	writeJSON(w, http.StatusCreated, map[string]int64{"id": requestID, "group_id": groupID})
}

// cancelJoinRequest handles DELETE /api/groups/{id}/request: I withdraw my
// pending request, and it leaves the creator's bell.
func (a *App) cancelJoinRequest(w http.ResponseWriter, r *http.Request) {
	groupID, ok := a.groupIDFromPath(w, r)
	if !ok {
		return
	}
	me := currentUserID(r)
	tx, err := a.DB.Begin()
	if err != nil {
		serverError(w, err)
		return
	}
	defer tx.Rollback()
	res, err := tx.Exec(`DELETE FROM group_requests WHERE group_id = ? AND user_id = ?`, groupID, me)
	if err != nil {
		serverError(w, err)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeError(w, http.StatusNotFound, "you have no pending request to join this group")
		return
	}
	var creatorID int64
	if err := tx.QueryRow(`SELECT creator_id FROM groups WHERE id = ?`, groupID).Scan(&creatorID); err != nil {
		serverError(w, err)
		return
	}
	var n notifier
	if err := n.removeRef(tx, creatorID, me, notifGroupJoinRequest, groupID); err != nil {
		serverError(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		serverError(w, err)
		return
	}
	a.push(&n)
	w.WriteHeader(http.StatusNoContent)
}

// joinRequest is one pending request to join a group.
type joinRequest struct {
	ID        int64              `json:"id"`
	User      models.UserSummary `json:"user"`
	CreatedAt string             `json:"created_at"`
}

// listJoinRequests handles GET /api/groups/{id}/requests: the pending
// requests, oldest first. Only the group's creator may see them.
func (a *App) listJoinRequests(w http.ResponseWriter, r *http.Request) {
	groupID, ok := a.groupIDFromPath(w, r)
	if !ok {
		return
	}
	var creatorID int64
	if err := a.DB.QueryRow(`SELECT creator_id FROM groups WHERE id = ?`, groupID).Scan(&creatorID); err != nil {
		serverError(w, err)
		return
	}
	if creatorID != currentUserID(r) {
		writeError(w, http.StatusForbidden, "only the group's creator can see join requests")
		return
	}
	rows, err := a.DB.Query(`
		SELECT r.id, u.id, u.first_name, u.last_name, u.nickname, u.avatar_path,
		       strftime('%Y-%m-%d %H:%M:%S', r.created_at)
		FROM group_requests r JOIN users u ON u.id = r.user_id
		WHERE r.group_id = ?
		ORDER BY r.id`, groupID)
	if err != nil {
		serverError(w, err)
		return
	}
	defer rows.Close()
	requests := []joinRequest{}
	for rows.Next() {
		var jr joinRequest
		u := &jr.User
		if err := rows.Scan(&jr.ID, &u.ID, &u.FirstName, &u.LastName, &u.Nickname, &u.AvatarPath, &jr.CreatedAt); err != nil {
			serverError(w, err)
			return
		}
		requests = append(requests, jr)
	}
	if err := rows.Err(); err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, requests)
}

// answerJoinRequest handles POST /api/group-requests/{requestId}/accept and
// /decline. Only the group's creator may answer (403 for anyone else).
// Either answer deletes the request and the creator's notification about
// it; accepting adds the member and tells them.
func (a *App) answerJoinRequest(accept bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestID, err := strconv.ParseInt(r.PathValue("requestId"), 10, 64)
		if err != nil || requestID <= 0 {
			writeError(w, http.StatusNotFound, "join request not found")
			return
		}
		me := currentUserID(r)
		tx, err := a.DB.Begin()
		if err != nil {
			serverError(w, err)
			return
		}
		defer tx.Rollback()

		var groupID, requesterID, creatorID int64
		err = tx.QueryRow(`SELECT r.group_id, r.user_id, g.creator_id
			FROM group_requests r JOIN groups g ON g.id = r.group_id WHERE r.id = ?`, requestID).Scan(&groupID, &requesterID, &creatorID)
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "join request not found")
			return
		}
		if err != nil {
			serverError(w, err)
			return
		}
		if creatorID != me {
			writeError(w, http.StatusForbidden, "only the group's creator can answer join requests")
			return
		}

		var n notifier
		if accept {
			// join deletes the request and its notification, and any
			// invitation the requester had in the meantime.
			err = join(tx, &n, groupID, requesterID)
			if err == nil {
				err = n.add(tx, requesterID, me, notifGroupRequestAccepted, &groupID)
			}
		} else {
			_, err = tx.Exec(`DELETE FROM group_requests WHERE id = ?`, requestID)
			if err == nil {
				err = n.removeRef(tx, me, requesterID, notifGroupJoinRequest, groupID)
			}
		}
		if err != nil {
			serverError(w, err)
			return
		}
		if err := tx.Commit(); err != nil {
			serverError(w, err)
			return
		}
		a.push(&n)
		w.WriteHeader(http.StatusNoContent)
	}
}

// groupPosts handles GET /api/groups/{id}/posts?before={postId}: the
// group's posts, newest first, 20 per page. Members only (403 otherwise);
// postVisibleSQL applies the same rule again inside the query.
func (a *App) groupPosts(w http.ResponseWriter, r *http.Request) {
	groupID, ok := a.memberGroupID(w, r, "only members can see this group's posts")
	if !ok {
		return
	}
	before, ok := beforeParam(w, r)
	if !ok {
		return
	}
	posts, err := a.queryPosts(`SELECT `+postColumns+` `+postFrom+`
		WHERE p.group_id = ?2 AND p.id < ?3 AND `+postVisibleSQL+`
		ORDER BY p.id DESC LIMIT ?4`, currentUserID(r), groupID, before, postsPerPage)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, posts)
}

// createGroupPost handles POST /api/groups/{id}/posts (multipart): content
// and/or an image. Members only. A group post has no privacy choice: its
// group decides who sees it, so it is stored as 'public' with group_id set.
func (a *App) createGroupPost(w http.ResponseWriter, r *http.Request) {
	defer removeFormFiles(r)
	groupID, ok := a.memberGroupID(w, r, "only members can post in this group")
	if !ok {
		return
	}
	if !parseForm(w, r) {
		return
	}
	content, ok := postContent(w, r)
	if !ok {
		return
	}
	a.savePost(w, r, newPost{authorID: currentUserID(r), content: content, privacy: privacyPublic, groupID: &groupID})
}
