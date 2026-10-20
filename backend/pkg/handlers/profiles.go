package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"social-network/pkg/models"
)

// maxSearchResults caps GET /api/users. maxSearchLen caps its search text,
// in characters; the search boxes stop at the same SEARCH_MAX_LENGTH
// (frontend/lib/search.js), so change the two together.
const (
	maxSearchResults = 20
	maxSearchLen     = 50
)

// fullProfile is what the owner, a follower, or anyone looking at a public
// profile gets.
type fullProfile struct {
	User           models.User `json:"user"`
	CanView        bool        `json:"can_view"`
	FollowStatus   string      `json:"follow_status"`
	FollowerCount  int         `json:"follower_count"`
	FollowingCount int         `json:"following_count"`
}

// limitedProfile is all that a private profile shows to someone who does not
// follow it. It is a separate type on purpose: it cannot carry the email,
// birth date or about text even by accident.
type limitedProfile struct {
	User         models.UserSummary `json:"user"`
	CanView      bool               `json:"can_view"`
	FollowStatus string             `json:"follow_status"`
}

// CanViewProfile reports whether viewerID may see ownerID's full profile:
// it is their own, it is public, or they follow it with an accepted request.
// It returns sql.ErrNoRows when ownerID does not exist. Phase 2 uses the same
// rule for followers lists and posts.
func (a *App) CanViewProfile(viewerID, ownerID int64) (bool, error) {
	var allowed bool
	err := a.DB.QueryRow(`
		SELECT u.id = ?1
		    OR u.is_public = 1
		    OR EXISTS (SELECT 1 FROM follows
		               WHERE follower_id = ?1 AND following_id = u.id AND status = 'accepted')
		FROM users u WHERE u.id = ?2`, viewerID, ownerID).Scan(&allowed)
	return allowed, err
}

// followStatus describes viewer -> owner: "self", "following", "pending" or "none".
func (a *App) followStatus(viewerID, ownerID int64) (string, error) {
	if viewerID == ownerID {
		return followSelf, nil
	}
	var status string
	err := a.DB.QueryRow(`SELECT status FROM follows WHERE follower_id = ? AND following_id = ?`,
		viewerID, ownerID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return followNone, nil
	}
	return apiFollowStatus(status), err
}

// getProfile handles GET /api/users/{id}.
func (a *App) getProfile(w http.ResponseWriter, r *http.Request) {
	ownerID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || ownerID <= 0 {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	viewerID := currentUserID(r)

	canView, err := a.CanViewProfile(viewerID, ownerID)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	status, err := a.followStatus(viewerID, ownerID)
	if err != nil {
		serverError(w, err)
		return
	}

	if !canView {
		var summary models.UserSummary
		err := a.DB.QueryRow(`SELECT id, first_name, last_name, nickname, avatar_path FROM users WHERE id = ?`, ownerID).
			Scan(&summary.ID, &summary.FirstName, &summary.LastName, &summary.Nickname, &summary.AvatarPath)
		if err != nil {
			serverError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, limitedProfile{User: summary, CanView: false, FollowStatus: status})
		return
	}

	profile := fullProfile{CanView: true, FollowStatus: status}
	if profile.User, err = a.userByID(ownerID); err != nil {
		serverError(w, err)
		return
	}
	err = a.DB.QueryRow(`
		SELECT (SELECT COUNT(*) FROM follows WHERE following_id = ?1 AND status = 'accepted'),
		       (SELECT COUNT(*) FROM follows WHERE follower_id = ?1 AND status = 'accepted')`, ownerID).
		Scan(&profile.FollowerCount, &profile.FollowingCount)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

// updateMe handles PATCH /api/me (multipart): any of nickname, about_me,
// avatar, remove_avatar. A text field that is not sent keeps its value;
// sending it empty clears it. remove_avatar=1 deletes the current picture.
func (a *App) updateMe(w http.ResponseWriter, r *http.Request) {
	defer removeFormFiles(r)
	if !parseForm(w, r) {
		return
	}
	user, err := a.userByID(currentUserID(r))
	if err != nil {
		serverError(w, err)
		return
	}

	changed := false
	if r.Form.Has("nickname") {
		user.Nickname = strings.TrimSpace(r.FormValue("nickname"))
		if problem := validateNickname(user.Nickname); problem != "" {
			writeFieldErrors(w, http.StatusBadRequest, fieldErrors{"nickname": problem})
			return
		}
		// Someone else's nickname, in any case, is taken; my own (even in
		// another case) is not. Checked before the avatar is stored.
		inUse, err := a.nicknameInUse(user.Nickname, user.ID)
		if err != nil {
			serverError(w, err)
			return
		}
		if inUse {
			writeFieldErrors(w, http.StatusConflict, fieldErrors{"nickname": nicknameTaken})
			return
		}
		changed = true
	}
	if r.Form.Has("about_me") {
		user.AboutMe = strings.TrimSpace(r.FormValue("about_me"))
		if problem := validateAbout(user.AboutMe); problem != "" {
			writeError(w, http.StatusBadRequest, problem)
			return
		}
		changed = true
	}

	// The file behind the current avatar is deleted once the row no longer
	// points to it: after a replacement or a removal.
	oldAvatar := user.AvatarPath
	switch r.FormValue("remove_avatar") {
	case "", "0":
	case "1":
		if _, _, err := r.FormFile("avatar"); err == nil {
			writeError(w, http.StatusBadRequest, "send either avatar or remove_avatar, not both")
			return
		}
		user.AvatarPath = ""
		changed = true
	default:
		writeError(w, http.StatusBadRequest, "remove_avatar must be 1")
		return
	}

	newAvatar, err := a.saveImage(r, "avatar")
	var uploadErr uploadError
	if errors.As(err, &uploadErr) {
		writeError(w, http.StatusBadRequest, "avatar: "+uploadErr.Error())
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	if newAvatar != "" {
		user.AvatarPath = newAvatar
		changed = true
	}
	if !changed {
		writeError(w, http.StatusBadRequest, "nothing to update: send nickname, about_me, avatar or remove_avatar")
		return
	}

	_, err = a.DB.Exec(`UPDATE users SET nickname = ?, about_me = ?, avatar_path = ? WHERE id = ?`,
		user.Nickname, user.AboutMe, user.AvatarPath, user.ID)
	if err != nil {
		a.removeImage(newAvatar)
		// Another request took the nickname since the check above.
		if uniqueConflict(err) == "nickname" {
			writeFieldErrors(w, http.StatusConflict, fieldErrors{"nickname": nicknameTaken})
			return
		}
		serverError(w, err)
		return
	}
	if user.AvatarPath != oldAvatar {
		a.removeImage(oldAvatar)
	}
	writeJSON(w, http.StatusOK, user)
}

// updatePrivacy handles PATCH /api/me/privacy. There is no user ID in the
// request: it always changes the profile of the session's owner. Going
// public accepts every pending follow request in the same transaction, so
// a public profile never has requests waiting.
func (a *App) updatePrivacy(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IsPublic *bool `json:"is_public"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if body.IsPublic == nil {
		writeError(w, http.StatusBadRequest, "is_public is required (true or false)")
		return
	}
	userID := currentUserID(r)
	tx, err := a.DB.Begin()
	if err != nil {
		serverError(w, err)
		return
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE users SET is_public = ? WHERE id = ?`, *body.IsPublic, userID); err != nil {
		serverError(w, err)
		return
	}
	var n notifier
	if *body.IsPublic {
		if err := acceptPendingRequests(tx, &n, userID); err != nil {
			serverError(w, err)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		serverError(w, err)
		return
	}
	a.push(&n)
	user, err := a.userByID(userID)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, user)
}

// likeEscaper makes user text literal inside a LIKE pattern (ESCAPE '\').
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// searchUsers handles GET /api/users?search=: people whose first name, last
// name, full name or nickname contains the text. An empty search lists users.
// The text is trimmed and each run of spaces or newlines counts as one space,
// as the search boxes send it; more than maxSearchLen characters is a 400.
func (a *App) searchUsers(w http.ResponseWriter, r *http.Request) {
	search := strings.Join(strings.Fields(r.URL.Query().Get("search")), " ")
	if utf8.RuneCountInString(search) > maxSearchLen {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("search must be %d characters or fewer", maxSearchLen))
		return
	}
	pattern := "%" + likeEscaper.Replace(search) + "%"
	rows, err := a.DB.Query(`
		SELECT id, first_name, last_name, nickname, avatar_path
		FROM users
		WHERE first_name LIKE ?1 ESCAPE '\'
		   OR last_name LIKE ?1 ESCAPE '\'
		   OR nickname LIKE ?1 ESCAPE '\'
		   OR first_name || ' ' || last_name LIKE ?1 ESCAPE '\'
		ORDER BY first_name, last_name, id
		LIMIT ?2`, pattern, maxSearchResults)
	if err != nil {
		serverError(w, err)
		return
	}
	defer rows.Close()

	users := []models.UserSummary{} // never null in JSON
	for rows.Next() {
		var u models.UserSummary
		if err := rows.Scan(&u.ID, &u.FirstName, &u.LastName, &u.Nickname, &u.AvatarPath); err != nil {
			serverError(w, err)
			return
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, users)
}
